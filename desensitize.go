/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2023-09-26 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2023-09-26 00:00:00
 * @FilePath: \go-pbmo\desensitize.go
 * @Description: 脱敏转换器 - 利用 go-toolbox/desensitize 实现数据脱敏
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"github.com/kamalyes/go-toolbox/pkg/desensitize"
)

// DesensitizeConverter 脱敏转换器
// 在 PB -> Model 转换后自动应用脱敏规则
type DesensitizeConverter struct {
	*BidiConverter
	enabled bool // 是否启用脱敏
}

// NewDesensitizeConverter 创建脱敏转换器
func NewDesensitizeConverter(pbType, modelType interface{}, opts ...Option) *DesensitizeConverter {
	options := ApplyOptions(opts...)
	return &DesensitizeConverter{
		BidiConverter: NewBidiConverter(pbType, modelType, opts...),
		enabled:       options.DesensitizeEnabled,
	}
}

// ConvertPBToModelWithDesensitize PB -> Model 转换并脱敏
func (dc *DesensitizeConverter) ConvertPBToModelWithDesensitize(pb interface{}, modelPtr interface{}) error {
	if err := dc.BidiConverter.ConvertPBToModel(pb, modelPtr); err != nil {
		return err
	}

	if dc.enabled {
		if err := desensitize.Desensitization(modelPtr); err != nil {
			return NewConversionError("脱敏处理失败: %v", err)
		}
	}

	return nil
}

// ConvertModelToPBWithDesensitize Model -> PB 转换（先脱敏再转换）
func (dc *DesensitizeConverter) ConvertModelToPBWithDesensitize(model interface{}, pbPtr interface{}) error {
	if dc.enabled {
		if err := desensitize.Desensitization(model); err != nil {
			return NewConversionError("脱敏处理失败: %v", err)
		}
	}

	return dc.BidiConverter.ConvertModelToPB(model, pbPtr)
}

// ApplyDesensitization 对对象应用脱敏
// 利用 go-toolbox/desensitize 的 tag 自动发现机制
func ApplyDesensitization(obj interface{}) error {
	return desensitize.Desensitization(obj)
}

// RegisterDesensitizer 注册自定义脱敏器
// 利用 go-toolbox/desensitize 的注册机制
func RegisterDesensitizer(desensitizerType string, d desensitize.Desensitizer) {
	desensitize.RegisterDesensitizer(desensitizerType, d)
}

// DesensitizeField 对单个字段值进行脱敏
func DesensitizeField(value string, desensitizeType desensitize.DesensitizeType) string {
	return desensitize.Desensitize(value, desensitizeType)
}

// SetEnabled 设置脱敏开关
func (dc *DesensitizeConverter) SetEnabled(enabled bool) {
	dc.enabled = enabled
}

// IsEnabled 检查脱敏是否启用
func (dc *DesensitizeConverter) IsEnabled() bool {
	return dc.enabled
}
