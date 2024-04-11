package geecache

import (
	"fmt"
	"geecache/consistenthash"
	pb "geecache/geecachepb"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/golang/protobuf/proto"
)

const (
	defaultBasePath = "/_geecache/"
	defaultReplicas = 50
)

// HTTPPool 作为承载节点间 HTTP 通信的核心数据结构（包括服务端和客户端）。
type HTTPPool struct {
	self        string                 //用来记录自己的地址，包括主机名和端口号
	basePath    string                 //作为节点间通信的前缀，默认是"/_geecache/"（上面那个defaultBasePath常量就是）
	mu          sync.Mutex             //锁
	peers       *consistenthash.Map    //不是简单的Map，这个Map是我们自己构建的，用来根据具体的key选择节点
	httpGetters map[string]*httpGetter //这个map用来保存 远程节点的名字 和 对应的httpGetter
}

// NewHTTPPool 初始化HTTPPool.
func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		self:     self,
		basePath: defaultBasePath,
	}
}

// Log 打印一些信息 类似： [Server localhost] GET /example
func (p *HTTPPool) Log(format string, v ...interface{}) {
	log.Printf("[Server %s] %s", p.self, fmt.Sprintf(format, v...))
}

// ServeHTTP 服务端方法处理所有的HTTP请求
func (p *HTTPPool) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//首先判断访问路径的前缀是否是basePath，如果不是就返回panic
	if !strings.HasPrefix(r.URL.Path, p.basePath) {
		panic("HTTPPool serving unexpected path: " + r.URL.Path)
	}
	//打印信息
	p.Log("%s %s", r.Method, r.URL.Path)
	// 由于我们规定信息格式是类似：<basepath>/<groupname>/<key> 这种的
	//所以需要进行截取，这里标识从<basepath>后进行拆，通过/进行拆成两个部分
	//也就是把groupname和key拆出来
	parts := strings.SplitN(r.URL.Path[len(p.basePath):], "/", 2)
	//如果不是2，就报错，格式不对
	if len(parts) != 2 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	//得到groupName
	groupName := parts[0]
	//得到key
	key := parts[1]

	// GetGroup 方法通过 name 得到Group对象指针
	group := GetGroup(groupName)
	if group == nil {
		http.Error(w, "no such group: "+groupName, http.StatusNotFound)
		return
	}
	// Get 方法通过传入 key 进行查询是否有这个key
	view, err := group.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Write the value to the response body as a proto message.
	body, err := proto.Marshal(&pb.Response{Value: view.ByteSlice()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(body)
}

// Set 方法主要是把服务器节点Set进去了，在初始化的时候进行使用。
// Set 方法实例化了一致性哈希算法，并且把传入的节点添加进去了
func (p *HTTPPool) Set(peers ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	//初始化一致性hash算法
	p.peers = consistenthash.New(defaultReplicas, nil)
	// Add 函数主要是通过把节点名传入进来，通过一致性hash算法添加到服务器的切片中
	p.peers.Add(peers...)
	//初始化map,这个map用来保存 远程节点的名字 和 对应的httpGetter
	//里面大概存储的是：
	//{节点一,节点一的httpGetter{baseURL:节点一/_geecache/}}
	p.httpGetters = make(map[string]*httpGetter, len(peers))
	for _, peer := range peers {
		//给每一个peer都创建httpGetter
		p.httpGetters[peer] = &httpGetter{baseURL: peer + p.basePath}
	}
}

// PickPeer 主要包装了一致性哈希算法的Get方法，具体的作用是根据key，选择节点，然后在根据节点返回对应的httpGetter
// PickPeer 实现了 PeerPicker 接口的 PickPeer 方法
func (p *HTTPPool) PickPeer(key string) (PeerGetter, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	//调用一致性hash的Get方法， Get 方法主要功能是获取与key最接近的节点
	if peer := p.peers.Get(key); peer != "" && peer != p.self { //这里存疑，为什么不能跟本机相等
		p.Log("Pick peer %s", peer)
		return p.httpGetters[peer], true
	}
	return nil, false
}

var _ PeerPicker = (*HTTPPool)(nil)

// 新建类httpGetter
type httpGetter struct {
	baseURL string
}

// Get 方法继承PeerGetter接口，这个接口是客户端用的，用于发起/<groupName>/<key> 类似的get请求
// Get 函数用于向指定的 URL 发起 HTTP GET 请求，并将响应解析为 Protocol Buffers 格式的消息
func (h *httpGetter) Get(in *pb.Request, out *pb.Response) error {
	// 构建请求的 URL
	u := fmt.Sprintf(
		"%v%v/%v",
		h.baseURL,
		url.QueryEscape(in.GetGroup()),
		url.QueryEscape(in.GetKey()),
	)
	// 发起 HTTP GET 请求
	res, err := http.Get(u)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	// 检查响应状态码
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned: %v", res.Status)
	}
	// 读取响应体内容
	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %v", err)
	}
	// 解码响应体内容为 Protocol Buffers 格式的消息
	if err = proto.Unmarshal(bytes, out); err != nil {
		return fmt.Errorf("decoding response body: %v", err)
	}

	return nil
}

// 这行代码的目的是在编译时确保 httpGetter 结构体类型实现了 PeerGetter 接口。
// 如果 httpGetter 没有实现 PeerGetter 接口中的所有方法，编译时将会产生错误。
var _ PeerGetter = (*httpGetter)(nil)
