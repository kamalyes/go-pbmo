/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2023-09-26 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2023-09-26 00:00:00
 * @FilePath: \go-pbmo\errors_test.go
 * @Description: 错误定义测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPredefinedErrors(t *testing.T) {
	assert.NotNil(t, ErrPBMessageNil)
	assert.NotNil(t, ErrModelNil)
	assert.NotNil(t, ErrMustBePointer)
	assert.NotNil(t, ErrMustBeSlice)
	assert.NotNil(t, ErrMustBeStruct)
	assert.NotNil(t, ErrConverterExists)
	assert.NotNil(t, ErrConverterNotFound)
}

func TestNewConversionError(t *testing.T) {
	err := NewConversionError("字段 %s 转换失败", "Name")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "字段 Name 转换失败")
}

func TestNewValidationError(t *testing.T) {
	err := NewValidationError("字段 %s 不合法", "Age")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "字段 Age 不合法")
}

func TestNewNilValueError(t *testing.T) {
	err := NewNilValueError("PB消息不能为空")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "PB消息不能为空")
}

func TestNewTypeMismatchError(t *testing.T) {
	err := NewTypeMismatchError("期望 %s，得到 %s", "struct", "int")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "期望 struct，得到 int")
}

func TestNewFieldMappingError(t *testing.T) {
	err := NewFieldMappingError("字段 %s 未找到", "ID")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "字段 ID 未找到")
}

func TestNewBatchError(t *testing.T) {
	err := NewBatchError("元素 %d 失败", 3)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "元素 3 失败")
}

func TestIsConversionError(t *testing.T) {
	err := NewConversionError("测试")
	assert.True(t, IsConversionError(err))
	assert.False(t, IsConversionError(NewValidationError("测试")))
}

func TestIsValidationError(t *testing.T) {
	err := NewValidationError("测试")
	assert.True(t, IsValidationError(err))
	assert.False(t, IsValidationError(NewConversionError("测试")))
}

func TestIsNilValueError(t *testing.T) {
	err := NewNilValueError("测试")
	assert.True(t, IsNilValueError(err))
	assert.False(t, IsNilValueError(NewConversionError("测试")))
}

func TestIsTypeMismatchError(t *testing.T) {
	err := NewTypeMismatchError("测试")
	assert.True(t, IsTypeMismatchError(err))
	assert.False(t, IsTypeMismatchError(NewConversionError("测试")))
}
