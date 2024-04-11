package geecache

import pb "geecache/geecachepb"

type PeerPicker interface {
	// PickPeer 用于根据传入的key选择相应节点，然后在根据节点返回对应的httpGetter
	PickPeer(key string) (peer PeerGetter, ok bool)
}

type PeerGetter interface {
	// Get 方法用于从group中查找缓存值
	Get(in *pb.Request, out *pb.Response) error
}
