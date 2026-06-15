/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-04-23 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-23 20:53:18
 * @FilePath: \go-pbmo\generic.go
 * @Description: 泛型便捷函数 - 一行注册、一行转换
 * 利用 reflect.Type 做 key + sync.Map 缓存，消除 Model 层样板代码
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"reflect"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/kamalyes/go-toolbox/pkg/types"
)

// typePair 类型对，用于缓存 key
type typePair [2]reflect.Type

// converterCache 转换器缓存，用于存储已注册的转换器
var converterCache sync.Map

// lastConverter 最近使用的转换器，用于缓存
var lastConverter atomic.Pointer[cachedConverter]

// cachedConverter 缓存的转换器，包含 key 和转换器实例
type cachedConverter struct {
	key       typePair
	converter *BidiConverter
}

// typeKey 生成类型对 key
func typeKey[M any, P any]() typePair {
	return typePair{
		reflect.TypeFor[M](),
		reflect.TypeFor[P](),
	}
}

// getOrInitConverter 获取或初始化转换器
func getOrInitConverter[M any, P any]() *BidiConverter {
	key := typeKey[M, P]()
	if cached := lastConverter.Load(); cached != nil && cached.key == key {
		return cached.converter
	}
	if c, ok := converterCache.Load(key); ok {
		converter := c.(*BidiConverter)
		lastConverter.Store(&cachedConverter{key: key, converter: converter})
		return converter
	}
	c := NewBidiConverter(new(P), new(M)).WithAutoTimeConversion(true)
	actual, _ := converterCache.LoadOrStore(key, c)
	converter := actual.(*BidiConverter)
	lastConverter.Store(&cachedConverter{key: key, converter: converter})
	return converter
}

// Register[M, P] 注册 Model-PB 转换对（使用默认配置）
func Register[M any, P any]() *BidiConverter {
	return getOrInitConverter[M, P]()
}

// RegisterWith[M, P] 注册 Model-PB 转换对（自定义配置）
func RegisterWith[M any, P any](opts ...Option) *BidiConverter {
	key := typeKey[M, P]()
	c := NewBidiConverter(new(P), new(M), opts...)
	converterCache.Store(key, c)
	lastConverter.Store(&cachedConverter{key: key, converter: c})
	return c
}

// ToPB 将 Model 转换为 PB 消息，nil 输入返回 nil
func ToPB[M any, P any](m *M) (*P, error) {
	if m == nil {
		return nil, nil
	}
	info := new(P)
	c := getOrInitConverter[M, P]()
	cache := c.modelToPBFieldCache()
	if cache.mergedCopyFunc != nil && !cache.hasTransformers && len(cache.slowEntries) == 0 {
		cache.mergedCopyFunc(unsafe.Pointer(m), unsafe.Pointer(info))
		return info, nil
	}
	return info, c.convertModelToPBPtrCached(unsafe.Pointer(m), unsafe.Pointer(info), cache)
}

// FromPB 将 PB 消息转换为 Model，nil 输入返回 nil
func FromPB[P any, M any](pb *P) (*M, error) {
	if pb == nil {
		return nil, nil
	}
	model := new(M)
	c := getOrInitConverter[M, P]()
	cache := c.pbToModelFieldCache()
	if cache.mergedCopyFunc != nil && !cache.hasTransformers && len(cache.slowEntries) == 0 {
		cache.mergedCopyFunc(unsafe.Pointer(pb), unsafe.Pointer(model))
		return model, nil
	}
	return model, c.convertPBToModelPtrCached(unsafe.Pointer(pb), unsafe.Pointer(model), cache)
}

// ToPBs 批量将 Model 指针切片转换为 PB 切片
func ToPBs[M any, P any](models []*M) ([]*P, error) {
	pbs := make([]*P, len(models))
	if len(models) == 0 {
		return pbs, nil
	}
	values := make([]P, len(models))
	c := getOrInitConverter[M, P]()
	cache := c.modelToPBFieldCache()
	if cache.mergedCopyFunc != nil && !cache.hasTransformers && len(cache.slowEntries) == 0 {
		for i, m := range models {
			p := &values[i]
			if m != nil {
				cache.mergedCopyFunc(unsafe.Pointer(m), unsafe.Pointer(p))
			}
			pbs[i] = p
		}
		return pbs, nil
	}
	for i, m := range models {
		p := &values[i]
		if err := c.convertModelToPBPtrCached(unsafe.Pointer(m), unsafe.Pointer(p), cache); err != nil {
			return nil, NewBatchError("元素 %d: %v", i, err)
		}
		pbs[i] = p
	}
	return pbs, nil
}

// FromPBs 批量将 PB 切片转换为 Model 指针切片
// nil 元素会被跳过，对应的 model 保持零值
func FromPBs[P any, M any](pbs []*P) ([]*M, error) {
	models := make([]*M, len(pbs))
	if len(pbs) == 0 {
		return models, nil
	}
	values := make([]M, len(pbs))
	c := getOrInitConverter[M, P]()
	cache := c.pbToModelFieldCache()
	if cache.mergedCopyFunc != nil && !cache.hasTransformers && len(cache.slowEntries) == 0 {
		for i, pb := range pbs {
			m := &values[i]
			if pb != nil {
				cache.mergedCopyFunc(unsafe.Pointer(pb), unsafe.Pointer(m))
			}
			models[i] = m
		}
		return models, nil
	}
	for i, pb := range pbs {
		m := &values[i]
		if pb == nil {
			models[i] = m
			continue
		}
		if err := c.convertPBToModelPtrCached(unsafe.Pointer(pb), unsafe.Pointer(m), cache); err != nil {
			return nil, NewBatchError("元素 %d: %v", i, err)
		}
		models[i] = m
	}
	return models, nil
}

// SafeToPBs 安全批量将 Model 切片转换为 PB 切片，不因单个失败而中断
// 自动支持 []M（值切片）和 []*M（指针切片），nil 指针元素标记为失败
func SafeToPBs[M any, P any](models []M) ([]*P, *BatchResult) {
	pbs := make([]*P, len(models))
	result := &BatchResult{Results: make([]BatchItem, 0, len(models))}
	if len(models) == 0 {
		return pbs, result
	}
	values := make([]P, len(models))

	mType := reflect.TypeOf(models).Elem()

	if mType.Kind() == reflect.Ptr {
		elemType := mType.Elem()
		pbType := reflect.TypeFor[P]()
		c, ok := findConverter(elemType, pbType)
		if !ok {
			c = getOrInitConverter[M, P]()
		}
		cache := c.modelToPBFieldCache()
		pureFast := cache.mergedCopyFunc != nil && !cache.hasTransformers && len(cache.slowEntries) == 0
		for i := range models {
			item := BatchItem{Index: i}
			v := reflect.ValueOf(models[i])
			if v.IsNil() {
				item.Success = false
				item.Error = NewConversionError("元素 %d: 输入为 nil", i)
				result.FailureCount++
				result.Results = append(result.Results, item)
				continue
			}
			p := &values[i]
			var err error
			if pureFast {
				cache.mergedCopyFunc(unsafe.Pointer(v.Pointer()), unsafe.Pointer(p))
			} else {
				err = c.convertModelToPBPtrCached(unsafe.Pointer(v.Pointer()), unsafe.Pointer(p), cache)
			}
			if err != nil {
				item.Success = false
				item.Error = err
				result.FailureCount++
			} else {
				item.Success = true
				item.Value = p
				pbs[i] = p
				result.SuccessCount++
			}
			result.Results = append(result.Results, item)
		}
	} else {
		c := getOrInitConverter[M, P]()
		cache := c.modelToPBFieldCache()
		pureFast := cache.mergedCopyFunc != nil && !cache.hasTransformers && len(cache.slowEntries) == 0
		for i := range models {
			item := BatchItem{Index: i}
			p := &values[i]
			var err error
			if pureFast {
				cache.mergedCopyFunc(unsafe.Pointer(&models[i]), unsafe.Pointer(p))
			} else {
				err = c.convertModelToPBPtrCached(unsafe.Pointer(&models[i]), unsafe.Pointer(p), cache)
			}
			if err != nil {
				item.Success = false
				item.Error = err
				result.FailureCount++
			} else {
				item.Success = true
				item.Value = p
				pbs[i] = p
				result.SuccessCount++
			}
			result.Results = append(result.Results, item)
		}
	}

	return pbs, result
}

// SafeFromPBs 安全批量将 PB 切片转换为 Model 切片，不因单个失败而中断
// 接受 []*P（指针切片），nil 元素标记为失败
func SafeFromPBs[P any, M any](pbs []*P) ([]*M, *BatchResult) {
	models := make([]*M, len(pbs))
	result := &BatchResult{Results: make([]BatchItem, 0, len(pbs))}
	if len(pbs) == 0 {
		return models, result
	}
	values := make([]M, len(pbs))
	c := getOrInitConverter[M, P]()
	cache := c.pbToModelFieldCache()
	pureFast := cache.mergedCopyFunc != nil && !cache.hasTransformers && len(cache.slowEntries) == 0

	for i, pb := range pbs {
		item := BatchItem{Index: i}
		if pb == nil {
			item.Success = false
			item.Error = NewConversionError("元素 %d: 输入为 nil", i)
			result.FailureCount++
			result.Results = append(result.Results, item)
			continue
		}
		m := &values[i]
		var err error
		if pureFast {
			cache.mergedCopyFunc(unsafe.Pointer(pb), unsafe.Pointer(m))
		} else {
			err = c.convertPBToModelPtrCached(unsafe.Pointer(pb), unsafe.Pointer(m), cache)
		}
		if err != nil {
			item.Success = false
			item.Error = err
			result.FailureCount++
		} else {
			item.Success = true
			item.Value = m
			models[i] = m
			result.SuccessCount++
		}
		result.Results = append(result.Results, item)
	}

	return models, result
}

// ConverterFor 获取已注册的转换器
func ConverterFor[M any, P any]() *BidiConverter {
	return getOrInitConverter[M, P]()
}

// PBToUpdates 将 PB 消息转换为 map[string]interface{}
// 遍历 proto 字段，使用 protobuf json tag 名作为 key（snake_case）
// 自动跳过零值、protobuf 内部字段，自动解包 wrapper 类型
func PBToUpdates[P any](pb *P) map[string]interface{} {
	if pb == nil {
		return map[string]interface{}{}
	}

	v := reflect.ValueOf(pb).Elem()

	t := v.Type()
	builder := NewUpdates()

	for i := 0; i < v.NumField(); i++ {
		fieldVal := v.Field(i)
		fieldType := t.Field(i)

		if !fieldVal.CanInterface() {
			continue
		}

		key := types.ResolvePBKey(fieldType)
		if key == "" || key == "-" {
			continue
		}

		iface := fieldVal.Interface()
		if isEmptyUpdateValue(iface) {
			continue
		}

		builder.Set(key, types.UnwrapPBValue(iface))
	}

	return builder.Build()
}

// ModelToUpdates 将 Model 结构体转换为 map[string]interface{}
// 使用 gorm column tag 或 json tag 作为 key，跳过零值字段和不可导出字段
func ModelToUpdates[M any](m *M) map[string]interface{} {
	if m == nil {
		return map[string]interface{}{}
	}

	v := reflect.ValueOf(m).Elem()

	t := v.Type()
	builder := NewUpdates()

	for i := 0; i < v.NumField(); i++ {
		fieldVal := v.Field(i)
		fieldType := t.Field(i)

		if !fieldVal.CanInterface() {
			continue
		}

		key := types.ResolveModelKey(fieldType)
		if key == "-" {
			continue
		}

		iface := fieldVal.Interface()
		if isEmptyUpdateValue(iface) {
			continue
		}

		builder.Set(key, types.UnwrapModelValue(iface))
	}

	return builder.Build()
}
