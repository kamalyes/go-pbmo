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

// ToPB 将 Model 转换为 PB 消息
func ToPB[M any, P any](m *M) (*P, error) {
	info := new(P)
	return info, getOrInitConverter[M, P]().ConvertModelToPB(m, info)
}

// FromPB 将 PB 消息转换为 Model
func FromPB[P any, M any](pb *P) (*M, error) {
	model := new(M)
	return model, getOrInitConverter[M, P]().ConvertPBToModel(pb, model)
}

// ConverterFor 获取已注册的转换器
func ConverterFor[M any, P any]() *BidiConverter {
	return getOrInitConverter[M, P]()
}
