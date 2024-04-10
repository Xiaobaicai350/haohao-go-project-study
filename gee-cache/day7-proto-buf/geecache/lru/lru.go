package lru

import "container/list"

// Cache 缓存是LRU缓存。并发访问是不安全的。
type Cache struct {
	maxBytes int64
	nbytes   int64
	//真正存储数据的东西，节点是entry，在下面定义了
	ll *list.List
	//key是链表的key，值是对应list的指针
	cache map[string]*list.Element
	//可选，并在清除条目时执行。
	OnEvicted func(key string, value Value)
}

type entry struct {
	key   string
	value Value
}

// Value 使用 Len() 计算它占用的字节数
type Value interface {
	Len() int
}

// New 是Cache的构造函数
func New(maxBytes int64, onEvicted func(string, Value)) *Cache {
	return &Cache{
		maxBytes:  maxBytes,
		ll:        list.New(),
		cache:     make(map[string]*list.Element),
		OnEvicted: onEvicted,
	}
}

// Add 向缓存添加值。
func (c *Cache) Add(key string, value Value) {
	//如果键存在
	if ele, ok := c.cache[key]; ok {
		//移到队尾，说明又被访问了
		c.ll.MoveToFront(ele)
		//得到entry。
		kv := ele.Value.(*entry)
		//更新大小
		c.nbytes += int64(value.Len()) - int64(kv.value.Len())
		//更新对应节点的值
		kv.value = value
	} else { //如果不存在
		//在队尾添加entry
		ele := c.ll.PushFront(&entry{key, value})
		//添加到map中
		c.cache[key] = ele
		//更新内存大小
		c.nbytes += int64(len(key)) + int64(value.Len())
	}
	//验证内存是否超了，如果超了，还要进行lru
	for c.maxBytes != 0 && c.maxBytes < c.nbytes {
		c.RemoveOldest()
	}
}

// Get 查找key的值
func (c *Cache) Get(key string) (value Value, ok bool) {
	//通过map找到map的value，map的value存储的是对应值链表的指针。
	if ele, ok := c.cache[key]; ok {
		//先把指针移到队尾，因为使用的是lru
		c.ll.MoveToFront(ele)
		//得到对应链表的指针，这里使用了类型断言
		kv := ele.Value.(*entry)
		//之后返回链表节点的值就可以了
		return kv.value, true
	}
	return
}

// RemoveOldest 这里的删除，实际上是缓存淘汰。即移除最近最少访问的节点（队首）
func (c *Cache) RemoveOldest() {
	//取到链表的首节点
	ele := c.ll.Back()
	if ele != nil {
		//把链表的首节点删除
		c.ll.Remove(ele)
		//得到entry键值对
		kv := ele.Value.(*entry)
		//把map里的key为这个的也删除
		delete(c.cache, kv.key)
		//更新当前所占用的内存
		c.nbytes -= int64(len(kv.key)) + int64(kv.value.Len())
		//如果回调函数不为空，就调用回调函数（回调函数是用户传入进来的）
		if c.OnEvicted != nil {
			c.OnEvicted(kv.key, kv.value)
		}
	}
}

// Len the number of cache entries
func (c *Cache) Len() int {
	return c.ll.Len()
}
