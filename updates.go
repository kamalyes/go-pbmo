/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-04-23 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-23 20:52:55
 * @FilePath: \go-pbmo\updates.go
 * @Description: 更新字段构建器 - 链式构建 map[string]interface{}
 * 灵感来自 go-toolbox/httpx.ParamsBuilder 和 go-sqlbuilder/AddFilterIfNotEmpty
 * 核心理念：只管传值，不用写 if，自动跳过空值/零值/nil 指针
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"fmt"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"reflect"
	"strings"
)

// UpdatesBuilder 更新字段构建器
type UpdatesBuilder struct {
	updates map[string]interface{}
}

// NewUpdates 初始化字段构建器
func NewUpdates() *UpdatesBuilder {
	return &UpdatesBuilder{
		updates: make(map[string]interface{}),
	}
}

// --- 无条件设置 ---

func (b *UpdatesBuilder) Set(key string, value interface{}) *UpdatesBuilder {
	b.updates[key] = value
	return b
}

func (b *UpdatesBuilder) SetIf(condition bool, key string, value interface{}) *UpdatesBuilder {
	if condition {
		b.updates[key] = value
	}
	return b
}

// --- IfNotEmpty 系列：值非空时才设置 ---

func (b *UpdatesBuilder) SetIfNotEmpty(key string, value string) *UpdatesBuilder {
	if value != "" {
		b.updates[key] = value
	}
	return b
}

// --- IfNotNil 系列：指针不为 nil 时才设置 ---

func (b *UpdatesBuilder) SetIfNotNil(key string, value interface{}) *UpdatesBuilder {
	if value != nil {
		b.updates[key] = value
	}
	return b
}

// --- IfNotZero 系列：非零值时才设置（使用反射判断） ---

func (b *UpdatesBuilder) SetIfNotZero(key string, value interface{}) *UpdatesBuilder {
	if !isZeroValue(value) {
		b.updates[key] = value
	}
	return b
}

// --- PB Wrapper 类型便捷方法 ---

func (b *UpdatesBuilder) SetStringVal(key string, val *wrapperspb.StringValue) *UpdatesBuilder {
	if val != nil && val.Value != "" {
		b.updates[key] = val.Value
	}
	return b
}

func (b *UpdatesBuilder) SetStringValAny(key string, val *wrapperspb.StringValue) *UpdatesBuilder {
	if val != nil {
		b.updates[key] = val.Value
	}
	return b
}

func (b *UpdatesBuilder) SetInt32Val(key string, val *wrapperspb.Int32Value) *UpdatesBuilder {
	if val != nil {
		b.updates[key] = val.Value
	}
	return b
}

func (b *UpdatesBuilder) SetInt64Val(key string, val *wrapperspb.Int64Value) *UpdatesBuilder {
	if val != nil {
		b.updates[key] = val.Value
	}
	return b
}

func (b *UpdatesBuilder) SetBoolVal(key string, val *wrapperspb.BoolValue) *UpdatesBuilder {
	if val != nil {
		b.updates[key] = val.Value
	}
	return b
}

func (b *UpdatesBuilder) SetFloatVal(key string, val *wrapperspb.FloatValue) *UpdatesBuilder {
	if val != nil {
		b.updates[key] = val.Value
	}
	return b
}

func (b *UpdatesBuilder) SetDoubleVal(key string, val *wrapperspb.DoubleValue) *UpdatesBuilder {
	if val != nil {
		b.updates[key] = val.Value
	}
	return b
}

// --- 管理方法 ---

func (b *UpdatesBuilder) Delete(key string) *UpdatesBuilder {
	delete(b.updates, key)
	return b
}

func (b *UpdatesBuilder) Has(key string) bool {
	_, ok := b.updates[key]
	return ok
}

func (b *UpdatesBuilder) Len() int {
	return len(b.updates)
}

func (b *UpdatesBuilder) IsEmpty() bool {
	return len(b.updates) == 0
}

func (b *UpdatesBuilder) Clear() *UpdatesBuilder {
	b.updates = make(map[string]interface{})
	return b
}

func (b *UpdatesBuilder) Merge(other *UpdatesBuilder) *UpdatesBuilder {
	if other != nil {
		for k, v := range other.updates {
			b.updates[k] = v
		}
	}
	return b
}

func (b *UpdatesBuilder) Build() map[string]interface{} {
	return b.updates
}

func (b *UpdatesBuilder) String() string {
	parts := make([]string, 0, len(b.updates))
	for k, v := range b.updates {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func isZeroValue(v interface{}) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		return rv.IsNil()
	}
	return rv.Interface() == reflect.Zero(rv.Type()).Interface()
}
