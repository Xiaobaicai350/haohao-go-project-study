package geecache

import (
	"geecache/lru"
	"sync"
)

// 这个是小cache，是对lru里面的Cache的一个封装
type cache struct {
	mu         sync.Mutex
	lru        *lru.Cache
	cacheBytes int64
}

// 封装lru的add方法
func (c *cache) add(key string, value ByteView) {
	c.mu.Lock()
	defer c.mu.Unlock()
	//如果c的lru为nil，说明里面还没数据，需要初始化，采用了懒加载的模式。只有第一次的时候才会使用
	if c.lru == nil {
		c.lru = lru.New(c.cacheBytes, nil)
	}
	//把数据存储到Cache中
	c.lru.Add(key, value)
}

// get 是从缓存中获取值
func (c *cache) get(key string) (value ByteView, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	//安全性校验。
	if c.lru == nil {
		return
	}
	//从Cache中获取值
	if v, ok := c.lru.Get(key); ok {
		return v.(ByteView), ok
	}

	return
}
