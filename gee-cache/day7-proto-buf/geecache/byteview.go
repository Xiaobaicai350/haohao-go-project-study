package geecache

// ByteView 用来表示缓存值。
// 他的存在意义是比如说我们查询出来了一个值，但是这个值是在切片中存的，我们返回肯定返回的是切片的地址，那肯定不行啊,会有风险的
// 因为如果我们把切片地址暴露给人家，如果人家拿着这个地址攻击我们怎么办。
// 所以下面最重要的方法其实就是ByteSlice，新创建一个数组，只返回内容，不返回切片地址，这样就没上述的风险了
type ByteView struct {
	//b 将会存储真实的缓存值。选择 byte 类型是为了能够支持任意的数据类型的存储，例如字符串、图片等。
	b []byte
}

// Len 返回ByteView里面的字节数组的长度
func (v ByteView) Len() int {
	return len(v.b)
}

// ByteSlice 方法返回一个拷贝，防止缓存值被外部程序修改
func (v ByteView) ByteSlice() []byte {
	return cloneBytes(v.b)
}

func (v ByteView) String() string {
	return string(v.b)
}

func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}
