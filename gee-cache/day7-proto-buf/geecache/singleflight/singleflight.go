package singleflight

import "sync"

/**
并发协程之间不需要消息传递，非常适合 sync.WaitGroup。
wg.Add(1) 锁加1。
wg.Wait() 阻塞，直到锁被释放。
wg.Done() 锁减1。
*/
// call 代表正在进行中，或已经结束的请求。使用 sync.WaitGroup 锁避免重入。
type call struct {
	wg  sync.WaitGroup
	val interface{}
	err error
}

// Group 是 singleflight 的主数据结构，管理不同 key 的请求(call)。
type Group struct {
	mu sync.Mutex
	m  map[string]*call //m存储着key对应的call
}

// Do 方法是Group的方法
// Do 方法，接收 2 个参数，第一个参数是 key，第二个参数是一个函数 fn。
// Do 的作用就是：
// 针对相同的 key，无论 Do 被调用多少次，函数 fn 都只会被调用一次，等待 fn 调用结束了，返回返回值或错误。
// 其实就是对fn套一层壳
func (g *Group) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	//g.mu 是保护 Group 的成员变量 m 不被并发读写而加上的锁。
	g.mu.Lock()
	//m进行懒加载初始化
	if g.m == nil {
		g.m = make(map[string]*call)
	}
	//如果能从map中找到key，说明正在查询
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		//如果请求正在进行中，就等待
		c.wg.Wait()
		//请求结束，返回结果
		return c.val, c.err
	} else { //如果找不到，说明对应的key没有正在读写。
		c := new(call)
		//发起请求前加锁
		c.wg.Add(1)
		//添加到g.m，表明key已经有对应的请求在处理了
		g.m[key] = c
		g.mu.Unlock()

		//调用fn，发起请求
		c.val, c.err = fn()
		//请求结束
		c.wg.Done()

		//防止并发读写map，加锁
		g.mu.Lock()
		//更新g.m
		delete(g.m, key)
		g.mu.Unlock()

		return c.val, c.err
	}
}
