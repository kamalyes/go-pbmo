/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2023-09-26 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2023-09-26 00:00:00
 * @FilePath: \go-pbmo\converter_test.go
 * @Description: 核心双向转换器测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewBidiConverter(t *testing.T) {
	bc := NewBidiConverter(TestPB{}, TestModel{})
	assert.NotNil(t, bc)
	assert.NotNil(t, bc.transformers)
	assert.NotNil(t, bc.validator)
	assert.NotNil(t, bc.options)
}

func TestConvertPBToModel_SimpleFields(t *testing.T) {
	bc := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})

	pb := TestSimplePB{Value: "hello", Count: 42}
	var model TestSimpleModel

	err := bc.ConvertPBToModel(pb, &model)
	assert.NoError(t, err)
	assert.Equal(t, "hello", model.Value)
	assert.Equal(t, int32(42), model.Count)
}

func TestConvertModelToPB_SimpleFields(t *testing.T) {
	bc := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})

	model := TestSimpleModel{Value: "world", Count: 99}
	var pb TestSimplePB

	err := bc.ConvertModelToPB(model, &pb)
	assert.NoError(t, err)
	assert.Equal(t, "world", pb.Value)
	assert.Equal(t, int32(99), pb.Count)
}

func TestConvertPBToModel_WithFieldMapping(t *testing.T) {
	bc := NewBidiConverter(TestPBWithMapping{}, TestModelWithMapping{})

	pb := TestPBWithMapping{ClientId: 1, UserName: "test", UserEmail: "test@example.com"}
	var model TestModelWithMapping

	err := bc.ConvertPBToModel(pb, &model)
	assert.NoError(t, err)
	assert.Equal(t, uint64(1), model.ID)
	assert.Equal(t, "test", model.Name)
	assert.Equal(t, "test@example.com", model.Email)
}

func TestConvertModelToPB_WithFieldMapping(t *testing.T) {
	bc := NewBidiConverter(TestPBWithMapping{}, TestModelWithMapping{})

	model := TestModelWithMapping{ID: 2, Name: "hello", Email: "hello@example.com"}
	var pb TestPBWithMapping

	err := bc.ConvertModelToPB(model, &pb)
	assert.NoError(t, err)
	assert.Equal(t, uint64(2), pb.ClientId)
	assert.Equal(t, "hello", pb.UserName)
	assert.Equal(t, "hello@example.com", pb.UserEmail)
}

func TestConvertPBToModel_TagMapping(t *testing.T) {
	bc := NewBidiConverter(TestPB{}, TestModel{})

	pb := TestPB{Id: 100, Name: "tag_test", Email: "tag@test.com", Age: 25, Score: 95.5, Active: true}
	var model TestModel

	err := bc.ConvertPBToModel(pb, &model)
	assert.NoError(t, err)
	assert.Equal(t, uint64(100), model.ID)
	assert.Equal(t, "tag_test", model.Name)
	assert.Equal(t, "tag@test.com", model.Email)
}

func TestConvertPBToModel_NilPB(t *testing.T) {
	bc := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})
	var model TestSimpleModel

	err := bc.ConvertPBToModel(nil, &model)
	assert.Empty(t, err)
}

func TestConvertPBToModel_NilModel(t *testing.T) {
	bc := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})
	pb := TestSimplePB{Value: "test"}

	err := bc.ConvertPBToModel(pb, nil)
	assert.Empty(t, err)
}

func TestConvertPBToModel_NonPointerModel(t *testing.T) {
	bc := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})
	pb := TestSimplePB{Value: "test"}
	var model TestSimpleModel

	err := bc.ConvertPBToModel(pb, model)
	assert.Error(t, err)
}

func TestConvertModelToPB_NilModel(t *testing.T) {
	bc := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})
	var pb TestSimplePB

	err := bc.ConvertModelToPB(nil, &pb)
	assert.Empty(t, err)
}

func TestConvertModelToPB_NilPB(t *testing.T) {
	bc := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})
	model := TestSimpleModel{Value: "test"}

	err := bc.ConvertModelToPB(model, nil)
	assert.Empty(t, err)
}

func TestConvertPBToModel_WithTransformer(t *testing.T) {
	bc := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})
	bc.RegisterTransformer("Value", func(v interface{}) interface{} {
		return "transformed_" + v.(string)
	})

	pb := TestSimplePB{Value: "original", Count: 10}
	var model TestSimpleModel

	err := bc.ConvertPBToModel(pb, &model)
	assert.NoError(t, err)
	assert.Equal(t, "transformed_original", model.Value)
}

func TestBidiConverter_WithFieldMapping(t *testing.T) {
	bc := NewBidiConverter(TestPBWithMapping{}, TestModelWithMapping{})
	bc.WithFieldMapping("ID", "ClientId")
	bc.WithFieldMapping("Name", "UserName")

	pb := TestPBWithMapping{ClientId: 1, UserName: "mapped"}
	var model TestModelWithMapping

	err := bc.ConvertPBToModel(pb, &model)
	assert.NoError(t, err)
	assert.Equal(t, uint64(1), model.ID)
	assert.Equal(t, "mapped", model.Name)
}

func TestBidiConverter_RegisterFieldMapping(t *testing.T) {
	bc := NewBidiConverter(TestPBWithMapping{}, TestModelWithMapping{})
	bc.RegisterFieldMapping(map[string]string{
		"ID":   "ClientId",
		"Name": "UserName",
	})

	pb := TestPBWithMapping{ClientId: 5, UserName: "batch_mapped"}
	var model TestModelWithMapping

	err := bc.ConvertPBToModel(pb, &model)
	assert.NoError(t, err)
	assert.Equal(t, uint64(5), model.ID)
	assert.Equal(t, "batch_mapped", model.Name)
}

func TestBidiConverter_GetModelType(t *testing.T) {
	bc := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})
	assert.Equal(t, "TestSimpleModel", bc.GetModelType().Name())
}

func TestBidiConverter_GetPBType(t *testing.T) {
	bc := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})
	assert.Equal(t, "TestSimplePB", bc.GetPBType().Name())
}

func TestBidiConverter_GetValidator(t *testing.T) {
	bc := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})
	assert.NotNil(t, bc.GetValidator())
}

func TestBidiConverter_GetTransformers(t *testing.T) {
	bc := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})
	assert.NotNil(t, bc.GetTransformers())
}

func TestConvertPBToModel_SliceField(t *testing.T) {
	bc := NewBidiConverter(TestPB{}, TestModel{})

	pb := TestPB{Id: 1, Name: "slice_test", Tags: []string{"go", "pb"}}
	var model TestModel

	err := bc.ConvertPBToModel(pb, &model)
	assert.NoError(t, err)
	assert.Equal(t, []string{"go", "pb"}, model.Tags)
}

func TestConvertPBToModel_PointerPB(t *testing.T) {
	bc := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})

	pb := &TestSimplePB{Value: "pointer", Count: 7}
	var model TestSimpleModel

	err := bc.ConvertPBToModel(pb, &model)
	assert.NoError(t, err)
	assert.Equal(t, "pointer", model.Value)
	assert.Equal(t, int32(7), model.Count)
}

func TestConvertModelToPB_PointerModel(t *testing.T) {
	bc := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})

	model := &TestSimpleModel{Value: "ptr_model", Count: 8}
	var pb TestSimplePB

	err := bc.ConvertModelToPB(model, &pb)
	assert.NoError(t, err)
	assert.Equal(t, "ptr_model", pb.Value)
	assert.Equal(t, int32(8), pb.Count)
}

func TestBidiConverter_ChainCall(t *testing.T) {
	bc := NewBidiConverter(TestPBWithMapping{}, TestModelWithMapping{}).
		WithFieldMapping("ID", "ClientId").
		WithFieldMapping("Name", "UserName").
		WithAutoTimeConversion(true).
		WithValidation(false).
		WithTagMapping(true)

	pb := TestPBWithMapping{ClientId: 1, UserName: "chain_test", UserEmail: "chain@test.com"}
	var model TestModelWithMapping

	err := bc.ConvertPBToModel(pb, &model)
	assert.NoError(t, err)
	assert.Equal(t, uint64(1), model.ID)
	assert.Equal(t, "chain_test", model.Name)
	assert.Equal(t, "chain@test.com", model.Email)
}

func TestBidiConverter_ChainCall_WithFieldMappings(t *testing.T) {
	bc := NewBidiConverter(TestPBWithMapping{}, TestModelWithMapping{}).
		WithFieldMappings(map[string]string{
			"ID":   "ClientId",
			"Name": "UserName",
		}).
		WithAutoTimeConversion(false)

	pb := TestPBWithMapping{ClientId: 3, UserName: "batch_chain", UserEmail: "batch@chain.com"}
	var model TestModelWithMapping

	err := bc.ConvertPBToModel(pb, &model)
	assert.NoError(t, err)
	assert.Equal(t, uint64(3), model.ID)
	assert.Equal(t, "batch_chain", model.Name)
}

func TestBidiConverter_ChainCall_OptionsModified(t *testing.T) {
	bc := NewBidiConverter(TestSimplePB{}, TestSimpleModel{}).
		WithAutoTimeConversion(false).
		WithValidation(true).
		WithDesensitize(true).
		WithSafeMode(true).
		WithTagName("custom").
		WithConcurrency(4).
		WithBatchSize(50).
		WithTimeout(10 * time.Second)

	assert.False(t, bc.options.AutoTimeConversion)
	assert.True(t, bc.options.ValidationEnabled)
	assert.True(t, bc.options.DesensitizeEnabled)
	assert.True(t, bc.options.SafeMode)
	assert.Equal(t, "custom", bc.options.TagName)
	assert.Equal(t, 4, bc.options.Concurrency)
	assert.Equal(t, 50, bc.options.BatchSize)
	assert.Equal(t, 10*time.Second, bc.options.Timeout)
}

func TestBidiConverter_RegisterTransformer_ChainCall(t *testing.T) {
	bc := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})

	result := bc.RegisterTransformer("Value", func(v interface{}) interface{} {
		return "transformed_" + v.(string)
	})

	assert.NotNil(t, result)
	assert.Same(t, bc, result, "RegisterTransformer should return the same BidiConverter for chaining")
}

func TestBidiConverter_RegisterTransformer_MultipleChain(t *testing.T) {
	bc := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})

	result := bc.
		RegisterTransformer("Value", func(v interface{}) interface{} { return v }).
		RegisterTransformer("Count", func(v interface{}) interface{} { return v })

	assert.NotNil(t, result)
	assert.Same(t, bc, result)
	assert.True(t, bc.transformers.Has("Value"))
	assert.True(t, bc.transformers.Has("Count"))
}

func TestBidiConverter_RegisterTransformer_WithFieldMapping_Chain(t *testing.T) {
	bc := NewBidiConverter(TestPBWithMapping{}, TestModelWithMapping{})

	result := bc.
		WithFieldMapping("ID", "ClientId").
		WithFieldMapping("Name", "UserName").
		RegisterTransformer("Email", func(v interface{}) interface{} {
			return "prefix_" + v.(string)
		})

	assert.NotNil(t, result)
	assert.Same(t, bc, result)
	assert.True(t, bc.transformers.Has("Email"))
}
