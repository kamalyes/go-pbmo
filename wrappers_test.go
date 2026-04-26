/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-04-26 00:00:00
 * @LastEditors: kamalyes 501893667@qq.com
 * @LastEditTime: 2026-04-26 00:00:00
 * @FilePath: \go-pbmo\wrappers_test.go
 * @Description: Wrappers 类型自动转换测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type WrapperTestModel struct {
	IntVal    *int32
	Int64Val  *int64
	StrVal    *string
	BoolVal   *bool
	FloatVal  *float64
	Uint32Val *uint32
}

type WrapperTestPB struct {
	IntVal    *wrapperspb.Int32Value
	Int64Val  *wrapperspb.Int64Value
	StrVal    *wrapperspb.StringValue
	BoolVal   *wrapperspb.BoolValue
	FloatVal  *wrapperspb.DoubleValue
	Uint32Val *wrapperspb.UInt32Value
}

func TestConvertField_WrapperInt32(t *testing.T) {
	v := int32(42)
	model := WrapperTestModel{IntVal: &v}
	var pb WrapperTestPB
	err := getOrInitConverter[WrapperTestModel, WrapperTestPB]().ConvertModelToPB(model, &pb)
	assert.NoError(t, err)
	assert.NotNil(t, pb.IntVal)
	assert.Equal(t, int32(42), pb.IntVal.Value)
}

func TestConvertField_WrapperInt32_Nil(t *testing.T) {
	model := WrapperTestModel{IntVal: nil}
	var pb WrapperTestPB
	err := getOrInitConverter[WrapperTestModel, WrapperTestPB]().ConvertModelToPB(model, &pb)
	assert.NoError(t, err)
	assert.Nil(t, pb.IntVal)
}

func TestConvertField_WrapperString(t *testing.T) {
	v := "hello"
	model := WrapperTestModel{StrVal: &v}
	var pb WrapperTestPB
	err := getOrInitConverter[WrapperTestModel, WrapperTestPB]().ConvertModelToPB(model, &pb)
	assert.NoError(t, err)
	assert.NotNil(t, pb.StrVal)
	assert.Equal(t, "hello", pb.StrVal.Value)
}

func TestConvertField_WrapperBool(t *testing.T) {
	v := true
	model := WrapperTestModel{BoolVal: &v}
	var pb WrapperTestPB
	err := getOrInitConverter[WrapperTestModel, WrapperTestPB]().ConvertModelToPB(model, &pb)
	assert.NoError(t, err)
	assert.NotNil(t, pb.BoolVal)
	assert.True(t, pb.BoolVal.Value)
}

func TestConvertField_WrapperDouble(t *testing.T) {
	v := 3.14
	model := WrapperTestModel{FloatVal: &v}
	var pb WrapperTestPB
	err := getOrInitConverter[WrapperTestModel, WrapperTestPB]().ConvertModelToPB(model, &pb)
	assert.NoError(t, err)
	assert.NotNil(t, pb.FloatVal)
	assert.Equal(t, 3.14, pb.FloatVal.Value)
}

func TestConvertField_WrapperUInt32(t *testing.T) {
	v := uint32(99)
	model := WrapperTestModel{Uint32Val: &v}
	var pb WrapperTestPB
	err := getOrInitConverter[WrapperTestModel, WrapperTestPB]().ConvertModelToPB(model, &pb)
	assert.NoError(t, err)
	assert.NotNil(t, pb.Uint32Val)
	assert.Equal(t, uint32(99), pb.Uint32Val.Value)
}

func TestConvertField_WrapperReverse_Int32(t *testing.T) {
	pb := WrapperTestPB{IntVal: wrapperspb.Int32(42)}
	var model WrapperTestModel
	err := getOrInitConverter[WrapperTestModel, WrapperTestPB]().ConvertPBToModel(pb, &model)
	assert.NoError(t, err)
	assert.NotNil(t, model.IntVal)
	assert.Equal(t, int32(42), *model.IntVal)
}

func TestConvertField_WrapperReverse_Int32_Nil(t *testing.T) {
	pb := WrapperTestPB{IntVal: nil}
	var model WrapperTestModel
	err := getOrInitConverter[WrapperTestModel, WrapperTestPB]().ConvertPBToModel(pb, &model)
	assert.NoError(t, err)
	assert.Nil(t, model.IntVal)
}

func TestConvertField_WrapperReverse_String(t *testing.T) {
	pb := WrapperTestPB{StrVal: wrapperspb.String("world")}
	var model WrapperTestModel
	err := getOrInitConverter[WrapperTestModel, WrapperTestPB]().ConvertPBToModel(pb, &model)
	assert.NoError(t, err)
	assert.NotNil(t, model.StrVal)
	assert.Equal(t, "world", *model.StrVal)
}

func TestConvertField_WrapperReverse_Bool(t *testing.T) {
	pb := WrapperTestPB{BoolVal: wrapperspb.Bool(false)}
	var model WrapperTestModel
	err := getOrInitConverter[WrapperTestModel, WrapperTestPB]().ConvertPBToModel(pb, &model)
	assert.NoError(t, err)
	assert.NotNil(t, model.BoolVal)
	assert.False(t, *model.BoolVal)
}

func TestConvertField_WrapperAllFields(t *testing.T) {
	i32 := int32(1)
	i64 := int64(2)
	s := "test"
	b := true
	f := 3.14
	u32 := uint32(5)

	model := WrapperTestModel{
		IntVal:    &i32,
		Int64Val:  &i64,
		StrVal:    &s,
		BoolVal:   &b,
		FloatVal:  &f,
		Uint32Val: &u32,
	}

	var pb WrapperTestPB
	err := getOrInitConverter[WrapperTestModel, WrapperTestPB]().ConvertModelToPB(model, &pb)
	assert.NoError(t, err)
	assert.Equal(t, int32(1), pb.IntVal.GetValue())
	assert.Equal(t, int64(2), pb.Int64Val.GetValue())
	assert.Equal(t, "test", pb.StrVal.GetValue())
	assert.True(t, pb.BoolVal.GetValue())
	assert.Equal(t, 3.14, pb.FloatVal.GetValue())
	assert.Equal(t, uint32(5), pb.Uint32Val.GetValue())

	var model2 WrapperTestModel
	err = getOrInitConverter[WrapperTestModel, WrapperTestPB]().ConvertPBToModel(pb, &model2)
	assert.NoError(t, err)
	assert.Equal(t, int32(1), *model2.IntVal)
	assert.Equal(t, int64(2), *model2.Int64Val)
	assert.Equal(t, "test", *model2.StrVal)
	assert.True(t, *model2.BoolVal)
	assert.Equal(t, 3.14, *model2.FloatVal)
	assert.Equal(t, uint32(5), *model2.Uint32Val)
}

func TestToPB_NilInput(t *testing.T) {
	result, err := ToPB[WrapperTestModel, WrapperTestPB](nil)
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestFromPB_NilInput(t *testing.T) {
	result, err := FromPB[WrapperTestPB, WrapperTestModel](nil)
	assert.NoError(t, err)
	assert.Nil(t, result)
}
