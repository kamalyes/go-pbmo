/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2023-09-26 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2023-09-26 00:00:00
 * @FilePath: \go-pbmo\desensitize_test.go
 * @Description: 脱敏转换器测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"testing"

	"github.com/kamalyes/go-toolbox/pkg/desensitize"
	"github.com/stretchr/testify/assert"
)

// DesensitizeTestModel 脱敏测试模型
type DesensitizeTestModel struct {
	Name  string `desensitize:"name"`
	Email string `desensitize:"email"`
	Phone string `desensitize:"phone"`
}

func TestNewDesensitizeConverter(t *testing.T) {
	dc := NewDesensitizeConverter(TestSimplePB{}, TestSimpleModel{}, WithDesensitize(true))
	assert.NotNil(t, dc)
	assert.True(t, dc.IsEnabled())
}

func TestDesensitizeConverter_SetEnabled(t *testing.T) {
	dc := NewDesensitizeConverter(TestSimplePB{}, TestSimpleModel{})
	assert.False(t, dc.IsEnabled())

	dc.SetEnabled(true)
	assert.True(t, dc.IsEnabled())

	dc.SetEnabled(false)
	assert.False(t, dc.IsEnabled())
}

func TestDesensitizeField(t *testing.T) {
	result := DesensitizeField("13812345678", desensitize.PhoneNumber)
	assert.Contains(t, result, "*")
	assert.Contains(t, result, "138")
	assert.Contains(t, result, "5678")
}

func TestDesensitizeField_Email(t *testing.T) {
	result := DesensitizeField("test@example.com", desensitize.Email)
	assert.Contains(t, result, "*")
}

func TestApplyDesensitization(t *testing.T) {
	model := DesensitizeTestModel{
		Name:  "张三",
		Email: "test@example.com",
		Phone: "13812345678",
	}

	err := ApplyDesensitization(&model)
	assert.NoError(t, err)
}

func TestDesensitizeConverter_Disabled(t *testing.T) {
	dc := NewDesensitizeConverter(TestSimplePB{}, TestSimpleModel{}, WithDesensitize(false))

	pb := TestSimplePB{Value: "test", Count: 1}
	var model TestSimpleModel

	err := dc.ConvertPBToModelWithDesensitize(pb, &model)
	assert.NoError(t, err)
	assert.Equal(t, "test", model.Value)
}
