package geecache

import (
	"fmt"
	pb "geecache/geecachepb"
	"geecache/singleflight"
	"log"
	"sync"
)

// Group 是一个缓存的命名空间
type Group struct {
	name      string              //Group的名称,每个 Group 拥有一个唯一的名称 name
	getter    Getter              //用户传入的回调函数,用于实现缓存未命中时获取源数据
	mainCache cache               //小cache，是封装大Cache的
	peers     PeerPicker          //用于根据传入的key选择相应节点，然后在根据节点返回对应的httpGetter
	loader    *singleflight.Group //避免多个key多次加载造成缓存击穿
}

// Getter 回调接口
type Getter interface {
	// Get 函数用于获取源数据。
	Get(key string) ([]byte, error)
}

// GetterFunc 是一个接口型函数,定义函数类型 GetterFunc，并实现 Getter 接口的 Get 方法。
type GetterFunc func(key string) ([]byte, error)

// Get 方便直接传入函数做参数
func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

var (
	mu sync.RWMutex
	//新建group的map，key为group的name，value指向这个Group对象
	groups = make(map[string]*Group)
)

// NewGroup 方法用来创建的新的 Group
func NewGroup(name string, cacheBytes int64, getter Getter) *Group {
	//如果没有传入用于查询源数据的函数，直接返回错误就行
	if getter == nil {
		panic("nil Getter")
	}
	mu.Lock()
	defer mu.Unlock()
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache{cacheBytes: cacheBytes},
		loader:    &singleflight.Group{},
	}
	groups[name] = g
	return g
}

// GetGroup 方法通过 name 得到Group对象指针
func GetGroup(name string) *Group {
	mu.RLock()
	g := groups[name]
	mu.RUnlock()
	return g
}

// Get 方法通过传入 key 进行查询是否有这个key
func (g *Group) Get(key string) (ByteView, error) {
	//如果为空，就报错
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}

	//先查询本地缓存
	//如果能在本地缓存中找到，返回对应的value
	if v, ok := g.mainCache.get(key); ok {
		log.Println("[GeeCache] hit")
		return v, nil
	}
	//如果在本地缓存找不到，就去其他分布式节点找/去数据库里找
	return g.load(key)
}

// RegisterPeers 实现了把传入的实现了 PeerPicker 接口的 HTTPPool 注入到 Group 的属性中
func (g *Group) RegisterPeers(peers PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeerPicker called more than once")
	}
	g.peers = peers
}

func (g *Group) load(key string) (value ByteView, err error) {
	// each key is only fetched once (either locally or remotely)
	// regardless of the number of concurrent callers.
	viewi, err := g.loader.Do(key, func() (interface{}, error) {
		//若非本机节点
		if g.peers != nil {
			if peer, ok := g.peers.PickPeer(key); ok {
				//则调用 getFromPeer() 从远程获取
				if value, err = g.getFromPeer(peer, key); err == nil {
					return value, nil
				}
				log.Println("[GeeCache] Failed to get from peer", err)
			}
		}
		//去数据库里找，这里会调用用户传入进来的回调函数
		return g.getLocally(key)
	})

	if err == nil {
		return viewi.(ByteView), nil
	}
	return
}

// 调用这个函数会把这个传入的数据添加到本地缓存中去
func (g *Group) populateCache(key string, value ByteView) {
	g.mainCache.add(key, value)
}

// 去数据库里找对应的key数据，这里会调用用户传入进来的回调函数
func (g *Group) getLocally(key string) (ByteView, error) {
	//通过传入的getter方法进行查询数据库
	bytes, err := g.getter.Get(key)
	//如果查询不到，就报错。
	if err != nil {
		return ByteView{}, err
	}
	//对数据进行封装，封装成ByteView对象
	value := ByteView{b: cloneBytes(bytes)}
	//把数据添加到本地缓存中
	g.populateCache(key, value)
	return value, nil
}

// getFromPeer 实现了访问远程节点，根据key获取返回值的功能
func (g *Group) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
	//构建req对象。
	req := &pb.Request{
		Group: g.name,
		Key:   key,
	}
	res := &pb.Response{}
	//这个接口是客户端用的，用于发起/<groupName>/<key> 类似的get请求
	err := peer.Get(req, res)
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{b: res.Value}, nil
}
