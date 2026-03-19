/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2023-09-26 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2023-09-26 00:00:00
 * @FilePath: \go-pbmo\errors.go
 * @Description: 错误定义 - 自包含的错误体系，不依赖外部错误包
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"fmt"

	"github.com/kamalyes/go-toolbox/pkg/errorx"
)

// 错误类型常量
const (
	ErrTypeConversion   errorx.ErrorType = iota + 1 // 转换错误
	ErrTypeValidation                               // 校验错误
	ErrTypeNilValue                                 // 空值错误
	ErrTypeTypeMismatch                             // 类型不匹配
	ErrTypeFieldMapping                             // 字段映射错误
	ErrTypeBatch                                    // 批量操作错误
	ErrTypeDesensitize                              // 脱敏错误
	ErrTypeRegistry                                 // 注册中心错误
)

func init() {
	errorx.RegisterError(ErrTypeConversion, "转换错误: %v")
	errorx.RegisterError(ErrTypeValidation, "校验错误: %v")
	errorx.RegisterError(ErrTypeNilValue, "空值错误: %v")
	errorx.RegisterError(ErrTypeTypeMismatch, "类型不匹配: %v")
	errorx.RegisterError(ErrTypeFieldMapping, "字段映射错误: %v")
	errorx.RegisterError(ErrTypeBatch, "批量操作错误: %v")
	errorx.RegisterError(ErrTypeDesensitize, "脱敏错误: %v")
	errorx.RegisterError(ErrTypeRegistry, "注册中心错误: %v")
}

// 预定义错误变量
var (
	ErrPBMessageNil      = errorx.NewBaseError("PB消息不能为空", ErrTypeNilValue)
	ErrModelNil          = errorx.NewBaseError("Model不能为空", ErrTypeNilValue)
	ErrMustBePointer     = errorx.NewBaseError("目标必须是指针类型", ErrTypeTypeMismatch)
	ErrMustBeSlice       = errorx.NewBaseError("源必须是切片类型", ErrTypeTypeMismatch)
	ErrMustBeStruct      = errorx.NewBaseError("目标必须是结构体类型", ErrTypeTypeMismatch)
	ErrConverterExists   = errorx.NewBaseError("转换器已存在", ErrTypeRegistry)
	ErrConverterNotFound = errorx.NewBaseError("转换器未找到", ErrTypeRegistry)
)

// NewConversionError 创建转换错误
func NewConversionError(format string, args ...interface{}) error {
	return errorx.NewError(ErrTypeConversion, fmt.Sprintf(format, args...))
}

// NewValidationError 创建校验错误
func NewValidationError(format string, args ...interface{}) error {
	return errorx.NewError(ErrTypeValidation, fmt.Sprintf(format, args...))
}

// NewNilValueError 创建空值错误
func NewNilValueError(format string, args ...interface{}) error {
	return errorx.NewError(ErrTypeNilValue, fmt.Sprintf(format, args...))
}

// NewTypeMismatchError 创建类型不匹配错误
func NewTypeMismatchError(format string, args ...interface{}) error {
	return errorx.NewError(ErrTypeTypeMismatch, fmt.Sprintf(format, args...))
}

// NewFieldMappingError 创建字段映射错误
func NewFieldMappingError(format string, args ...interface{}) error {
	return errorx.NewError(ErrTypeFieldMapping, fmt.Sprintf(format, args...))
}

// NewBatchError 创建批量操作错误
func NewBatchError(format string, args ...interface{}) error {
	return errorx.NewError(ErrTypeBatch, fmt.Sprintf(format, args...))
}

// IsConversionError 判断是否为转换错误
func IsConversionError(err error) bool {
	return errorx.ClassifyError(err) == ErrTypeConversion
}

// IsValidationError 判断是否为校验错误
func IsValidationError(err error) bool {
	return errorx.ClassifyError(err) == ErrTypeValidation
}

// IsNilValueError 判断是否为空值错误
func IsNilValueError(err error) bool {
	return errorx.ClassifyError(err) == ErrTypeNilValue
}

// IsTypeMismatchError 判断是否为类型不匹配错误
func IsTypeMismatchError(err error) bool {
	return errorx.ClassifyError(err) == ErrTypeTypeMismatch
}
