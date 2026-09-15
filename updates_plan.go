/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-16 21:08:23
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-16 21:08:23
 * @FilePath: \go-pbmo\updates_plan.go
 * @Description: Updates 构建计划缓存 - 按类型预解析字段 key 与偏移量
 * 热路径零 tag 解析、零 reflect.Value 构造，空值判断按字段种类特化为直读比较
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"reflect"
	"time"
	"unsafe"

	"github.com/kamalyes/go-argus/validate"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
	"github.com/kamalyes/go-toolbox/pkg/types"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// updatesPlanKey 更新计划缓存 key（结构体类型 + 构建方向）
type updatesPlanKey struct {
	typ     reflect.Type
	isModel bool
}

// updateFieldEntry 单字段的更新构建计划
type updateFieldEntry struct {
	key    string                                // 预解析的 map key（proto name / gorm column / json tag）
	offset uintptr                               // 字段在结构体中的偏移量
	empty  func(base unsafe.Pointer) bool        // 特化空值判断（零分配）
	emit   func(base unsafe.Pointer) interface{} // 装箱并解包，输出最终 map 值
}

// updatesPlan 单类型的更新构建计划
type updatesPlan struct {
	fields []updateFieldEntry
}

// updatesPlans 全局更新计划缓存，每个类型只构建一次
var updatesPlans syncx.Map[updatesPlanKey, *updatesPlan]

// stringWrapperPtrType *wrapperspb.StringValue 类型，空值判断走专用语义
var stringWrapperPtrType = reflect.TypeOf((*wrapperspb.StringValue)(nil))

// wrapperspbPkgPath protobuf wrapper 包路径
const wrapperspbPkgPath = "google.golang.org/protobuf/types/known/wrapperspb"

// getUpdatesPlan 获取（或构建）指定类型的更新计划
func getUpdatesPlan[M any](isModel bool) *updatesPlan {
	t := reflect.TypeFor[M]()
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	key := updatesPlanKey{t, isModel}
	if p, ok := updatesPlans.Load(key); ok {
		return p
	}
	p, _ := updatesPlans.LoadOrStore(key, buildUpdatesPlan(t, isModel))
	return p
}

// buildUpdatesPlan 构建更新计划：解析 key、特化空值判断与值输出
// 仅保留可导出且 key 可解析的字段，与旧反射实现的遍历语义一致
func buildUpdatesPlan(t reflect.Type, isModel bool) *updatesPlan {
	plan := &updatesPlan{fields: make([]updateFieldEntry, 0, t.NumField())}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		var key string
		if isModel {
			key = types.ResolveModelKey(field)
			if key == "-" {
				continue
			}
		} else {
			key = types.ResolvePBKey(field)
			if key == "" || key == "-" {
				continue
			}
		}
		plan.fields = append(plan.fields, updateFieldEntry{
			key:    key,
			offset: field.Offset,
			empty:  makeUpdateEmptyCheck(field.Type, field.Offset),
			emit:   makeUpdateEmitter(field.Type, field.Offset, isModel),
		})
	}
	return plan
}

// makeUpdateEmptyCheck 按字段类型生成特化空值判断
// 语义与 isEmptyUpdateValue 一致：nil 指针、空字符串、零值、空集合、零值时间视为空
func makeUpdateEmptyCheck(ft reflect.Type, off uintptr) func(unsafe.Pointer) bool {
	switch ft.Kind() {
	case reflect.String:
		return func(base unsafe.Pointer) bool {
			s := *(*string)(addPtr(base, off))
			return validate.IsBlankString(s) || validate.IfNullOrUndefined(s)
		}
	case reflect.Bool:
		return func(base unsafe.Pointer) bool {
			return !*(*bool)(addPtr(base, off))
		}
	case reflect.Int8:
		return func(base unsafe.Pointer) bool {
			return *(*int8)(addPtr(base, off)) == 0
		}
	case reflect.Int16:
		return func(base unsafe.Pointer) bool {
			return *(*int16)(addPtr(base, off)) == 0
		}
	case reflect.Int32:
		return func(base unsafe.Pointer) bool {
			return *(*int32)(addPtr(base, off)) == 0
		}
	case reflect.Int, reflect.Int64:
		return func(base unsafe.Pointer) bool {
			return *(*int64)(addPtr(base, off)) == 0
		}
	case reflect.Uint8:
		return func(base unsafe.Pointer) bool {
			return *(*uint8)(addPtr(base, off)) == 0
		}
	case reflect.Uint16:
		return func(base unsafe.Pointer) bool {
			return *(*uint16)(addPtr(base, off)) == 0
		}
	case reflect.Uint32:
		return func(base unsafe.Pointer) bool {
			return *(*uint32)(addPtr(base, off)) == 0
		}
	case reflect.Uint, reflect.Uint64, reflect.Uintptr:
		return func(base unsafe.Pointer) bool {
			return *(*uint64)(addPtr(base, off)) == 0
		}
	case reflect.Float32:
		return func(base unsafe.Pointer) bool {
			return *(*float32)(addPtr(base, off)) == 0
		}
	case reflect.Float64:
		return func(base unsafe.Pointer) bool {
			return *(*float64)(addPtr(base, off)) == 0
		}
	case reflect.Ptr:
		if ft == stringWrapperPtrType {
			return func(base unsafe.Pointer) bool {
				sv := *(**wrapperspb.StringValue)(addPtr(base, off))
				return sv == nil || isEmptyUpdateString(sv.Value)
			}
		}
		elemCheck := makeUpdateDerefEmptyCheck(ft.Elem())
		return func(base unsafe.Pointer) bool {
			p := *(*unsafe.Pointer)(addPtr(base, off))
			return p == nil || elemCheck(p)
		}
	case reflect.Slice, reflect.Map, reflect.Array:
		return func(base unsafe.Pointer) bool {
			return reflect.NewAt(ft, addPtr(base, off)).Elem().Len() == 0
		}
	case reflect.Struct:
		if ft == timeType {
			return func(base unsafe.Pointer) bool {
				tm := *(*time.Time)(addPtr(base, off))
				return tm.IsZero() || tm.Unix() <= 0
			}
		}
		return func(base unsafe.Pointer) bool {
			return reflect.NewAt(ft, addPtr(base, off)).Elem().IsZero()
		}
	default:
		// Interface / Func / Chan / 复数等动态语义类型，回退通用判断保证语义一致
		return func(base unsafe.Pointer) bool {
			return validate.IsEmptyValue(reflect.NewAt(ft, addPtr(base, off)).Elem())
		}
	}
}

// makeUpdateDerefEmptyCheck 生成对指针已解引用值的空值判断
// 递归展开多级指针，与 IsEmptyValue 的循环解引用语义一致
func makeUpdateDerefEmptyCheck(et reflect.Type) func(unsafe.Pointer) bool {
	if et.Kind() == reflect.Ptr {
		inner := makeUpdateDerefEmptyCheck(et.Elem())
		return func(p unsafe.Pointer) bool {
			pp := *(*unsafe.Pointer)(p)
			return pp == nil || inner(pp)
		}
	}
	return makeUpdateEmptyCheck(et, 0)
}

// makeUpdateEmitter 按字段类型生成值输出闭包（装箱 + 解包）
// PB 侧 wrapper 输出 Value 字段值，Model 侧指针解引用输出底层值
func makeUpdateEmitter(ft reflect.Type, off uintptr, isModel bool) func(unsafe.Pointer) interface{} {
	if !isModel && ft.Kind() == reflect.Ptr &&
		ft.Elem().Kind() == reflect.Struct && ft.Elem().PkgPath() == wrapperspbPkgPath {
		if valueField, ok := ft.Elem().FieldByName("Value"); ok {
			vt, vo := valueField.Type, valueField.Offset
			return func(base unsafe.Pointer) interface{} {
				p := *(*unsafe.Pointer)(addPtr(base, off))
				if p == nil {
					return nil
				}
				return reflect.NewAt(vt, addPtr(p, vo)).Elem().Interface()
			}
		}
	}
	if isModel && ft.Kind() == reflect.Ptr {
		et := ft.Elem()
		return func(base unsafe.Pointer) interface{} {
			p := *(*unsafe.Pointer)(addPtr(base, off))
			if p == nil {
				// typed nil 保持原值语义，输出装箱后的指针本身
				return reflect.NewAt(ft, addPtr(base, off)).Elem().Interface()
			}
			return reflect.NewAt(et, p).Elem().Interface()
		}
	}
	return func(base unsafe.Pointer) interface{} {
		return reflect.NewAt(ft, addPtr(base, off)).Elem().Interface()
	}
}
