package main

/*
$ curl "http://localhost:9999/api?key=Tom"
630

$ curl "http://localhost:9999/api?key=kkk"
kkk not exist
*/

import (
	"flag"
	"fmt"
	"geecache"
	"log"
	"net/http"
)

var db = map[string]string{
	"Tom":  "630",
	"Jack": "589",
	"Sam":  "567",
}

func createGroup() *geecache.Group {
	return geecache.NewGroup("scores", 2<<10, geecache.GetterFunc(
		func(key string) ([]byte, error) {
			log.Println("[SlowDB] search key", key)
			if v, ok := db[key]; ok {
				return []byte(v), nil
			}
			return nil, fmt.Errorf("%s not exist", key)
		}))
}

// 用来启动缓存服务器：创建 HTTPPool，添加节点信息，注册到 gee 中，
// 启动 HTTP 服务（共3个端口，8001/8002/8003），用户不感知。
func startCacheServer(addr string, addrs []string, gee *geecache.Group) {
	//新建HTTPPool，用于服务端
	peers := geecache.NewHTTPPool(addr)
	peers.Set(addrs...)
	gee.RegisterPeers(peers)
	log.Println("geecache is running at", addr)
	log.Fatal(http.ListenAndServe(addr[7:], peers))
}

// 启动api服务器，提供给用户进行访问
func startAPIServer(apiAddr string, gee *geecache.Group) {
	http.Handle("/api", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			key := r.URL.Query().Get("key")
			view, err := gee.Get(key)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(view.ByteSlice())

		}))
	log.Println("fontend server is running at", apiAddr)
	log.Fatal(http.ListenAndServe(apiAddr[7:], nil))

}

// main() 函数需要命令行传入 port 和 api 2 个参数，用来在指定端口启动 HTTP 服务。
func main() {
	var port int // 定义一个整数类型的变量 port，用于存储 Geecache 服务器的端口号
	var api bool // 定义一个布尔类型的变量 api，用于标识是否启动 API 服务器

	// 定义一个整数类型的命令行标志 port，并将其与变量 port 相关联
	// 如果在命令行中指定了该标志，则更新变量 port 的值；否则使用默认值 8001
	// "Geecache server port" 是该标志的简短描述，用于在命令行帮助信息中显示
	flag.IntVar(&port, "port", 8001, "Geecache server port")

	// 定义一个布尔类型的命令行标志 api，并将其与变量 api 相关联
	// 如果在命令行中指定了该标志，则更新变量 api 的值；否则使用默认值 false
	// "Start a api server?" 是该标志的简短描述，用于在命令行帮助信息中显示
	flag.BoolVar(&api, "api", true, "Start a api server?")

	// 解析命令行参数，将命令行参数的值更新到对应的变量中
	flag.Parse()

	apiAddr := "http://localhost:9999"
	addrMap := map[int]string{
		8001: "http://localhost:8001",
		8002: "http://localhost:8002",
		8003: "http://localhost:8003",
	}

	var addrs []string
	for _, v := range addrMap {
		addrs = append(addrs, v)
	}

	gee := createGroup()
	if api {
		// 用来启动一个 API 服务（端口 9999），与用户进行交互，用户感知
		go startAPIServer(apiAddr, gee)
	}
	//用来启动缓存服务器：创建 HTTPPool，添加节点信息，注册到 gee 中，
	//启动 HTTP 服务（共3个端口，8001/8002/8003），用户不感知。
	startCacheServer(addrMap[port], addrs, gee)
}
