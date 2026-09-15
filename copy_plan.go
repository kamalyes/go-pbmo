/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-16 21:38:23
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-16 21:38:23
 * @FilePath: \go-pbmo\copy_plan.go
 * @Description: 分组拷贝计划 - 泛型拷贝核为唯一实现，直拷与分组两种执行形态派生
 * 直拷闭包以静态实例化调用烘焙偏移常量，组拷贝体在 pairs 循环内联同一语义
 * 同特征字段共享组拷贝体单次间接调用，不可分组种类保留独立闭包走 singles 通路
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"reflect"
	"time"
	"unsafe"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// groupCopyFunc 组拷贝体，按平铺偏移对循环处理同组全部字段
// 偏移对交替排列为 (srcOff, dstOff)，整组仅一次间接调用
type groupCopyFunc func(dstBase, srcBase unsafe.Pointer, pairs []uintptr)

// mergedUnrollMax 合并闭包的展开调用上限，与 buildMergedCopyFuncFromFns 的 switch 分支数一致
// 字段数不超过该值时走直拷闭包展开执行，分组循环反而引入遍历开销
const mergedUnrollMax = 8

// copyGroup 同拷贝体字段分组，pairs 为组内字段的平铺偏移对
type copyGroup struct {
	copy  groupCopyFunc
	pairs []uintptr
}

// groupSrc 分组分类所需的字段视图，统一 fieldMappingEntry 与 structFieldEntry
type groupSrc struct {
	kind      fieldKind
	srcType   reflect.Type
	dstType   reflect.Type
	srcOff    uintptr
	dstOff    uintptr
	groupable bool          // 拷贝体是否为可共享的纯拷贝，transform 等自定义闭包为 false
	legacy    fieldCopyFunc // 不可分组时回退的独立闭包
}

// ══════════════════════════════════════════════════════════════════════════════
// 泛型拷贝核：每种拷贝语义唯一实现
// 泛型按类型实例化为静态代码，直拷闭包静态调用可被内联，组拷贝体循环内联同一语义
// ══════════════════════════════════════════════════════════════════════════════

// copyValue 同类型字段标量拷贝
func copyValue[T any](dstBase, srcBase unsafe.Pointer, dstOff, srcOff uintptr) {
	*(*T)(addPtr(dstBase, dstOff)) = *(*T)(addPtr(srcBase, srcOff))
}

// copyValues 同类型组拷贝
func copyValues[T any](dstBase, srcBase unsafe.Pointer, pairs []uintptr) {
	for i := 0; i < len(pairs); i += 2 {
		*(*T)(addPtr(dstBase, pairs[i+1])) = *(*T)(addPtr(srcBase, pairs[i]))
	}
}

// intKindConv 整数种类约束，覆盖全部有符号与无符号整数
type intKindConv interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}

// convertValue 整数转换标量
func convertValue[S, D intKindConv](dstBase, srcBase unsafe.Pointer, dstOff, srcOff uintptr) {
	*(*D)(addPtr(dstBase, dstOff)) = D(*(*S)(addPtr(srcBase, srcOff)))
}

// convertValues 整数转换组拷贝
func convertValues[S, D intKindConv](dstBase, srcBase unsafe.Pointer, pairs []uintptr) {
	for i := 0; i < len(pairs); i += 2 {
		*(*D)(addPtr(dstBase, pairs[i+1])) = D(*(*S)(addPtr(srcBase, pairs[i])))
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// 时间转换核：四种方向各一个语义函数，组拷贝与直拷闭包共用
// ══════════════════════════════════════════════════════════════════════════════

// timeFromTS 解引用时间戳为 time.Time，nil 返回 false
func timeFromTS(ts *timestamppb.Timestamp) (time.Time, bool) {
	if ts == nil {
		return time.Time{}, false
	}
	return ts.AsTime(), true
}

// tsFromTime 零值时间返回 nil，其余转换为时间戳
func tsFromTime(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

// timePtrFromTS 时间戳转 *time.Time，nil 时间戳返回 nil
func timePtrFromTS(ts *timestamppb.Timestamp) *time.Time {
	if ts == nil {
		return nil
	}
	t := ts.AsTime()
	return &t
}

// tsFromTimePtr nil 或零值的 *time.Time 返回 nil，其余转换为时间戳
func tsFromTimePtr(t *time.Time) *timestamppb.Timestamp {
	if t == nil || t.IsZero() {
		return nil
	}
	return timestamppb.New(*t)
}

// memmoveGroup 按字节尺寸整块拷贝的组拷贝体
// 尺寸为运行时闭包变量，同一代码指针以尺寸作 aux 区分分组
func memmoveGroup(sz uintptr) groupCopyFunc {
	return func(dstBase, srcBase unsafe.Pointer, pairs []uintptr) {
		for i := 0; i < len(pairs); i += 2 {
			copy((*[1 << 28]byte)(addPtr(dstBase, pairs[i+1]))[:sz],
				(*[1 << 28]byte)(addPtr(srcBase, pairs[i]))[:sz])
		}
	}
}

// memmoveDirect 按字节尺寸整块拷贝的直拷闭包
func memmoveDirect(sz, srcOffset, dstOffset uintptr) fieldCopyFunc {
	return func(srcBase, dstBase unsafe.Pointer) {
		copy((*[1 << 28]byte)(addPtr(dstBase, dstOffset))[:sz],
			(*[1 << 28]byte)(addPtr(srcBase, srcOffset))[:sz])
	}
}

// pureSameTypeCopy 同类型字段组拷贝体，与 makeSameTypeCopyFunc 共享泛型拷贝核
// 返回值 aux 为 memmove 分支的分组辅助键，typed 分支各占独立实例化代码指针无需辅助
func pureSameTypeCopy(t reflect.Type) (groupCopyFunc, uintptr) {
	switch t.Kind() {
	case reflect.Bool:
		return copyValues[bool], 0
	case reflect.Int:
		return copyValues[int], 0
	case reflect.Int8:
		return copyValues[int8], 0
	case reflect.Int16:
		return copyValues[int16], 0
	case reflect.Int32:
		return copyValues[int32], 0
	case reflect.Int64:
		return copyValues[int64], 0
	case reflect.Uint:
		return copyValues[uint], 0
	case reflect.Uint8:
		return copyValues[uint8], 0
	case reflect.Uint16:
		return copyValues[uint16], 0
	case reflect.Uint32:
		return copyValues[uint32], 0
	case reflect.Uint64:
		return copyValues[uint64], 0
	case reflect.Float32:
		return copyValues[float32], 0
	case reflect.Float64:
		return copyValues[float64], 0
	case reflect.String:
		return copyValues[string], 0
	case reflect.Slice:
		sz := unsafe.Sizeof([]byte{})
		return memmoveGroup(sz), sz
	default:
		sz := t.Size()
		return memmoveGroup(sz), sz
	}
}

// pureIntegerCopy 整数转换组拷贝体，与 makeIntegerCopyFunc 共享泛型转换核
// 常见组合走 typed 特化分支，其余组合回退宽度泛化实现并以 kind 对作辅助键
func pureIntegerCopy(srcKind, dstKind reflect.Kind) (groupCopyFunc, uintptr) {
	switch {
	// ====== int32 类型的所有组合（最常见：protobuf int32 ↔ Go int） ======
	case srcKind == reflect.Int32 && dstKind == reflect.Int:
		return convertValues[int32, int], 0
	case srcKind == reflect.Int && dstKind == reflect.Int32:
		return convertValues[int, int32], 0
	case srcKind == reflect.Int32 && dstKind == reflect.Int64:
		return convertValues[int32, int64], 0
	case srcKind == reflect.Int64 && dstKind == reflect.Int32:
		return convertValues[int64, int32], 0
	case srcKind == reflect.Int32 && dstKind == reflect.Int16:
		return convertValues[int32, int16], 0
	case srcKind == reflect.Int16 && dstKind == reflect.Int32:
		return convertValues[int16, int32], 0
	case srcKind == reflect.Int32 && dstKind == reflect.Int8:
		return convertValues[int32, int8], 0
	case srcKind == reflect.Int8 && dstKind == reflect.Int32:
		return convertValues[int8, int32], 0

	// ====== int64 类型的组合 ======
	case srcKind == reflect.Int64 && dstKind == reflect.Int:
		return convertValues[int64, int], 0
	case srcKind == reflect.Int && dstKind == reflect.Int64:
		return convertValues[int, int64], 0

	// ====== uint32 类型的组合 ======
	case srcKind == reflect.Uint32 && dstKind == reflect.Uint64:
		return convertValues[uint32, uint64], 0
	case srcKind == reflect.Uint64 && dstKind == reflect.Uint32:
		return convertValues[uint64, uint32], 0
	case srcKind == reflect.Uint32 && dstKind == reflect.Uint:
		return convertValues[uint32, uint], 0
	case srcKind == reflect.Uint && dstKind == reflect.Uint32:
		return convertValues[uint, uint32], 0

	// ====== 有符号 ↔ 无符号的转换 ======
	case srcKind == reflect.Int32 && dstKind == reflect.Uint32:
		return convertValues[int32, uint32], 0
	case srcKind == reflect.Uint32 && dstKind == reflect.Int32:
		return convertValues[uint32, int32], 0
	case srcKind == reflect.Int32 && dstKind == reflect.Uint64:
		return convertValues[int32, uint64], 0
	case srcKind == reflect.Int64 && dstKind == reflect.Uint64:
		return convertValues[int64, uint64], 0
	case srcKind == reflect.Uint64 && dstKind == reflect.Int64:
		return convertValues[uint64, int64], 0
	case srcKind == reflect.Uint32 && dstKind == reflect.Int:
		return convertValues[uint32, int], 0
	case srcKind == reflect.Int && dstKind == reflect.Uint32:
		return convertValues[int, uint32], 0

	// ====== 通用的有符号读取 → 有符号写入 ======
	default:
		return pureIntegerCopyGeneric(srcKind, dstKind)
	}
}

// pureIntegerCopyGeneric 宽度泛化整数转换组拷贝体
// 不同 kind 组合共享同一代码指针，以 kind 对作分组辅助键避免错误合并
func pureIntegerCopyGeneric(srcKind, dstKind reflect.Kind) (groupCopyFunc, uintptr) {
	srcSigned := IsSignedInt(srcKind)
	dstSigned := IsSignedInt(dstKind)
	srcSize := kindToSize(srcKind)
	dstSize := kindToSize(dstKind)

	return func(dstBase, srcBase unsafe.Pointer, pairs []uintptr) {
		for i := 0; i < len(pairs); i += 2 {
			srcPtr := addPtr(srcBase, pairs[i])
			dstPtr := addPtr(dstBase, pairs[i+1])

			var ival int64
			var uval uint64

			if srcSigned {
				switch srcSize {
				case 1:
					ival = int64(*(*int8)(srcPtr))
				case 2:
					ival = int64(*(*int16)(srcPtr))
				case 4:
					ival = int64(*(*int32)(srcPtr))
				case 8:
					ival = *(*int64)(srcPtr)
				}
				if dstSigned {
					switch dstSize {
					case 1:
						*(*int8)(dstPtr) = int8(ival)
					case 2:
						*(*int16)(dstPtr) = int16(ival)
					case 4:
						*(*int32)(dstPtr) = int32(ival)
					case 8:
						*(*int64)(dstPtr) = ival
					}
				} else {
					uval = uint64(ival)
					switch dstSize {
					case 1:
						*(*uint8)(dstPtr) = uint8(uval)
					case 2:
						*(*uint16)(dstPtr) = uint16(uval)
					case 4:
						*(*uint32)(dstPtr) = uint32(uval)
					case 8:
						*(*uint64)(dstPtr) = uval
					}
				}
			} else {
				switch srcSize {
				case 1:
					uval = uint64(*(*uint8)(srcPtr))
				case 2:
					uval = uint64(*(*uint16)(srcPtr))
				case 4:
					uval = uint64(*(*uint32)(srcPtr))
				case 8:
					uval = *(*uint64)(srcPtr)
				}
				if dstSigned {
					ival = int64(uval)
					switch dstSize {
					case 1:
						*(*int8)(dstPtr) = int8(ival)
					case 2:
						*(*int16)(dstPtr) = int16(ival)
					case 4:
						*(*int32)(dstPtr) = int32(ival)
					case 8:
						*(*int64)(dstPtr) = ival
					}
				} else {
					switch dstSize {
					case 1:
						*(*uint8)(dstPtr) = uint8(uval)
					case 2:
						*(*uint16)(dstPtr) = uint16(uval)
					case 4:
						*(*uint32)(dstPtr) = uint32(uval)
					case 8:
						*(*uint64)(dstPtr) = uval
					}
				}
			}
		}
	}, uintptr(srcKind)<<8 | uintptr(dstKind)
}

// pureTSToTimeCopy *timestamppb.Timestamp → time.Time 组拷贝体
func pureTSToTimeCopy(dstBase, srcBase unsafe.Pointer, pairs []uintptr) {
	for i := 0; i < len(pairs); i += 2 {
		if t, ok := timeFromTS(*(**timestamppb.Timestamp)(addPtr(srcBase, pairs[i]))); ok {
			*(*time.Time)(addPtr(dstBase, pairs[i+1])) = t
		}
	}
}

// pureTimeToTSCopy time.Time → *timestamppb.Timestamp 组拷贝体
func pureTimeToTSCopy(dstBase, srcBase unsafe.Pointer, pairs []uintptr) {
	for i := 0; i < len(pairs); i += 2 {
		*(**timestamppb.Timestamp)(addPtr(dstBase, pairs[i+1])) = tsFromTime(*(*time.Time)(addPtr(srcBase, pairs[i])))
	}
}

// pureTSToTimePtrCopy *timestamppb.Timestamp → *time.Time 组拷贝体
func pureTSToTimePtrCopy(dstBase, srcBase unsafe.Pointer, pairs []uintptr) {
	for i := 0; i < len(pairs); i += 2 {
		*(**time.Time)(addPtr(dstBase, pairs[i+1])) = timePtrFromTS(*(**timestamppb.Timestamp)(addPtr(srcBase, pairs[i])))
	}
}

// pureTimePtrToTSCopy *time.Time → *timestamppb.Timestamp 组拷贝体
func pureTimePtrToTSCopy(dstBase, srcBase unsafe.Pointer, pairs []uintptr) {
	for i := 0; i < len(pairs); i += 2 {
		*(**timestamppb.Timestamp)(addPtr(dstBase, pairs[i+1])) = tsFromTimePtr(*(**time.Time)(addPtr(srcBase, pairs[i])))
	}
}

// pureCopierOf 按字段种类返回可共享组拷贝体与分组辅助键
// 返回 nil 表示闭包捕获字段外状态（wrapper/嵌套/集合/transform 等），保留独立通路
func pureCopierOf(kind fieldKind, srcType, dstType reflect.Type) (groupCopyFunc, uintptr) {
	switch kind {
	case fieldSameType, fieldAssignable:
		return pureSameTypeCopy(srcType)
	case fieldInteger:
		return pureIntegerCopy(srcType.Kind(), dstType.Kind())
	case fieldConvertible:
		if srcType.Size() == dstType.Size() {
			sz := srcType.Size()
			return memmoveGroup(sz), sz
		}
		return nil, 0
	case fieldTSToTime:
		return pureTSToTimeCopy, 0
	case fieldTimeToTS:
		return pureTimeToTSCopy, 0
	case fieldTSToTimePtr:
		return pureTSToTimePtrCopy, 0
	case fieldTimePtrToTS:
		return pureTimePtrToTSCopy, 0
	case fieldStringFallback:
		return copyValues[string], 0
	default:
		return nil, 0
	}
}

// buildGroupedCopyFunc 将字段视图按拷贝体特征分组构建合并执行闭包
// 可分组种类以 (拷贝体代码指针, 辅助键) 聚合，同组字段共享一次间接调用
// 不可分组种类收集为 singles，复用 buildMergedCopyFuncFromFns 的展开执行
// 字段数不超过展开上限时走直拷闭包旧通路，pairs 循环开销大于省下的间接调用
// 各字段仅写自身目标位置互不重叠，组间执行顺序不影响结果
func buildGroupedCopyFunc(srcs []groupSrc) func(srcBase, dstBase unsafe.Pointer) {
	if len(srcs) <= mergedUnrollMax {
		fns := make([]fieldCopyFunc, 0, len(srcs))
		for i := range srcs {
			if srcs[i].legacy != nil {
				fns = append(fns, srcs[i].legacy)
			}
		}
		return buildMergedCopyFuncFromFns(fns)
	}

	var groups []copyGroup
	var singles []fieldCopyFunc

	type groupKey struct {
		code uintptr
		aux  uintptr
	}
	index := make(map[groupKey]int, len(srcs))
	for i := range srcs {
		s := &srcs[i]
		var pure groupCopyFunc
		var aux uintptr
		if s.groupable {
			pure, aux = pureCopierOf(s.kind, s.srcType, s.dstType)
		}
		if pure == nil {
			if s.legacy != nil {
				singles = append(singles, s.legacy)
			}
			continue
		}
		key := groupKey{code: reflect.ValueOf(pure).Pointer(), aux: aux}
		id, ok := index[key]
		if !ok {
			id = len(groups)
			index[key] = id
			groups = append(groups, copyGroup{copy: pure})
		}
		g := &groups[id]
		g.pairs = append(g.pairs, s.srcOff, s.dstOff)
	}

	singlesFn := buildMergedCopyFuncFromFns(singles)
	if len(groups) == 0 {
		return singlesFn
	}
	if singlesFn == nil {
		return func(srcBase, dstBase unsafe.Pointer) {
			for i := range groups {
				g := &groups[i]
				g.copy(dstBase, srcBase, g.pairs)
			}
		}
	}
	return func(srcBase, dstBase unsafe.Pointer) {
		for i := range groups {
			g := &groups[i]
			g.copy(dstBase, srcBase, g.pairs)
		}
		singlesFn(srcBase, dstBase)
	}
}
