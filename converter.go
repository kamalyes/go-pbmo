/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-04-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-20 00:00:00
 * @FilePath: \go-pbmo\converter.go
 * @Description: 核心双向转换器 - PB ↔ Model 高性能转换
 * 使用 go-toolbox/syncx.Map 管理字段映射，替代手动 sync.RWMutex
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"reflect"
	"sync"
	"time"

	"github.com/kamalyes/go-toolbox/pkg/syncx"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Converter 核心转换器接口
type Converter interface {
	ConvertPBToModel(pb interface{}, model interface{}) error
	ConvertModelToPB(model interface{}, pb interface{}) error
}

// BidiConverter 双向转换器
// 支持 PB ↔ Model 转换、字段映射、字段转换器、自动时间转换
type BidiConverter struct {
	pbType         reflect.Type               // PB 类型
	modelType      reflect.Type               // Model 类型
	transformers   *TransformerRegistry       // 字段转换器注册表
	validator      *Validator                 // 校验器
	options        *Options                   // 配置选项
	fieldMapping   *syncx.Map[string, string] // 字段名映射: Model字段名 -> PB字段名
	reverseMapping map[string]string          // 缓存的反向映射: PB字段名 -> Model字段名
	tagCached      bool                       // struct tag映射是否已缓存
	tagOnce        sync.Once                  // 确保 tag 映射只加载一次
	mappingOnce    sync.Once                  // 确保反向映射只构建一次
}

// NewBidiConverter 创建双向转换器
func NewBidiConverter(pbType, modelType interface{}, opts ...Option) *BidiConverter {
	options := ApplyOptions(opts...)

	fm := syncx.NewMap[string, string]()
	for k, v := range options.FieldMapping {
		fm.Store(k, v)
	}

	return &BidiConverter{
		pbType:       reflect.TypeOf(pbType),
		modelType:    reflect.TypeOf(modelType),
		transformers: NewTransformerRegistry(),
		validator:    NewValidator(),
		options:      options,
		fieldMapping: fm,
		tagCached:    false,
	}
}

// RegisterTransformer 注册字段转换器
func (bc *BidiConverter) RegisterTransformer(field string, fn TransformerFunc) {
	bc.transformers.Register(field, fn)
}

// RegisterValidationRules 注册校验规则
func (bc *BidiConverter) RegisterValidationRules(typeName string, rules ...FieldRule) {
	bc.validator.RegisterRules(typeName, rules...)
}

// WithFieldMapping 设置字段名映射（链式调用）
func (bc *BidiConverter) WithFieldMapping(modelFieldName, pbFieldName string) *BidiConverter {
	bc.fieldMapping.Store(modelFieldName, pbFieldName)
	return bc
}

// WithFieldMappings 批量设置字段名映射（链式调用）
func (bc *BidiConverter) WithFieldMappings(mappings map[string]string) *BidiConverter {
	for modelField, pbField := range mappings {
		bc.fieldMapping.Store(modelField, pbField)
	}
	return bc
}

// RegisterFieldMapping 批量注册字段映射
func (bc *BidiConverter) RegisterFieldMapping(mappings map[string]string) {
	for modelField, pbField := range mappings {
		bc.fieldMapping.Store(modelField, pbField)
	}
}

// WithAutoTimeConversion 设置自动时间转换（链式调用）
func (bc *BidiConverter) WithAutoTimeConversion(enabled bool) *BidiConverter {
	bc.options.AutoTimeConversion = enabled
	return bc
}

// WithTagMapping 启用/禁用 struct tag 映射（链式调用）
func (bc *BidiConverter) WithTagMapping(enabled bool) *BidiConverter {
	bc.options.TagMappingEnabled = enabled
	return bc
}

// WithTagName 设置 struct tag 名称（链式调用）
func (bc *BidiConverter) WithTagName(name string) *BidiConverter {
	bc.options.TagName = name
	return bc
}

// WithValidation 启用/禁用校验（链式调用）
func (bc *BidiConverter) WithValidation(enabled bool) *BidiConverter {
	bc.options.ValidationEnabled = enabled
	return bc
}

// WithDesensitize 启用/禁用脱敏（链式调用）
func (bc *BidiConverter) WithDesensitize(enabled bool) *BidiConverter {
	bc.options.DesensitizeEnabled = enabled
	return bc
}

// WithSafeMode 启用/禁用安全模式（链式调用）
func (bc *BidiConverter) WithSafeMode(enabled bool) *BidiConverter {
	bc.options.SafeMode = enabled
	return bc
}

// WithConcurrency 设置并发数（链式调用）
func (bc *BidiConverter) WithConcurrency(n int) *BidiConverter {
	bc.options.Concurrency = n
	return bc
}

// WithBatchSize 设置批处理大小（链式调用）
func (bc *BidiConverter) WithBatchSize(size int) *BidiConverter {
	bc.options.BatchSize = size
	return bc
}

// WithTimeout 设置超时时间（链式调用）
func (bc *BidiConverter) WithTimeout(timeout time.Duration) *BidiConverter {
	bc.options.Timeout = timeout
	return bc
}

// GetModelType 获取 Model 类型
func (bc *BidiConverter) GetModelType() reflect.Type {
	return bc.modelType
}

// GetPBType 获取 PB 类型
func (bc *BidiConverter) GetPBType() reflect.Type {
	return bc.pbType
}

// GetValidator 获取校验器
func (bc *BidiConverter) GetValidator() *Validator {
	return bc.validator
}

// GetTransformers 获取字段转换器注册表
func (bc *BidiConverter) GetTransformers() *TransformerRegistry {
	return bc.transformers
}

// Validate 校验数据
func (bc *BidiConverter) Validate(data interface{}) error {
	return bc.validator.Validate(data)
}

// ConvertPBToModel PB -> Model 转换
func (bc *BidiConverter) ConvertPBToModel(pb interface{}, modelPtr interface{}) error {
	bc.loadTagMappings()
	bc.buildReverseMapping()

	if pb == nil || modelPtr == nil {
		return nil
	}

	modelVal := reflect.ValueOf(modelPtr)
	if modelVal.Kind() != reflect.Ptr {
		return ErrMustBePointer
	}
	if modelVal.IsNil() {
		return nil
	}

	modelVal = modelVal.Elem()

	for modelVal.Kind() == reflect.Interface && !modelVal.IsNil() {
		modelVal = modelVal.Elem()
	}

	if modelVal.Kind() != reflect.Struct {
		return NewTypeMismatchError("目标必须是结构体，得到 %v", modelVal.Kind())
	}

	pbVal := reflect.ValueOf(pb)
	if pbVal.Kind() == reflect.Ptr {
		if pbVal.IsNil() {
			return nil
		}
		pbVal = pbVal.Elem()
	}

	pbType := pbVal.Type()

	for i := 0; i < pbVal.NumField(); i++ {
		pbField := pbVal.Field(i)
		pbFieldName := pbType.Field(i).Name

		modelFieldName := pbFieldName
		if mappedName, ok := bc.reverseMapping[pbFieldName]; ok {
			modelFieldName = mappedName
		}

		modelField := modelVal.FieldByName(modelFieldName)
		if !modelField.IsValid() || !modelField.CanSet() {
			continue
		}

		if bc.transformers.Has(pbFieldName) {
			pbField = bc.transformers.Apply(pbFieldName, pbField)
		}

		if err := convertField(pbField, modelField, bc.options.AutoTimeConversion); err != nil {
			return NewConversionError("字段 %s->%s: %v", pbFieldName, modelFieldName, err)
		}
	}

	if bc.options.ValidationEnabled {
		if err := bc.validator.Validate(modelPtr); err != nil {
			return err
		}
	}

	return nil
}

// ConvertModelToPB Model -> PB 转换
func (bc *BidiConverter) ConvertModelToPB(model interface{}, pbPtr interface{}) error {
	bc.loadTagMappings()

	if model == nil || pbPtr == nil {
		return nil
	}

	pbVal := reflect.ValueOf(pbPtr)
	if pbVal.Kind() != reflect.Ptr {
		return ErrMustBePointer
	}
	if pbVal.IsNil() {
		return nil
	}

	pbVal = pbVal.Elem()

	modelVal := reflect.ValueOf(model)
	if modelVal.Kind() == reflect.Ptr {
		if modelVal.IsNil() {
			return nil
		}
		modelVal = modelVal.Elem()
	}

	modelType := modelVal.Type()

	for i := 0; i < modelVal.NumField(); i++ {
		modelField := modelVal.Field(i)
		modelFieldName := modelType.Field(i).Name

		pbFieldName := modelFieldName
		if mappedName, ok := bc.fieldMapping.Load(modelFieldName); ok {
			pbFieldName = mappedName
		}

		pbField := pbVal.FieldByName(pbFieldName)
		if !pbField.IsValid() || !pbField.CanSet() {
			continue
		}

		if bc.transformers.Has(modelFieldName) {
			modelField = bc.transformers.Apply(modelFieldName, modelField)
		}

		if err := convertField(modelField, pbField, bc.options.AutoTimeConversion); err != nil {
			return NewConversionError("字段 %s->%s: %v", modelFieldName, pbFieldName, err)
		}
	}

	return nil
}

// loadTagMappings 从 Model 结构体的 tag 加载字段映射（延迟加载，只执行一次）
// 使用 sync.Once 确保线程安全且只执行一次
func (bc *BidiConverter) loadTagMappings() {
	if !bc.options.TagMappingEnabled {
		return
	}

	bc.tagOnce.Do(func() {
		modelType := DereferenceType(bc.modelType)
		if modelType.Kind() != reflect.Struct {
			return
		}

		tagName := bc.options.TagName
		if tagName == "" {
			tagName = "pbmo"
		}

		for i := 0; i < modelType.NumField(); i++ {
			field := modelType.Field(i)
			if pbFieldName := field.Tag.Get(tagName); pbFieldName != "" {
				if _, exists := bc.fieldMapping.Load(field.Name); !exists {
					bc.fieldMapping.Store(field.Name, pbFieldName)
				}
			}
		}
		bc.tagCached = true
	})
}

func (bc *BidiConverter) buildReverseMapping() {
	bc.mappingOnce.Do(func() {
		size := bc.fieldMapping.Size()
		if size == 0 {
			bc.reverseMapping = make(map[string]string)
			return
		}
		bc.reverseMapping = make(map[string]string, size)
		bc.fieldMapping.Range(func(modelField, pbField string) bool {
			bc.reverseMapping[pbField] = modelField
			return true
		})
	})
}

// convertField 字段级转换核心逻辑
func convertField(src, dst reflect.Value, autoTime bool) error {
	if !src.IsValid() {
		return nil
	}

	srcType := src.Type()
	dstType := dst.Type()

	// 快速路径：类型完全相同且非指针
	if srcType == dstType && srcType.Kind() != reflect.Ptr {
		dst.Set(src)
		return nil
	}

	// 时间戳转换：time.Time / *time.Time ↔ *timestamppb.Timestamp
	// 仅在 autoTime 开启时生效，支持以下四种转换路径：
	if autoTime {
		// time.Time → *timestamppb.Timestamp（值类型时间 → PB 时间戳指针）
		if srcType == timeType && dstType == timestampPtrType {
			t := src.Interface().(time.Time)
			dst.Set(reflect.ValueOf(timestamppb.New(t)))
			return nil
		}
		// *timestamppb.Timestamp → time.Time（PB 时间戳指针 → 值类型时间）
		if srcType == timestampPtrType && dstType == timeType {
			return convertTimestampToTime(src, dst)
		}
		// *time.Time → *timestamppb.Timestamp（时间指针 → PB 时间戳指针）
		// 当源值为 nil 时直接返回，目标保持零值
		if srcType == timePtrType && dstType == timestampPtrType {
			if src.IsNil() {
				return nil
			}
			t := src.Interface().(*time.Time)
			dst.Set(reflect.ValueOf(timestamppb.New(*t)))
			return nil
		}
		// *timestamppb.Timestamp → *time.Time（PB 时间戳指针 → 时间指针）
		// 当源值为 nil 时直接返回，目标保持零值
		if srcType == timestampPtrType && dstType == timePtrType {
			if src.IsNil() {
				return nil
			}
			ts := src.Interface().(*timestamppb.Timestamp)
			t := ts.AsTime()
			dst.Set(reflect.ValueOf(&t))
			return nil
		}
	}

	// Wrappers 类型自动转换: *T ↔ *wrapperspb.XxxValue
	if handled, err := tryConvertWrapper(src, dst); handled {
		return err
	}

	// 整数类型转换
	if IsIntegerType(srcType) && IsIntegerType(dstType) {
		return convertInteger(src, dst)
	}

	// 直接赋值
	if srcType.AssignableTo(dstType) {
		dst.Set(src)
		return nil
	}

	// 可转换类型
	if srcType.ConvertibleTo(dstType) {
		dst.Set(src.Convert(dstType))
		return nil
	}

	// 指针处理
	if dstType.Kind() == reflect.Ptr {
		return convertToPointer(src, dst, autoTime)
	}
	if srcType.Kind() == reflect.Ptr {
		if src.IsNil() {
			return nil
		}
		return convertField(src.Elem(), dst, autoTime)
	}

	// 切片转换
	if srcType.Kind() == reflect.Slice && dstType.Kind() == reflect.Slice {
		return convertSlice(src, dst, autoTime)
	}

	// 结构体转换
	if srcType.Kind() == reflect.Struct && dstType.Kind() == reflect.Struct {
		return convertStruct(src, dst, autoTime)
	}

	return nil
}

// convertTimestampToTime *timestamppb.Timestamp -> time.Time
func convertTimestampToTime(src, dst reflect.Value) error {
	if src.IsNil() {
		return nil
	}
	ts := src.Interface().(*timestamppb.Timestamp)
	dst.Set(reflect.ValueOf(ts.AsTime()))
	return nil
}

// convertInteger 整数类型转换
func convertInteger(src, dst reflect.Value) error {
	srcKind := src.Type().Kind()
	dstKind := dst.Type().Kind()

	if IsUnsignedInt(srcKind) {
		val := src.Uint()
		if IsSignedInt(dstKind) {
			dst.SetInt(int64(val))
		} else {
			dst.SetUint(val)
		}
	} else {
		val := src.Int()
		if IsUnsignedInt(dstKind) {
			dst.SetUint(uint64(val))
		} else {
			dst.SetInt(val)
		}
	}
	return nil
}

// convertToPointer 转换到指针类型
func convertToPointer(src, dst reflect.Value, autoTime bool) error {
	if src.IsZero() {
		return nil
	}

	if !dst.IsNil() {
		if src.Type().Kind() == reflect.Ptr {
			return convertField(src.Elem(), dst.Elem(), autoTime)
		}
		return convertField(src, dst.Elem(), autoTime)
	}

	newVal := reflect.New(dst.Type().Elem())
	var err error
	if src.Type().Kind() == reflect.Ptr {
		err = convertField(src.Elem(), newVal.Elem(), autoTime)
	} else {
		err = convertField(src, newVal.Elem(), autoTime)
	}
	if err == nil {
		dst.Set(newVal)
	}
	return err
}

// convertSlice 切片转换
func convertSlice(src, dst reflect.Value, autoTime bool) error {
	if src.IsNil() {
		return nil
	}
	length := src.Len()
	dstSlice := reflect.MakeSlice(dst.Type(), length, length)
	for i := 0; i < length; i++ {
		if err := convertField(src.Index(i), dstSlice.Index(i), autoTime); err != nil {
			return NewBatchError("元素 %d: %v", i, err)
		}
	}
	dst.Set(dstSlice)
	return nil
}

// convertStruct 结构体转换
func convertStruct(src, dst reflect.Value, autoTime bool) error {
	srcType := src.Type()
	for i := 0; i < src.NumField(); i++ {
		srcField := src.Field(i)
		srcFieldName := srcType.Field(i).Name
		dstField := dst.FieldByName(srcFieldName)
		if !dstField.IsValid() || !dstField.CanSet() {
			continue
		}
		if err := convertField(srcField, dstField, autoTime); err != nil {
			return NewConversionError("结构体字段 %s: %v", srcFieldName, err)
		}
	}
	return nil
}
