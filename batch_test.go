/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-04-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-20 00:00:00
 * @FilePath: \go-pbmo\batch_test.go
 * @Description: 批量转换测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestBatchConvertPBToModel 测试批量 PB -> Model 转换
func TestBatchConvertPBToModel(t *testing.T) {
	bc := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})

	pbs := []TestSimplePB{
		{Value: "first", Count: 1},
		{Value: "second", Count: 2},
		{Value: "third", Count: 3},
	}
	var models []TestSimpleModel

	err := bc.BatchConvertPBToModel(pbs, &models)
	assert.NoError(t, err)
	assert.Len(t, models, 3)
	assert.Equal(t, "first", models[0].Value)
	assert.Equal(t, int32(2), models[1].Count)
}

// TestBatchConvertModelToPB 测试批量 Model -> PB 转换
func TestBatchConvertModelToPB(t *testing.T) {
	bc := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})

	models := []TestSimpleModel{
		{Value: "m1", Count: 10},
		{Value: "m2", Count: 20},
	}
	var pbs []TestSimplePB

	err := bc.BatchConvertModelToPB(models, &pbs)
	assert.NoError(t, err)
	assert.Len(t, pbs, 2)
	assert.Equal(t, "m1", pbs[0].Value)
	assert.Equal(t, int32(20), pbs[1].Count)
}

// TestBatchConvertPBToModel_PointerSlice 测试指针切片的批量转换
func TestBatchConvertPBToModel_PointerSlice(t *testing.T) {
	bc := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})

	pbs := []*TestSimplePB{
		{Value: "ptr1", Count: 11},
		{Value: "ptr2", Count: 22},
	}
	var models []*TestSimpleModel

	err := bc.BatchConvertPBToModel(pbs, &models)
	assert.NoError(t, err)
	assert.Len(t, models, 2)
	assert.Equal(t, "ptr1", models[0].Value)
	assert.Equal(t, int32(22), models[1].Count)
}

// TestBatchConvert_NonSlice 测试传入非切片类型时返回错误
func TestBatchConvert_NonSlice(t *testing.T) {
	bc := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})

	notSlice := TestSimplePB{Value: "not_slice"}
	var models []TestSimpleModel

	err := bc.BatchConvertPBToModel(notSlice, &models)
	assert.Error(t, err)
}

// TestBatchConvert_NonPointerTarget 测试目标参数非指针时返回错误
func TestBatchConvert_NonPointerTarget(t *testing.T) {
	bc := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})

	pbs := []TestSimplePB{{Value: "test"}}
	var models []TestSimpleModel

	err := bc.BatchConvertPBToModel(pbs, models)
	assert.Error(t, err)
}

// TestSafeBatchConvertPBToModel 测试安全批量 PB -> Model 转换
func TestSafeBatchConvertPBToModel(t *testing.T) {
	bc := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})

	pbs := []TestSimplePB{
		{Value: "ok1", Count: 1},
		{Value: "ok2", Count: 2},
	}
	var models []TestSimpleModel

	result := bc.SafeBatchConvertPBToModel(pbs, &models)
	assert.Equal(t, 2, result.SuccessCount)
	assert.Equal(t, 0, result.FailureCount)
	assert.Len(t, models, 2)
}

// TestSafeBatchConvertModelToPB 测试安全批量 Model -> PB 转换
func TestSafeBatchConvertModelToPB(t *testing.T) {
	bc := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})

	models := []TestSimpleModel{
		{Value: "safe1", Count: 100},
		{Value: "safe2", Count: 200},
	}
	var pbs []TestSimplePB

	result := bc.SafeBatchConvertModelToPB(models, &pbs)
	assert.Equal(t, 2, result.SuccessCount)
	assert.Equal(t, 0, result.FailureCount)
	assert.Len(t, pbs, 2)
}

// TestBatchResult 测试批量转换结果结构体
func TestBatchResult(t *testing.T) {
	result := &BatchResult{
		Results: make([]BatchItem, 0),
	}

	item1 := BatchItem{Index: 0, Success: true, Value: "v1"}
	item2 := BatchItem{Index: 1, Success: false, Error: NewConversionError("失败")}

	result.Results = append(result.Results, item1, item2)
	result.SuccessCount = 1
	result.FailureCount = 1

	assert.Equal(t, 1, result.SuccessCount)
	assert.Equal(t, 1, result.FailureCount)
	assert.Len(t, result.Results, 2)
	assert.True(t, result.Results[0].Success)
	assert.False(t, result.Results[1].Success)
}
