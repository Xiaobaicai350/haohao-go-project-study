package geecache

// ByteView 只读数据结构  用来表示缓存值。
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

// String returns the data as a string, making a copy if necessary.
func (v ByteView) String() string {
	return string(v.b)
}

func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}
