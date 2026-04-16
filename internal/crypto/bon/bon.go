package bon

import (
	"encoding/json"
	"errors"
	"xyzw_study/internal/crypto"
)

var (
	// 全局单例解码器和编码器
	globalDecoder = NewBonDecoder()
	globalEncoder = NewBonEncoder()
)

// Decode 解码二进制数据为Go对象
func _Decode(data []byte) (interface{}, error) {
	if data == nil || len(data) == 0 {
		return nil, errors.New("empty data")
	}

	globalDecoder.Reset(data)
	return globalDecoder.Decode()
}

// Encode 将Go对象编码为二进制数据
func _Encode(value interface{}, copy bool) ([]byte, error) {
	globalEncoder.Reset()
	if err := globalEncoder.Encode(value); err != nil {
		return nil, err
	}
	return globalEncoder.GetBytes(copy), nil
}

// EncodeToBytes 简化版的Encode，总是返回复制的字节数组
func EncodeToBytes(value interface{}) []byte {
	data, err := _Encode(value, true)
	if err != nil {
		return nil
	}
	return data
}

// EncodeAndEncrypt 统一加密解密接口
func EncodeAndEncrypt(value interface{}, method string) ([]byte, error) {
	data, err := _Encode(value, true)
	if err != nil {
		return nil, err
	}

	switch method {
	case "LX":
		return crypto.EncryptLX(data), nil
	case "X":
		return crypto.EncryptX(data), nil
	default:
		return data, nil
	}
}

// DecryptAndDecode 统一解密解码接口
func DecryptAndDecode(data []byte, method string) (interface{}, error) {
	if data == nil || len(data) == 0 {
		return nil, errors.New("empty data")
	}

	var decrypted []byte
	switch method {
	case "LX":
		decrypted = crypto.DecryptLX(data)
	case "X":
		decrypted = crypto.DecryptX(data)
	default:
		decrypted = data
	}

	return _Decode(decrypted)
}

// EncodeAndEncryptLX 为了兼容性保留的原始函数
func EncodeAndEncryptLX(value interface{}) ([]byte, error) {
	return EncodeAndEncrypt(value, "LX")
}

func DecryptLXAndDecode(data []byte) (interface{}, error) {
	return DecryptAndDecode(data, "LX")
}

func EncodeAndEncryptX(value interface{}) ([]byte, error) {
	return EncodeAndEncrypt(value, "X")
}

func DecryptXAndDecode(data []byte) (interface{}, error) {
	return DecryptAndDecode(data, "X")
}

func EncodeReplaceSeq(data []byte, seq int32) []byte {
	result, err := DecryptXAndDecode(data)
	if err != nil || result == nil {
		return nil
	}
	m := result.(map[string]any)
	m["seq"] = seq
	bs, err := EncodeAndEncryptX(m)
	if err != nil {
		return nil
	}
	return bs
}
func EncodeReplaceResp(data []byte, seq int32) []byte {
	result, err := DecryptXAndDecode(data)
	if err != nil || result == nil {
		return nil
	}
	m := result.(map[string]any)
	m["resp"] = seq
	bs, err := EncodeAndEncryptX(m)
	if err != nil {
		return nil
	}
	return bs
}
func EncodeReplaceAck(data []byte, ack int32) []byte {
	result, err := DecryptXAndDecode(data)
	if err != nil || result == nil {
		return nil
	}
	m := result.(map[string]any)
	m["ack"] = ack
	bs, err := EncodeAndEncryptX(m)
	if err != nil {
		return nil
	}
	return bs
}
func DecodeX(data []byte) string {
	result, err := DecryptXAndDecode(data)
	if err != nil || result == nil {
		return ""
	}
	m := result.(map[string]any)
	body, ok := m["body"].([]byte)
	if ok {
		bodyR := DecodeFromBytes(body)
		m["body"] = bodyR
	}
	r, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(r)
}
func DecodeXAsMap(data []byte) map[string]any {
	result, err := DecryptXAndDecode(data)
	if err != nil || result == nil {
		return nil
	}
	return result.(map[string]any)

}
func DecodeFromBytes(data []byte) interface{} {
	result, err := _Decode(data)
	if err != nil {
		return nil
	}
	return result
}
