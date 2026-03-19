/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2023-09-26 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2023-09-26 00:00:00
 * @FilePath: \go-pbmo\safe_test.go
 * @Description: 安全转换器测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSafeConverter(t *testing.T) {
	sc := NewSafeConverter(TestSimplePB{}, TestSimpleModel{})
	assert.NotNil(t, sc)
}

func TestSafeConvertPBToModel(t *testing.T) {
	sc := NewSafeConverter(TestSimplePB{}, TestSimpleModel{})

	pb := TestSimplePB{Value: "safe_test", Count: 42}
	var model TestSimpleModel

	err := sc.SafeConvertPBToModel(pb, &model)
	assert.NoError(t, err)
	assert.Equal(t, "safe_test", model.Value)
}

func TestSafeConvertPBToModel_NilPB(t *testing.T) {
	sc := NewSafeConverter(TestSimplePB{}, TestSimpleModel{})
	var model TestSimpleModel

	err := sc.SafeConvertPBToModel(nil, &model)
	assert.Error(t, err)
}

func TestSafeConvertModelToPB(t *testing.T) {
	sc := NewSafeConverter(TestSimplePB{}, TestSimpleModel{})

	model := TestSimpleModel{Value: "safe_model", Count: 99}
	var pb TestSimplePB

	err := sc.SafeConvertModelToPB(model, &pb)
	assert.NoError(t, err)
	assert.Equal(t, "safe_model", pb.Value)
}

func TestSafeConvertModelToPB_NilModel(t *testing.T) {
	sc := NewSafeConverter(TestSimplePB{}, TestSimpleModel{})
	var pb TestSimplePB

	err := sc.SafeConvertModelToPB(nil, &pb)
	assert.Error(t, err)
}

func TestSafeGetField(t *testing.T) {
	pb := TestSimplePB{Value: "field_test", Count: 7}

	sa := SafeGetField(pb, "Value")
	assert.True(t, sa.IsValid())
}

func TestSafeGetNestedField(t *testing.T) {
	pb := TestSimplePB{Value: "nested_test", Count: 3}

	sa := SafeGetNestedField(pb, "Value")
	assert.True(t, sa.IsValid())
}

func TestSafeFieldAccess(t *testing.T) {
	sc := NewSafeConverter(TestSimplePB{}, TestSimpleModel{})

	pb := TestSimplePB{Value: "access_test", Count: 5}
	sa := sc.SafeFieldAccess(pb, "Value")
	assert.True(t, sa.IsValid())
}
