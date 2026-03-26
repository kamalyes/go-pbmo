/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2023-09-26 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2023-09-26 00:00:00
 * @FilePath: \go-pbmo\validate.go
 * @Description: 校验器 - 字段级校验规则定义与执行
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"fmt"
	"reflect"
	"regexp"
	"sync"
)

// FieldRule 字段校验规则
type FieldRule struct {
	Name      string                  // 字段名
	Required  bool                    // 是否必填
	MinLen    int                     // 最小长度
	MaxLen    int                     // 最大长度
	Min       float64                 // 最小值
	Max       float64                 // 最大值
	Pattern   string                  // 正则表达式
	Custom    func(interface{}) error // 自定义校验函数
	Transform TransformerFunc         // 校验通过后的转换函数
}

// ValidationError 单个校验错误
type ValidationError struct {
	Field   string // 字段名
	Message string // 错误信息
}

// ValidationErrors 多个校验错误集合
type ValidationErrors []ValidationError

// Error 实现 error 接口
func (ve ValidationErrors) Error() string {
	if len(ve) == 0 {
		return "校验通过"
	}
	msg := "校验错误:\n"
	for _, e := range ve {
		msg += fmt.Sprintf("  - %s: %s\n", e.Field, e.Message)
	}
	return msg
}

// HasErrors 判断是否有错误
func (ve ValidationErrors) HasErrors() bool {
	return len(ve) > 0
}

// Fields 获取所有错误字段名
func (ve ValidationErrors) Fields() []string {
	fields := make([]string, 0, len(ve))
	for _, e := range ve {
		fields = append(fields, e.Field)
	}
	return fields
}

// Validator 字段校验器
// 管理校验规则并执行校验，支持并发安全
type Validator struct {
	rules map[string][]FieldRule // 结构体名 -> 校验规则列表
	mu    sync.RWMutex
}

// NewValidator 创建校验器
func NewValidator() *Validator {
	return &Validator{
		rules: make(map[string][]FieldRule),
	}
}

// RegisterRules 注册字段校验规则
func (v *Validator) RegisterRules(structName string, rules ...FieldRule) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.rules[structName] = append(v.rules[structName], rules...)
}

// RegisterBatch 批量注册校验规则
func (v *Validator) RegisterBatch(rulesMap map[string][]FieldRule) {
	v.mu.Lock()
	defer v.mu.Unlock()
	for structName, rules := range rulesMap {
		v.rules[structName] = append(v.rules[structName], rules...)
	}
}

// Validate 校验数据
func (v *Validator) Validate(data interface{}) error {
	if data == nil {
		return NewNilValueError("数据不能为nil")
	}

	val := reflect.ValueOf(data)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return NewNilValueError("数据不能为nil")
		}
		val = val.Elem()
	}

	structName := val.Type().Name()
	v.mu.RLock()
	rules, ok := v.rules[structName]
	v.mu.RUnlock()

	if !ok {
		return nil
	}

	var errs ValidationErrors
	for _, rule := range rules {
		field := val.FieldByName(rule.Name)
		if !field.IsValid() {
			continue
		}

		if field.Kind() == reflect.Ptr && field.IsNil() {
			if rule.Required {
				errs = append(errs, ValidationError{
					Field:   rule.Name,
					Message: "必填字段",
				})
			}
			continue
		}

		dereferencedField := field
		if field.Kind() == reflect.Ptr {
			dereferencedField = field.Elem()
		}

		if rule.Required && IsZeroValue(dereferencedField) {
			errs = append(errs, ValidationError{
				Field:   rule.Name,
				Message: "必填字段",
			})
			continue
		}

		if IsZeroValue(dereferencedField) {
			continue
		}

		errs = v.validateField(rule, dereferencedField, errs)
	}

	if errs.HasErrors() {
		return errs
	}
	return nil
}

// validateField 校验单个字段
func (v *Validator) validateField(rule FieldRule, field reflect.Value, errs ValidationErrors) ValidationErrors {
	// 字符串长度校验
	if field.Kind() == reflect.String {
		str := field.String()
		if rule.MinLen > 0 && len(str) < rule.MinLen {
			errs = append(errs, ValidationError{
				Field:   rule.Name,
				Message: fmt.Sprintf("最小长度为 %d", rule.MinLen),
			})
		}
		if rule.MaxLen > 0 && len(str) > rule.MaxLen {
			errs = append(errs, ValidationError{
				Field:   rule.Name,
				Message: fmt.Sprintf("最大长度为 %d", rule.MaxLen),
			})
		}
		if rule.Pattern != "" {
			if match, _ := regexp.MatchString(rule.Pattern, str); !match {
				errs = append(errs, ValidationError{
					Field:   rule.Name,
					Message: "格式不合法",
				})
			}
		}
	}

	// 数值范围校验
	if IsNumeric(field) {
		num := GetNumericValue(field)
		if (rule.Min != 0 || rule.Max != 0) && num < rule.Min {
			errs = append(errs, ValidationError{
				Field:   rule.Name,
				Message: fmt.Sprintf("最小值为 %.2f", rule.Min),
			})
		}
		if rule.Max > 0 && num > rule.Max {
			errs = append(errs, ValidationError{
				Field:   rule.Name,
				Message: fmt.Sprintf("最大值为 %.2f", rule.Max),
			})
		}
	}

	// 自定义校验
	if rule.Custom != nil {
		if err := rule.Custom(field.Interface()); err != nil {
			errs = append(errs, ValidationError{
				Field:   rule.Name,
				Message: err.Error(),
			})
		}
	}

	return errs
}

// ValidateWithTransform 校验并转换数据
func (v *Validator) ValidateWithTransform(data interface{}) (interface{}, error) {
	val := reflect.ValueOf(data)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil, NewNilValueError("数据不能为nil")
		}
		val = val.Elem()
	}

	if err := v.Validate(data); err != nil {
		return nil, err
	}

	structName := val.Type().Name()
	v.mu.RLock()
	rules, ok := v.rules[structName]
	v.mu.RUnlock()

	if !ok {
		return data, nil
	}

	for _, rule := range rules {
		if rule.Transform == nil {
			continue
		}
		field := val.FieldByName(rule.Name)
		if !field.IsValid() || !field.CanSet() {
			continue
		}
		transformed := rule.Transform(field.Interface())
		field.Set(reflect.ValueOf(transformed))
	}

	return data, nil
}

// GetRules 获取指定结构体的校验规则
func (v *Validator) GetRules(structName string) []FieldRule {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.rules[structName]
}

// HasRules 检查指定结构体是否有校验规则
func (v *Validator) HasRules(structName string) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	_, ok := v.rules[structName]
	return ok
}

// RemoveRules 移除指定结构体的校验规则
func (v *Validator) RemoveRules(structName string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.rules, structName)
}

// Clear 清空所有校验规则
func (v *Validator) Clear() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.rules = make(map[string][]FieldRule)
}
