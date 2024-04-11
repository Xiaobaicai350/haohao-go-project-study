package consistenthash

import (
	"hash/crc32"
	"sort"
	"strconv"
)

// Hash 函数型接口，跟 GetterFunc 一样都是函数型接口
type Hash func(data []byte) uint32

// Map 类型主要功能是根据具体的key选择节点
type Map struct {
	hash     Hash           //hash函数
	replicas int            //虚拟节点倍数，如果是3的话，原来有2个节点，现在就有6个虚拟节点
	keys     []int          // 哈希环
	hashMap  map[int]string //虚拟节点和真实节点的映射表，key为虚拟节点的hash值，value是真实节点的名称
}

// New 方法进行构建一个Map，可以传入hash函数和虚拟节点倍数的值，也会对hashMap进行懒加载。对hash函数赋默认值
func New(replicas int, fn Hash) *Map {
	m := &Map{
		replicas: replicas,
		hash:     fn,
		hashMap:  make(map[int]string),
	}
	if m.hash == nil {
		m.hash = crc32.ChecksumIEEE
	}
	return m
}

// Add 函数主要是通过把节点名传入进来，通过一致性hash算法添加到服务器的切片中
// Add 函数允许传入   0-n个   真实节点的名称
func (m *Map) Add(keys ...string) {
	for _, key := range keys {
		for i := 0; i < m.replicas; i++ {
			//strconv.Itoa(i)函数用于把数字转换成字符串 其实计算的是"0节点1"的hash值
			hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
			//把hash值添加到切片中
			m.keys = append(m.keys, hash)
			//添加到虚拟节点和真实节点的映射表中，key为虚拟节点的hash值，value是真实节点的名称
			m.hashMap[hash] = key
		}
	}
	//将哈希环进行排序
	sort.Ints(m.keys)
}

// Get 获取与key最接近的节点
func (m *Map) Get(key string) string {
	//安全性校验
	if len(m.keys) == 0 {
		return ""
	}
	//计算key的hash值
	hash := int(m.hash([]byte(key)))
	/**
	sort.Search 函数用于在已排序的切片中执行二分查找。它接受两个参数：
		1.查找范围的长度
		2.一个返回布尔值的比较函数。
	*/
	//翻译一下下面的代码：其实就是找keys里面大于等于hash值的第一个索引
	idx := sort.Search(len(m.keys), func(i int) bool {
		return m.keys[i] >= hash
	})
	//本来应该选择m.keys[idx]的，但是如果idx==len(m.keys)的话，应该选择m.keys[0]，所以进行了取模运算
	dummyKey := m.keys[idx%len(m.keys)]
	//从虚拟节点映射到真实节点
	return m.hashMap[dummyKey]
}
