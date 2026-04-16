package bon

import (
	"reflect"
	"time"
	"xyzw_study/internal/crypto"
)

// BonEncoder 实现二进制对象表示法的编码器
type BonEncoder struct {
	dw     *crypto.DataWriter
	strMap map[string]int
}

// NewBonEncoder 创建一个新的BonEncoder实例
func NewBonEncoder() *BonEncoder {
	return &BonEncoder{
		dw:     crypto.NewDataWriter(),
		strMap: make(map[string]int, 64), // 预分配更合理的容量
	}
}

// Reset 重置编码器状态
func (e *BonEncoder) Reset() {
	e.dw.Reset()
	e.strMap = make(map[string]int, 64)
}

// 优化: 合并数字类型编码逻辑
func (e *BonEncoder) EncodeNumber(val interface{}) error {
	switch v := val.(type) {
	case int:
		return e.EncodeInt(v)
	case int8, int16, int32, uint8, uint16:
		return e.EncodeInt(int(reflect.ValueOf(v).Int()))
	case int64, uint, uint32, uint64:
		return e.EncodeLong(reflect.ValueOf(v).Int())
	case float32:
		return e.EncodeInt(int(v))
	case float64:
		// 检查是否可以表示为更小的类型
		intVal := int(v)
		return e.EncodeInt(intVal)
	default:
		return e.EncodeDouble(reflect.ValueOf(v).Float())
	}
}

// Encode 编码任意类型的值
func (e *BonEncoder) Encode(val interface{}) error {
	if val == nil {
		return e.EncodeNull()
	}

	// 使用反射优化类型判断
	v := reflect.ValueOf(val)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return e.EncodeNumber(val)
	case reflect.Bool:
		return e.EncodeBoolean(v.Bool())
	case reflect.String:
		return e.EncodeString(v.String())
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			// []byte 类型
			return e.EncodeBinary(v.Bytes())
		}
		// 其他切片类型
		length := v.Len()
		e.dw.WriteInt8(9)
		e.dw.Write7BitInt(length)
		for i := 0; i < length; i++ {
			if err := e.Encode(v.Index(i).Interface()); err != nil {
				return err
			}
		}
		return nil
	case reflect.Map:
		e.dw.WriteInt8(8)
		e.dw.Write7BitInt(v.Len())
		for _, key := range v.MapKeys() {
			if err := e.Encode(key.Interface()); err != nil {
				return err
			}
			if err := e.Encode(v.MapIndex(key).Interface()); err != nil {
				return err
			}
		}
		return nil
	case reflect.Struct:
		// 特殊处理已知类型
		if t, ok := val.(time.Time); ok {
			return e.EncodeDateTime(t)
		}
		if t, ok := val.(Int64); ok {
			return e.EncodeLong(t)
		}
		// 将结构体转换为map
		return e.encodeStruct(v)
	default:
		return e.EncodeNull()
	}
}

// 新增: 结构体编码方法
func (e *BonEncoder) encodeStruct(v reflect.Value) error {
	t := v.Type()
	fields := make(map[string]interface{})

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		// 跳过非导出字段
		if field.PkgPath != "" {
			continue
		}
		// 获取json标签或使用字段名
		name := field.Name
		if tag, ok := field.Tag.Lookup("json"); ok && tag != "-" {
			name = tag
		}
		fields[name] = v.Field(i).Interface()
	}

	// 编码为map
	e.dw.WriteInt8(8)
	e.dw.Write7BitInt(len(fields))
	for k, v := range fields {
		if err := e.Encode(k); err != nil {
			return err
		}
		if err := e.Encode(v); err != nil {
			return err
		}
	}
	return nil
}

// EncodeInt 编码整数
func (e *BonEncoder) EncodeInt(val int) error {
	e.dw.WriteInt8(1)
	e.dw.WriteInt32(val)
	return nil
}

// EncodeLong 编码长整数
func (e *BonEncoder) EncodeLong(val interface{}) error {
	e.dw.WriteInt8(2)

	e.dw.WriteInt64(reflect.ValueOf(val).Int())

	return nil
}

// EncodeFloat 编码32位浮点数
func (e *BonEncoder) EncodeFloat(val float32) error {
	e.dw.WriteInt8(3)
	e.dw.WriteFloat32(val)
	return nil
}

// EncodeDouble 编码64位浮点数
func (e *BonEncoder) EncodeDouble(val float64) error {
	e.dw.WriteInt8(4)
	e.dw.WriteFloat64(val)
	return nil
}

// EncodeString 编码字符串
func (e *BonEncoder) EncodeString(val string) error {
	// 检查字符串是否已经编码过
	if index, exists := e.strMap[val]; exists {
		e.dw.WriteInt8(99)
		e.dw.Write7BitInt(index)
	} else {
		e.dw.WriteInt8(5)
		e.dw.WriteUTF(val)
		e.strMap[val] = len(e.strMap)
	}
	return nil
}

// EncodeBoolean 编码布尔值
func (e *BonEncoder) EncodeBoolean(val bool) error {
	e.dw.WriteInt8(6)
	if val {
		e.dw.WriteInt8(1)
	} else {
		e.dw.WriteInt8(0)
	}
	return nil
}

// EncodeNull 编码空值
func (e *BonEncoder) EncodeNull() error {
	e.dw.WriteInt8(0)
	return nil
}

// EncodeDateTime 编码日期时间
func (e *BonEncoder) EncodeDateTime(val time.Time) error {
	e.dw.WriteInt8(10)
	e.dw.WriteInt64(val.UnixNano() / int64(time.Millisecond))
	return nil
}

// EncodeBinary 编码二进制数据
func (e *BonEncoder) EncodeBinary(val []byte) error {
	e.dw.WriteInt8(7)
	e.dw.Write7BitInt(len(val))
	e.dw.WriteUint8Array(val, 0, len(val))
	return nil
}

// EncodeArray 编码数组
func (e *BonEncoder) EncodeArray(val []interface{}) error {
	e.dw.WriteInt8(9)
	e.dw.Write7BitInt(len(val))

	for _, item := range val {
		if err := e.Encode(item); err != nil {
			return err
		}
	}

	return nil
}

// EncodeMap 编码映射
func (e *BonEncoder) EncodeMap(val map[string]interface{}) error {
	e.dw.WriteInt8(8)
	e.dw.Write7BitInt(len(val))

	for k, v := range val {
		if err := e.Encode(k); err != nil {
			return err
		}
		if err := e.Encode(v); err != nil {
			return err
		}
	}

	return nil
}

// EncodeObject 编码对象
func (e *BonEncoder) EncodeObject(val interface{}) error {
	// 使用反射获取对象的字段
	v := reflect.ValueOf(val)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		// 如果不是结构体，尝试转换为map
		return e.EncodeMap(structToMap(val))
	}

	// 计算有效字段数量
	validFields := 0
	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)
		// 跳过以$开头的字段和函数类型字段
		if field.Name[0] == '$' || field.Type.Kind() == reflect.Func {
			continue
		}
		validFields++
	}

	e.dw.WriteInt8(8)
	e.dw.Write7BitInt(validFields)

	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)
		// 跳过以$开头的字段和函数类型字段
		if field.Name[0] == '$' || field.Type.Kind() == reflect.Func {
			continue
		}

		// 编码字段名
		if err := e.Encode(field.Name); err != nil {
			return err
		}

		// 编码字段值
		if err := e.Encode(v.Field(i).Interface()); err != nil {
			return err
		}
	}

	return nil
}

// GetBytes 获取编码后的字节数据
func (e *BonEncoder) GetBytes(copy bool) []byte {
	return e.dw.GetBytes(copy)
}

// structToMap 将结构体转换为map
func structToMap(obj interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	val := reflect.ValueOf(obj)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return result
	}

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		// 跳过以$开头的字段和函数类型字段
		if field.Name[0] == '$' || field.Type.Kind() == reflect.Func {
			continue
		}

		fieldValue := val.Field(i).Interface()
		result[field.Name] = fieldValue
	}

	return result
}
