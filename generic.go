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
)

// typePair 类型对，用于缓存 key
type typePair [2]reflect.Type

// converterCache 转换器缓存，用于存储已注册的转换器
var converterCache sync.Map

// typeKey 生成类型对 key
func typeKey[M any, P any]() typePair {
	return typePair{
		reflect.TypeOf((*M)(nil)).Elem(),
		reflect.TypeOf((*P)(nil)).Elem(),
	}
}

// getOrInitConverter 获取或初始化转换器
func getOrInitConverter[M any, P any]() *BidiConverter {
	key := typeKey[M, P]()
	if c, ok := converterCache.Load(key); ok {
		return c.(*BidiConverter)
	}
	c := NewBidiConverter(new(P), new(M)).WithAutoTimeConversion(true)
	actual, _ := converterCache.LoadOrStore(key, c)
	return actual.(*BidiConverter)
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
	return c
}

// ToPB 将 Model 转换为 PB 消息，nil 输入返回 nil
func ToPB[M any, P any](m *M) (*P, error) {
	if m == nil {
		return nil, nil
	}
	info := new(P)
	return info, getOrInitConverter[M, P]().ConvertModelToPB(m, info)
}

// FromPB 将 PB 消息转换为 Model，nil 输入返回 nil
func FromPB[P any, M any](pb *P) (*M, error) {
	if pb == nil {
		return nil, nil
	}
	model := new(M)
	return model, getOrInitConverter[M, P]().ConvertPBToModel(pb, model)
}

// ToPBs 批量将 Model 切片转换为 PB 切片
// nil 元素转换为对应零值 PB，不返回错误
func ToPBs[M any, P any](models []*M) ([]*P, error) {
	pbs := make([]*P, len(models))
	c := getOrInitConverter[M, P]()
	for i, m := range models {
		p := new(P)
		if err := c.ConvertModelToPB(m, p); err != nil {
			return nil, NewBatchError("元素 %d: %v", i, err)
		}
		pbs[i] = p
	}
	return pbs, nil
}

// FromPBs 批量将 PB 切片转换为 Model 切片
// nil 元素转换为对应零值 Model，不返回错误
func FromPBs[P any, M any](pbs []*P) ([]*M, error) {
	models := make([]*M, len(pbs))
	c := getOrInitConverter[M, P]()
	for i, pb := range pbs {
		m := new(M)
		if err := c.ConvertPBToModel(pb, m); err != nil {
			return nil, NewBatchError("元素 %d: %v", i, err)
		}
		models[i] = m
	}
	return models, nil
}

// SafeToPBs 安全批量将 Model 切片转换为 PB 切片，不因单个失败而中断
// nil 元素标记为失败，输出对应位置保持 nil
func SafeToPBs[M any, P any](models []*M) ([]*P, *BatchResult) {
	pbs := make([]*P, len(models))
	result := &BatchResult{Results: make([]BatchItem, 0, len(models))}
	c := getOrInitConverter[M, P]()

	for i, m := range models {
		item := BatchItem{Index: i}
		if m == nil {
			item.Success = false
			item.Error = NewConversionError("元素 %d: 输入为 nil", i)
			result.FailureCount++
			result.Results = append(result.Results, item)
			continue
		}
		p := new(P)
		if err := c.ConvertModelToPB(m, p); err != nil {
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

	return pbs, result
}

// SafeFromPBs 安全批量将 PB 切片转换为 Model 切片，不因单个失败而中断
// nil 元素标记为失败，输出对应位置保持 nil
func SafeFromPBs[P any, M any](pbs []*P) ([]*M, *BatchResult) {
	models := make([]*M, len(pbs))
	result := &BatchResult{Results: make([]BatchItem, 0, len(pbs))}
	c := getOrInitConverter[M, P]()

	for i, pb := range pbs {
		item := BatchItem{Index: i}
		if pb == nil {
			item.Success = false
			item.Error = NewConversionError("元素 %d: 输入为 nil", i)
			result.FailureCount++
			result.Results = append(result.Results, item)
			continue
		}
		m := new(M)
		if err := c.ConvertPBToModel(pb, m); err != nil {
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
