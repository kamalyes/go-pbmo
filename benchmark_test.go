/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-04-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-20 00:00:00
 * @FilePath: \go-pbmo\benchmark_test.go
 * @Description: 性能基准测试 - 验证低 OC 和高性能目标
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

// BenchmarkConvertPBToModel_Simple 基准测试简单 PB -> Model 转换
func BenchmarkConvertPBToModel_Simple(b *testing.B) {
	converter := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})
	pb := TestSimplePB{Value: "hello", Count: 42}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var model TestSimpleModel
		if err := converter.ConvertPBToModel(pb, &model); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkConvertModelToPB_Simple 基准测试简单 Model -> PB 转换
func BenchmarkConvertModelToPB_Simple(b *testing.B) {
	converter := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})
	model := TestSimpleModel{Value: "hello", Count: 42}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var pb TestSimplePB
		if err := converter.ConvertModelToPB(model, &pb); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkConvertPBToModel_WithFieldMapping 基准测试带字段映射的 PB -> Model 转换
func BenchmarkConvertPBToModel_WithFieldMapping(b *testing.B) {
	converter := NewBidiConverter(
		TestPBWithMapping{}, TestModelWithMapping{},
		WithFieldMapping("ID", "ClientId"),
		WithFieldMapping("Name", "UserName"),
		WithFieldMapping("Email", "UserEmail"),
	)
	pb := TestPBWithMapping{ClientId: 1, UserName: "test", UserEmail: "test@example.com"}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var model TestModelWithMapping
		if err := converter.ConvertPBToModel(pb, &model); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkConvertPBToModel_WithTagMapping 基准测试带 struct tag 映射的转换
func BenchmarkConvertPBToModel_WithTagMapping(b *testing.B) {
	converter := NewBidiConverter(TestPB{}, TestModel{})
	pb := TestPB{Id: 1, Name: "test", Email: "test@example.com", Age: 25, Score: 99.5, Active: true}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var model TestModel
		if err := converter.ConvertPBToModel(pb, &model); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkConvertPBToModel_WithTransformer 基准测试带字段转换器的转换
func BenchmarkConvertPBToModel_WithTransformer(b *testing.B) {
	converter := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})
	converter.RegisterTransformer("Value", func(v interface{}) interface{} {
		return fmt.Sprintf("prefix_%s", v.(string))
	})
	pb := TestSimplePB{Value: "hello", Count: 42}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var model TestSimpleModel
		if err := converter.ConvertPBToModel(pb, &model); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkBatchConvertPBToModel 基准测试批量 PB -> Model 转换
func BenchmarkBatchConvertPBToModel(b *testing.B) {
	converter := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})
	pbs := make([]TestSimplePB, 100)
	for i := range pbs {
		pbs[i] = TestSimplePB{Value: fmt.Sprintf("item_%d", i), Count: int32(i)}
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var models []TestSimpleModel
		if err := converter.BatchConvertPBToModel(pbs, &models); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkBatchConvertPBToModel_Large 基准测试大批量转换（1000条）
func BenchmarkBatchConvertPBToModel_Large(b *testing.B) {
	converter := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})
	pbs := make([]TestSimplePB, 1000)
	for i := range pbs {
		pbs[i] = TestSimplePB{Value: fmt.Sprintf("item_%d", i), Count: int32(i)}
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var models []TestSimpleModel
		if err := converter.BatchConvertPBToModel(pbs, &models); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRegistryConvertPBToModel 基准测试通过注册中心转换
func BenchmarkRegistryConvertPBToModel(b *testing.B) {
	registry := NewRegistry()
	converter := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})
	registry.MustRegister(converter)

	pb := TestSimplePB{Value: "hello", Count: 42}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var model TestSimpleModel
		if err := registry.ConvertPBToModel(pb, &model); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNewBidiConverter 基准测试创建转换器
func BenchmarkNewBidiConverter(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		NewBidiConverter(TestSimplePB{}, TestSimpleModel{})
	}
}

// BenchmarkNewBidiConverter_WithOptions 基准测试带选项创建转换器
func BenchmarkNewBidiConverter_WithOptions(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		NewBidiConverter(
			TestSimplePB{}, TestSimpleModel{},
			WithAutoTimeConversion(true),
			WithValidation(false),
			WithFieldMapping("Value", "Val"),
		)
	}
}

// BenchmarkEnumMapper_Map 基准测试枚举映射
func BenchmarkEnumMapper_Map(b *testing.B) {
	mapper := NewEnumMapper()
	for i := int32(0); i < 100; i++ {
		mapper.AddMapping(i, i*10)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		mapper.Map(int32(i%100), 0)
	}
}

// BenchmarkGenericEnumMapper_Map 基准测试泛型枚举映射
func BenchmarkGenericEnumMapper_Map(b *testing.B) {
	mapper := NewGenericEnumMapper[int32, int32](0)
	for i := int32(0); i < 100; i++ {
		mapper.Register(i, i*10)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		mapper.Map(int32(i % 100))
	}
}

// BenchmarkAutoEnumConverter_Convert 基准测试自动枚举转换器
func BenchmarkAutoEnumConverter_Convert(b *testing.B) {
	type ProtoStatus int32
	type WsStatus int32

	converter := NewAutoEnumConverter[ProtoStatus, WsStatus](0)
	mappings := make(map[ProtoStatus]WsStatus, 100)
	for i := ProtoStatus(0); i < 100; i++ {
		mappings[i] = WsStatus(i) * 10
	}
	converter.AutoRegister(mappings)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		converter.Convert(ProtoStatus(i % 100))
	}
}

// BenchmarkValidator_Validate 基准测试校验器
func BenchmarkValidator_Validate(b *testing.B) {
	validator := NewValidator()
	validator.RegisterRules("TestSimpleModel",
		FieldRule{Name: "Value", Required: true, MinLen: 1, MaxLen: 100},
		FieldRule{Name: "Count", Min: 0, Max: 1000},
	)

	model := TestSimpleModel{Value: "hello", Count: 42}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := validator.Validate(&model); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRegistry_Register 基准测试注册中心注册
func BenchmarkRegistry_Register(b *testing.B) {
	registry := NewRegistry()
	converter := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		registry.Unregister(converter.GetPBType(), converter.GetModelType())
		registry.MustRegister(converter)
	}
}

// BenchmarkRegistry_Lookup 基准测试注册中心查找
func BenchmarkRegistry_Lookup(b *testing.B) {
	registry := NewRegistry()
	converter := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})
	registry.MustRegister(converter)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := registry.LookupByInstance(TestSimplePB{}, TestSimpleModel{}); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkTransformerRegistry_Apply 基准测试字段转换器应用
func BenchmarkTransformerRegistry_Apply(b *testing.B) {
	tr := NewTransformerRegistry()
	tr.Register("Value", func(v interface{}) interface{} {
		return fmt.Sprintf("prefix_%s", v.(string))
	})

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		tr.Apply("Value", reflectValueOf("hello"))
	}
}

// BenchmarkSafeConverter_SafeConvertPBToModel 基准测试安全转换器
func BenchmarkSafeConverter_SafeConvertPBToModel(b *testing.B) {
	converter := NewSafeConverter(TestSimplePB{}, TestSimpleModel{})
	pb := TestSimplePB{Value: "hello", Count: 42}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var model TestSimpleModel
		if err := converter.SafeConvertPBToModel(pb, &model); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkConvertPBToModel_Parallel 基准测试并发 PB -> Model 转换
func BenchmarkConvertPBToModel_Parallel(b *testing.B) {
	converter := NewBidiConverter(TestSimplePB{}, TestSimpleModel{})
	pbData := TestSimplePB{Value: "hello", Count: 42}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var model TestSimpleModel
			if err := converter.ConvertPBToModel(pbData, &model); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkConvertPBToModel_FullModel 基准测试完整模型转换
func BenchmarkConvertPBToModel_FullModel(b *testing.B) {
	converter := NewBidiConverter(TestPB{}, TestModel{})
	pb := TestPB{
		Id:     1,
		Name:   "张三",
		Email:  "zhangsan@example.com",
		Age:    25,
		Score:  99.5,
		Active: true,
		Tags:   []string{"go", "pbmo", "test"},
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var model TestModel
		if err := converter.ConvertPBToModel(pb, &model); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkConvertModelToPB_FullModel 基准测试完整模型反向转换
func BenchmarkConvertModelToPB_FullModel(b *testing.B) {
	converter := NewBidiConverter(TestPB{}, TestModel{})
	model := TestModel{
		ID:     1,
		Name:   "张三",
		Email:  "zhangsan@example.com",
		Age:    25,
		Score:  99.5,
		Active: true,
		Tags:   []string{"go", "pbmo", "test"},
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var pb TestPB
		if err := converter.ConvertModelToPB(model, &pb); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSafeBatchConvertPBToModel 基准测试安全批量转换
func BenchmarkSafeBatchConvertPBToModel(b *testing.B) {
	converter := NewSafeConverter(TestSimplePB{}, TestSimpleModel{})
	pbs := make([]TestSimplePB, 100)
	for i := range pbs {
		pbs[i] = TestSimplePB{Value: fmt.Sprintf("item_%d", i), Count: int32(i)}
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var models []TestSimpleModel
		converter.SafeBatchConvertPBToModel(pbs, &models)
	}
}

// reflectValueOf 辅助函数，避免在 benchmark 中重复创建 reflect.Value
func reflectValueOf(v interface{}) reflect.Value {
	return reflect.ValueOf(v)
}

// BenchmarkNamedSliceConversion 命名切片类型自动转换（ConvertibleTo 快速路径）
func BenchmarkNamedSliceConversion_ModelToPB(b *testing.B) {
	Register[TestNamedSliceModel, TestNamedSlicePB]()
	model := &TestNamedSliceModel{
		Name:  "bench",
		Tags:  TestStringSlice{"a", "b", "c", "d", "e"},
		Items: TestStringSlice{"1", "2", "3"},
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = ToPB[TestNamedSliceModel, TestNamedSlicePB](model)
	}
}

// BenchmarkNamedSliceConversion_PBToModel 命名切片反向转换
func BenchmarkNamedSliceConversion_PBToModel(b *testing.B) {
	Register[TestNamedSliceModel, TestNamedSlicePB]()
	pb := &TestNamedSlicePB{
		Name:  "bench",
		Tags:  []string{"a", "b", "c", "d", "e"},
		Items: []string{"1", "2", "3"},
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = FromPB[TestNamedSlicePB, TestNamedSliceModel](pb)
	}
}

// BenchmarkNestedStructAutoConversion 嵌套结构体自动转换
func BenchmarkNestedStructAutoConversion_ModelToPB(b *testing.B) {
	Register[TestInnerModel, TestInnerPB]()
	Register[TestNestedAutoModel, TestNestedAutoPB]()
	model := &TestNestedAutoModel{
		Name: "bench-nested",
		Inner: &TestInnerModel{
			Label: "inner-label",
			Count: 42,
		},
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = ToPB[TestNestedAutoModel, TestNestedAutoPB](model)
	}
}

// BenchmarkTimePtrAutoConversion 时间指针自动转换
func BenchmarkTimePtrAutoConversion_ModelToPB(b *testing.B) {
	Register[TestTimePtrModel, TestTimePtrPB]()
	scheduled := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	model := &TestTimePtrModel{
		Name:        "bench-time",
		ScheduledAt: &scheduled,
		ReleasedAt:  nil,
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = ToPB[TestTimePtrModel, TestTimePtrPB](model)
	}
}

// BenchmarkWrapperFieldAutoConversion Wrapper 字段自动转换
func BenchmarkWrapperFieldAutoConversion_ModelToPB(b *testing.B) {
	Register[TestWrapperFieldModel, TestWrapperFieldPB]()
	minVal := int32(10)
	maxVal := int32(100)
	model := &TestWrapperFieldModel{
		Name:   "bench-wrapper",
		MinVal: &minVal,
		MaxVal: &maxVal,
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = ToPB[TestWrapperFieldModel, TestWrapperFieldPB](model)
	}
}

// BenchmarkFindConverter_Cached findConverter 缓存命中
func BenchmarkFindConverter_Cached(b *testing.B) {
	Register[TestSimpleModel, TestSimplePB]()
	srcType := reflect.TypeOf((*TestSimpleModel)(nil)).Elem()
	dstType := reflect.TypeOf((*TestSimplePB)(nil)).Elem()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		findConverter(srcType, dstType)
	}
}

// BenchmarkFindConverter_Uncached findConverter 首次查找
func BenchmarkFindConverter_Uncached(b *testing.B) {
	srcType := reflect.TypeOf((*TestSimpleModel)(nil)).Elem()
	dstType := reflect.TypeOf((*TestSimplePB)(nil)).Elem()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		converterLookupCache = sync.Map{}
		findConverter(srcType, dstType)
	}
}

// BenchmarkTimeZeroValue_ZeroTime 零值时间转换
func BenchmarkTimeZeroValue_ZeroTime(b *testing.B) {
	Register[TestTimeZeroModel, TestTimeZeroPB]()
	model := &TestTimeZeroModel{
		Name:      "zero",
		CreatedAt: time.Time{},
		UpdatedAt: time.Time{},
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = ToPB[TestTimeZeroModel, TestTimeZeroPB](model)
	}
}
