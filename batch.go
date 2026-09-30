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
	"unsafe"
)

// convertPtrCached 以 unsafe 指针为基准执行缓存驱动的结构体转换
// srcPtr/dstPtr 分别指向源与目标结构体首字段，方向由调用方的缓存决定
func (bc *BidiConverter) convertPtrCached(srcPtr, dstPtr unsafe.Pointer, cache *fieldCache) error {
	if srcPtr == nil || dstPtr == nil {
		return nil
	}
	if cache.mergedCopyFunc != nil {
		cache.mergedCopyFunc(srcPtr, dstPtr)
		if len(cache.slowEntries) == 0 {
			return nil
		}
	} else if len(cache.fastEntries) > 0 {
		for i := range cache.fastEntries {
			cache.fastEntries[i].copyFunc(srcPtr, dstPtr)
		}
	}
	if len(cache.slowEntries) == 0 {
		return nil
	}

	// 使用预计算的偏移量直接定位字段，避免 FieldByIndex 的开销
	for i := range cache.slowEntries {
		entry := &cache.slowEntries[i]
		srcField := reflect.NewAt(entry.srcType, addPtr(srcPtr, entry.srcOffset)).Elem()
		dstField := reflect.NewAt(entry.dstType, addPtr(dstPtr, entry.dstOffset)).Elem()
		if !srcField.IsValid() || !dstField.IsValid() {
			continue
		}
		if err := convertFieldByKind(srcField, dstField, entry); err != nil {
			return NewConversionError("field %s conversion failed: %v", entry.srcName, err)
		}
	}
	return nil
}

// convertCached 以 reflect.Value 执行缓存驱动的结构体转换
// dst 为 nil 指针时报错，src 为 nil 指针时跳过；方向由调用方的缓存决定
func (bc *BidiConverter) convertCached(srcVal, dstVal reflect.Value, cache *fieldCache) error {
	if !srcVal.IsValid() || !dstVal.IsValid() {
		return nil
	}
	if dstVal.Kind() == reflect.Ptr {
		if dstVal.IsNil() {
			return ErrMustBePointer
		}
		dstVal = dstVal.Elem()
	}
	if srcVal.Kind() == reflect.Ptr {
		if srcVal.IsNil() {
			return nil
		}
		srcVal = srcVal.Elem()
	}

	canFastPath := srcVal.CanAddr() && dstVal.CanAddr()
	if canFastPath && cache.mergedCopyFunc != nil {
		srcBase := unsafe.Pointer(srcVal.UnsafeAddr())
		dstBase := unsafe.Pointer(dstVal.UnsafeAddr())
		cache.mergedCopyFunc(srcBase, dstBase)
		if len(cache.slowEntries) == 0 {
			return nil
		}
	} else if canFastPath && len(cache.fastEntries) > 0 {
		srcBase := unsafe.Pointer(srcVal.UnsafeAddr())
		dstBase := unsafe.Pointer(dstVal.UnsafeAddr())
		for i := range cache.fastEntries {
			cache.fastEntries[i].copyFunc(srcBase, dstBase)
		}
	} else if len(cache.fastEntries) > 0 {
		for i := range cache.fastEntries {
			entry := &cache.fastEntries[i]
			srcField := srcVal.FieldByIndex(entry.srcIndex)
			dstField := dstVal.FieldByIndex(entry.dstIndex)
			if !srcField.IsValid() || !dstField.IsValid() || !dstField.CanSet() {
				continue
			}
			if entry.transformFn != nil {
				srcField = reflect.ValueOf(entry.transformFn(srcField.Interface()))
			}
			if err := convertFieldByKind(srcField, dstField, entry); err != nil {
				return NewConversionError("字段 %s 转换失败: %v", entry.srcName, err)
			}
		}
	}

	if len(cache.slowEntries) == 0 {
		return nil
	}

	// 优先使用 unsafe 指针直接定位字段，避免 FieldByIndex 开销
	if canFastPath {
		srcBase := unsafe.Pointer(srcVal.UnsafeAddr())
		dstBase := unsafe.Pointer(dstVal.UnsafeAddr())
		for i := range cache.slowEntries {
			entry := &cache.slowEntries[i]
			srcField := reflect.NewAt(entry.srcType, addPtr(srcBase, entry.srcOffset)).Elem()
			dstField := reflect.NewAt(entry.dstType, addPtr(dstBase, entry.dstOffset)).Elem()
			if !srcField.IsValid() || !dstField.IsValid() {
				continue
			}
			if err := convertFieldByKind(srcField, dstField, entry); err != nil {
				return NewConversionError("字段 %s 转换失败: %v", entry.srcName, err)
			}
		}
	} else {
		for i := range cache.slowEntries {
			entry := &cache.slowEntries[i]
			srcField := srcVal.FieldByIndex(entry.srcIndex)
			dstField := dstVal.FieldByIndex(entry.dstIndex)
			if !srcField.IsValid() || !dstField.IsValid() {
				continue
			}
			if err := convertFieldByKind(srcField, dstField, entry); err != nil {
				return NewConversionError("字段 %s 转换失败: %v", entry.srcName, err)
			}
		}
	}
	return nil
}

// BatchConvertPBToModel 批量 PB -> Model 转换
//
// Deprecated: 使用泛型函数 FromPBs[P, M] 替代，类型更安全、调用更简洁
// 示例: models, err := pbmo.FromPBs[UserPB, UserModel](pbs)
func batchElementPtr(slice reflect.Value, elemType reflect.Type, isPtr bool, index int) unsafe.Pointer {
	if isPtr {
		elem := slice.Index(index)
		if elem.IsNil() {
			return nil
		}
		return unsafe.Pointer(elem.Pointer())
	}
	return addPtr(slice.UnsafePointer(), uintptr(index)*elemType.Size())
}

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
	cache := bc.pbToModelFieldCache()
	pbType := pbsVal.Type().Elem()
	isPBPtr := pbType.Kind() == reflect.Ptr
	if isPBPtr {
		pbType = pbType.Elem()
	}
	var modelBase unsafe.Pointer
	var modelSize uintptr
	if !isModelPtr {
		modelBase = models.UnsafePointer()
		modelSize = modelType.Size()
	}
	if !isModelPtr && cache.mergedCopyFunc != nil && len(cache.slowEntries) == 0 {
		for i := 0; i < pbsVal.Len(); i++ {
			pbPtr := batchElementPtr(pbsVal, pbType, isPBPtr, i)
			if pbPtr != nil {
				cache.mergedCopyFunc(pbPtr, addPtr(modelBase, uintptr(i)*modelSize))
			}
		}
		modelsVal.Set(models)
		return nil
	}

	for i := 0; i < pbsVal.Len(); i++ {
		pbPtr := batchElementPtr(pbsVal, pbType, isPBPtr, i)

		if isModelPtr {
			model := models.Index(i)
			modelPtr := reflect.New(modelType)
			if err := bc.convertPtrCached(pbPtr, modelPtr.UnsafePointer(), cache); err != nil {
				return NewBatchError("元素 %d: %v", i, err)
			}

			model.Set(modelPtr)
		} else {
			modelPtr := addPtr(modelBase, uintptr(i)*modelSize)
			if err := bc.convertPtrCached(pbPtr, modelPtr, cache); err != nil {
				return NewBatchError("元素 %d: %v", i, err)
			}
		}
	}

	modelsVal.Set(models)
	return nil
}

// BatchConvertModelToPB 批量 Model -> PB 转换
//
// Deprecated: 使用泛型函数 ToPBs[M, P] 替代，类型更安全、调用更简洁
// 示例: pbs, err := pbmo.ToPBs[UserModel, UserPB](models)
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
	cache := bc.modelToPBFieldCache()
	modelType := modelsVal.Type().Elem()
	isModelPtr := modelType.Kind() == reflect.Ptr
	if isModelPtr {
		modelType = modelType.Elem()
	}
	var pbBase unsafe.Pointer
	var pbSize uintptr
	if !isPBPtr {
		pbBase = pbs.UnsafePointer()
		pbSize = pbType.Size()
	}
	if !isPBPtr && cache.mergedCopyFunc != nil && len(cache.slowEntries) == 0 {
		for i := 0; i < modelsVal.Len(); i++ {
			modelPtr := batchElementPtr(modelsVal, modelType, isModelPtr, i)
			if modelPtr != nil {
				cache.mergedCopyFunc(modelPtr, addPtr(pbBase, uintptr(i)*pbSize))
			}
		}
		pbsVal.Set(pbs)
		return nil
	}

	for i := 0; i < modelsVal.Len(); i++ {
		modelPtr := batchElementPtr(modelsVal, modelType, isModelPtr, i)

		if isPBPtr {
			pb := pbs.Index(i)
			pbPtr := reflect.New(pbType)
			if err := bc.convertPtrCached(modelPtr, pbPtr.UnsafePointer(), cache); err != nil {
				return NewBatchError("元素 %d: %v", i, err)
			}

			pb.Set(pbPtr)
		} else {
			pbPtr := addPtr(pbBase, uintptr(i)*pbSize)
			if err := bc.convertPtrCached(modelPtr, pbPtr, cache); err != nil {
				return NewBatchError("element %d: %v", i, err)
			}
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
//
// Deprecated: 使用泛型函数 SafeFromPBs[P, M] 替代，类型更安全、调用更简洁
// 示例: models, result := pbmo.SafeFromPBs[UserPB, UserModel](pbs)
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
	cache := bc.pbToModelFieldCache()

	for i := 0; i < pbsVal.Len(); i++ {
		pb := pbsVal.Index(i)
		item := BatchItem{Index: i}

		modelPtr := reflect.New(modelType)
		if err := bc.convertCached(pb, modelPtr, cache); err != nil {
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
//
// Deprecated: 使用泛型函数 SafeToPBs[M, P] 替代，类型更安全、调用更简洁
// 示例: pbs, result := pbmo.SafeToPBs[UserModel, UserPB](models)
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
	cache := bc.modelToPBFieldCache()

	for i := 0; i < modelsVal.Len(); i++ {
		model := modelsVal.Index(i)
		item := BatchItem{Index: i}

		pbPtr := reflect.New(pbType)
		if err := bc.convertCached(model, pbPtr, cache); err != nil {
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
