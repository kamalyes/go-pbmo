/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-04-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-20 00:00:00
 * @FilePath: \go-pbmo\helpers.go
 * @Description: 辅助函数 - 反射工具、类型判断
 * 复用 go-toolbox/types 的类型约束体系，减少重复定义
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"reflect"

	"github.com/kamalyes/go-toolbox/pkg/types"
)

// IsZeroValue 判断是否为零值
func IsZeroValue(v reflect.Value) bool {
	return v.Interface() == reflect.Zero(v.Type()).Interface()
}

// IsNumericKind 判断 reflect.Kind 是否为数值类型
// 基于 go-toolbox/types.Numerical 约束体系
func IsNumericKind(kind reflect.Kind) bool {
	return IsSignedInt(kind) || IsUnsignedInt(kind) || IsFloatKind(kind)
}

// IsNumeric 判断 reflect.Value 是否为数值类型
func IsNumeric(v reflect.Value) bool {
	return IsNumericKind(v.Kind())
}

// GetNumericValue 获取数值的 float64 表示
func GetNumericValue(v reflect.Value) float64 {
	kind := v.Kind()
	if IsSignedInt(kind) {
		return float64(v.Int())
	}
	if IsUnsignedInt(kind) {
		return float64(v.Uint())
	}
	return v.Float()
}

// IsIntegerType 判断是否为整数类型
// 对应 go-toolbox/types.Integer + types.Unsigned
func IsIntegerType(t reflect.Type) bool {
	return IsSignedInt(t.Kind()) || IsUnsignedInt(t.Kind())
}

// IsSignedInt 判断是否为有符号整数
// 对应 go-toolbox/types.Integer 约束
func IsSignedInt(kind reflect.Kind) bool {
	return kind >= reflect.Int && kind <= reflect.Int64
}

// IsUnsignedInt 判断是否为无符号整数
// 对应 go-toolbox/types.Unsigned 约束
func IsUnsignedInt(kind reflect.Kind) bool {
	return kind >= reflect.Uint && kind <= reflect.Uint64
}

// IsFloatType 判断是否为浮点数类型
// 对应 go-toolbox/types.Float 约束
func IsFloatType(t reflect.Type) bool {
	return IsFloatKind(t.Kind())
}

// IsFloatKind 判断 Kind 是否为浮点数
func IsFloatKind(kind reflect.Kind) bool {
	return kind == reflect.Float32 || kind == reflect.Float64
}

// IsNumericalType 判断是否为数值类型
// 对应 go-toolbox/types.Numerical 约束
func IsNumericalType(t reflect.Type) bool {
	return IsIntegerType(t) || IsFloatType(t)
}

// GetTypeName 获取类型名称
func GetTypeName(t reflect.Type) string {
	if t == nil {
		return "nil"
	}
	if t.Kind() == reflect.Ptr {
		return "*" + t.Elem().Name()
	}
	return t.Name()
}

// DereferenceType 解引用指针类型，返回实际类型
func DereferenceType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}

// DereferenceValue 解引用指针值，返回实际值
func DereferenceValue(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return v
		}
		v = v.Elem()
	}
	return v
}

// IsTypeOf 判断值是否为指定泛型类型
// 利用 go-toolbox/types 约束进行类型检查
func IsTypeOf[T types.Numerical](v reflect.Value) bool {
	var zero T
	return v.Type() == reflect.TypeOf(zero)
}
