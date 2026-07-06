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
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
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

// TestFieldMapping_NoPbmoTag 测试 Model 无 pbmo tag 时 WithFieldMapping 是否生效
// 回归测试：修复 hasPbmoTag 推断方向失败导致 FieldMapping 被跳过的 bug
func TestFieldMapping_NoPbmoTag(t *testing.T) {
	bc := NewBidiConverter(TestPBNoPbmoTag{}, TestModelNoPbmoTag{})
	bc.WithFieldMapping("ConfigKey", "Key")

	t.Run("ModelToPB", func(t *testing.T) {
		model := TestModelNoPbmoTag{ConfigKey: 42, Name: "test"}
		var pb TestPBNoPbmoTag
		err := bc.ConvertModelToPB(model, &pb)
		assert.NoError(t, err)
		assert.Equal(t, int32(42), pb.Key, "ConfigKey 应通过 FieldMapping 映射到 Key")
		assert.Equal(t, "test", pb.Name)
	})

	t.Run("PBToModel", func(t *testing.T) {
		pb := TestPBNoPbmoTag{Key: 99, Name: "hello"}
		var model TestModelNoPbmoTag
		err := bc.ConvertPBToModel(pb, &model)
		assert.NoError(t, err)
		assert.Equal(t, 99, model.ConfigKey, "Key 应通过反向 FieldMapping 映射到 ConfigKey")
		assert.Equal(t, "hello", model.Name)
	})
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

func TestConvertSlice_ValuePointerElements(t *testing.T) {
	type valueSlicePB struct {
		Name   string
		Counts []int32
	}
	type ptrSliceModel struct {
		Name   string
		Counts []*int32
	}

	bc := NewBidiConverter(valueSlicePB{}, ptrSliceModel{})

	pb := valueSlicePB{Name: "offset", Counts: []int32{1, 0, 3}}
	var model ptrSliceModel
	assert.NoError(t, bc.ConvertPBToModel(pb, &model))
	assert.Equal(t, "offset", model.Name)
	if assert.Len(t, model.Counts, 3) {
		if assert.NotNil(t, model.Counts[0]) {
			assert.Equal(t, int32(1), *model.Counts[0])
		}
		assert.Nil(t, model.Counts[1])
		if assert.NotNil(t, model.Counts[2]) {
			assert.Equal(t, int32(3), *model.Counts[2])
		}
	}

	count := int32(7)
	model = ptrSliceModel{Name: "back", Counts: []*int32{&count, nil}}
	var pbBack valueSlicePB
	assert.NoError(t, bc.ConvertModelToPB(model, &pbBack))
	assert.Equal(t, valueSlicePB{Name: "back", Counts: []int32{7, 0}}, pbBack)
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

// TestIsProtoMessage 验证 isProtoMessage 能正确识别真实 PB 类型
func TestIsProtoMessage(t *testing.T) {
	// 真实 PB 类型应被识别（实现 proto.Message 接口）
	assert.True(t, isProtoMessage(reflect.TypeFor[wrapperspb.BoolValue]()), "wrapperspb.BoolValue 应被识别为 PB 类型")
	assert.True(t, isProtoMessage(reflect.TypeFor[wrapperspb.StringValue]()), "wrapperspb.StringValue 应被识别为 PB 类型")
	assert.True(t, isProtoMessage(reflect.TypeFor[wrapperspb.Int32Value]()), "wrapperspb.Int32Value 应被识别为 PB 类型")

	// 普通 struct 不应被识别（未实现 ProtoReflect 方法）
	assert.False(t, isProtoMessage(reflect.TypeFor[TestPB]()), "TestPB 不应被识别为 PB 类型")
	assert.False(t, isProtoMessage(reflect.TypeFor[TestModel]()), "TestModel 不应被识别为 PB 类型")
	assert.False(t, isProtoMessage(nil), "nil 不应被识别为 PB 类型")
}

// TestPBToPBSkipJsonTag 验证 PB↔PB 转换时跳过 json tag
// 用同类型 wrapperspb.StringValue → wrapperspb.StringValue，
// 两者 Value 字段（string）类型兼容，应能匹配
// 关键点：跳过 json tag 后，仍能通过 Go 字段名 "Value" 匹配，
// 而不依赖 json tag "value,omitempty" + EqualFold fallback
func TestPBToPBSkipJsonTag(t *testing.T) {
	c := Register[wrapperspb.StringValue, wrapperspb.StringValue]()
	cache := c.modelToPBFieldCache()

	// 应该能匹配到 Value 字段
	assert.NotEmpty(t, cache.fastEntries, "Value 字段应被匹配")
	for _, entry := range cache.fastEntries {
		assert.Equal(t, "Value", entry.srcName, "源字段名应为 Value（Go 字段名，非 json tag）")
		assert.Equal(t, "Value", entry.dstName, "目标字段名应为 Value（Go 字段名，非 json tag）")
	}
}

// TestPBToPBFieldMappingWithDifferentNames 验证 PB↔PB 字段名不同时通过 FieldMapping 匹配
// 模拟 opengamepb.ProviderInfo → commonpb.PaasGameProviderInfo 的场景：
// ProviderId (源) → Id (目标)，需要显式 FieldMapping
func TestPBToPBFieldMappingWithDifferentNames(t *testing.T) {
	// wrapperspb.StringValue 有 Value 字段
	// 我们注册时通过 FieldMapping 把 "Value" 映射到不存在的目标字段，
	// 验证 PB↔PB 时 FieldMapping 仍然生效
	c := Register[wrapperspb.StringValue, wrapperspb.StringValue]().
		WithFieldMapping("Value", "Value")
	cache := c.modelToPBFieldCache()

	assert.NotEmpty(t, cache.fastEntries, "通过 FieldMapping 应能匹配 Value 字段")
}

func TestConvertMapToStruct_ModelToPB(t *testing.T) {
	bc := NewBidiConverter(TestMapStructPB{}, TestMapStructModel{})

	model := TestMapStructModel{
		Name: "test",
		Params: map[string]interface{}{
			"timeout":  float64(30),
			"enabled":  true,
			"host":     "localhost",
			"nested":   map[string]interface{}{"key": "value"},
			"tags":     []interface{}{"a", "b"},
			"null_val": nil,
		},
	}

	var pb TestMapStructPB
	err := bc.ConvertModelToPB(model, &pb)
	assert.NoError(t, err)
	assert.Equal(t, "test", pb.Name)
	assert.NotNil(t, pb.Params)

	m := pb.Params.AsMap()
	assert.Equal(t, float64(30), m["timeout"])
	assert.Equal(t, true, m["enabled"])
	assert.Equal(t, "localhost", m["host"])
	assert.Equal(t, nil, m["null_val"])

	nested, ok := m["nested"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "value", nested["key"])

	tags, ok := m["tags"].([]interface{})
	assert.True(t, ok)
	assert.Equal(t, []interface{}{"a", "b"}, tags)
}

func TestConvertStructToMap_PBToModel(t *testing.T) {
	bc := NewBidiConverter(TestMapStructPB{}, TestMapStructModel{})

	st, err := structpb.NewStruct(map[string]interface{}{
		"timeout": float64(60),
		"enabled": false,
		"host":    "0.0.0.0",
		"nested":  map[string]interface{}{"depth": float64(3)},
		"ports":   []interface{}{float64(8080), float64(8443)},
	})
	assert.NoError(t, err)

	pb := TestMapStructPB{
		Name:   "prod",
		Params: st,
	}

	var model TestMapStructModel
	err = bc.ConvertPBToModel(pb, &model)
	assert.NoError(t, err)
	assert.Equal(t, "prod", model.Name)
	assert.NotNil(t, model.Params)

	assert.Equal(t, float64(60), model.Params["timeout"])
	assert.Equal(t, false, model.Params["enabled"])
	assert.Equal(t, "0.0.0.0", model.Params["host"])

	nested, ok := model.Params["nested"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, float64(3), nested["depth"])

	ports, ok := model.Params["ports"].([]interface{})
	assert.True(t, ok)
	assert.Equal(t, float64(8080), ports[0])
	assert.Equal(t, float64(8443), ports[1])
}

func TestConvertMapStruct_NilValues(t *testing.T) {
	bc := NewBidiConverter(TestMapStructPB{}, TestMapStructModel{})

	// Model→PB: nil map → nil Struct
	t.Run("nil_map_to_struct", func(t *testing.T) {
		model := TestMapStructModel{Name: "empty"}
		var pb TestMapStructPB
		err := bc.ConvertModelToPB(model, &pb)
		assert.NoError(t, err)
		assert.Equal(t, "empty", pb.Name)
		assert.Nil(t, pb.Params)
	})

	// PB→Model: nil Struct → nil map
	t.Run("nil_struct_to_map", func(t *testing.T) {
		pb := TestMapStructPB{Name: "empty"}
		var model TestMapStructModel
		err := bc.ConvertPBToModel(pb, &model)
		assert.NoError(t, err)
		assert.Equal(t, "empty", model.Name)
		assert.Nil(t, model.Params)
	})
}

func TestConvertMapStruct_EmptyMap(t *testing.T) {
	bc := NewBidiConverter(TestMapStructPB{}, TestMapStructModel{})

	// Model→PB: 空 map → 空 Struct（非 nil）
	t.Run("empty_map_to_struct", func(t *testing.T) {
		model := TestMapStructModel{
			Name:   "test",
			Params: map[string]interface{}{},
		}
		var pb TestMapStructPB
		err := bc.ConvertModelToPB(model, &pb)
		assert.NoError(t, err)
		assert.NotNil(t, pb.Params)
		assert.Equal(t, 0, len(pb.Params.GetFields()))
	})

	// PB→Model: 空 Struct → 空 map
	t.Run("empty_struct_to_map", func(t *testing.T) {
		st, err := structpb.NewStruct(map[string]interface{}{})
		assert.NoError(t, err)
		pb := TestMapStructPB{Name: "test", Params: st}
		var model TestMapStructModel
		err = bc.ConvertPBToModel(pb, &model)
		assert.NoError(t, err)
		assert.NotNil(t, model.Params)
		assert.Equal(t, 0, len(model.Params))
	})
}

func TestConvertMapStruct_RoundTrip(t *testing.T) {
	// Model→PB→Model 往返一致性
	bc := NewBidiConverter(TestMapStructPB{}, TestMapStructModel{})

	original := TestMapStructModel{
		Name: "roundtrip",
		Params: map[string]interface{}{
			"int_val":   float64(42),
			"float_val": float64(3.14),
			"str_val":   "hello",
			"bool_val":  true,
			"null_val":  nil,
			"nested":    map[string]interface{}{"inner": float64(1)},
			"list_val":  []interface{}{float64(1), float64(2), float64(3)},
		},
	}

	var pb TestMapStructPB
	err := bc.ConvertModelToPB(original, &pb)
	assert.NoError(t, err)

	var roundTrip TestMapStructModel
	err = bc.ConvertPBToModel(pb, &roundTrip)
	assert.NoError(t, err)

	assert.Equal(t, original.Name, roundTrip.Name)
	assert.Equal(t, original.Params["int_val"], roundTrip.Params["int_val"])
	assert.Equal(t, original.Params["float_val"], roundTrip.Params["float_val"])
	assert.Equal(t, original.Params["str_val"], roundTrip.Params["str_val"])
	assert.Equal(t, original.Params["bool_val"], roundTrip.Params["bool_val"])
	assert.Equal(t, original.Params["null_val"], roundTrip.Params["null_val"])

	// 嵌套 struct
	origNested := original.Params["nested"].(map[string]interface{})
	rtNested := roundTrip.Params["nested"].(map[string]interface{})
	assert.Equal(t, origNested["inner"], rtNested["inner"])

	// 列表
	origList := original.Params["list_val"].([]interface{})
	rtList := roundTrip.Params["list_val"].([]interface{})
	assert.Equal(t, origList, rtList)
}
