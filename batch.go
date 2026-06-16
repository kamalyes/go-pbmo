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

// convertPBToModelPtrCached 批量将 PB 指针转换为 Model 指针，使用缓存优化
func (bc *BidiConverter) convertPBToModelPtrCached(pbPtr, modelPtr unsafe.Pointer, cache *fieldCache) error {
	if pbPtr == nil || modelPtr == nil {
		return nil
	}
	if cache.mergedCopyFunc != nil && !cache.hasTransformers {
		cache.mergedCopyFunc(pbPtr, modelPtr)
		if len(cache.slowEntries) == 0 {
			return nil
		}
	} else if len(cache.fastEntries) > 0 {
		if cache.mergedCopyFunc != nil {
			cache.mergedCopyFunc(pbPtr, modelPtr)
		} else {
			for i := range cache.fastEntries {
				cache.fastEntries[i].copyFunc(pbPtr, modelPtr)
			}
		}
	}
	if len(cache.slowEntries) == 0 {
		return nil
	}

	// 使用预计算的偏移量直接定位字段，避免 FieldByIndex 的开销
	for i := range cache.slowEntries {
		entry := &cache.slowEntries[i]
		srcField := reflect.NewAt(entry.srcType, addPtr(pbPtr, entry.srcOffset)).Elem()
		dstField := reflect.NewAt(entry.dstType, addPtr(modelPtr, entry.dstOffset)).Elem()
		if !srcField.IsValid() || !dstField.IsValid() {
			continue
		}
		if cache.hasTransformers && bc.transformers.Has(entry.srcName) {
			srcField = bc.transformers.Apply(entry.srcName, srcField)
		}
		if err := convertFieldByKind(srcField, dstField, entry); err != nil {
			return NewConversionError("field %s conversion failed: %v", entry.srcName, err)
		}
	}
	return nil
}

// convertModelToPBPtrCached 批量将 Model 指针转换为 PB 指针，使用缓存优化
func (bc *BidiConverter) convertModelToPBPtrCached(modelPtr, pbPtr unsafe.Pointer, cache *fieldCache) error {
	if modelPtr == nil || pbPtr == nil {
		return nil
	}
	if cache.mergedCopyFunc != nil && !cache.hasTransformers {
		cache.mergedCopyFunc(modelPtr, pbPtr)
		if len(cache.slowEntries) == 0 {
			return nil
		}
	} else if len(cache.fastEntries) > 0 {
		if cache.mergedCopyFunc != nil {
			cache.mergedCopyFunc(modelPtr, pbPtr)
		} else {
			for i := range cache.fastEntries {
				cache.fastEntries[i].copyFunc(modelPtr, pbPtr)
			}
		}
	}
	if len(cache.slowEntries) == 0 {
		return nil
	}

	// 使用预计算的偏移量直接定位字段，避免 FieldByIndex 的开销
	for i := range cache.slowEntries {
		entry := &cache.slowEntries[i]
		srcField := reflect.NewAt(entry.srcType, addPtr(modelPtr, entry.srcOffset)).Elem()
		dstField := reflect.NewAt(entry.dstType, addPtr(pbPtr, entry.dstOffset)).Elem()
		if !srcField.IsValid() || !dstField.IsValid() {
			continue
		}
		if cache.hasTransformers && bc.transformers.Has(entry.srcName) {
			srcField = bc.transformers.Apply(entry.srcName, srcField)
		}
		if err := convertFieldByKind(srcField, dstField, entry); err != nil {
			return NewConversionError("field %s conversion failed: %v", entry.srcName, err)
		}
	}
	return nil
}

// convertPBToModelCached 批量将 PB 结构体转换为 Model 结构体，使用缓存优化
func (bc *BidiConverter) convertPBToModelCached(pbVal, modelVal reflect.Value, cache *fieldCache) error {
	if !pbVal.IsValid() || !modelVal.IsValid() {
		return nil
	}
	if modelVal.Kind() == reflect.Ptr {
		if modelVal.IsNil() {
			return ErrMustBePointer
		}
		modelVal = modelVal.Elem()
	}
	if pbVal.Kind() == reflect.Ptr {
		if pbVal.IsNil() {
			return nil
		}
		pbVal = pbVal.Elem()
	}

	canFastPath := pbVal.CanAddr() && modelVal.CanAddr()
	if canFastPath && cache.mergedCopyFunc != nil && !cache.hasTransformers {
		srcBase := unsafe.Pointer(pbVal.UnsafeAddr())
		dstBase := unsafe.Pointer(modelVal.UnsafeAddr())
		cache.mergedCopyFunc(srcBase, dstBase)
		if len(cache.slowEntries) == 0 {
			return nil
		}
	} else if canFastPath && len(cache.fastEntries) > 0 {
		srcBase := unsafe.Pointer(pbVal.UnsafeAddr())
		dstBase := unsafe.Pointer(modelVal.UnsafeAddr())
		if cache.mergedCopyFunc != nil {
			cache.mergedCopyFunc(srcBase, dstBase)
		} else {
			for i := range cache.fastEntries {
				cache.fastEntries[i].copyFunc(srcBase, dstBase)
			}
		}
	} else if len(cache.fastEntries) > 0 {
		for i := range cache.fastEntries {
			entry := &cache.fastEntries[i]
			srcField := pbVal.FieldByIndex(entry.srcIndex)
			dstField := modelVal.FieldByIndex(entry.dstIndex)
			if !srcField.IsValid() || !dstField.IsValid() || !dstField.CanSet() {
				continue
			}
			if err := convertFastEntryByReflect(srcField, dstField, entry); err != nil {
				return NewConversionError("字段 %s 转换失败: %v", entry.srcName, err)
			}
		}
	}

	if len(cache.slowEntries) == 0 {
		return nil
	}

	// 优先使用 unsafe 指针直接定位字段，避免 FieldByIndex 开销
	if canFastPath {
		srcBase := unsafe.Pointer(pbVal.UnsafeAddr())
		dstBase := unsafe.Pointer(modelVal.UnsafeAddr())
		for i := range cache.slowEntries {
			entry := &cache.slowEntries[i]
			srcField := reflect.NewAt(entry.srcType, addPtr(srcBase, entry.srcOffset)).Elem()
			dstField := reflect.NewAt(entry.dstType, addPtr(dstBase, entry.dstOffset)).Elem()
			if !srcField.IsValid() || !dstField.IsValid() {
				continue
			}
			if cache.hasTransformers && bc.transformers.Has(entry.srcName) {
				srcField = bc.transformers.Apply(entry.srcName, srcField)
			}
			if err := convertFieldByKind(srcField, dstField, entry); err != nil {
				return NewConversionError("字段 %s 转换失败: %v", entry.srcName, err)
			}
		}
	} else {
		for i := range cache.slowEntries {
			entry := &cache.slowEntries[i]
			srcField := pbVal.FieldByIndex(entry.srcIndex)
			dstField := modelVal.FieldByIndex(entry.dstIndex)
			if !srcField.IsValid() || !dstField.IsValid() {
				continue
			}
			if cache.hasTransformers && bc.transformers.Has(entry.srcName) {
				srcField = bc.transformers.Apply(entry.srcName, srcField)
			}
			if err := convertFieldByKind(srcField, dstField, entry); err != nil {
				return NewConversionError("字段 %s 转换失败: %v", entry.srcName, err)
			}
		}
	}
	return nil
}

// convertModelToPBCached 批量将 Model 结构体转换为 PB 结构体，使用缓存优化
func (bc *BidiConverter) convertModelToPBCached(modelVal, pbVal reflect.Value, cache *fieldCache) error {
	if !modelVal.IsValid() || !pbVal.IsValid() {
		return nil
	}
	if pbVal.Kind() == reflect.Ptr {
		if pbVal.IsNil() {
			return ErrMustBePointer
		}
		pbVal = pbVal.Elem()
	}
	if modelVal.Kind() == reflect.Ptr {
		if modelVal.IsNil() {
			return nil
		}
		modelVal = modelVal.Elem()
	}

	canFastPath := modelVal.CanAddr() && pbVal.CanAddr()
	if canFastPath && cache.mergedCopyFunc != nil && !cache.hasTransformers {
		srcBase := unsafe.Pointer(modelVal.UnsafeAddr())
		dstBase := unsafe.Pointer(pbVal.UnsafeAddr())
		cache.mergedCopyFunc(srcBase, dstBase)
		if len(cache.slowEntries) == 0 {
			return nil
		}
	} else if canFastPath && len(cache.fastEntries) > 0 {
		srcBase := unsafe.Pointer(modelVal.UnsafeAddr())
		dstBase := unsafe.Pointer(pbVal.UnsafeAddr())
		if cache.mergedCopyFunc != nil {
			cache.mergedCopyFunc(srcBase, dstBase)
		} else {
			for i := range cache.fastEntries {
				cache.fastEntries[i].copyFunc(srcBase, dstBase)
			}
		}
	} else if len(cache.fastEntries) > 0 {
		for i := range cache.fastEntries {
			entry := &cache.fastEntries[i]
			srcField := modelVal.FieldByIndex(entry.srcIndex)
			dstField := pbVal.FieldByIndex(entry.dstIndex)
			if !srcField.IsValid() || !dstField.IsValid() || !dstField.CanSet() {
				continue
			}
			if err := convertFastEntryByReflect(srcField, dstField, entry); err != nil {
				return NewConversionError("字段 %s 转换失败: %v", entry.srcName, err)
			}
		}
	}

	if len(cache.slowEntries) == 0 {
		return nil
	}

	// 优先使用 unsafe 指针直接定位字段，避免 FieldByIndex 开销
	if canFastPath {
		srcBase := unsafe.Pointer(modelVal.UnsafeAddr())
		dstBase := unsafe.Pointer(pbVal.UnsafeAddr())
		for i := range cache.slowEntries {
			entry := &cache.slowEntries[i]
			srcField := reflect.NewAt(entry.srcType, addPtr(srcBase, entry.srcOffset)).Elem()
			dstField := reflect.NewAt(entry.dstType, addPtr(dstBase, entry.dstOffset)).Elem()
			if !srcField.IsValid() || !dstField.IsValid() {
				continue
			}
			if cache.hasTransformers && bc.transformers.Has(entry.srcName) {
				srcField = bc.transformers.Apply(entry.srcName, srcField)
			}
			if err := convertFieldByKind(srcField, dstField, entry); err != nil {
				return NewConversionError("字段 %s 转换失败: %v", entry.srcName, err)
			}
		}
	} else {
		for i := range cache.slowEntries {
			entry := &cache.slowEntries[i]
			srcField := modelVal.FieldByIndex(entry.srcIndex)
			dstField := pbVal.FieldByIndex(entry.dstIndex)
			if !srcField.IsValid() || !dstField.IsValid() {
				continue
			}
			if cache.hasTransformers && bc.transformers.Has(entry.srcName) {
				srcField = bc.transformers.Apply(entry.srcName, srcField)
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
	if !isModelPtr && cache.mergedCopyFunc != nil && !cache.hasTransformers && len(cache.slowEntries) == 0 {
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
			if err := bc.convertPBToModelPtrCached(pbPtr, modelPtr.UnsafePointer(), cache); err != nil {
				return NewBatchError("元素 %d: %v", i, err)
			}

			model.Set(modelPtr)
		} else {
			modelPtr := addPtr(modelBase, uintptr(i)*modelSize)
			if err := bc.convertPBToModelPtrCached(pbPtr, modelPtr, cache); err != nil {
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
	if !isPBPtr && cache.mergedCopyFunc != nil && !cache.hasTransformers && len(cache.slowEntries) == 0 {
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
			if err := bc.convertModelToPBPtrCached(modelPtr, pbPtr.UnsafePointer(), cache); err != nil {
				return NewBatchError("元素 %d: %v", i, err)
			}

			pb.Set(pbPtr)
		} else {
			pbPtr := addPtr(pbBase, uintptr(i)*pbSize)
			if err := bc.convertModelToPBPtrCached(modelPtr, pbPtr, cache); err != nil {
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
		if err := bc.convertPBToModelCached(pb, modelPtr, cache); err != nil {
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
		if err := bc.convertModelToPBCached(model, pbPtr, cache); err != nil {
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
