/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2023-09-26 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2023-09-26 00:00:00
 * @FilePath: \go-pbmo\batch.go
 * @Description: 批量转换 - 支持批量 PB ↔ Model 转换
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"reflect"
)

// BatchConvertPBToModel 批量 PB -> Model 转换
func (bc *BidiConverter) BatchConvertPBToModel(pbs interface{}, modelsPtr interface{}) error {
	pbsVal := reflect.ValueOf(pbs)
	if pbsVal.Kind() == reflect.Ptr {
		pbsVal = pbsVal.Elem()
	}
	if pbsVal.Kind() != reflect.Slice {
		return ErrMustBeSlice
	}

	modelsVal := reflect.ValueOf(modelsPtr)
	if modelsVal.Kind() != reflect.Ptr {
		return ErrMustBePointer
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
		model := models.Index(i)

		modelPtr := reflect.New(modelType)
		if err := bc.ConvertPBToModel(pb.Interface(), modelPtr.Interface()); err != nil {
			return NewBatchError("元素 %d: %v", i, err)
		}

		if isModelPtr {
			model.Set(modelPtr)
		} else {
			model.Set(modelPtr.Elem())
		}
	}

	modelsVal.Set(models)
	return nil
}

// BatchConvertModelToPB 批量 Model -> PB 转换
func (bc *BidiConverter) BatchConvertModelToPB(models interface{}, pbsPtr interface{}) error {
	modelsVal := reflect.ValueOf(models)
	if modelsVal.Kind() == reflect.Ptr {
		modelsVal = modelsVal.Elem()
	}
	if modelsVal.Kind() != reflect.Slice {
		return ErrMustBeSlice
	}

	pbsVal := reflect.ValueOf(pbsPtr)
	if pbsVal.Kind() != reflect.Ptr {
		return ErrMustBePointer
	}
	pbsVal = pbsVal.Elem()

	pbType := pbsVal.Type().Elem()
	isPBPtr := pbType.Kind() == reflect.Ptr
	if isPBPtr {
		pbType = pbType.Elem()
	}

	pbs := reflect.MakeSlice(pbsVal.Type(), modelsVal.Len(), modelsVal.Len())

	for i := 0; i < modelsVal.Len(); i++ {
		model := modelsVal.Index(i)
		pb := pbs.Index(i)

		pbPtr := reflect.New(pbType)
		if err := bc.ConvertModelToPB(model.Interface(), pbPtr.Interface()); err != nil {
			return NewBatchError("元素 %d: %v", i, err)
		}

		if isPBPtr {
			pb.Set(pbPtr)
		} else {
			pb.Set(pbPtr.Elem())
		}
	}

	pbsVal.Set(pbs)
	return nil
}

// BatchResult 批量转换结果
type BatchResult struct {
	SuccessCount int         // 成功数量
	FailureCount int         // 失败数量
	Results      []BatchItem // 每个元素的转换结果
}

// BatchItem 单个转换结果
type BatchItem struct {
	Index   int         // 索引
	Success bool        // 是否成功
	Value   interface{} // 转换后的值
	Error   error       // 错误信息
}

// SafeBatchConvertPBToModel 安全批量 PB -> Model 转换
// 不因单个失败而中断，收集所有结果
func (bc *BidiConverter) SafeBatchConvertPBToModel(pbs interface{}, modelsPtr interface{}) *BatchResult {
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

		modelPtr := reflect.New(modelType)
		if err := bc.ConvertPBToModel(pb.Interface(), modelPtr.Interface()); err != nil {
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

// SafeBatchConvertModelToPB 安全批量 Model -> PB 转换
func (bc *BidiConverter) SafeBatchConvertModelToPB(models interface{}, pbsPtr interface{}) *BatchResult {
	result := &BatchResult{
		Results: make([]BatchItem, 0),
	}

	modelsVal := reflect.ValueOf(models)
	if modelsVal.Kind() == reflect.Ptr {
		modelsVal = modelsVal.Elem()
	}
	if modelsVal.Kind() != reflect.Slice {
		return result
	}

	pbsVal := reflect.ValueOf(pbsPtr)
	if pbsVal.Kind() != reflect.Ptr {
		return result
	}
	pbsVal = pbsVal.Elem()

	pbType := pbsVal.Type().Elem()
	isPBPtr := pbType.Kind() == reflect.Ptr
	if isPBPtr {
		pbType = pbType.Elem()
	}

	pbs := reflect.MakeSlice(pbsVal.Type(), modelsVal.Len(), modelsVal.Len())

	for i := 0; i < modelsVal.Len(); i++ {
		model := modelsVal.Index(i)
		item := BatchItem{Index: i}

		pbPtr := reflect.New(pbType)
		if err := bc.ConvertModelToPB(model.Interface(), pbPtr.Interface()); err != nil {
			item.Success = false
			item.Error = err
			result.FailureCount++
		} else {
			item.Success = true
			if isPBPtr {
				item.Value = pbPtr.Interface()
				pbs.Index(i).Set(pbPtr)
			} else {
				item.Value = pbPtr.Elem().Interface()
				pbs.Index(i).Set(pbPtr.Elem())
			}
			result.SuccessCount++
		}

		result.Results = append(result.Results, item)
	}

	pbsVal.Set(pbs)
	return result
}
