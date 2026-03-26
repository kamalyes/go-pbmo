/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-04-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-20 00:00:00
 * @FilePath: \go-pbmo\transform.go
 * @Description: 字段转换器 - 注册和应用字段级别的转换函数
 * 注意：TransformerFunc 为函数类型，不可比较（non-comparable），
 * 因此无法直接使用 syncx.Map（要求 V comparable），
 * 改用 sync.Map 并封装为类型安全的 TransformerRegistry
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"reflect"
	"sync"
)

// TransformerFunc 字段转换函数类型
// 输入原始字段值，输出转换后的值
type TransformerFunc func(interface{}) interface{}

// TransformerRegistry 字段转换器注册表
// 封装 sync.Map 提供类型安全的字段转换器管理，天然并发安全
type TransformerRegistry struct {
	transformers sync.Map
}

// NewTransformerRegistry 创建字段转换器注册表
func NewTransformerRegistry() *TransformerRegistry {
	return &TransformerRegistry{}
}

// Register 注册字段转换器
func (tr *TransformerRegistry) Register(field string, fn TransformerFunc) {
	tr.transformers.Store(field, fn)
}

// RegisterBatch 批量注册字段转换器
func (tr *TransformerRegistry) RegisterBatch(transformers map[string]TransformerFunc) {
	for field, fn := range transformers {
		tr.transformers.Store(field, fn)
	}
}

// Lookup 查找字段转换器
func (tr *TransformerRegistry) Lookup(field string) (TransformerFunc, bool) {
	val, ok := tr.transformers.Load(field)
	if !ok {
		return nil, false
	}
	return val.(TransformerFunc), true
}

// Apply 应用字段转换器
// 如果字段有注册的转换器，则应用转换；否则返回原始值
func (tr *TransformerRegistry) Apply(field string, value reflect.Value) reflect.Value {
	fn, ok := tr.Lookup(field)
	if !ok {
		return value
	}
	return reflect.ValueOf(fn(value.Interface()))
}

// Has 检查字段是否有注册的转换器
func (tr *TransformerRegistry) Has(field string) bool {
	_, ok := tr.transformers.Load(field)
	return ok
}

// Remove 移除字段转换器
func (tr *TransformerRegistry) Remove(field string) {
	tr.transformers.Delete(field)
}

// Clear 清空所有转换器
func (tr *TransformerRegistry) Clear() {
	tr.transformers.Range(func(key, value interface{}) bool {
		tr.transformers.Delete(key)
		return true
	})
}

// Fields 获取所有已注册的字段名
func (tr *TransformerRegistry) Fields() []string {
	var fields []string
	tr.transformers.Range(func(key, value interface{}) bool {
		fields = append(fields, key.(string))
		return true
	})
	return fields
}

// Count 获取已注册的转换器数量
func (tr *TransformerRegistry) Count() int {
	count := 0
	tr.transformers.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

// Clone 克隆转换器注册表
func (tr *TransformerRegistry) Clone() *TransformerRegistry {
	cloned := NewTransformerRegistry()
	tr.transformers.Range(func(key, value interface{}) bool {
		cloned.transformers.Store(key, value)
		return true
	})
	return cloned
}
