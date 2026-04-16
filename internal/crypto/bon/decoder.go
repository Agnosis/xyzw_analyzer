package bon

import (
	"fmt"
	"time"
	"xyzw_study/internal/crypto"
)

// Int64 表示一个64位整数
type Int64 struct {
	High int
	Low  int
}

// BonDecoder 实现二进制对象表示法的解码器
type BonDecoder struct {
	dr     *crypto.DataReader
	strArr []string
}

// NewBonDecoder 创建一个新的BonDecoder实例
func NewBonDecoder() *BonDecoder {
	return &BonDecoder{
		dr:     crypto.NewDataReader(nil),
		strArr: make([]string, 0),
	}
}

// Reset 重置解码器状态
func (d *BonDecoder) Reset(data []byte) {
	d.dr.Reset(data)
	d.strArr = d.strArr[:0]
}

// Decode 解码二进制数据为Go对象
func (d *BonDecoder) Decode() (interface{}, error) {
	typeCode, err := d.dr.ReadUInt8()
	if err != nil {
		return nil, err
	}

	switch typeCode {
	default:
		return nil, nil
	case 1: // Int
		return d.dr.ReadInt32()
	case 2: // Long
		return d.dr.ReadInt64()
	case 3: // Float
		return d.dr.ReadFloat32()
	case 4: // Double
		return d.dr.ReadFloat64()
	case 5: // String
		str, err := d.dr.ReadUTF()
		if err != nil {
			return nil, err
		}
		d.strArr = append(d.strArr, str)
		return str, nil
	case 6: // Boolean
		b, err := d.dr.ReadUInt8()
		if err != nil {
			return nil, err
		}
		return b == 1, nil
	case 7: // Binary
		length, err := d.dr.Read7BitInt()
		if err != nil {
			return nil, err
		}
		return d.dr.ReadUint8Array(length, false)
	case 8: // Object/Map
		count, err := d.dr.Read7BitInt()
		if err != nil {
			return nil, err
		}

		result := make(map[string]interface{})
		for i := 0; i < count; i++ {
			key, err := d.Decode()
			if err != nil {
				return nil, err
			}

			value, err := d.Decode()
			if err != nil {
				return nil, err
			}

			// 将键转换为字符串
			keyStr, ok := key.(string)
			if !ok {
				keyStr = fmt.Sprintf("%v", key)
			}

			result[keyStr] = value
		}
		return result, nil
	case 9: // Array
		length, err := d.dr.Read7BitInt()
		if err != nil {
			return nil, err
		}

		result := make([]interface{}, length)
		for i := 0; i < length; i++ {
			value, err := d.Decode()
			if err != nil {
				return nil, err
			}
			result[i] = value
		}
		return result, nil
	case 10: // DateTime
		timestamp, err := d.dr.ReadInt64()
		if err != nil {
			return nil, err
		}
		return time.Unix(0, timestamp*int64(time.Millisecond)), nil
	case 99: // String reference
		index, err := d.dr.Read7BitInt()
		if err != nil {
			return nil, err
		}
		if index < 0 || index >= len(d.strArr) {
			return "", nil
		}
		return d.strArr[index], nil
	}
}

// toString 将任意值转换为字符串
