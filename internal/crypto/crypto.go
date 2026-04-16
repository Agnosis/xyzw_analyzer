package crypto

import (
	"bytes"
	"github.com/pierrec/lz4"
	"io"
	"log"
	"math/rand"
	"time"
)

func init() {
	// 使用更安全的随机数种子
	rand.Seed(time.Now().UnixNano() ^ int64(rand.Uint64()))
}

// CompressLZ4 压缩与解压缩
func CompressLZ4(data []byte) []byte {
	if len(data) == 0 {
		return []byte{}
	}

	var compressedData bytes.Buffer
	encoder := lz4.NewWriter(&compressedData)
	if _, err := encoder.Write(data); err != nil {
		log.Printf("压缩数据失败: %v", err)
		return data // 失败时返回原始数据
	}
	encoder.Close()
	return compressedData.Bytes()
}

func DecompressLZ4(data []byte) []byte {
	if len(data) == 0 {
		return []byte{}
	}

	r := lz4.NewReader(bytes.NewReader(data))
	r2, err := io.ReadAll(r)
	if err != nil {
		log.Printf("解压数据失败: %v", err)
		return data // 失败时返回原始数据
	}
	return r2
}

// Encrypt 统一加密接口
func Encrypt(data []byte, method string, key ...byte) []byte {
	switch method {
	case "LX":
		return EncryptLX(data)
	case "X":
		return EncryptX(data)
	default:
		return data
	}
}

// Decrypt 统一解密接口
func Decrypt(data []byte, method string) []byte {
	switch method {
	case "LX":
		return DecryptLX(data)
	case "X":
		return DecryptX(data)
	default:
		return data
	}
}

// EncryptLX 优化: 添加错误处理和边界检查
func EncryptLX(data []byte) []byte {
	if len(data) == 0 {
		return []byte{}
	}

	// 首先压缩数据
	compressed := CompressLZ4(data)

	// 生成随机密钥 (2-250范围内)
	key := byte(2 + rand.Intn(248))

	// 对前100个字节进行异或加密
	n := min(len(compressed), 100)
	for i := 0; i < n; i++ {
		compressed[i] ^= key
	}

	// 设置标记字节
	if len(compressed) >= 4 {
		compressed[0] = 112
		compressed[1] = 108
		compressed[2] = 170&compressed[2] | (key>>7&1)<<6 | (key>>6&1)<<4 | (key>>5&1)<<2 | (key>>4&1)<<0
		compressed[3] = 170&compressed[3] | (key>>3&1)<<6 | (key>>2&1)<<4 | (key>>1&1)<<2 | (key>>0&1)<<0
	}

	return compressed
}

// DecryptLX 实现"lx"解密算法
// 对应JavaScript中的y函数
func DecryptLX(data []byte) []byte {
	if len(data) < 4 {
		return data
	}

	// 从头部字节中提取密钥
	key := ((data[2] >> 6 & 1) << 7) |
		((data[2] >> 4 & 1) << 6) |
		((data[2] >> 2 & 1) << 5) |
		((data[2] >> 0 & 1) << 4) |
		((data[3] >> 6 & 1) << 3) |
		((data[3] >> 4 & 1) << 2) |
		((data[3] >> 2 & 1) << 1) |
		((data[3] >> 0 & 1) << 0)

	// 对前100个字节进行异或解密
	n := min(len(data), 100)
	for i := 2; i < n; i++ {
		data[i] ^= key
	}

	// 设置LZ4头部字节
	data[0] = 4
	data[1] = 34
	data[2] = 77
	data[3] = 24

	// 解压缩数据
	return DecompressLZ4(data)
}

// EncryptX 实现"x"加密算法
// 对应JavaScript中的R函数
func EncryptX(data []byte) []byte {
	// 生成随机ID (32位整数)
	randomID := rand.Uint32()

	// 创建新的缓冲区，前4字节存放随机ID
	result := make([]byte, len(data)+4)
	result[0] = byte(randomID & 0xFF)
	result[1] = byte((randomID >> 8) & 0xFF)
	result[2] = byte((randomID >> 16) & 0xFF)
	result[3] = byte((randomID >> 24) & 0xFF)

	// 复制原始数据
	copy(result[4:], data)

	// 生成随机密钥 (2-250范围内)
	key := byte(2 + rand.Intn(248))

	// 对所有字节进行异或加密
	for i := len(result) - 1; i >= 0; i-- {
		result[i] ^= key
	}

	// 设置特定的头部字节
	result[0] = 112 // 'p'
	result[1] = 120 // 'x'

	// 将密钥信息嵌入到头部字节中
	result[2] = 170&result[2] |
		((key >> 7 & 1) << 6) |
		((key >> 6 & 1) << 4) |
		((key >> 5 & 1) << 2) |
		((key >> 4 & 1) << 0)

	result[3] = 170&result[3] |
		((key >> 3 & 1) << 6) |
		((key >> 2 & 1) << 4) |
		((key >> 1 & 1) << 2) |
		((key >> 0 & 1) << 0)

	return result
}

// DecryptX 实现"x"解密算法
// 对应JavaScript中的M函数
func DecryptX(data []byte) []byte {
	if len(data) < 4 {
		return data
	}

	// 从头部字节中提取密钥
	key := byte(
		((data[2] >> 6 & 1) << 7) |
			((data[2] >> 4 & 1) << 6) |
			((data[2] >> 2 & 1) << 5) |
			((data[2] >> 0 & 1) << 4) |
			((data[3] >> 6 & 1) << 3) |
			((data[3] >> 4 & 1) << 2) |
			((data[3] >> 2 & 1) << 1) |
			((data[3] >> 0 & 1) << 0))

	// 对所有字节进行异或解密
	for i := len(data) - 1; i >= 4; i-- {
		data[i] ^= key
	}

	// 返回去除前4个字节的数据
	return data[4:]
}

// 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
