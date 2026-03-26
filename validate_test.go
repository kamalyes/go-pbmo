/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2023-09-26 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2023-09-26 00:00:00
 * @FilePath: \go-pbmo\validate_test.go
 * @Description: 校验器测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ValidateTestModel 校验测试模型
type ValidateTestModel struct {
	Name  string
	Email string
	Age   int
}

func TestNewValidator(t *testing.T) {
	v := NewValidator()
	assert.NotNil(t, v)
}

func TestValidator_RegisterRules(t *testing.T) {
	v := NewValidator()
	v.RegisterRules("ValidateTestModel",
		FieldRule{Name: "Name", Required: true, MinLen: 2, MaxLen: 50},
		FieldRule{Name: "Age", Min: 0, Max: 150},
	)

	assert.True(t, v.HasRules("ValidateTestModel"))
	assert.False(t, v.HasRules("NotExist"))
}

func TestValidator_Validate_Required(t *testing.T) {
	v := NewValidator()
	v.RegisterRules("ValidateTestModel",
		FieldRule{Name: "Name", Required: true},
	)

	model := ValidateTestModel{Name: ""}
	err := v.Validate(&model)
	assert.Error(t, err)
}

func TestValidator_Validate_Required_NonEmpty(t *testing.T) {
	v := NewValidator()
	v.RegisterRules("ValidateTestModel",
		FieldRule{Name: "Name", Required: true},
	)

	model := ValidateTestModel{Name: "test"}
	err := v.Validate(&model)
	assert.NoError(t, err)
}

func TestValidator_Validate_MinLen(t *testing.T) {
	v := NewValidator()
	v.RegisterRules("ValidateTestModel",
		FieldRule{Name: "Name", MinLen: 3},
	)

	model := ValidateTestModel{Name: "ab"}
	err := v.Validate(&model)
	assert.Error(t, err)
}

func TestValidator_Validate_MaxLen(t *testing.T) {
	v := NewValidator()
	v.RegisterRules("ValidateTestModel",
		FieldRule{Name: "Name", MaxLen: 5},
	)

	model := ValidateTestModel{Name: "toolong"}
	err := v.Validate(&model)
	assert.Error(t, err)
}

func TestValidator_Validate_Min(t *testing.T) {
	v := NewValidator()
	v.RegisterRules("ValidateTestModel",
		FieldRule{Name: "Age", Min: 18},
	)

	model := ValidateTestModel{Age: 10}
	err := v.Validate(&model)
	assert.Error(t, err)
}

func TestValidator_Validate_Max(t *testing.T) {
	v := NewValidator()
	v.RegisterRules("ValidateTestModel",
		FieldRule{Name: "Age", Max: 100},
	)

	model := ValidateTestModel{Age: 150}
	err := v.Validate(&model)
	assert.Error(t, err)
}

func TestValidator_Validate_Pattern(t *testing.T) {
	v := NewValidator()
	v.RegisterRules("ValidateTestModel",
		FieldRule{Name: "Email", Pattern: `^[a-zA-Z0-9]+@[a-zA-Z0-9]+\.[a-zA-Z0-9]+$`},
	)

	model := ValidateTestModel{Email: "invalid-email"}
	err := v.Validate(&model)
	assert.Error(t, err)

	model2 := ValidateTestModel{Email: "test@example.com"}
	err2 := v.Validate(&model2)
	assert.NoError(t, err2)
}

func TestValidator_Validate_Custom(t *testing.T) {
	v := NewValidator()
	v.RegisterRules("ValidateTestModel",
		FieldRule{
			Name: "Name",
			Custom: func(v interface{}) error {
				s := v.(string)
				if s == "banned" {
					return errors.New("禁止使用的名称")
				}
				return nil
			},
		},
	)

	model := ValidateTestModel{Name: "banned"}
	err := v.Validate(&model)
	assert.Error(t, err)

	model2 := ValidateTestModel{Name: "ok"}
	err2 := v.Validate(&model2)
	assert.NoError(t, err2)
}

func TestValidator_Validate_NilData(t *testing.T) {
	v := NewValidator()
	err := v.Validate(nil)
	assert.Error(t, err)
}

func TestValidator_Validate_NoRules(t *testing.T) {
	v := NewValidator()
	model := ValidateTestModel{Name: "test"}
	err := v.Validate(&model)
	assert.NoError(t, err)
}

func TestValidator_Validate_NonPointer(t *testing.T) {
	v := NewValidator()
	v.RegisterRules("ValidateTestModel",
		FieldRule{Name: "Name", Required: true},
	)

	model := ValidateTestModel{Name: "test"}
	err := v.Validate(model)
	assert.NoError(t, err)
}

func TestValidator_GetRules(t *testing.T) {
	v := NewValidator()
	v.RegisterRules("ValidateTestModel",
		FieldRule{Name: "Name", Required: true},
	)

	rules := v.GetRules("ValidateTestModel")
	assert.Len(t, rules, 1)
	assert.Equal(t, "Name", rules[0].Name)
}

func TestValidator_RemoveRules(t *testing.T) {
	v := NewValidator()
	v.RegisterRules("ValidateTestModel",
		FieldRule{Name: "Name", Required: true},
	)
	assert.True(t, v.HasRules("ValidateTestModel"))

	v.RemoveRules("ValidateTestModel")
	assert.False(t, v.HasRules("ValidateTestModel"))
}

func TestValidator_Clear(t *testing.T) {
	v := NewValidator()
	v.RegisterRules("ValidateTestModel",
		FieldRule{Name: "Name", Required: true},
	)
	v.RegisterRules("OtherModel",
		FieldRule{Name: "ID", Required: true},
	)

	v.Clear()
	assert.False(t, v.HasRules("ValidateTestModel"))
	assert.False(t, v.HasRules("OtherModel"))
}

func TestValidationErrors_HasErrors(t *testing.T) {
	var errs ValidationErrors
	assert.False(t, errs.HasErrors())

	errs = append(errs, ValidationError{Field: "Name", Message: "必填"})
	assert.True(t, errs.HasErrors())
}

func TestValidationErrors_Error(t *testing.T) {
	errs := ValidationErrors{
		{Field: "Name", Message: "必填"},
		{Field: "Age", Message: "超出范围"},
	}

	errStr := errs.Error()
	assert.Contains(t, errStr, "Name")
	assert.Contains(t, errStr, "Age")
}

func TestValidationErrors_Fields(t *testing.T) {
	errs := ValidationErrors{
		{Field: "Name", Message: "必填"},
		{Field: "Age", Message: "超出范围"},
	}

	fields := errs.Fields()
	assert.Contains(t, fields, "Name")
	assert.Contains(t, fields, "Age")
}

func TestValidator_ValidateWithTransform(t *testing.T) {
	v := NewValidator()
	v.RegisterRules("ValidateTestModel",
		FieldRule{
			Name: "Name",
			Transform: func(v interface{}) interface{} {
				return "transformed_" + v.(string)
			},
		},
	)

	model := ValidateTestModel{Name: "original"}
	result, err := v.ValidateWithTransform(&model)
	assert.NoError(t, err)
	assert.Equal(t, "transformed_original", result.(*ValidateTestModel).Name)
}

func TestValidator_RegisterBatch(t *testing.T) {
	v := NewValidator()
	v.RegisterBatch(map[string][]FieldRule{
		"ValidateTestModel": {{Name: "Name", Required: true}},
		"OtherModel":        {{Name: "ID", Required: true}},
	})

	assert.True(t, v.HasRules("ValidateTestModel"))
	assert.True(t, v.HasRules("OtherModel"))
}
