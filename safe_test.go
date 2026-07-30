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

// TestSafeConverter_SafeBatchConvertPBToModel 覆盖 SafeConverter.SafeBatchConvertPBToModel 各分支
func TestSafeConverter_SafeBatchConvertPBToModel(t *testing.T) {
	sc := NewSafeConverter(TestSimplePB{}, TestSimpleModel{})

	pbs := []TestSimplePB{
		{Value: "a", Count: 1},
		{Value: "b", Count: 2},
		{Value: "c", Count: 3},
	}
	var models []TestSimpleModel

	result := sc.SafeBatchConvertPBToModel(pbs, &models)
	assert.Equal(t, 3, result.SuccessCount)
	assert.Equal(t, 0, result.FailureCount)
	assert.Len(t, models, 3)
	assert.Equal(t, "a", models[0].Value)
	assert.Equal(t, int32(1), models[0].Count)
	assert.Equal(t, "c", models[2].Value)
}

// TestSafeConverter_SafeBatchConvertPBToModel_PtrSlice 覆盖指针切片元素的 nil 分支
func TestSafeConverter_SafeBatchConvertPBToModel_PtrSlice(t *testing.T) {
	sc := NewSafeConverter(TestSimplePB{}, TestSimpleModel{})

	// 指针 PB 切片，含 nil 元素
	pbs := []*TestSimplePB{
		{Value: "ok", Count: 1},
		nil, // 触发 nil 元素分支
	}
	var models []*TestSimpleModel

	result := sc.SafeBatchConvertPBToModel(pbs, &models)
	assert.Equal(t, 1, result.SuccessCount)
	assert.Equal(t, 1, result.FailureCount)
	assert.Len(t, models, 2)
	assert.NotNil(t, models[0])
	assert.Equal(t, "ok", models[0].Value)
	assert.Nil(t, models[1])
}

// TestSafeConverter_SafeBatchConvertPBToModel_InvalidInput 覆盖非切片输入和非指针 models 分支
func TestSafeConverter_SafeBatchConvertPBToModel_InvalidInput(t *testing.T) {
	sc := NewSafeConverter(TestSimplePB{}, TestSimpleModel{})

	// 非切片输入 → 返回空结果
	var models []TestSimpleModel
	result := sc.SafeBatchConvertPBToModel("not a slice", &models)
	assert.Equal(t, 0, result.SuccessCount)
	assert.Equal(t, 0, result.FailureCount)

	// 非指针 models → 返回空结果
	models2 := []TestSimpleModel{}
	result2 := sc.SafeBatchConvertPBToModel([]TestSimplePB{{Value: "x"}}, models2)
	assert.Equal(t, 0, result2.SuccessCount)
	assert.Equal(t, 0, result2.FailureCount)
}

// TestSafeConverter_SafeBatchConvertPBToModel_ZeroElement 覆盖零值元素失败分支
func TestSafeConverter_SafeBatchConvertPBToModel_ZeroElement(t *testing.T) {
	sc := NewSafeConverter(TestSimplePB{}, TestSimpleModel{})

	// 零值元素（Value="" Count=0）触发 IsZero 分支
	pbs := []TestSimplePB{
		{},
	}
	var models []TestSimpleModel

	result := sc.SafeBatchConvertPBToModel(pbs, &models)
	assert.Equal(t, 0, result.SuccessCount)
	assert.Equal(t, 1, result.FailureCount)
}

// TestSafeConvertPBToModel_InvalidPB 覆盖 SafeConvertPBToModel 的 !IsValid 分支
func TestSafeConvertPBToModel_InvalidPB(t *testing.T) {
	sc := NewSafeConverter(TestSimplePB{}, TestSimpleModel{})

	// 传入 typed-nil 指针触发 invalid 分支
	var nilPtr *TestSimplePB
	var model TestSimpleModel
	err := sc.SafeConvertPBToModel(nilPtr, &model)
	assert.Error(t, err)
}

// TestSafeConvertModelToPB_InvalidModel 覆盖 SafeConvertModelToPB 的 !IsValid 分支
func TestSafeConvertModelToPB_InvalidModel(t *testing.T) {
	sc := NewSafeConverter(TestSimplePB{}, TestSimpleModel{})

	var nilPtr *TestSimpleModel
	var pb TestSimplePB
	err := sc.SafeConvertModelToPB(nilPtr, &pb)
	assert.Error(t, err)
}
