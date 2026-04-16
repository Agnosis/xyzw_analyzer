package crypto

import (
	"bytes"
	"encoding/binary"
	"math"
)

// DataWriter 实现类似JavaScript中的二进制数据写入器
type DataWriter struct {
	data     []byte
	position int
	buffer   *bytes.Buffer
}

// NewDataWriter 创建一个新的DataWriter实例
func NewDataWriter() *DataWriter {
	initialSize := 524288 // 与JS代码中的524288相同
	return &DataWriter{
		data:     make([]byte, initialSize),
		position: 0,
		buffer:   bytes.NewBuffer(make([]byte, 0, initialSize)),
	}
}

// Reset 重置数据和位置
func (w *DataWriter) Reset() {
	w.position = 0
	w.buffer.Reset()
}

// EnsureBuffer 确保缓冲区有足够的空间
func (w *DataWriter) EnsureBuffer(size int) {
	if w.position+size > len(w.data) {
		// 扩展缓冲区
		newSize := int(float64(len(w.data)) * 1.2)
		if newSize < w.position+size {
			newSize = w.position + size
		}

		newData := make([]byte, newSize)
		copy(newData, w.data)
		w.data = newData
	}
}

// WriteInt8 写入一个8位整数
func (w *DataWriter) WriteInt8(val int) {
	w.EnsureBuffer(1)
	w.data[w.position] = byte(val)
	w.position++
}

// WriteInt16 写入一个16位整数
func (w *DataWriter) WriteInt16(val int) {
	w.EnsureBuffer(2)
	w.data[w.position] = byte(val)
	w.data[w.position+1] = byte(val >> 8)
	w.position += 2
}

// WriteInt32 写入一个32位整数
func (w *DataWriter) WriteInt32(val int) {
	w.EnsureBuffer(4)
	w.data[w.position] = byte(val)
	w.data[w.position+1] = byte(val >> 8)
	w.data[w.position+2] = byte(val >> 16)
	w.data[w.position+3] = byte(val >> 24)
	w.position += 4
}

// WriteInt64 写入一个64位整数
func (w *DataWriter) WriteInt64(val int64) {
	w.WriteInt32(int(val))
	if val < 0 {
		w.WriteInt32(^int(val / 4294967296))
	} else {
		w.WriteInt32(int(val / 4294967296))
	}
}

// WriteFloat32 写入一个32位浮点数
func (w *DataWriter) WriteFloat32(val float32) {
	w.EnsureBuffer(4)
	bits := math.Float32bits(val)
	binary.LittleEndian.PutUint32(w.data[w.position:], bits)
	w.position += 4
}

// WriteFloat64 写入一个64位浮点数
func (w *DataWriter) WriteFloat64(val float64) {
	w.EnsureBuffer(8)
	bits := math.Float64bits(val)
	binary.LittleEndian.PutUint64(w.data[w.position:], bits)
	w.position += 8
}

// Write7BitInt 写入7位编码的整数
func (w *DataWriter) Write7BitInt(val int) {
	w.EnsureBuffer(5) // 最多需要5个字节
	w._write7BitInt(val)
}

// _write7BitInt 内部方法，写入7位编码的整数
func (w *DataWriter) _write7BitInt(val int) {
	for val >= 128 {
		w.data[w.position] = byte(val | 0x80)
		w.position++
		val >>= 7
	}
	w.data[w.position] = byte(val)
	w.position++
}

// _7BitIntLen 计算7位编码整数需要的字节数
func (w *DataWriter) _7BitIntLen(val int) int {
	if val < 0 {
		return 5
	} else if val < 128 {
		return 1
	} else if val < 16384 {
		return 2
	} else if val < 2097152 {
		return 3
	} else if val < 268435456 {
		return 4
	} else {
		return 5
	}
}

// WriteUTF 写入UTF-8编码的字符串
func (w *DataWriter) WriteUTF(val string) {
	if len(val) == 0 {
		w.Write7BitInt(0)
		return
	}

	// 预估字节长度
	estimatedLen := len(val) * 6
	w.EnsureBuffer(5 + estimatedLen)

	// 保存当前位置，稍后回写实际长度
	startPos := w.position
	w.position += w._7BitIntLen(estimatedLen)

	// 写入字符串内容
	contentStartPos := w.position
	w._writeUTFBytes(val)

	// 计算实际写入的字节数
	actualLen := w.position - contentStartPos

	// 回到开始位置写入实际长度
	currentPos := w.position
	w.position = startPos
	w._write7BitInt(actualLen)

	// 如果长度字段占用的字节数与预估的不同，需要移动数据
	lenFieldSize := w.position - startPos
	expectedLenFieldSize := w._7BitIntLen(estimatedLen)

	if lenFieldSize != expectedLenFieldSize {
		// 修复：正确计算源和目标切片
		diff := lenFieldSize - expectedLenFieldSize
		if diff > 0 {
			// 长度字段比预期长，需要向后移动数据
			copy(w.data[contentStartPos+diff:], w.data[contentStartPos:contentStartPos+(currentPos-contentStartPos)])
		} else {
			// 长度字段比预期短，需要向前移动数据
			copy(w.data[contentStartPos+diff:], w.data[contentStartPos:currentPos])
		}
	}

	// 更新最终位置
	w.position = contentStartPos + (lenFieldSize - expectedLenFieldSize) + actualLen
}

// WriteUint8Array 写入字节数组
func (w *DataWriter) WriteUint8Array(data []byte, offset, length int) {
	if offset < 0 {
		offset = 0
	}

	if length <= 0 || offset >= len(data) {
		return
	}

	if offset+length > len(data) {
		length = len(data) - offset
	}

	w.EnsureBuffer(length)
	copy(w.data[w.position:], data[offset:offset+length])
	w.position += length
}

// _writeUTFBytes 内部方法，写入UTF-8编码的字符串
func (w *DataWriter) _writeUTFBytes(val string) {
	bytes := []byte(val)
	copy(w.data[w.position:], bytes)
	w.position += len(bytes)
}

// WriteUTFBytes 写入UTF-8编码的字符串，不包含长度前缀
func (w *DataWriter) WriteUTFBytes(val string) {
	w.EnsureBuffer(len(val) * 6) // 预留足够空间
	w._writeUTFBytes(val)
}

// GetBytes 获取写入的数据
func (w *DataWriter) GetBytes(c bool) []byte {
	if c {
		result := make([]byte, w.position)
		copy(result, w.data[:w.position])
		return result
	} else {
		return w.data[:w.position]
	}
}
