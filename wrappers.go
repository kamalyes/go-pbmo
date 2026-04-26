/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-04-26 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-26 00:00:00
 * @FilePath: \go-pbmo\wrappers.go
 * @Description: Protobuf Wrappers 类型自动转换 - *T ↔ *wrapperspb.XxxValue
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
