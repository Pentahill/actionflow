package protocol

// Decodable 可解码接口
// 实现此接口的类型可以从字节数据解码
type Decodable interface {
	// Decode 从字节数据解码到自身
	Decode(data []byte) error
}

// Encodable 可编码接口
// 实现此接口的类型可以编码为字节数据
type Encodable interface {
	// Encode 将自身编码为字节数据
	Encode() ([]byte, error)
}

// Codec 编解码接口
// 同时实现 Decodable 和 Encodable
type Codec interface {
	Decodable
	Encodable
}
