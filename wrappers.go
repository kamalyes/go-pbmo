/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-04-26 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-26 00:00:00
 * @FilePath: \go-pbmo\wrappers.go
 * @Description: Protobuf Wrappers 类型自动转换 - *T ↔ *wrapperspb.XxxValue
 * 同时提供公开的 DerefXxxVal / XxxValuePtr / PtrToXxxValue / XxxValueToPtr 辅助函数
 * 在 convertField 中自动识别并转换，无需手动编写辅助函数
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"reflect"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

// wrapperspb.XxxValue 指针类型变量，用于反射类型匹配
var (
	int32ValuePtrType  = reflect.TypeOf((*wrapperspb.Int32Value)(nil))  // *wrapperspb.Int32Value 类型
	int64ValuePtrType  = reflect.TypeOf((*wrapperspb.Int64Value)(nil))  // *wrapperspb.Int64Value 类型
	uint32ValuePtrType = reflect.TypeOf((*wrapperspb.UInt32Value)(nil)) // *wrapperspb.UInt32Value 类型
	uint64ValuePtrType = reflect.TypeOf((*wrapperspb.UInt64Value)(nil)) // *wrapperspb.UInt64Value 类型
	floatValuePtrType  = reflect.TypeOf((*wrapperspb.FloatValue)(nil))  // *wrapperspb.FloatValue 类型
	doubleValuePtrType = reflect.TypeOf((*wrapperspb.DoubleValue)(nil)) // *wrapperspb.DoubleValue 类型
	boolValuePtrType   = reflect.TypeOf((*wrapperspb.BoolValue)(nil))   // *wrapperspb.BoolValue 类型
	stringValuePtrType = reflect.TypeOf((*wrapperspb.StringValue)(nil)) // *wrapperspb.StringValue 类型
)

// Go 基本类型指针类型变量，用于反射类型匹配
var (
	int32PtrType   = reflect.TypeOf((*int32)(nil))   // *int32 类型
	int64PtrType   = reflect.TypeOf((*int64)(nil))   // *int64 类型
	uint32PtrType  = reflect.TypeOf((*uint32)(nil))  // *uint32 类型
	uint64PtrType  = reflect.TypeOf((*uint64)(nil))  // *uint64 类型
	float32PtrType = reflect.TypeOf((*float32)(nil)) // *float32 类型
	float64PtrType = reflect.TypeOf((*float64)(nil)) // *float64 类型
	boolPtrType    = reflect.TypeOf((*bool)(nil))    // *bool 类型
	stringPtrType  = reflect.TypeOf((*string)(nil))  // *string 类型
)

// wrapperConverter 单个 Wrapper 类型的转换器
// wrapperType: *wrapperspb.XxxValue 的反射类型
// ptrType: *T（Go 基本类型指针）的反射类型
// wrap: 将 *T 包装为 *wrapperspb.XxxValue（Model → PB 方向）
// unwrap: 将 *wrapperspb.XxxValue 解包为 *T（PB → Model 方向）
type wrapperConverter struct {
	wrapperType reflect.Type                      // *wrapperspb.XxxValue 反射类型
	ptrType     reflect.Type                      // *T Go 基本类型指针反射类型
	wrap        func(reflect.Value) reflect.Value // *T → *wrapperspb.XxxValue 包装函数
	unwrap      func(reflect.Value) reflect.Value // *wrapperspb.XxxValue → *T 解包函数
}

// wrapperConverters 所有 Wrapper 类型的转换器列表，在 init 中初始化
var wrapperConverters []wrapperConverter

// init 注册所有 *T ↔ *wrapperspb.XxxValue 的转换器
// 每个转换器包含 wrap（*T → *wrapperspb.XxxValue）和 unwrap（*wrapperspb.XxxValue → *T）两个方向
// 当源值为 nil 指针时，wrap/unwrap 返回对应类型的零值（nil 指针）
func init() {
	wrapperConverters = []wrapperConverter{
		{ // *int32 ↔ *wrapperspb.Int32Value
			wrapperType: int32ValuePtrType,
			ptrType:     int32PtrType,
			wrap: func(v reflect.Value) reflect.Value {
				if v.IsNil() {
					return reflect.Zero(int32ValuePtrType)
				}
				return reflect.ValueOf(wrapperspb.Int32(*v.Interface().(*int32)))
			},
			unwrap: func(v reflect.Value) reflect.Value {
				if v.IsNil() {
					return reflect.Zero(int32PtrType)
				}
				val := v.Interface().(*wrapperspb.Int32Value).Value
				return reflect.ValueOf(&val)
			},
		},
		{ // *int64 ↔ *wrapperspb.Int64Value
			wrapperType: int64ValuePtrType,
			ptrType:     int64PtrType,
			wrap: func(v reflect.Value) reflect.Value {
				if v.IsNil() {
					return reflect.Zero(int64ValuePtrType)
				}
				return reflect.ValueOf(wrapperspb.Int64(*v.Interface().(*int64)))
			},
			unwrap: func(v reflect.Value) reflect.Value {
				if v.IsNil() {
					return reflect.Zero(int64PtrType)
				}
				val := v.Interface().(*wrapperspb.Int64Value).Value
				return reflect.ValueOf(&val)
			},
		},
		{ // *uint32 ↔ *wrapperspb.UInt32Value
			wrapperType: uint32ValuePtrType,
			ptrType:     uint32PtrType,
			wrap: func(v reflect.Value) reflect.Value {
				if v.IsNil() {
					return reflect.Zero(uint32ValuePtrType)
				}
				return reflect.ValueOf(wrapperspb.UInt32(*v.Interface().(*uint32)))
			},
			unwrap: func(v reflect.Value) reflect.Value {
				if v.IsNil() {
					return reflect.Zero(uint32PtrType)
				}
				val := v.Interface().(*wrapperspb.UInt32Value).Value
				return reflect.ValueOf(&val)
			},
		},
		{ // *uint64 ↔ *wrapperspb.UInt64Value
			wrapperType: uint64ValuePtrType,
			ptrType:     uint64PtrType,
			wrap: func(v reflect.Value) reflect.Value {
				if v.IsNil() {
					return reflect.Zero(uint64ValuePtrType)
				}
				return reflect.ValueOf(wrapperspb.UInt64(*v.Interface().(*uint64)))
			},
			unwrap: func(v reflect.Value) reflect.Value {
				if v.IsNil() {
					return reflect.Zero(uint64PtrType)
				}
				val := v.Interface().(*wrapperspb.UInt64Value).Value
				return reflect.ValueOf(&val)
			},
		},
		{ // *float32 ↔ *wrapperspb.FloatValue
			wrapperType: floatValuePtrType,
			ptrType:     float32PtrType,
			wrap: func(v reflect.Value) reflect.Value {
				if v.IsNil() {
					return reflect.Zero(floatValuePtrType)
				}
				return reflect.ValueOf(wrapperspb.Float(*v.Interface().(*float32)))
			},
			unwrap: func(v reflect.Value) reflect.Value {
				if v.IsNil() {
					return reflect.Zero(float32PtrType)
				}
				val := v.Interface().(*wrapperspb.FloatValue).Value
				return reflect.ValueOf(&val)
			},
		},
		{ // *float64 ↔ *wrapperspb.DoubleValue
			wrapperType: doubleValuePtrType,
			ptrType:     float64PtrType,
			wrap: func(v reflect.Value) reflect.Value {
				if v.IsNil() {
					return reflect.Zero(doubleValuePtrType)
				}
				return reflect.ValueOf(wrapperspb.Double(*v.Interface().(*float64)))
			},
			unwrap: func(v reflect.Value) reflect.Value {
				if v.IsNil() {
					return reflect.Zero(float64PtrType)
				}
				val := v.Interface().(*wrapperspb.DoubleValue).Value
				return reflect.ValueOf(&val)
			},
		},
		{ // *bool ↔ *wrapperspb.BoolValue
			wrapperType: boolValuePtrType,
			ptrType:     boolPtrType,
			wrap: func(v reflect.Value) reflect.Value {
				if v.IsNil() {
					return reflect.Zero(boolValuePtrType)
				}
				return reflect.ValueOf(wrapperspb.Bool(*v.Interface().(*bool)))
			},
			unwrap: func(v reflect.Value) reflect.Value {
				if v.IsNil() {
					return reflect.Zero(boolPtrType)
				}
				val := v.Interface().(*wrapperspb.BoolValue).Value
				return reflect.ValueOf(&val)
			},
		},
		{ // *string ↔ *wrapperspb.StringValue
			wrapperType: stringValuePtrType,
			ptrType:     stringPtrType,
			wrap: func(v reflect.Value) reflect.Value {
				if v.IsNil() {
					return reflect.Zero(stringValuePtrType)
				}
				return reflect.ValueOf(wrapperspb.String(*v.Interface().(*string)))
			},
			unwrap: func(v reflect.Value) reflect.Value {
				if v.IsNil() {
					return reflect.Zero(stringPtrType)
				}
				val := v.Interface().(*wrapperspb.StringValue).Value
				return reflect.ValueOf(&val)
			},
		},
	}
}

// tryConvertWrapper 尝试在 *T 和 *wrapperspb.XxxValue 之间自动转换
// 遍历所有已注册的 wrapperConverter，匹配源类型和目标类型：
//   - srcType == *T 且 dstType == *wrapperspb.XxxValue → 调用 wrap（Model → PB）
//   - srcType == *wrapperspb.XxxValue 且 dstType == *T → 调用 unwrap（PB → Model）
//
// 返回值：
//   - handled: 是否已处理该转换（true 表示匹配成功并完成转换）
//   - error: 转换过程中的错误
func tryConvertWrapper(src, dst reflect.Value) (bool, error) {
	srcType := src.Type()
	dstType := dst.Type()

	for _, wc := range wrapperConverters {
		// *T → *wrapperspb.XxxValue（Model → PB 方向）
		if srcType == wc.ptrType && dstType == wc.wrapperType {
			result := wc.wrap(src)
			if result.IsValid() {
				dst.Set(result)
			}
			return true, nil
		}
		// *wrapperspb.XxxValue → *T（PB → Model 方向）
		if srcType == wc.wrapperType && dstType == wc.ptrType {
			result := wc.unwrap(src)
			if result.IsValid() {
				dst.Set(result)
			}
			return true, nil
		}
	}

	return false, nil
}

// DerefInt32Val 从 *wrapperspb.Int32Value 中提取整数值
// 当源值为 nil 指针时，DerefInt32Val 返回 0
func DerefInt32Val(v *wrapperspb.Int32Value) int32 {
	if v == nil {
		return 0
	}
	return v.Value
}

// DerefInt64Val 从 *wrapperspb.Int64Value 中提取整数值
// 当源值为 nil 指针时，DerefInt64Val 返回 0
func DerefInt64Val(v *wrapperspb.Int64Value) int64 {
	if v == nil {
		return 0
	}
	return v.Value
}

// DerefUInt32Val 从 *wrapperspb.UInt32Value 中提取无符号整数值
// 当源值为 nil 指针时，DerefUInt32Val 返回 0
func DerefUInt32Val(v *wrapperspb.UInt32Value) uint32 {
	if v == nil {
		return 0
	}
	return v.Value
}

// DerefUInt64Val 从 *wrapperspb.UInt64Value 中提取无符号整数值
// 当源值为 nil 指针时，DerefUInt64Val 返回 0
func DerefUInt64Val(v *wrapperspb.UInt64Value) uint64 {
	if v == nil {
		return 0
	}
	return v.Value
}

// DerefFloatVal 从 *wrapperspb.FloatValue 中提取浮点数值
// 当源值为 nil 指针时，DerefFloatVal 返回 0.0
func DerefFloatVal(v *wrapperspb.FloatValue) float32 {
	if v == nil {
		return 0
	}
	return v.Value
}

// DerefDoubleVal 从 *wrapperspb.DoubleValue 中提取浮点数值
// 当源值为 nil 指针时，DerefDoubleVal 返回 0.0
func DerefDoubleVal(v *wrapperspb.DoubleValue) float64 {
	if v == nil {
		return 0
	}
	return v.Value
}

// DerefBoolVal 从 *wrapperspb.BoolValue 中提取布尔值
// 当源值为 nil 指针时，DerefBoolVal 返回 false
func DerefBoolVal(v *wrapperspb.BoolValue) bool {
	if v == nil {
		return false
	}
	return v.Value
}

// DerefStringVal 从 *wrapperspb.StringValue 中提取字符串值
// 当源值为 nil 指针时，DerefStringVal 返回空字符串 ""
func DerefStringVal(v *wrapperspb.StringValue) string {
	if v == nil {
		return ""
	}
	return v.Value
}

// DerefSlice 将 []*T 转换为 []T，nil 元素转为零值
func DerefSlice[T any](ptrs []*T) []T {
	result := make([]T, len(ptrs))
	for i, p := range ptrs {
		if p != nil {
			result[i] = *p
		}
	}
	return result
}

// Int32ValueToPtr 将 *wrapperspb.Int32Value 转换为 *int32
// 当源值为 nil 指针时，Int32ValueToPtr 返回 nil 指针
func Int32ValueToPtr(v *wrapperspb.Int32Value) *int32 {
	if v == nil {
		return nil
	}
	val := v.Value
	return &val
}

// Int64ValueToPtr 将 *wrapperspb.Int64Value 转换为 *int64
// 当源值为 nil 指针时，Int64ValueToPtr 返回 nil 指针
func Int64ValueToPtr(v *wrapperspb.Int64Value) *int64 {
	if v == nil {
		return nil
	}
	val := v.Value
	return &val
}

// UInt32ValueToPtr 将 *wrapperspb.UInt32Value 转换为 *uint32
// 当源值为 nil 指针时，UInt32ValueToPtr 返回 nil 指针
func UInt32ValueToPtr(v *wrapperspb.UInt32Value) *uint32 {
	if v == nil {
		return nil
	}
	val := v.Value
	return &val
}

// UInt64ValueToPtr 将 *wrapperspb.UInt64Value 转换为 *uint64
// 当源值为 nil 指针时，UInt64ValueToPtr 返回 nil 指针
func UInt64ValueToPtr(v *wrapperspb.UInt64Value) *uint64 {
	if v == nil {
		return nil
	}
	val := v.Value
	return &val
}

// FloatValueToPtr 将 *wrapperspb.FloatValue 转换为 *float32
// 当源值为 nil 指针时，FloatValueToPtr 返回 nil 指针
func FloatValueToPtr(v *wrapperspb.FloatValue) *float32 {
	if v == nil {
		return nil
	}
	val := v.Value
	return &val
}

// DoubleValueToPtr 将 *wrapperspb.DoubleValue 转换为 *float64
// 当源值为 nil 指针时，DoubleValueToPtr 返回 nil 指针
func DoubleValueToPtr(v *wrapperspb.DoubleValue) *float64 {
	if v == nil {
		return nil
	}
	val := v.Value
	return &val
}

// BoolValueToPtr 将 *wrapperspb.BoolValue 转换为 *bool
// 当源值为 nil 指针时，BoolValueToPtr 返回 nil 指针
func BoolValueToPtr(v *wrapperspb.BoolValue) *bool {
	if v == nil {
		return nil
	}
	val := v.Value
	return &val
}

// StringValueToPtr 将 *wrapperspb.StringValue 转换为 *string
// 当源值为 nil 指针时，StringValueToPtr 返回 nil 指针
func StringValueToPtr(v *wrapperspb.StringValue) *string {
	if v == nil {
		return nil
	}
	val := v.Value
	return &val
}

// PtrToInt32Value 将 *int32 转换为 *wrapperspb.Int32Value
// 当源值为 nil 指针时，PtrToInt32Value 返回 nil 指针
func PtrToInt32Value(v *int32) *wrapperspb.Int32Value {
	if v == nil {
		return nil
	}
	return wrapperspb.Int32(*v)
}

// PtrToInt64Value 将 *int64 转换为 *wrapperspb.Int64Value
// 当源值为 nil 指针时，PtrToInt64Value 返回 nil 指针
func PtrToInt64Value(v *int64) *wrapperspb.Int64Value {
	if v == nil {
		return nil
	}
	return wrapperspb.Int64(*v)
}

// PtrToUInt32Value 将 *uint32 转换为 *wrapperspb.UInt32Value
// 当源值为 nil 指针时，PtrToUInt32Value 返回 nil 指针
func PtrToUInt32Value(v *uint32) *wrapperspb.UInt32Value {
	if v == nil {
		return nil
	}
	return wrapperspb.UInt32(*v)
}

// PtrToUInt64Value 将 *uint64 转换为 *wrapperspb.UInt64Value
// 当源值为 nil 指针时，PtrToUInt64Value 返回 nil 指针
func PtrToUInt64Value(v *uint64) *wrapperspb.UInt64Value {
	if v == nil {
		return nil
	}
	return wrapperspb.UInt64(*v)
}

// PtrToFloatValue 将 *float32 转换为 *wrapperspb.FloatValue
// 当源值为 nil 指针时，PtrToFloatValue 返回 nil 指针
func PtrToFloatValue(v *float32) *wrapperspb.FloatValue {
	if v == nil {
		return nil
	}
	return wrapperspb.Float(*v)
}

// PtrToDoubleValue 将 *float64 转换为 *wrapperspb.DoubleValue
// 当源值为 nil 指针时，PtrToDoubleValue 返回 nil 指针
func PtrToDoubleValue(v *float64) *wrapperspb.DoubleValue {
	if v == nil {
		return nil
	}
	return wrapperspb.Double(*v)
}

// PtrToBoolValue 将 *bool 转换为 *wrapperspb.BoolValue
// 当源值为 nil 指针时，PtrToBoolValue 返回 nil 指针
func PtrToBoolValue(v *bool) *wrapperspb.BoolValue {
	if v == nil {
		return nil
	}
	return wrapperspb.Bool(*v)
}

// PtrToStringValue 将 *string 转换为 *wrapperspb.StringValue
// 当源值为 nil 指针时，PtrToStringValue 返回 nil 指针
func PtrToStringValue(v *string) *wrapperspb.StringValue {
	if v == nil {
		return nil
	}
	return wrapperspb.String(*v)
}
