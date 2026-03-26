/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2023-09-26 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2023-09-26 00:00:00
 * @FilePath: \go-pbmo\option.go
 * @Description: 选项模式 - 使用 Functional Options 配置转换器
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package pbmo

import "time"

// Option 转换器配置选项函数
type Option func(*Options)

// Options 转换器配置项
type Options struct {
	AutoTimeConversion bool              // 自动时间转换开关
	FieldMapping       map[string]string // 字段名映射: Model字段名 -> PB字段名
	TagMappingEnabled  bool              // 是否启用 struct tag 自动映射
	TagName            string            // struct tag 名称（默认 pbmo）
	ValidationEnabled  bool              // 是否启用校验
	DesensitizeEnabled bool              // 是否启用脱敏
	SafeMode           bool              // 是否启用安全模式
	Concurrency        int               // 并发数（批量转换时使用）
	BatchSize          int               // 批处理大小
	Timeout            time.Duration     // 超时时间
}

// DefaultOptions 默认配置
func DefaultOptions() *Options {
	return &Options{
		AutoTimeConversion: true,
		FieldMapping:       make(map[string]string),
		TagMappingEnabled:  true,
		TagName:            "pbmo",
		ValidationEnabled:  false,
		DesensitizeEnabled: false,
		SafeMode:           false,
		Concurrency:        0,
		BatchSize:          100,
		Timeout:            30 * time.Second,
	}
}

// ApplyOptions 应用选项到默认配置
func ApplyOptions(opts ...Option) *Options {
	o := DefaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// WithAutoTimeConversion 设置自动时间转换
func WithAutoTimeConversion(enabled bool) Option {
	return func(o *Options) {
		o.AutoTimeConversion = enabled
	}
}

// WithFieldMapping 设置字段映射
func WithFieldMapping(modelField, pbField string) Option {
	return func(o *Options) {
		o.FieldMapping[modelField] = pbField
	}
}

// WithFieldMappings 批量设置字段映射
func WithFieldMappings(mappings map[string]string) Option {
	return func(o *Options) {
		for k, v := range mappings {
			o.FieldMapping[k] = v
		}
	}
}

// WithTagMapping 启用/禁用 struct tag 映射
func WithTagMapping(enabled bool) Option {
	return func(o *Options) {
		o.TagMappingEnabled = enabled
	}
}

// WithTagName 设置 struct tag 名称
func WithTagName(name string) Option {
	return func(o *Options) {
		o.TagName = name
	}
}

// WithValidation 启用/禁用校验
func WithValidation(enabled bool) Option {
	return func(o *Options) {
		o.ValidationEnabled = enabled
	}
}

// WithDesensitize 启用/禁用脱敏
func WithDesensitize(enabled bool) Option {
	return func(o *Options) {
		o.DesensitizeEnabled = enabled
	}
}

// WithSafeMode 启用/禁用安全模式
func WithSafeMode(enabled bool) Option {
	return func(o *Options) {
		o.SafeMode = enabled
	}
}

// WithConcurrency 设置并发数
func WithConcurrency(n int) Option {
	return func(o *Options) {
		o.Concurrency = n
	}
}

// WithBatchSize 设置批处理大小
func WithBatchSize(size int) Option {
	return func(o *Options) {
		o.BatchSize = size
	}
}

// WithTimeout 设置超时时间
func WithTimeout(timeout time.Duration) Option {
	return func(o *Options) {
		o.Timeout = timeout
	}
}
