/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2023-09-26 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2023-09-26 00:00:00
 * @FilePath: \go-pbmo\safe.go
 * @Description: 安全转换器 - 利用 go-toolbox/safe 实现安全的字段访问和转换
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"reflect"

	"github.com/kamalyes/go-toolbox/pkg/safe"
)

// SafeConverter 安全转换器
// 使用 go-toolbox/safe 进行链式安全访问，避免 nil panic
type SafeConverter struct {
	*BidiConverter
}

// NewSafeConverter 创建安全转换器
func NewSafeConverter(pbType, modelType interface{}, opts ...Option) *SafeConverter {
	return &SafeConverter{
		BidiConverter: NewBidiConverter(pbType, modelType, opts...),
	}
}

// SafeConvertPBToModel 安全的 PB -> Model 转换
// 自动处理 nil 值，避免 panic
func (sc *SafeConverter) SafeConvertPBToModel(pb interface{}, modelPtr interface{}) error {
	if pb == nil {
		return NewNilValueError("PB消息不能为空")
	}

	sa := safe.Safe(pb)
	if !sa.IsValid() {
		return NewNilValueError("PB消息无效")
	}

	return sc.BidiConverter.ConvertPBToModel(pb, modelPtr)
}

// SafeConvertModelToPB 安全的 Model -> PB 转换
func (sc *SafeConverter) SafeConvertModelToPB(model interface{}, pbPtr interface{}) error {
	if model == nil {
		return NewNilValueError("Model不能为空")
	}

	sa := safe.Safe(model)
	if !sa.IsValid() {
		return NewNilValueError("Model无效")
	}

	return sc.BidiConverter.ConvertModelToPB(model, pbPtr)
}

// SafeFieldAccess 安全字段访问
// 使用 go-toolbox/safe 的链式调用访问嵌套字段，避免 nil panic
func (sc *SafeConverter) SafeFieldAccess(obj interface{}, fieldNames ...string) *safe.SafeAccess {
	sa := safe.Safe(obj)
	for _, name := range fieldNames {
		sa = sa.Field(name)
	}
	return sa
}

// SafeGetField 安全获取字段值
// 返回字段的 SafeAccess，可进一步调用 .String()、.Int() 等方法
func SafeGetField(obj interface{}, fieldName string) *safe.SafeAccess {
	return safe.Safe(obj).Field(fieldName)
}

// SafeGetNestedField 安全获取嵌套字段值
// 支持路径访问，如 "User.Profile.Name"
func SafeGetNestedField(obj interface{}, path string) *safe.SafeAccess {
	return safe.Safe(obj).At(path)
}

// SafeBatchConvertPBToModel 安全批量 PB -> Model 转换
// 不因单个失败而中断，使用 go-toolbox/safe 检查 nil
func (sc *SafeConverter) SafeBatchConvertPBToModel(pbs interface{}, modelsPtr interface{}) *BatchResult {
	result := &BatchResult{
		Results: make([]BatchItem, 0),
	}

	pbsVal := reflect.ValueOf(pbs)
	if pbsVal.Kind() == reflect.Ptr {
		pbsVal = pbsVal.Elem()
	}
	if pbsVal.Kind() != reflect.Slice {
		return result
	}

	modelsVal := reflect.ValueOf(modelsPtr)
	if modelsVal.Kind() != reflect.Ptr {
		return result
	}
	modelsVal = modelsVal.Elem()

	modelType := modelsVal.Type().Elem()
	isModelPtr := modelType.Kind() == reflect.Ptr
	if isModelPtr {
		modelType = modelType.Elem()
	}

	models := reflect.MakeSlice(modelsVal.Type(), pbsVal.Len(), pbsVal.Len())

	for i := 0; i < pbsVal.Len(); i++ {
		pb := pbsVal.Index(i)
		item := BatchItem{Index: i}

		if pb.IsZero() || (pb.Kind() == reflect.Ptr && pb.IsNil()) {
			item.Success = false
			item.Error = NewNilValueError("元素 %d 为空", i)
			result.FailureCount++
			result.Results = append(result.Results, item)
			continue
		}

		modelPtr := reflect.New(modelType)
		if err := sc.BidiConverter.ConvertPBToModel(pb.Interface(), modelPtr.Interface()); err != nil {
			item.Success = false
			item.Error = err
			result.FailureCount++
		} else {
			item.Success = true
			if isModelPtr {
				item.Value = modelPtr.Interface()
				models.Index(i).Set(modelPtr)
			} else {
				item.Value = modelPtr.Elem().Interface()
				models.Index(i).Set(modelPtr.Elem())
			}
			result.SuccessCount++
		}

		result.Results = append(result.Results, item)
	}

	modelsVal.Set(models)
	return result
}
