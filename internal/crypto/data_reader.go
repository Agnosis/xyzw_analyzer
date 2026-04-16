package crypto

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
)

// DataReader 实现二进制数据读取器
type DataReader struct {
	data     []byte
	position int
	dataView *bytes.Reader
}

// NewDataReader 创建一个新的DataReader实例
func NewDataReader(data []byte) *DataReader {
	return &DataReader{
		data:     data,
		position: 0,
		dataView: bytes.NewReader(data),
	}
}

// Reset 重置数据和位置
func (r *DataReader) Reset(data []byte) {
	r.data = data
	r.position = 0
	r.dataView = bytes.NewReader(data)
}

// Validate 验证是否有足够的数据可读
func (r *DataReader) Validate(size int) bool {
	if r.position+size > len(r.data) {
		return false
	}
	return true
}

// ReadUInt8 读取一个无符号8位整数
func (r *DataReader) ReadUInt8() (uint8, error) {
	if !r.Validate(1) {
		return 0, errors.New("read eof")
	}
	val := r.data[r.position]
	r.position++
	return val, nil
}

// ReadInt16 读取一个16位整数
func (r *DataReader) ReadInt16() (int16, error) {
	if !r.Validate(2) {
		return 0, errors.New("read eof")
	}
	val := int16(r.data[r.position]) | int16(r.data[r.position+1])<<8
	r.position += 2
	return val, nil
}

// ReadInt32 读取一个32位整数
func (r *DataReader) ReadInt32() (int32, error) {
	if !r.Validate(4) {
		return 0, errors.New("read eof")
	}
	val := int32(r.data[r.position]) |
		int32(r.data[r.position+1])<<8 |
		int32(r.data[r.position+2])<<16 |
		int32(r.data[r.position+3])<<24
	r.position += 4
	return val, nil
}

// ReadInt64 读取一个64位整数
func (r *DataReader) ReadInt64() (int64, error) {
	val, err := r.ReadInt32()
	if err != nil {
		return 0, err
	}

	var val64 int64
	if val < 0 {
		val64 = int64(val) + 4294967296
	} else {
		val64 = int64(val)
	}

	val2, err := r.ReadInt32()
	if err != nil {
		return 0, err
	}

	return val64 + 4294967296*int64(val2), nil
}

// ReadFloat32 读取一个32位浮点数
func (r *DataReader) ReadFloat32() (float32, error) {
	if !r.Validate(4) {
		return 0, errors.New("read eof")
	}

	// 保存当前位置
	r.dataView.Reset(r.data[r.position:])
	var val uint32
	err := binary.Read(r.dataView, binary.LittleEndian, &val)
	if err != nil {
		return 0, err
	}

	r.position += 4
	return math.Float32frombits(val), nil
}

// ReadFloat64 读取一个64位浮点数
func (r *DataReader) ReadFloat64() (float64, error) {
	if !r.Validate(8) {
		return 0, errors.New("read eof")
	}

	// 保存当前位置
	r.dataView.Reset(r.data[r.position:])
	var val uint64
	err := binary.Read(r.dataView, binary.LittleEndian, &val)
	if err != nil {
		return 0, err
	}

	r.position += 8
	return math.Float64frombits(val), nil
}

// Read7BitInt 读取7位编码的整数
func (r *DataReader) Read7BitInt() (int, error) {
	var result int
	var shift int

	for {
		if shift >= 35 {
			return 0, errors.New("Format_Bad7BitInt32")
		}

		b, err := r.ReadUInt8()
		if err != nil {
			return 0, err
		}

		result |= int(b&0x7F) << shift
		shift += 7

		if b&0x80 == 0 {
			break
		}
	}

	return result, nil
}

// ReadUTF 读取UTF-8编码的字符串
func (r *DataReader) ReadUTF() (string, error) {
	length, err := r.Read7BitInt()
	if err != nil {
		return "", err
	}

	return r.ReadUTFBytes(length)
}

// ReadUint8Array 读取指定长度的字节数组
func (r *DataReader) ReadUint8Array(length int, c bool) ([]byte, error) {
	if !r.Validate(length) {
		return nil, errors.New("read eof")
	}

	var result []byte
	if c {
		result = make([]byte, length)
		copy(result, r.data[r.position:r.position+length])
	} else {
		result = r.data[r.position : r.position+length]
	}

	r.position += length
	return result, nil
}

// ReadUTFBytes 读取指定长度的UTF-8字符串
func (r *DataReader) ReadUTFBytes(length int) (string, error) {
	if length == 0 {
		return "", nil
	}

	if !r.Validate(length) {
		return "", errors.New("read eof")
	}

	result := string(r.data[r.position : r.position+length])
	r.position += length
	return result, nil
}

// ReadBytes 优化: 添加批量读取方法
func (r *DataReader) ReadBytes(size int) ([]byte, error) {
	if !r.Validate(size) {
		return nil, errors.New("read eof")
	}

	result := r.data[r.position : r.position+size]
	r.position += size
	return result, nil
}
