/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-04-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-20 00:00:00
 * @FilePath: \go-pbmo\converter.go
 * @Description: 核心双向转换器 - PB ↔ Model 高性能转换
 *
 * ═══════════════════════════════════════════════════════════════════════════
 * 架构设计思想（核心优化策略）
 * ═══════════════════════════════════════════════════════════════════════════
 *
 * 本文件实现了 PB ↔ Model 的高性能双向转换器，核心思想是：
 *
 *   原生手写代码 ≈ unsafe 指针操作 + 预计算偏移量 + 闭包内联
 *
 * 具体策略分三层：
 *
 * 【第一层：预计算（初始化时一次性完成）】
 *   - classifyField：在首次转换时对每个字段进行类型分类（fieldKind）
 *   - buildFieldCache / buildStructFieldCache：预计算每个字段的 srcOffset/dstOffset，
 *     生成 copyFunc 闭包；structFieldCache 额外携带 srcType/dstType 供慢速路径定位字段
 *   - 将字段分为 fastEntries（有 copyFunc）和 slowEntries（需 reflect 回退）
 *   - 合并所有 fastEntries 的 copyFunc 为 mergedCopyFunc（单次闭包调用），
 *     嵌套结构体则合并为 structFieldCache.mergedFn
 *
 * 【第二层：快速路径（运行时热路径，O(N) 次指针操作）】
 *   - ConvertPBToModel / ConvertModelToPB 统一委托给 convertWithCache 处理两个方向：
 *     1. reflect.ValueOf 获取结构体指针 + UnsafeAddr 获取基地址
 *     2. 若无 Transformer 且 mergedCopyFunc 存在 → 单次调用完成所有 fast 字段拷贝
 *     3. 否则逐个 fastEntries[i].copyFunc(srcBase, dstBase)
 *   - 每个 copyFunc 是闭包，捕获了 srcOffset/dstOffset，直接操作内存
 *   - 嵌套结构体的 make*CopyFunc 闭包通过 applyStructSlowEntriesUnsafe 统一处理
 *     其子缓存的 slowEntries，消除 7 处重复的 reflect.NewAt + FieldByIndex 模板
 *   - 支持：同类型拷贝、整数转换、时间戳↔时间、Wrapper 指针、
 *           结构体嵌套、结构体指针嵌套、值→指针、指针→值 等
 *
 * 【第三层：慢速路径（reflect 回退）】
 *   - 对于无法生成 copyFunc 的字段（slice、map、复杂嵌套等）
 *   - 使用 convertFieldByKind 分发到对应的 reflect 处理函数
 *   - 嵌套结构体使用 globalStructFieldCache 缓存，避免重复构建映射
 *   - convertStruct / convertStructPtr / convertStructWithCache 共用
 *     applyStructFieldCacheReflect 处理 CanAddr 快速路径与 reflect 回退
 *
 * 性能对比（vs 原生手写）：
 *   - Simple PBToModel: 3-4x（vs 原生 1x）
 *   - Config ModelToPB: 2-3x 快于原生（因原生 timestamppb.New 分配开销大）
 *   - Store PBToModel: ≈1x（接近原生）
 *   - Enterprise PBToModel: 1.4-2x（嵌套结构体优化后显著提升）
 *
 * ═══════════════════════════════════════════════════════════════════════════
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// isProtoMessage 检查类型是否为 protobuf 生成的消息类型
// 用于在 PB↔PB 转换时跳过 json tag 读取
// 设计原因：PB 的 json tag 是 protoc 生成的 snake_case 名字（如 "vendor_game_fields"），
// 不是 Go 字段名（如 "VendorGameFields"），读 json tag 会导致字段映射混乱，
// 依赖 strings.EqualFold fallback 才能匹配，不是设计意图
// 注意：proto.Message 接口（ProtoReflect 方法）通常用指针接收者实现，
// 所以必须检查指针类型，否则值类型（如 wrapperspb.BoolValue）会被漏判
func isProtoMessage(t reflect.Type) bool {
	if t == nil {
		return false
	}
	if t.Kind() != reflect.Ptr {
		t = reflect.PointerTo(t)
	}
	return t.Implements(reflect.TypeFor[proto.Message]())
}

// fieldCopyFunc 安全指针转换函数，在字段缓存构建时预生成
// 接收源/目标结构体基地址，直接通过偏移量+类型完成赋值，完全绕过 reflect.Value
// 设计思想：每个 copyFunc 是一个闭包，捕获 srcOffset 和 dstOffset，
// 在运行时只需传入结构体基地址即可完成字段级拷贝，无需任何类型判断
type fieldCopyFunc func(srcBase, dstBase unsafe.Pointer)

// fieldKind 字段类型分类，用于预计算转换路径，避免运行时类型判断开销
// 分类在 buildFieldCache 时一次性完成，运行时直接按 kind 分发
type fieldKind int

const (
	fieldSameType       fieldKind = iota // 类型完全相同 → 直接内存拷贝（unsafe.Pointer 类型转换）
	fieldTimeToTS                        // time.Time → *timestamppb.Timestamp（时间→时间戳指针）
	fieldTSToTime                        // *timestamppb.Timestamp → time.Time（时间戳指针→时间值）
	fieldTimePtrToTS                     // *time.Time → *timestamppb.Timestamp（时间指针→时间戳指针）
	fieldTSToTimePtr                     // *timestamppb.Timestamp → *time.Time（时间戳指针→时间指针）
	fieldWrapper                         // *T ↔ *wrapperspb.XxxValue（Proto Wrapper 类型转换）
	fieldInteger                         // 整数类型转换（int32↔int, int32↔int64 等）
	fieldAssignable                      // 可直接赋值（类型相同或接口兼容）
	fieldConvertible                     // 可转换类型（命名切片、相同布局不同名称等）
	fieldDataWrapper                     // DataWrapper[T] ↔ T/*T 数据包装器转换
	fieldStringFallback                  // 字符串兜底转换（string 命名类型）
	fieldSlice                           // 切片转换（需要逐元素递归，走 reflect 慢路径）
	fieldMap                             // Map 转换（需要遍历 key/value，走 reflect 慢路径）
	fieldMapStruct                       // map[string]any ↔ *structpb.Struct 转换（JSON 字段在持久化层与传输层间的映射）
	fieldToPtr                           // 值→指针转换（T → *T，需要 reflect.New 分配）
	fieldFromPtr                         // 指针→值解引用（*T → T，nil 检查后值拷贝）
	fieldStruct                          // 值结构体转换（嵌套结构体，走缓存的 unsafe 快路径或 reflect）
	fieldStructPtr                       // 指针结构体转换（*Struct → *Struct，走缓存快路径或 reflect）
	fieldNoop                            // 无需操作（源字段不存在于目标类型中）
)

// fieldMappingEntry 字段映射条目，预计算转换路径
// 每个条目在 buildFieldCache 时生成，包含转换所需的全部信息
// fastEntries：有 copyFunc 的字段（走 unsafe 快速路径）
// slowEntries：无 copyFunc 的字段（走 reflect 慢速路径）
type fieldMappingEntry struct {
	srcIndex     []int             // 源字段索引（用于 FieldByIndex 回退路径）
	dstIndex     []int             // 目标字段索引
	srcType      reflect.Type      // 源字段类型
	dstType      reflect.Type      // 目标字段类型
	kind         fieldKind         // 字段类型分类
	srcName      string            // 源字段名（用于 Transformer 查找）
	dstName      string            // 目标字段名
	wrapperConv  *wrapperConverter // Wrapper 类型转换器（仅 fieldWrapper 使用）
	wrapperDir   int               // Wrapper 方向：0=value→wrapper, 1=wrapper→value
	srcOffset    uintptr           // 源字段在结构体中的偏移量（unsafe 路径直接使用）
	dstOffset    uintptr           // 目标字段在结构体中的偏移量
	fieldSize    uintptr           // 源字段大小（用于内存拷贝）
	copyFunc     fieldCopyFunc     // 预生成的 unsafe 转换闭包（nil 则走 reflect 慢路径）
	hasTransform bool              // 是否有 Transformer 注册（影响缓存分层）
}

// fieldCache 字段缓存，包含 PB→Model 和 Model→PB 双向映射
// 生命周期：在 ensureFieldCache 中懒加载构建，随 BidiConverter 缓存
// 核心优化：fastEntries 全部有 copyFunc，可完全绕过 reflect
type fieldCache struct {
	srcType         reflect.Type                          // 源类型（用于类型校验）
	dstType         reflect.Type                          // 目标类型（用于类型校验）
	fastEntries     []fieldMappingEntry                   // 快速路径字段（有 copyFunc）
	slowEntries     []fieldMappingEntry                   // 慢速路径字段（需 reflect）
	autoTime        bool                                  // 是否启用自动时间转换
	hasTransformers bool                                  // 是否有 Transformer（预计算，避免每次 Count()）
	mergedCopyFunc  func(srcBase, dstBase unsafe.Pointer) // 合并的快速路径闭包，一次调用完成所有 fast 字段拷贝
}

// structFieldEntry 嵌套结构体字段映射条目
// 与 fieldMappingEntry 类似，但用于嵌套结构体的子字段缓存
type structFieldEntry struct {
	srcIndex    []int             // 源字段索引
	dstIndex    []int             // 目标字段索引
	srcType     reflect.Type      // 源字段类型
	dstType     reflect.Type      // 目标字段类型
	kind        fieldKind         // 字段类型分类
	srcName     string            // 源字段名
	wrapperConv *wrapperConverter // Wrapper 转换器
	wrapperDir  int               // Wrapper 方向
	srcOffset   uintptr           // 源字段偏移量
	dstOffset   uintptr           // 目标字段偏移量
	fieldSize   uintptr           // 源字段大小
	copyFunc    fieldCopyFunc     // 预生成的 unsafe 转换闭包
}

// structFieldCache 嵌套结构体字段缓存
// 通过 globalStructFieldCache 缓存，避免每次嵌套转换重复构建映射
type structFieldCache struct {
	srcType     reflect.Type                          // 源结构体类型（用于 reflect.NewAt 定位字段）
	dstType     reflect.Type                          // 目标结构体类型
	fastEntries []structFieldEntry                    // 快速路径字段
	slowEntries []structFieldEntry                    // 慢速路径字段
	autoTime    bool                                  // 是否自动时间转换
	mergedFn    func(srcBase, dstBase unsafe.Pointer) // 合并的快速路径闭包
}

// structFieldTypePair 结构体字段缓存 key（源类型+目标类型）
type structFieldTypePair [2]reflect.Type

// globalStructFieldCache 全局嵌套结构体字段缓存
// 避免每次嵌套转换时重复构建字段映射，提升性能
var globalStructFieldCache sync.Map

// sliceHeader 替代 sliceHeader，使用 unsafe.Pointer 避免 go vet 警告
// sliceHeader.Data 是 uintptr 类型，转换为 unsafe.Pointer 会触发 vet 警告
// 自定义 header 使用 unsafe.Pointer.Data，与运行时切片布局一致
type sliceHeader struct {
	Data unsafe.Pointer
	Len  int
	Cap  int
}

func addPtr(base unsafe.Pointer, offset uintptr) unsafe.Pointer {
	return unsafe.Pointer(uintptr(base) + offset)
}

// ifaceDataPtr 从 interface{} 中提取数据指针
// interface{} 内部布局：{type *rtype, data unsafe.Pointer}
// 对于大小 > ptrSize 的类型，data 指向堆上的值副本
// 对于大小 <= ptrSize 的类型，data 可能直接存储值（而非指向值的指针）
// 因此仅对大类型安全使用，小类型需要走 reflect 回退路径
func ifaceDataPtr(iface interface{}) unsafe.Pointer {
	type ifaceHeader struct {
		_    unsafe.Pointer
		Data unsafe.Pointer
	}
	h := (*ifaceHeader)(unsafe.Pointer(&iface))
	// 检查 interface 的类型标志，判断 data 是直接值还是指针
	// 对于 struct 类型，Go 总是使用间接引用（data 指向堆上的副本）
	// 但为了安全，我们只对大小 > ptrSize 的 struct 使用此优化
	return h.Data
}

// applyFastPath 应用快速路径的 unsafe 拷贝
func (bc *BidiConverter) applyFastPath(cache *fieldCache, srcBase, dstBase unsafe.Pointer, srcVal, dstVal reflect.Value) error {
	if cache.mergedCopyFunc != nil && !cache.hasTransformers {
		cache.mergedCopyFunc(srcBase, dstBase)
	} else if len(cache.fastEntries) > 0 {
		if cache.mergedCopyFunc != nil {
			cache.mergedCopyFunc(srcBase, dstBase)
		} else {
			for i := range cache.fastEntries {
				cache.fastEntries[i].copyFunc(srcBase, dstBase)
			}
		}
	} else {
		// 没有 fast entries，走 reflect 路径
		for i := range cache.fastEntries {
			entry := &cache.fastEntries[i]
			srcField := srcVal.FieldByIndex(entry.srcIndex)
			dstField := dstVal.FieldByIndex(entry.dstIndex)
			if !srcField.IsValid() || !dstField.IsValid() || !dstField.CanSet() {
				continue
			}
			if err := convertFieldByKind(srcField, dstField, entry); err != nil {
				return NewConversionError("字段 %s 转换失败: %v", entry.srcName, err)
			}
		}
	}
	return nil
}

// applySlowEntries 应用慢速路径的 reflect 转换
func (bc *BidiConverter) applySlowEntries(cache *fieldCache, srcVal, dstVal reflect.Value) error {
	if len(cache.slowEntries) == 0 {
		return nil
	}
	// 优先使用 unsafe 指针直接定位字段，避免 FieldByIndex 开销
	if srcVal.CanAddr() && dstVal.CanAddr() {
		srcBase := unsafe.Pointer(srcVal.UnsafeAddr())
		dstBase := unsafe.Pointer(dstVal.UnsafeAddr())
		for i := range cache.slowEntries {
			entry := &cache.slowEntries[i]
			srcField := reflect.NewAt(entry.srcType, addPtr(srcBase, entry.srcOffset)).Elem()
			dstField := reflect.NewAt(entry.dstType, addPtr(dstBase, entry.dstOffset)).Elem()
			if !srcField.IsValid() || !dstField.IsValid() {
				continue
			}
			if cache.hasTransformers && bc.transformers.Has(entry.srcName) {
				srcField = bc.transformers.Apply(entry.srcName, srcField)
			}
			if err := convertFieldByKind(srcField, dstField, entry); err != nil {
				return NewConversionError("字段 %s 转换失败: %v", entry.srcName, err)
			}
		}
	} else {
		for i := range cache.slowEntries {
			entry := &cache.slowEntries[i]
			srcField := srcVal.FieldByIndex(entry.srcIndex)
			dstField := dstVal.FieldByIndex(entry.dstIndex)
			if !srcField.IsValid() || !dstField.IsValid() {
				continue
			}
			if cache.hasTransformers && bc.transformers.Has(entry.srcName) {
				srcField = bc.transformers.Apply(entry.srcName, srcField)
			}
			if err := convertFieldByKind(srcField, dstField, entry); err != nil {
				return NewConversionError("字段 %s 转换失败: %v", entry.srcName, err)
			}
		}
	}
	return nil
}

// makeSameTypeCopyFunc 根据字段类型生成安全指针转换函数
// 设计思想：对于同类型字段（srcType == dstType），直接通过 unsafe.Pointer
// 读取源地址的值并写入目标地址，按 Kind 分支生成类型特化的闭包
// 对于 bool/int/float/string 等基础类型，生成直接的类型化指针读写
// 对于 struct 和其他类型，使用 copy() 进行原始字节拷贝
func makeSameTypeCopyFunc(t reflect.Type, srcOffset, dstOffset uintptr) fieldCopyFunc {
	switch t.Kind() {
	case reflect.Bool:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*bool)(addPtr(dstBase, dstOffset)) = *(*bool)(addPtr(srcBase, srcOffset))
		}
	case reflect.Int:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*int)(addPtr(dstBase, dstOffset)) = *(*int)(addPtr(srcBase, srcOffset))
		}
	case reflect.Int8:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*int8)(addPtr(dstBase, dstOffset)) = *(*int8)(addPtr(srcBase, srcOffset))
		}
	case reflect.Int16:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*int16)(addPtr(dstBase, dstOffset)) = *(*int16)(addPtr(srcBase, srcOffset))
		}
	case reflect.Int32:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*int32)(addPtr(dstBase, dstOffset)) = *(*int32)(addPtr(srcBase, srcOffset))
		}
	case reflect.Int64:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*int64)(addPtr(dstBase, dstOffset)) = *(*int64)(addPtr(srcBase, srcOffset))
		}
	case reflect.Uint:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*uint)(addPtr(dstBase, dstOffset)) = *(*uint)(addPtr(srcBase, srcOffset))
		}
	case reflect.Uint8:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*uint8)(addPtr(dstBase, dstOffset)) = *(*uint8)(addPtr(srcBase, srcOffset))
		}
	case reflect.Uint16:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*uint16)(addPtr(dstBase, dstOffset)) = *(*uint16)(addPtr(srcBase, srcOffset))
		}
	case reflect.Uint32:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*uint32)(addPtr(dstBase, dstOffset)) = *(*uint32)(addPtr(srcBase, srcOffset))
		}
	case reflect.Uint64:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*uint64)(addPtr(dstBase, dstOffset)) = *(*uint64)(addPtr(srcBase, srcOffset))
		}
	case reflect.Float32:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*float32)(addPtr(dstBase, dstOffset)) = *(*float32)(addPtr(srcBase, srcOffset))
		}
	case reflect.Float64:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*float64)(addPtr(dstBase, dstOffset)) = *(*float64)(addPtr(srcBase, srcOffset))
		}
	case reflect.String:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*string)(addPtr(dstBase, dstOffset)) = *(*string)(addPtr(srcBase, srcOffset))
		}
	case reflect.Slice:
		sz := unsafe.Sizeof([]byte{})
		return func(srcBase, dstBase unsafe.Pointer) {
			copy((*[1 << 28]byte)(addPtr(dstBase, dstOffset))[:sz],
				(*[1 << 28]byte)(addPtr(srcBase, srcOffset))[:sz])
		}
	default:
		sz := t.Size()
		return func(srcBase, dstBase unsafe.Pointer) {
			copy((*[1 << 28]byte)(addPtr(dstBase, dstOffset))[:sz],
				(*[1 << 28]byte)(addPtr(srcBase, srcOffset))[:sz])
		}
	}
}

// makeIntegerCopyFunc 生成整数类型转换的 unsafe 闭包
// 设计思想：Protobuf 使用 int32/int64/uint32/uint64，Go Model 使用 int/int64/uint 等
// 通过按 Kind 分支生成特化闭包，避免运行时类型判断和 reflect.Convert 开销
// 例如：int32 → int 只需一次 int32 读取 + int 写入，比 reflect.Convert 快 5-10 倍
func makeIntegerCopyFunc(srcKind, dstKind reflect.Kind, srcOffset, dstOffset uintptr) fieldCopyFunc {
	switch {
	// ====== int32 类型的所有组合（最常见：protobuf int32 ↔ Go int） ======
	case srcKind == reflect.Int32 && dstKind == reflect.Int:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*int)(addPtr(dstBase, dstOffset)) = int(*(*int32)(addPtr(srcBase, srcOffset)))
		}
	case srcKind == reflect.Int && dstKind == reflect.Int32:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*int32)(addPtr(dstBase, dstOffset)) = int32(*(*int)(addPtr(srcBase, srcOffset)))
		}
	case srcKind == reflect.Int32 && dstKind == reflect.Int64:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*int64)(addPtr(dstBase, dstOffset)) = int64(*(*int32)(addPtr(srcBase, srcOffset)))
		}
	case srcKind == reflect.Int64 && dstKind == reflect.Int32:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*int32)(addPtr(dstBase, dstOffset)) = int32(*(*int64)(addPtr(srcBase, srcOffset)))
		}
	case srcKind == reflect.Int32 && dstKind == reflect.Int16:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*int16)(addPtr(dstBase, dstOffset)) = int16(*(*int32)(addPtr(srcBase, srcOffset)))
		}
	case srcKind == reflect.Int16 && dstKind == reflect.Int32:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*int32)(addPtr(dstBase, dstOffset)) = int32(*(*int16)(addPtr(srcBase, srcOffset)))
		}
	case srcKind == reflect.Int32 && dstKind == reflect.Int8:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*int8)(addPtr(dstBase, dstOffset)) = int8(*(*int32)(addPtr(srcBase, srcOffset)))
		}
	case srcKind == reflect.Int8 && dstKind == reflect.Int32:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*int32)(addPtr(dstBase, dstOffset)) = int32(*(*int8)(addPtr(srcBase, srcOffset)))
		}

	// ====== int64 类型的组合 ======
	case srcKind == reflect.Int64 && dstKind == reflect.Int:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*int)(addPtr(dstBase, dstOffset)) = int(*(*int64)(addPtr(srcBase, srcOffset)))
		}
	case srcKind == reflect.Int && dstKind == reflect.Int64:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*int64)(addPtr(dstBase, dstOffset)) = int64(*(*int)(addPtr(srcBase, srcOffset)))
		}

	// ====== uint32 类型的组合 ======
	case srcKind == reflect.Uint32 && dstKind == reflect.Uint64:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*uint64)(addPtr(dstBase, dstOffset)) = uint64(*(*uint32)(addPtr(srcBase, srcOffset)))
		}
	case srcKind == reflect.Uint64 && dstKind == reflect.Uint32:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*uint32)(addPtr(dstBase, dstOffset)) = uint32(*(*uint64)(addPtr(srcBase, srcOffset)))
		}
	case srcKind == reflect.Uint32 && dstKind == reflect.Uint:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*uint)(addPtr(dstBase, dstOffset)) = uint(*(*uint32)(addPtr(srcBase, srcOffset)))
		}
	case srcKind == reflect.Uint && dstKind == reflect.Uint32:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*uint32)(addPtr(dstBase, dstOffset)) = uint32(*(*uint)(addPtr(srcBase, srcOffset)))
		}

	// ====== 有符号 ↔ 无符号的转换 ======
	case srcKind == reflect.Int32 && dstKind == reflect.Uint32:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*uint32)(addPtr(dstBase, dstOffset)) = uint32(*(*int32)(addPtr(srcBase, srcOffset)))
		}
	case srcKind == reflect.Uint32 && dstKind == reflect.Int32:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*int32)(addPtr(dstBase, dstOffset)) = int32(*(*uint32)(addPtr(srcBase, srcOffset)))
		}
	case srcKind == reflect.Int32 && dstKind == reflect.Uint64:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*uint64)(addPtr(dstBase, dstOffset)) = uint64(*(*int32)(addPtr(srcBase, srcOffset)))
		}
	case srcKind == reflect.Int64 && dstKind == reflect.Uint64:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*uint64)(addPtr(dstBase, dstOffset)) = uint64(*(*int64)(addPtr(srcBase, srcOffset)))
		}
	case srcKind == reflect.Uint64 && dstKind == reflect.Int64:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*int64)(addPtr(dstBase, dstOffset)) = int64(*(*uint64)(addPtr(srcBase, srcOffset)))
		}
	case srcKind == reflect.Uint32 && dstKind == reflect.Int:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*int)(addPtr(dstBase, dstOffset)) = int(*(*uint32)(addPtr(srcBase, srcOffset)))
		}
	case srcKind == reflect.Int && dstKind == reflect.Uint32:
		return func(srcBase, dstBase unsafe.Pointer) {
			*(*uint32)(addPtr(dstBase, dstOffset)) = uint32(*(*int)(addPtr(srcBase, srcOffset)))
		}

	// ====== 通用的有符号读取 → 有符号写入 ======
	default:
		return makeIntegerCopyFuncGeneric(srcKind, dstKind, srcOffset, dstOffset)
	}
}

// makeIntegerCopyFuncGeneric 通用整数转换闭包生成
func makeIntegerCopyFuncGeneric(srcKind, dstKind reflect.Kind, srcOffset, dstOffset uintptr) fieldCopyFunc {
	srcSigned := IsSignedInt(srcKind)
	dstSigned := IsSignedInt(dstKind)
	srcSize := kindToSize(srcKind)
	dstSize := kindToSize(dstKind)

	return func(srcBase, dstBase unsafe.Pointer) {
		srcPtr := addPtr(srcBase, srcOffset)
		dstPtr := addPtr(dstBase, dstOffset)

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
}

func kindToSize(k reflect.Kind) uintptr {
	switch k {
	case reflect.Int8, reflect.Uint8, reflect.Bool:
		return 1
	case reflect.Int16, reflect.Uint16:
		return 2
	case reflect.Int32, reflect.Uint32, reflect.Float32:
		return 4
	case reflect.Int64, reflect.Uint64, reflect.Float64, reflect.Int, reflect.Uint:
		return 8
	default:
		return 0
	}
}

// makeTSToTimeCopyFunc *timestamppb.Timestamp → time.Time
func makeTSToTimeCopyFunc(srcOffset, dstOffset uintptr) fieldCopyFunc {
	return func(srcBase, dstBase unsafe.Pointer) {
		tsPtr := *(**timestamppb.Timestamp)(addPtr(srcBase, srcOffset))
		if tsPtr == nil {
			return
		}
		*(*time.Time)(addPtr(dstBase, dstOffset)) = tsPtr.AsTime()
	}
}

// makeTimeToTSCopyFunc time.Time → *timestamppb.Timestamp
func makeTimeToTSCopyFunc(srcOffset, dstOffset uintptr) fieldCopyFunc {
	return func(srcBase, dstBase unsafe.Pointer) {
		t := *(*time.Time)(addPtr(srcBase, srcOffset))
		if t.IsZero() {
			*(**timestamppb.Timestamp)(addPtr(dstBase, dstOffset)) = nil
			return
		}
		*(**timestamppb.Timestamp)(addPtr(dstBase, dstOffset)) = timestamppb.New(t)
	}
}

// makeTSToTimePtrCopyFunc *timestamppb.Timestamp → *time.Time
func makeTSToTimePtrCopyFunc(srcOffset, dstOffset uintptr) fieldCopyFunc {
	return func(srcBase, dstBase unsafe.Pointer) {
		tsPtr := *(**timestamppb.Timestamp)(addPtr(srcBase, srcOffset))
		if tsPtr == nil {
			*(**time.Time)(addPtr(dstBase, dstOffset)) = nil
			return
		}
		t := tsPtr.AsTime()
		*(**time.Time)(addPtr(dstBase, dstOffset)) = &t
	}
}

// makeTimePtrToTSCopyFunc *time.Time → *timestamppb.Timestamp
func makeTimePtrToTSCopyFunc(srcOffset, dstOffset uintptr) fieldCopyFunc {
	return func(srcBase, dstBase unsafe.Pointer) {
		tPtr := *(**time.Time)(addPtr(srcBase, srcOffset))
		if tPtr == nil || tPtr.IsZero() {
			*(**timestamppb.Timestamp)(addPtr(dstBase, dstOffset)) = nil
			return
		}
		*(**timestamppb.Timestamp)(addPtr(dstBase, dstOffset)) = timestamppb.New(*tPtr)
	}
}

// makeWrapperCopyFunc 根据 wrapper 类型和方向生成 unsafe 转换闭包
func makeWrapperCopyFunc(wc *wrapperConverter, dir int, srcOffset, dstOffset uintptr) fieldCopyFunc {
	wt := wc.wrapperType

	switch dir {
	case 0: // *T → *wrapperspb.XxxValue (Model→PB 包装)
		switch {
		case wt == boolValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				srcPtr := *(**bool)(addPtr(srcBase, srcOffset))
				if srcPtr == nil {
					*(**wrapperspb.BoolValue)(addPtr(dstBase, dstOffset)) = nil
					return
				}
				*(**wrapperspb.BoolValue)(addPtr(dstBase, dstOffset)) = wrapperspb.Bool(*srcPtr)
			}
		case wt == int32ValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				srcPtr := *(**int32)(addPtr(srcBase, srcOffset))
				if srcPtr == nil {
					*(**wrapperspb.Int32Value)(addPtr(dstBase, dstOffset)) = nil
					return
				}
				*(**wrapperspb.Int32Value)(addPtr(dstBase, dstOffset)) = wrapperspb.Int32(*srcPtr)
			}
		case wt == int64ValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				srcPtr := *(**int64)(addPtr(srcBase, srcOffset))
				if srcPtr == nil {
					*(**wrapperspb.Int64Value)(addPtr(dstBase, dstOffset)) = nil
					return
				}
				*(**wrapperspb.Int64Value)(addPtr(dstBase, dstOffset)) = wrapperspb.Int64(*srcPtr)
			}
		case wt == uint32ValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				srcPtr := *(**uint32)(addPtr(srcBase, srcOffset))
				if srcPtr == nil {
					*(**wrapperspb.UInt32Value)(addPtr(dstBase, dstOffset)) = nil
					return
				}
				*(**wrapperspb.UInt32Value)(addPtr(dstBase, dstOffset)) = wrapperspb.UInt32(*srcPtr)
			}
		case wt == uint64ValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				srcPtr := *(**uint64)(addPtr(srcBase, srcOffset))
				if srcPtr == nil {
					*(**wrapperspb.UInt64Value)(addPtr(dstBase, dstOffset)) = nil
					return
				}
				*(**wrapperspb.UInt64Value)(addPtr(dstBase, dstOffset)) = wrapperspb.UInt64(*srcPtr)
			}
		case wt == floatValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				srcPtr := *(**float32)(addPtr(srcBase, srcOffset))
				if srcPtr == nil {
					*(**wrapperspb.FloatValue)(addPtr(dstBase, dstOffset)) = nil
					return
				}
				*(**wrapperspb.FloatValue)(addPtr(dstBase, dstOffset)) = wrapperspb.Float(*srcPtr)
			}
		case wt == doubleValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				srcPtr := *(**float64)(addPtr(srcBase, srcOffset))
				if srcPtr == nil {
					*(**wrapperspb.DoubleValue)(addPtr(dstBase, dstOffset)) = nil
					return
				}
				*(**wrapperspb.DoubleValue)(addPtr(dstBase, dstOffset)) = wrapperspb.Double(*srcPtr)
			}
		case wt == stringValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				srcPtr := *(**string)(addPtr(srcBase, srcOffset))
				if srcPtr == nil {
					*(**wrapperspb.StringValue)(addPtr(dstBase, dstOffset)) = nil
					return
				}
				*(**wrapperspb.StringValue)(addPtr(dstBase, dstOffset)) = wrapperspb.String(*srcPtr)
			}
		}

	case 1: // *wrapperspb.XxxValue → *T (PB→Model 解包)
		switch {
		case wt == boolValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				srcPtr := *(**wrapperspb.BoolValue)(addPtr(srcBase, srcOffset))
				if srcPtr == nil {
					*(**bool)(addPtr(dstBase, dstOffset)) = nil
					return
				}
				val := srcPtr.Value
				*(**bool)(addPtr(dstBase, dstOffset)) = &val
			}
		case wt == int32ValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				srcPtr := *(**wrapperspb.Int32Value)(addPtr(srcBase, srcOffset))
				if srcPtr == nil {
					*(**int32)(addPtr(dstBase, dstOffset)) = nil
					return
				}
				val := srcPtr.Value
				*(**int32)(addPtr(dstBase, dstOffset)) = &val
			}
		case wt == int64ValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				srcPtr := *(**wrapperspb.Int64Value)(addPtr(srcBase, srcOffset))
				if srcPtr == nil {
					*(**int64)(addPtr(dstBase, dstOffset)) = nil
					return
				}
				val := srcPtr.Value
				*(**int64)(addPtr(dstBase, dstOffset)) = &val
			}
		case wt == uint32ValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				srcPtr := *(**wrapperspb.UInt32Value)(addPtr(srcBase, srcOffset))
				if srcPtr == nil {
					*(**uint32)(addPtr(dstBase, dstOffset)) = nil
					return
				}
				val := srcPtr.Value
				*(**uint32)(addPtr(dstBase, dstOffset)) = &val
			}
		case wt == uint64ValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				srcPtr := *(**wrapperspb.UInt64Value)(addPtr(srcBase, srcOffset))
				if srcPtr == nil {
					*(**uint64)(addPtr(dstBase, dstOffset)) = nil
					return
				}
				val := srcPtr.Value
				*(**uint64)(addPtr(dstBase, dstOffset)) = &val
			}
		case wt == floatValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				srcPtr := *(**wrapperspb.FloatValue)(addPtr(srcBase, srcOffset))
				if srcPtr == nil {
					*(**float32)(addPtr(dstBase, dstOffset)) = nil
					return
				}
				val := srcPtr.Value
				*(**float32)(addPtr(dstBase, dstOffset)) = &val
			}
		case wt == doubleValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				srcPtr := *(**wrapperspb.DoubleValue)(addPtr(srcBase, srcOffset))
				if srcPtr == nil {
					*(**float64)(addPtr(dstBase, dstOffset)) = nil
					return
				}
				val := srcPtr.Value
				*(**float64)(addPtr(dstBase, dstOffset)) = &val
			}
		case wt == stringValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				srcPtr := *(**wrapperspb.StringValue)(addPtr(srcBase, srcOffset))
				if srcPtr == nil {
					*(**string)(addPtr(dstBase, dstOffset)) = nil
					return
				}
				val := srcPtr.Value
				*(**string)(addPtr(dstBase, dstOffset)) = &val
			}
		}

	case 2:
		switch {
		case wt == boolValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				val := *(*bool)(addPtr(srcBase, srcOffset))
				*(**wrapperspb.BoolValue)(addPtr(dstBase, dstOffset)) = wrapperspb.Bool(val)
			}
		case wt == int32ValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				val := *(*int32)(addPtr(srcBase, srcOffset))
				*(**wrapperspb.Int32Value)(addPtr(dstBase, dstOffset)) = wrapperspb.Int32(val)
			}
		case wt == int64ValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				val := *(*int64)(addPtr(srcBase, srcOffset))
				*(**wrapperspb.Int64Value)(addPtr(dstBase, dstOffset)) = wrapperspb.Int64(val)
			}
		case wt == uint32ValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				val := *(*uint32)(addPtr(srcBase, srcOffset))
				*(**wrapperspb.UInt32Value)(addPtr(dstBase, dstOffset)) = wrapperspb.UInt32(val)
			}
		case wt == uint64ValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				val := *(*uint64)(addPtr(srcBase, srcOffset))
				*(**wrapperspb.UInt64Value)(addPtr(dstBase, dstOffset)) = wrapperspb.UInt64(val)
			}
		case wt == floatValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				val := *(*float32)(addPtr(srcBase, srcOffset))
				*(**wrapperspb.FloatValue)(addPtr(dstBase, dstOffset)) = wrapperspb.Float(val)
			}
		case wt == doubleValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				val := *(*float64)(addPtr(srcBase, srcOffset))
				*(**wrapperspb.DoubleValue)(addPtr(dstBase, dstOffset)) = wrapperspb.Double(val)
			}
		case wt == stringValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				val := *(*string)(addPtr(srcBase, srcOffset))
				*(**wrapperspb.StringValue)(addPtr(dstBase, dstOffset)) = wrapperspb.String(val)
			}
		}

	case 3:
		switch {
		case wt == boolValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				srcPtr := *(**wrapperspb.BoolValue)(addPtr(srcBase, srcOffset))
				if srcPtr == nil {
					*(*bool)(addPtr(dstBase, dstOffset)) = false
					return
				}
				*(*bool)(addPtr(dstBase, dstOffset)) = srcPtr.Value
			}
		case wt == int32ValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				srcPtr := *(**wrapperspb.Int32Value)(addPtr(srcBase, srcOffset))
				if srcPtr == nil {
					*(*int32)(addPtr(dstBase, dstOffset)) = 0
					return
				}
				*(*int32)(addPtr(dstBase, dstOffset)) = srcPtr.Value
			}
		case wt == int64ValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				srcPtr := *(**wrapperspb.Int64Value)(addPtr(srcBase, srcOffset))
				if srcPtr == nil {
					*(*int64)(addPtr(dstBase, dstOffset)) = 0
					return
				}
				*(*int64)(addPtr(dstBase, dstOffset)) = srcPtr.Value
			}
		case wt == uint32ValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				srcPtr := *(**wrapperspb.UInt32Value)(addPtr(srcBase, srcOffset))
				if srcPtr == nil {
					*(*uint32)(addPtr(dstBase, dstOffset)) = 0
					return
				}
				*(*uint32)(addPtr(dstBase, dstOffset)) = srcPtr.Value
			}
		case wt == uint64ValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				srcPtr := *(**wrapperspb.UInt64Value)(addPtr(srcBase, srcOffset))
				if srcPtr == nil {
					*(*uint64)(addPtr(dstBase, dstOffset)) = 0
					return
				}
				*(*uint64)(addPtr(dstBase, dstOffset)) = srcPtr.Value
			}
		case wt == floatValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				srcPtr := *(**wrapperspb.FloatValue)(addPtr(srcBase, srcOffset))
				if srcPtr == nil {
					*(*float32)(addPtr(dstBase, dstOffset)) = 0
					return
				}
				*(*float32)(addPtr(dstBase, dstOffset)) = srcPtr.Value
			}
		case wt == doubleValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				srcPtr := *(**wrapperspb.DoubleValue)(addPtr(srcBase, srcOffset))
				if srcPtr == nil {
					*(*float64)(addPtr(dstBase, dstOffset)) = 0
					return
				}
				*(*float64)(addPtr(dstBase, dstOffset)) = srcPtr.Value
			}
		case wt == stringValuePtrType:
			return func(srcBase, dstBase unsafe.Pointer) {
				srcPtr := *(**wrapperspb.StringValue)(addPtr(srcBase, srcOffset))
				if srcPtr == nil {
					*(*string)(addPtr(dstBase, dstOffset)) = ""
					return
				}
				*(*string)(addPtr(dstBase, dstOffset)) = srcPtr.Value
			}
		}
	}
	return nil
}

// makeConvertibleCopyFunc 生成可转换类型（相同底层类型）的 unsafe 拷贝闭包
func makeConvertibleCopyFunc(srcType, dstType reflect.Type, srcOffset, dstOffset uintptr) fieldCopyFunc {
	if srcType.Size() != dstType.Size() {
		return nil
	}
	sz := srcType.Size()
	return func(srcBase, dstBase unsafe.Pointer) {
		copy((*[1 << 28]byte)(addPtr(dstBase, dstOffset))[:sz],
			(*[1 << 28]byte)(addPtr(srcBase, srcOffset))[:sz])
	}
}

// makeFieldCopyFunc 根据字段类型分类生成对应的 copyFunc，无法快速路径时返回 nil
// 这是 unsafe 快速路径的核心调度器，根据 fieldKind 路由到对应的生成函数：
//   - fieldSameType/fieldAssignable → makeSameTypeCopyFunc（直接内存拷贝）
//   - fieldInteger                → makeIntegerCopyFunc（类型化整数转换）
//   - fieldTSToTime 等            → 时间戳/时间转换闭包
//   - fieldWrapper                → makeWrapperCopyFunc（Proto Wrapper 指针转换）
//   - fieldToPtr                  → makeToPtrCopyFunc（值→指针，含 reflect.New 分配）
//   - fieldFromPtr                → makeFromPtrCopyFunc（指针→值解引用）
//   - fieldStringFallback         → makeStringFallbackCopyFunc（字符串命名类型）
//   - fieldDataWrapper            → makeDataWrapperCopyFunc（DataWrapper[T] 解/包装）
//   - fieldStructPtr              → makeStructPtrCopyFunc（嵌套结构体指针，使用子缓存）
//   - fieldStruct                 → makeStructCopyFunc（嵌套值结构体，使用子缓存）
//   - fieldSlice                 → makeSliceCopyFunc（切片逐元素转换，使用元素级 copyFunc）
//   - fieldMapStruct             → makeMapStructCopyFunc（map[string]any → *structpb.Struct，跳过反射）
//   - fieldMap/fieldNoop         → 返回 nil，走 reflect 慢路径
func makeFieldCopyFunc(kind fieldKind, srcType, dstType reflect.Type, srcOffset, dstOffset uintptr, wc *wrapperConverter, wrapperDir int) fieldCopyFunc {
	switch kind {
	case fieldSameType:
		return makeSameTypeCopyFunc(srcType, srcOffset, dstOffset)
	case fieldInteger:
		return makeIntegerCopyFunc(srcType.Kind(), dstType.Kind(), srcOffset, dstOffset)
	case fieldAssignable:
		return makeSameTypeCopyFunc(srcType, srcOffset, dstOffset)
	case fieldConvertible:
		if srcType.Size() == dstType.Size() {
			return makeConvertibleCopyFunc(srcType, dstType, srcOffset, dstOffset)
		}
		return nil
	case fieldTSToTime:
		return makeTSToTimeCopyFunc(srcOffset, dstOffset)
	case fieldTimeToTS:
		return makeTimeToTSCopyFunc(srcOffset, dstOffset)
	case fieldTSToTimePtr:
		return makeTSToTimePtrCopyFunc(srcOffset, dstOffset)
	case fieldTimePtrToTS:
		return makeTimePtrToTSCopyFunc(srcOffset, dstOffset)
	case fieldWrapper:
		return makeWrapperCopyFunc(wc, wrapperDir, srcOffset, dstOffset)
	case fieldToPtr:
		return makeToPtrCopyFunc(srcType, dstType, srcOffset, dstOffset)
	case fieldFromPtr:
		return makeFromPtrCopyFunc(srcType, dstType, srcOffset, dstOffset)
	case fieldStringFallback:
		return makeStringFallbackCopyFunc(srcOffset, dstOffset)
	case fieldDataWrapper:
		return makeDataWrapperCopyFunc(srcType, dstType, srcOffset, dstOffset)
	case fieldStructPtr:
		return makeStructPtrCopyFunc(srcType, dstType, srcOffset, dstOffset)
	case fieldStruct:
		return makeStructCopyFunc(srcType, dstType, srcOffset, dstOffset)
	case fieldSlice:
		return makeSliceCopyFunc(srcType, dstType, srcOffset, dstOffset)
	case fieldMap:
		return makeMapCopyFunc(srcType, dstType, srcOffset, dstOffset)
	case fieldMapStruct:
		return makeMapStructCopyFunc(srcType, dstType, srcOffset, dstOffset)
	}
	return nil
}

// makeMapStructCopyFunc 生成 map[string]any → *structpb.Struct 的 unsafe 快速路径 copyFunc
// 设计思想：fieldMapStruct 字段原先无 copyFunc，落入 slowEntries 经 convertMapStruct 反射处理，
// 每次都要 srcField.Type().ConvertibleTo() + srcField.Convert() + .Interface() + dstField.Set() 等反射开销
// 本函数在构建缓存时预判方向，直接生成 unsafe 指针闭包，跳过全部反射路径
//
// 支持方向（Model→PB 热路径）：
//  1. map[string][]string → *structpb.Struct  直接 buildStructFromMapStringSlice（永不失败）
//  2. map[string]interface{} → *structpb.Struct  先 structpb.NewStruct，失败时 normalizeValue 兜底
//
// 反向 *structpb.Struct → map 及其他复杂场景返回 nil，沿用 convertMapStruct 反射慢路径
func makeMapStructCopyFunc(srcType, dstType reflect.Type, srcOffset, dstOffset uintptr) fieldCopyFunc {
	if srcType.Kind() != reflect.Map || dstType != structPtrType {
		return nil
	}
	// 快路径 1：map[string][]string（含 CompressedTextMap 等命名类型），直接构建 structpb.Struct
	// buildStructFromMapStringSlice 不会返回错误，可安全跳过反射全部开销
	if srcType.ConvertibleTo(mapStringSliceType) {
		return func(srcBase, dstBase unsafe.Pointer) {
			m := *(*map[string][]string)(addPtr(srcBase, srcOffset))
			if m == nil {
				*(**structpb.Struct)(addPtr(dstBase, dstOffset)) = nil
				return
			}
			*(**structpb.Struct)(addPtr(dstBase, dstOffset)) = buildStructFromMapStringSlice(m)
		}
	}
	// 快路径 2：map[string]interface{}，先尝试 buildStructFromMapAny 批量预分配路径
	// 失败时（值含非扁平类型或非法类型）才走 normalizeValue 逐项修复 + structpb.NewStruct
	if srcType.ConvertibleTo(mapStringAnyType) {
		return func(srcBase, dstBase unsafe.Pointer) {
			m := *(*map[string]interface{})(addPtr(srcBase, srcOffset))
			if m == nil {
				*(**structpb.Struct)(addPtr(dstBase, dstOffset)) = nil
				return
			}
			st, err := buildStructFromMapAny(m)
			if err == nil && st != nil {
				*(**structpb.Struct)(addPtr(dstBase, dstOffset)) = st
				return
			}
			// 兜底：normalizeValue 归一化后 structpb.NewStruct
			normalized := make(map[string]interface{}, len(m))
			for k, v := range m {
				normalized[k] = normalizeValue(v)
			}
			if st, err = structpb.NewStruct(normalized); err != nil {
				// 值类型非法（chan/func 等），copyFunc 签名不支持 error，置 nil 与零值语义一致
				*(**structpb.Struct)(addPtr(dstBase, dstOffset)) = nil
				return
			}
			*(**structpb.Struct)(addPtr(dstBase, dstOffset)) = st
		}
	}
	return nil
}

// makeStructPtrCopyFunc 生成结构体指针转换的 copyFunc（*StructA → *StructB）
// 设计思想：利用缓存的子结构体字段映射（structFieldCache），将嵌套结构体转换
// 从反射路径转为 unsafe 快速路径：
//  1. 解引用源指针，若 nil 则设置目标为 nil
//  2. reflect.New(dstElemType) 分配目标结构体
//  3. 执行子缓存的 fastEntries（mergedFn 或逐个 copyFunc）
//  4. 执行子缓存的 slowEntries（reflect 回退）
//
// 性能提升：从 3-5x 原生 优化到 ≈1.5x 原生（消除 FieldByName/convertFieldAuto 开销）
func makeStructPtrCopyFunc(srcType, dstType reflect.Type, srcOffset, dstOffset uintptr) fieldCopyFunc {
	srcElemType := srcType.Elem()
	dstElemType := dstType.Elem()

	subCache, ok := globalStructFieldCache.Load(structFieldTypePair{srcElemType, dstElemType})
	if !ok {
		subCache = buildStructFieldCache(srcElemType, dstElemType, true)
		globalStructFieldCache.Store(structFieldTypePair{srcElemType, dstElemType}, subCache)
	}

	sc := subCache.(*structFieldCache)
	fastFns := make([]fieldCopyFunc, len(sc.fastEntries))
	for i := range sc.fastEntries {
		fastFns[i] = sc.fastEntries[i].copyFunc
	}
	needSubConvert := len(sc.slowEntries) > 0

	if !needSubConvert && len(fastFns) > 0 {
		return func(srcBase, dstBase unsafe.Pointer) {
			srcPtr := *(*unsafe.Pointer)(addPtr(srcBase, srcOffset))
			if srcPtr == nil {
				*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = nil
				return
			}
			dstPtr := reflect.New(dstElemType).UnsafePointer()
			*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = dstPtr
			for i := range fastFns {
				fastFns[i](srcPtr, dstPtr)
			}
		}
	}

	if len(fastFns) > 0 {
		return func(srcBase, dstBase unsafe.Pointer) {
			srcPtr := *(*unsafe.Pointer)(addPtr(srcBase, srcOffset))
			if srcPtr == nil {
				*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = nil
				return
			}
			dstPtrVal := reflect.New(dstElemType)
			dstPtr := dstPtrVal.UnsafePointer()
			*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = dstPtr
			for i := range fastFns {
				fastFns[i](srcPtr, dstPtr)
			}
			applyStructSlowEntriesUnsafe(srcPtr, dstPtr, sc)
		}
	}

	return nil
}

// makeStructCopyFunc 生成值结构体转换的 copyFunc（StructA → StructB）
// 与 makeStructPtrCopyFunc 类似，但处理值类型而非指针类型
// 通过预构建的 structFieldCache 子缓存实现嵌套结构体的快速转换
func makeStructCopyFunc(srcType, dstType reflect.Type, srcOffset, dstOffset uintptr) fieldCopyFunc {
	subCache, ok := globalStructFieldCache.Load(structFieldTypePair{srcType, dstType})
	if !ok {
		subCache = buildStructFieldCache(srcType, dstType, true)
		globalStructFieldCache.Store(structFieldTypePair{srcType, dstType}, subCache)
	}

	sc := subCache.(*structFieldCache)
	fastFns := make([]fieldCopyFunc, len(sc.fastEntries))
	for i := range sc.fastEntries {
		fastFns[i] = sc.fastEntries[i].copyFunc
	}
	needSubConvert := len(sc.slowEntries) > 0

	if len(fastFns) == 0 {
		return nil
	}

	if !needSubConvert {
		return func(srcBase, dstBase unsafe.Pointer) {
			srcPtr := addPtr(srcBase, srcOffset)
			dstPtr := addPtr(dstBase, dstOffset)
			for i := range fastFns {
				fastFns[i](srcPtr, dstPtr)
			}
		}
	}

	return func(srcBase, dstBase unsafe.Pointer) {
		srcPtr := addPtr(srcBase, srcOffset)
		dstPtr := addPtr(dstBase, dstOffset)
		for i := range fastFns {
			fastFns[i](srcPtr, dstPtr)
		}
		applyStructSlowEntriesUnsafe(srcPtr, dstPtr, sc)
	}
}

// makeSliceCopyFunc 生成切片类型转换的 copyFunc（[]SrcElem → []DstElem）
// 设计思想：切片转换是最常见的慢路径场景，原生写法是 for 循环逐元素赋值
// 通过预计算元素级转换函数（elemCopyFunc），将 reflect 循环转为 unsafe 指针循环
//
// 策略分层：
//  1. 元素类型相同 → 已由 fieldSameType 处理（此函数不会收到）
//  2. 元素类型可转换（integer/convertible）→ 生成类型特化的逐元素拷贝闭包
//  3. 元素为结构体指针 → 使用缓存的 structFieldCache 做元素级快速转换
//  4. 元素为指针（*T → *U）→ 生成元素级 convert+New 闭包
//  5. 其他（无法优化）→ 返回 nil，走 reflect 慢路径
//
// Go 切片头布局（3 个 uintptr）：
//
//	type sliceHeader struct { Data unsafe.Pointer; Len int; Cap int }
//
// 通过 unsafe 读取源切片头部，再 reflect.MakeSlice 分配目标切片，
// 最后通过 unsafe 指针算术逐元素调用 elemCopyFunc
func makeSliceCopyFunc(srcType, dstType reflect.Type, srcOffset, dstOffset uintptr) fieldCopyFunc {
	srcElemType := srcType.Elem()
	dstElemType := dstType.Elem()

	// 分类元素类型，决定使用哪种转换策略
	elemKind, _, _ := classifyField(srcElemType, dstElemType, true)

	// 策略 1：元素为整数类型 → 生成特化的逐元素转换闭包
	if elemKind == fieldInteger {
		srcElemKind := srcElemType.Kind()
		dstElemKind := dstElemType.Kind()
		srcElemSize := srcElemType.Size()
		dstElemSize := dstElemType.Size()
		return func(srcBase, dstBase unsafe.Pointer) {
			srcSlice := *(*sliceHeader)(addPtr(srcBase, srcOffset))
			if srcSlice.Len == 0 {
				dstSlice := reflect.MakeSlice(dstType, 0, 0)
				*(*sliceHeader)(addPtr(dstBase, dstOffset)) = sliceHeader{
					Data: dstSlice.UnsafePointer(), Len: 0, Cap: 0,
				}
				return
			}
			dstSlice := reflect.MakeSlice(dstType, srcSlice.Len, srcSlice.Len)
			srcData := srcSlice.Data
			dstData := dstSlice.UnsafePointer()
			for i := 0; i < srcSlice.Len; i++ {
				srcAddr := addPtr(srcData, uintptr(i)*srcElemSize)
				dstAddr := addPtr(dstData, uintptr(i)*dstElemSize)
				convertIntegerValue(srcAddr, dstAddr, srcElemKind, dstElemKind)
			}
			*(*sliceHeader)(addPtr(dstBase, dstOffset)) = sliceHeader{
				Data: dstSlice.UnsafePointer(), Len: srcSlice.Len, Cap: srcSlice.Len,
			}
		}
	}

	// 策略 2：元素类型可直接赋值/转换 → 逐元素 Convert
	if elemKind == fieldAssignable || elemKind == fieldConvertible {
		srcElemSize := srcElemType.Size()
		needConvert := srcElemType != dstElemType
		return func(srcBase, dstBase unsafe.Pointer) {
			srcSlice := *(*sliceHeader)(addPtr(srcBase, srcOffset))
			if srcSlice.Len == 0 {
				dstSlice := reflect.MakeSlice(dstType, 0, 0)
				*(*sliceHeader)(addPtr(dstBase, dstOffset)) = sliceHeader{
					Data: dstSlice.UnsafePointer(), Len: 0, Cap: 0,
				}
				return
			}
			dstSlice := reflect.MakeSlice(dstType, srcSlice.Len, srcSlice.Len)
			if needConvert {
				srcData := srcSlice.Data
				for i := 0; i < srcSlice.Len; i++ {
					srcElem := reflect.NewAt(srcElemType, addPtr(srcData, uintptr(i)*srcElemSize)).Elem()
					dstSlice.Index(i).Set(srcElem.Convert(dstElemType))
				}
			} else {
				srcData := srcSlice.Data
				dstData := dstSlice.UnsafePointer()
				elemSize := srcElemSize
				for i := 0; i < srcSlice.Len; i++ {
					srcAddr := addPtr(srcData, uintptr(i)*elemSize)
					dstAddr := addPtr(dstData, uintptr(i)*elemSize)
					copy((*[1 << 28]byte)(dstAddr)[:elemSize], (*[1 << 28]byte)(srcAddr)[:elemSize])
				}
			}
			*(*sliceHeader)(addPtr(dstBase, dstOffset)) = sliceHeader{
				Data: dstSlice.UnsafePointer(), Len: srcSlice.Len, Cap: srcSlice.Len,
			}
		}
	}

	// 策略 3：元素为 *Struct 指针 → 使用 structFieldCache 做元素级 unsafe 快速转换
	// 优化：使用连续内存批量分配（backing slice），将 N+1 次分配减少到 2 次
	if elemKind == fieldStructPtr {
		srcInnerType := srcElemType.Elem()
		dstInnerType := dstElemType.Elem()
		dstInnerSize := dstInnerType.Size()
		subCache, ok := globalStructFieldCache.Load(structFieldTypePair{srcInnerType, dstInnerType})
		if !ok {
			subCache = buildStructFieldCache(srcInnerType, dstInnerType, true)
			globalStructFieldCache.Store(structFieldTypePair{srcInnerType, dstInnerType}, subCache)
		}
		sc := subCache.(*structFieldCache)
		fastFns := make([]fieldCopyFunc, len(sc.fastEntries))
		for i := range sc.fastEntries {
			fastFns[i] = sc.fastEntries[i].copyFunc
		}
		hasSlowEntries := len(sc.slowEntries) > 0
		backingSliceType := reflect.SliceOf(dstInnerType)

		if !hasSlowEntries && len(fastFns) > 0 {
			return func(srcBase, dstBase unsafe.Pointer) {
				srcSlice := *(*sliceHeader)(addPtr(srcBase, srcOffset))
				if srcSlice.Len == 0 {
					dstSlice := reflect.MakeSlice(dstType, 0, 0)
					*(*sliceHeader)(addPtr(dstBase, dstOffset)) = sliceHeader{
						Data: dstSlice.UnsafePointer(), Len: 0, Cap: 0,
					}
					return
				}
				// 批量分配：一次分配所有元素 + 一次分配指针切片
				backing := reflect.MakeSlice(backingSliceType, srcSlice.Len, srcSlice.Len)
				dstSlice := reflect.MakeSlice(dstType, srcSlice.Len, srcSlice.Len)
				srcData := srcSlice.Data
				dstData := dstSlice.UnsafePointer()
				backingData := backing.UnsafePointer()
				for i := 0; i < srcSlice.Len; i++ {
					srcPtr := *(*unsafe.Pointer)(addPtr(srcData, uintptr(i)*PtrSize))
					if srcPtr == nil {
						*(*unsafe.Pointer)(addPtr(dstData, uintptr(i)*PtrSize)) = nil
						continue
					}
					// 直接在 backing slice 中定位元素地址，无需 reflect.New
					dstElemPtr := addPtr(backingData, uintptr(i)*dstInnerSize)
					for j := range fastFns {
						fastFns[j](srcPtr, dstElemPtr)
					}
					*(*unsafe.Pointer)(addPtr(dstData, uintptr(i)*PtrSize)) = dstElemPtr
				}
				*(*sliceHeader)(addPtr(dstBase, dstOffset)) = sliceHeader{
					Data: dstSlice.UnsafePointer(), Len: srcSlice.Len, Cap: srcSlice.Len,
				}
			}
		}

		if len(fastFns) > 0 {
			return func(srcBase, dstBase unsafe.Pointer) {
				srcSlice := *(*sliceHeader)(addPtr(srcBase, srcOffset))
				if srcSlice.Len == 0 {
					dstSlice := reflect.MakeSlice(dstType, 0, 0)
					*(*sliceHeader)(addPtr(dstBase, dstOffset)) = sliceHeader{
						Data: dstSlice.UnsafePointer(), Len: 0, Cap: 0,
					}
					return
				}
				backing := reflect.MakeSlice(backingSliceType, srcSlice.Len, srcSlice.Len)
				dstSlice := reflect.MakeSlice(dstType, srcSlice.Len, srcSlice.Len)
				srcData := srcSlice.Data
				dstData := dstSlice.UnsafePointer()
				backingData := backing.UnsafePointer()
				for i := 0; i < srcSlice.Len; i++ {
					srcPtr := *(*unsafe.Pointer)(addPtr(srcData, uintptr(i)*PtrSize))
					if srcPtr == nil {
						*(*unsafe.Pointer)(addPtr(dstData, uintptr(i)*PtrSize)) = nil
						continue
					}
					dstElemPtr := addPtr(backingData, uintptr(i)*dstInnerSize)
					for j := range fastFns {
						fastFns[j](srcPtr, dstElemPtr)
					}
					applyStructSlowEntriesUnsafe(srcPtr, dstElemPtr, sc)
					*(*unsafe.Pointer)(addPtr(dstData, uintptr(i)*PtrSize)) = dstElemPtr
				}
				*(*sliceHeader)(addPtr(dstBase, dstOffset)) = sliceHeader{
					Data: dstSlice.UnsafePointer(), Len: srcSlice.Len, Cap: srcSlice.Len,
				}
			}
		}

		return nil
	}

	// 策略 4：元素为值结构体 → 使用 structFieldCache 做元素级 unsafe 快速转换
	if elemKind == fieldStruct {
		subCache, ok := globalStructFieldCache.Load(structFieldTypePair{srcElemType, dstElemType})
		if !ok {
			subCache = buildStructFieldCache(srcElemType, dstElemType, true)
			globalStructFieldCache.Store(structFieldTypePair{srcElemType, dstElemType}, subCache)
		}
		sc := subCache.(*structFieldCache)
		fastFns := make([]fieldCopyFunc, len(sc.fastEntries))
		for i := range sc.fastEntries {
			fastFns[i] = sc.fastEntries[i].copyFunc
		}
		hasSlowEntries := len(sc.slowEntries) > 0

		if !hasSlowEntries && len(fastFns) > 0 {
			return func(srcBase, dstBase unsafe.Pointer) {
				srcSlice := *(*sliceHeader)(addPtr(srcBase, srcOffset))
				if srcSlice.Len == 0 {
					dstSlice := reflect.MakeSlice(dstType, 0, 0)
					*(*sliceHeader)(addPtr(dstBase, dstOffset)) = sliceHeader{
						Data: dstSlice.UnsafePointer(), Len: 0, Cap: 0,
					}
					return
				}
				dstSlice := reflect.MakeSlice(dstType, srcSlice.Len, srcSlice.Len)
				srcData := srcSlice.Data
				dstData := dstSlice.UnsafePointer()
				srcElemSize := srcElemType.Size()
				dstElemSize := dstElemType.Size()
				for i := 0; i < srcSlice.Len; i++ {
					srcElemPtr := addPtr(srcData, uintptr(i)*srcElemSize)
					dstElemPtr := addPtr(dstData, uintptr(i)*dstElemSize)
					for j := range fastFns {
						fastFns[j](srcElemPtr, dstElemPtr)
					}
				}
				*(*sliceHeader)(addPtr(dstBase, dstOffset)) = sliceHeader{
					Data: dstSlice.UnsafePointer(), Len: srcSlice.Len, Cap: srcSlice.Len,
				}
			}
		}

		if len(fastFns) > 0 {
			return func(srcBase, dstBase unsafe.Pointer) {
				srcSlice := *(*sliceHeader)(addPtr(srcBase, srcOffset))
				if srcSlice.Len == 0 {
					dstSlice := reflect.MakeSlice(dstType, 0, 0)
					*(*sliceHeader)(addPtr(dstBase, dstOffset)) = sliceHeader{
						Data: dstSlice.UnsafePointer(), Len: 0, Cap: 0,
					}
					return
				}
				dstSlice := reflect.MakeSlice(dstType, srcSlice.Len, srcSlice.Len)
				srcData := srcSlice.Data
				dstData := dstSlice.UnsafePointer()
				srcElemSize := srcElemType.Size()
				dstElemSize := dstElemType.Size()
				for i := 0; i < srcSlice.Len; i++ {
					srcElemPtr := addPtr(srcData, uintptr(i)*srcElemSize)
					dstElemPtr := addPtr(dstData, uintptr(i)*dstElemSize)
					for j := range fastFns {
						fastFns[j](srcElemPtr, dstElemPtr)
					}
					applyStructSlowEntriesUnsafe(srcElemPtr, dstElemPtr, sc)
				}
				*(*sliceHeader)(addPtr(dstBase, dstOffset)) = sliceHeader{
					Data: dstSlice.UnsafePointer(), Len: srcSlice.Len, Cap: srcSlice.Len,
				}
			}
		}

		return nil
	}

	// 策略 5：元素为 *T → *U（T→U 有快速的 toPtr/fromPtr 等转换路径）
	if elemKind == fieldToPtr || elemKind == fieldFromPtr {
		elemCopyFn := makeFieldCopyFunc(elemKind, srcElemType, dstElemType, 0, 0, nil, 0)
		if elemCopyFn == nil {
			return nil
		}
		srcElemSize := srcElemType.Size()
		dstElemSize := dstElemType.Size()
		return func(srcBase, dstBase unsafe.Pointer) {
			srcSlice := *(*sliceHeader)(addPtr(srcBase, srcOffset))
			if srcSlice.Len == 0 {
				dstSlice := reflect.MakeSlice(dstType, 0, 0)
				*(*sliceHeader)(addPtr(dstBase, dstOffset)) = sliceHeader{
					Data: dstSlice.UnsafePointer(), Len: 0, Cap: 0,
				}
				return
			}
			dstSlice := reflect.MakeSlice(dstType, srcSlice.Len, srcSlice.Len)
			srcData := srcSlice.Data
			dstData := dstSlice.UnsafePointer()
			for i := 0; i < srcSlice.Len; i++ {
				srcElemPtr := addPtr(srcData, uintptr(i)*srcElemSize)
				dstElemPtr := addPtr(dstData, uintptr(i)*dstElemSize)
				elemCopyFn(srcElemPtr, dstElemPtr)
			}
			*(*sliceHeader)(addPtr(dstBase, dstOffset)) = sliceHeader{
				Data: dstSlice.UnsafePointer(), Len: srcSlice.Len, Cap: srcSlice.Len,
			}
		}
	}

	// 策略 6：时间戳→时间、时间→时间戳等元素转换
	if elemKind == fieldTSToTime || elemKind == fieldTimeToTS || elemKind == fieldWrapper {
		srcElemSize := srcElemType.Size()
		dstElemSize := dstElemType.Size()
		return func(srcBase, dstBase unsafe.Pointer) {
			srcSlice := *(*sliceHeader)(addPtr(srcBase, srcOffset))
			if srcSlice.Len == 0 {
				dstSlice := reflect.MakeSlice(dstType, 0, 0)
				*(*sliceHeader)(addPtr(dstBase, dstOffset)) = sliceHeader{
					Data: dstSlice.UnsafePointer(), Len: 0, Cap: 0,
				}
				return
			}
			dstSlice := reflect.MakeSlice(dstType, srcSlice.Len, srcSlice.Len)
			srcData := srcSlice.Data
			dstData := dstSlice.UnsafePointer()
			for i := 0; i < srcSlice.Len; i++ {
				srcElem := reflect.NewAt(srcElemType, addPtr(srcData, uintptr(i)*srcElemSize)).Elem()
				converted := srcElem.Convert(dstElemType)
				dstElemPtr := addPtr(dstData, uintptr(i)*dstElemSize)
				reflect.NewAt(dstElemType, dstElemPtr).Elem().Set(converted)
			}
			*(*sliceHeader)(addPtr(dstBase, dstOffset)) = sliceHeader{
				Data: dstSlice.UnsafePointer(), Len: srcSlice.Len, Cap: srcSlice.Len,
			}
		}
	}

	return nil
}

// makeMapCopyFunc 生成 map 转换的 unsafe 闭包（map[K1]V1 → map[K2]V2）
// 优化策略：
//  1. key 类型相同 + value 类型相同 → 直接赋值（map 内部是指针，浅拷贝即可）
//  2. key 可转换 + value 可转换 → 遍历 + Convert
//  3. key 相同 + value 为结构体 → 遍历 + structFieldCache
//  4. 其他 → 返回 nil 走 reflect 慢路径
func makeMapCopyFunc(srcType, dstType reflect.Type, srcOffset, dstOffset uintptr) fieldCopyFunc {
	srcKeyType := srcType.Key()
	dstKeyType := dstType.Key()
	srcValType := srcType.Elem()
	dstValType := dstType.Elem()

	// 策略 1：key 和 value 类型都相同 → map 本身是指针，直接赋值
	if srcKeyType == dstKeyType && srcValType == dstValType {
		return func(srcBase, dstBase unsafe.Pointer) {
			srcMap := *(*unsafe.Pointer)(addPtr(srcBase, srcOffset))
			*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = srcMap
		}
	}

	// 策略 2：key 和 value 都可直接 Convert → 遍历 + Convert
	keyConvertible := srcKeyType != dstKeyType && srcKeyType.ConvertibleTo(dstKeyType)
	valConvertible := srcValType != dstValType && srcValType.ConvertibleTo(dstValType)
	keySame := srcKeyType == dstKeyType
	valSame := srcValType == dstValType

	if (keySame || keyConvertible) && (valSame || valConvertible) {
		return func(srcBase, dstBase unsafe.Pointer) {
			srcMapVal := reflect.NewAt(srcType, addPtr(srcBase, srcOffset)).Elem()
			if srcMapVal.IsNil() {
				*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = nil
				return
			}
			dstMap := reflect.MakeMapWithSize(dstType, srcMapVal.Len())
			iter := srcMapVal.MapRange()
			for iter.Next() {
				srcKey := iter.Key()
				srcVal := iter.Value()
				dstKey := srcKey
				if !keySame {
					dstKey = srcKey.Convert(dstKeyType)
				}
				dstVal := srcVal
				if !valSame {
					dstVal = srcVal.Convert(dstValType)
				}
				dstMap.SetMapIndex(dstKey, dstVal)
			}
			dstMapVal := reflect.NewAt(dstType, addPtr(dstBase, dstOffset)).Elem()
			dstMapVal.Set(dstMap)
		}
	}

	// 策略 3：key 相同 + value 为结构体 → 使用 structFieldCache
	if keySame && srcValType.Kind() == reflect.Struct && dstValType.Kind() == reflect.Struct {
		subCache, ok := globalStructFieldCache.Load(structFieldTypePair{srcValType, dstValType})
		if !ok {
			subCache = buildStructFieldCache(srcValType, dstValType, true)
			globalStructFieldCache.Store(structFieldTypePair{srcValType, dstValType}, subCache)
		}
		sc := subCache.(*structFieldCache)
		fastFns := make([]fieldCopyFunc, len(sc.fastEntries))
		for i := range sc.fastEntries {
			fastFns[i] = sc.fastEntries[i].copyFunc
		}
		needSubConvert := len(sc.slowEntries) > 0

		if len(fastFns) > 0 {
			return func(srcBase, dstBase unsafe.Pointer) {
				srcMapVal := reflect.NewAt(srcType, addPtr(srcBase, srcOffset)).Elem()
				if srcMapVal.IsNil() {
					*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = nil
					return
				}
				dstMap := reflect.MakeMapWithSize(dstType, srcMapVal.Len())
				iter := srcMapVal.MapRange()
				for iter.Next() {
					srcKey := iter.Key()
					srcValPtr := iter.Value().Addr().UnsafePointer()
					dstVal := reflect.New(dstValType)
					dstValPtr := dstVal.UnsafePointer()
					for i := range fastFns {
						fastFns[i](srcValPtr, dstValPtr)
					}
					if needSubConvert {
						srcElemVal := reflect.NewAt(srcValType, srcValPtr).Elem()
						dstElemVal := dstVal.Elem()
						for i := range sc.slowEntries {
							entry := &sc.slowEntries[i]
							s := srcElemVal.FieldByIndex(entry.srcIndex)
							d := dstElemVal.FieldByIndex(entry.dstIndex)
							if !s.IsValid() || !d.IsValid() {
								continue
							}
							convertStructFieldSlow(s, d, entry)
						}
					}
					dstMap.SetMapIndex(srcKey, dstVal.Elem())
				}
				dstMapVal := reflect.NewAt(dstType, addPtr(dstBase, dstOffset)).Elem()
				dstMapVal.Set(dstMap)
			}
		}
	}

	return nil
}

// convertIntegerValue 通过 unsafe 指针完成整数类型转换
// 由 makeSliceCopyFunc 调用，在逐元素循环中替代 reflect.Convert
// 支持所有整数类型组合：int/int8/int16/int32/int64/uint/uint8/uint16/uint32/uint64
func convertIntegerValue(srcAddr, dstAddr unsafe.Pointer, srcKind, dstKind reflect.Kind) {
	switch srcKind {
	case reflect.Int32:
		v := int32(*(*int32)(srcAddr))
		switch dstKind {
		case reflect.Int:
			*(*int)(dstAddr) = int(v)
		case reflect.Int64:
			*(*int64)(dstAddr) = int64(v)
		case reflect.Int16:
			*(*int16)(dstAddr) = int16(v)
		case reflect.Int8:
			*(*int8)(dstAddr) = int8(v)
		case reflect.Uint:
			*(*uint)(dstAddr) = uint(v)
		case reflect.Uint32:
			*(*uint32)(dstAddr) = uint32(v)
		case reflect.Uint64:
			*(*uint64)(dstAddr) = uint64(v)
		case reflect.Uint16:
			*(*uint16)(dstAddr) = uint16(v)
		case reflect.Uint8:
			*(*uint8)(dstAddr) = uint8(v)
		}
	case reflect.Int64:
		v := int64(*(*int64)(srcAddr))
		switch dstKind {
		case reflect.Int:
			*(*int)(dstAddr) = int(v)
		case reflect.Int32:
			*(*int32)(dstAddr) = int32(v)
		case reflect.Int16:
			*(*int16)(dstAddr) = int16(v)
		case reflect.Int8:
			*(*int8)(dstAddr) = int8(v)
		case reflect.Uint:
			*(*uint)(dstAddr) = uint(v)
		case reflect.Uint32:
			*(*uint32)(dstAddr) = uint32(v)
		case reflect.Uint64:
			*(*uint64)(dstAddr) = uint64(v)
		case reflect.Uint16:
			*(*uint16)(dstAddr) = uint16(v)
		case reflect.Uint8:
			*(*uint8)(dstAddr) = uint8(v)
		}
	case reflect.Int:
		v := int(*(*int)(srcAddr))
		switch dstKind {
		case reflect.Int32:
			*(*int32)(dstAddr) = int32(v)
		case reflect.Int64:
			*(*int64)(dstAddr) = int64(v)
		case reflect.Int16:
			*(*int16)(dstAddr) = int16(v)
		case reflect.Int8:
			*(*int8)(dstAddr) = int8(v)
		case reflect.Uint:
			*(*uint)(dstAddr) = uint(v)
		case reflect.Uint32:
			*(*uint32)(dstAddr) = uint32(v)
		case reflect.Uint64:
			*(*uint64)(dstAddr) = uint64(v)
		case reflect.Uint16:
			*(*uint16)(dstAddr) = uint16(v)
		case reflect.Uint8:
			*(*uint8)(dstAddr) = uint8(v)
		}
	case reflect.Uint32:
		v := uint32(*(*uint32)(srcAddr))
		switch dstKind {
		case reflect.Uint:
			*(*uint)(dstAddr) = uint(v)
		case reflect.Uint64:
			*(*uint64)(dstAddr) = uint64(v)
		case reflect.Uint16:
			*(*uint16)(dstAddr) = uint16(v)
		case reflect.Uint8:
			*(*uint8)(dstAddr) = uint8(v)
		case reflect.Int:
			*(*int)(dstAddr) = int(v)
		case reflect.Int32:
			*(*int32)(dstAddr) = int32(v)
		case reflect.Int64:
			*(*int64)(dstAddr) = int64(v)
		}
	case reflect.Uint64:
		v := uint64(*(*uint64)(srcAddr))
		switch dstKind {
		case reflect.Uint:
			*(*uint)(dstAddr) = uint(v)
		case reflect.Uint32:
			*(*uint32)(dstAddr) = uint32(v)
		case reflect.Int:
			*(*int)(dstAddr) = int(v)
		case reflect.Int32:
			*(*int32)(dstAddr) = int32(v)
		case reflect.Int64:
			*(*int64)(dstAddr) = int64(v)
		}
	case reflect.Uint:
		v := uint(*(*uint)(srcAddr))
		switch dstKind {
		case reflect.Uint32:
			*(*uint32)(dstAddr) = uint32(v)
		case reflect.Uint64:
			*(*uint64)(dstAddr) = uint64(v)
		case reflect.Int:
			*(*int)(dstAddr) = int(v)
		case reflect.Int32:
			*(*int32)(dstAddr) = int32(v)
		case reflect.Int64:
			*(*int64)(dstAddr) = int64(v)
		}
	case reflect.Int16:
		v := int16(*(*int16)(srcAddr))
		switch dstKind {
		case reflect.Int:
			*(*int)(dstAddr) = int(v)
		case reflect.Int32:
			*(*int32)(dstAddr) = int32(v)
		case reflect.Int64:
			*(*int64)(dstAddr) = int64(v)
		}
	case reflect.Int8:
		v := int8(*(*int8)(srcAddr))
		switch dstKind {
		case reflect.Int:
			*(*int)(dstAddr) = int(v)
		case reflect.Int32:
			*(*int32)(dstAddr) = int32(v)
		}
	case reflect.Uint16:
		v := uint16(*(*uint16)(srcAddr))
		switch dstKind {
		case reflect.Uint32:
			*(*uint32)(dstAddr) = uint32(v)
		case reflect.Int32:
			*(*int32)(dstAddr) = int32(v)
		}
	case reflect.Uint8:
		v := uint8(*(*uint8)(srcAddr))
		switch dstKind {
		case reflect.Uint32:
			*(*uint32)(dstAddr) = uint32(v)
		case reflect.Int32:
			*(*int32)(dstAddr) = int32(v)
		}
	}
}

// PtrSize 是 unsafe.Sizeof(unsafe.Pointer(nil)) 的常量，用于指针算术
const PtrSize = uintptr(unsafe.Sizeof(unsafe.Pointer(nil)))

// convertStructFieldSlow 处理嵌套结构体中的慢速字段（reflect 回退路径）
// 当嵌套结构体的子缓存中存在 slowEntries 时，使用此函数逐字段处理
// 支持：同类型、整数、时间戳/时间、Wrapper、切片、Map、指针、结构体、DataWrapper 等所有种类
func convertStructFieldSlow(srcField, dstField reflect.Value, entry *structFieldEntry) {
	kind := entry.kind
	switch kind {
	case fieldSameType, fieldAssignable:
		if srcField.Type().AssignableTo(dstField.Type()) {
			dstField.Set(srcField)
		} else {
			dstField.Set(srcField.Convert(dstField.Type()))
		}
	case fieldInteger, fieldConvertible:
		dstField.Set(srcField.Convert(dstField.Type()))
	case fieldTSToTime:
		if srcField.IsNil() {
			dstField.Set(reflect.Zero(dstField.Type()))
			return
		}
		if ts, ok := srcField.Interface().(*timestamppb.Timestamp); ok {
			dstField.Set(reflect.ValueOf(ts.AsTime()))
		}
	case fieldTimeToTS:
		if t, ok := srcField.Interface().(time.Time); ok {
			if t.IsZero() {
				dstField.Set(reflect.Zero(dstField.Type()))
			} else {
				dstField.Set(reflect.ValueOf(timestamppb.New(t)))
			}
		}
	case fieldTSToTimePtr:
		if srcField.IsNil() {
			dstField.Set(reflect.Zero(dstField.Type()))
			return
		}
		if ts, ok := srcField.Interface().(*timestamppb.Timestamp); ok {
			t := ts.AsTime()
			dstField.Set(reflect.ValueOf(&t))
		}
	case fieldTimePtrToTS:
		if tPtr, ok := srcField.Interface().(*time.Time); ok && tPtr != nil {
			if tPtr.IsZero() {
				dstField.Set(reflect.Zero(dstField.Type()))
			} else {
				dstField.Set(reflect.ValueOf(timestamppb.New(*tPtr)))
			}
		} else {
			dstField.Set(reflect.Zero(dstField.Type()))
		}
	case fieldWrapper:
		tryConvertWrapper(srcField, dstField)
	case fieldSlice:
		convertSlice(srcField, dstField)
	case fieldMap:
		convertMap(srcField, dstField)
	case fieldMapStruct:
		convertMapStruct(srcField, dstField)
	case fieldToPtr:
		convertToPtr(srcField, dstField)
	case fieldFromPtr:
		convertFromPtr(srcField, dstField)
	case fieldStruct:
		convertStruct(srcField, dstField)
	case fieldStructPtr:
		convertStructPtr(srcField, dstField)
	case fieldDataWrapper:
		convertDataWrapper(srcField, dstField)
	case fieldStringFallback:
		dstField.SetString(srcField.String())
	case fieldNoop:
	}
}

// makeToPtrCopyFunc 生成 值→指针 的转换闭包（T → *T）
// 设计思想：protobuf 中常用 *int32 等表示可选字段，而 Go Model 中常用 int
// 转换时需要：1) 检查零值 → 设置 nil；2) 非零值 → reflect.New 分配 + 值拷贝
// 对于常见类型（bool, int, int32, int64, string, float 等）生成类型特化闭包
// 对于 int↔int32 等跨尺寸转换也支持零值检查
// 零值语义：protobuf 中零值映射为 nil 指针，非零值映射为 &非零值
func makeToPtrCopyFunc(srcType, dstType reflect.Type, srcOffset, dstOffset uintptr) fieldCopyFunc {
	if dstType.Kind() != reflect.Ptr {
		return nil
	}
	elemType := dstType.Elem()

	if srcType == elemType {
		elemSize := elemType.Size()
		switch elemType.Kind() {
		case reflect.Bool:
			return func(srcBase, dstBase unsafe.Pointer) {
				src := *(*bool)(addPtr(srcBase, srcOffset))
				if !src {
					*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = nil
					return
				}
				p := reflect.New(elemType)
				p.Elem().SetBool(true)
				*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = p.UnsafePointer()
			}
		case reflect.Int:
			return func(srcBase, dstBase unsafe.Pointer) {
				src := *(*int)(addPtr(srcBase, srcOffset))
				if src == 0 {
					*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = nil
					return
				}
				p := reflect.New(elemType)
				copy((*[1 << 30]byte)(p.UnsafePointer())[:elemSize],
					(*[1 << 30]byte)(addPtr(srcBase, srcOffset))[:elemSize])
				*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = p.UnsafePointer()
			}
		case reflect.Int32:
			return func(srcBase, dstBase unsafe.Pointer) {
				src := *(*int32)(addPtr(srcBase, srcOffset))
				if src == 0 {
					*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = nil
					return
				}
				p := reflect.New(elemType)
				copy((*[1 << 30]byte)(p.UnsafePointer())[:elemSize],
					(*[1 << 30]byte)(addPtr(srcBase, srcOffset))[:elemSize])
				*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = p.UnsafePointer()
			}
		case reflect.Int64:
			return func(srcBase, dstBase unsafe.Pointer) {
				src := *(*int64)(addPtr(srcBase, srcOffset))
				if src == 0 {
					*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = nil
					return
				}
				p := reflect.New(elemType)
				copy((*[1 << 30]byte)(p.UnsafePointer())[:elemSize],
					(*[1 << 30]byte)(addPtr(srcBase, srcOffset))[:elemSize])
				*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = p.UnsafePointer()
			}
		case reflect.Uint32:
			return func(srcBase, dstBase unsafe.Pointer) {
				src := *(*uint32)(addPtr(srcBase, srcOffset))
				if src == 0 {
					*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = nil
					return
				}
				p := reflect.New(elemType)
				copy((*[1 << 30]byte)(p.UnsafePointer())[:elemSize],
					(*[1 << 30]byte)(addPtr(srcBase, srcOffset))[:elemSize])
				*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = p.UnsafePointer()
			}
		case reflect.Uint64:
			return func(srcBase, dstBase unsafe.Pointer) {
				src := *(*uint64)(addPtr(srcBase, srcOffset))
				if src == 0 {
					*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = nil
					return
				}
				p := reflect.New(elemType)
				copy((*[1 << 30]byte)(p.UnsafePointer())[:elemSize],
					(*[1 << 30]byte)(addPtr(srcBase, srcOffset))[:elemSize])
				*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = p.UnsafePointer()
			}
		case reflect.Float64:
			return func(srcBase, dstBase unsafe.Pointer) {
				src := *(*float64)(addPtr(srcBase, srcOffset))
				if src == 0 {
					*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = nil
					return
				}
				p := reflect.New(elemType)
				copy((*[1 << 30]byte)(p.UnsafePointer())[:8],
					(*[1 << 30]byte)(addPtr(srcBase, srcOffset))[:8])
				*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = p.UnsafePointer()
			}
		case reflect.Float32:
			return func(srcBase, dstBase unsafe.Pointer) {
				src := *(*float32)(addPtr(srcBase, srcOffset))
				if src == 0 {
					*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = nil
					return
				}
				p := reflect.New(elemType)
				copy((*[1 << 30]byte)(p.UnsafePointer())[:4],
					(*[1 << 30]byte)(addPtr(srcBase, srcOffset))[:4])
				*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = p.UnsafePointer()
			}
		case reflect.String:
			return func(srcBase, dstBase unsafe.Pointer) {
				src := *(*string)(addPtr(srcBase, srcOffset))
				if src == "" {
					*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = nil
					return
				}
				p := reflect.New(elemType)
				p.Elem().SetString(src)
				*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = p.UnsafePointer()
			}
		default:
			return func(srcBase, dstBase unsafe.Pointer) {
				p := reflect.New(elemType)
				copy((*[1 << 30]byte)(p.UnsafePointer())[:elemSize],
					(*[1 << 30]byte)(addPtr(srcBase, srcOffset))[:elemSize])
				*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = p.UnsafePointer()
			}
		}
	}

	if srcType.Kind() == reflect.Int && elemType.Kind() == reflect.Int32 {
		return func(srcBase, dstBase unsafe.Pointer) {
			src := *(*int)(addPtr(srcBase, srcOffset))
			if src == 0 {
				*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = nil
				return
			}
			p := reflect.New(elemType)
			p.Elem().SetInt(int64(src))
			*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = p.UnsafePointer()
		}
	}
	if srcType.Kind() == reflect.Int32 && elemType.Kind() == reflect.Int {
		return func(srcBase, dstBase unsafe.Pointer) {
			src := *(*int32)(addPtr(srcBase, srcOffset))
			if src == 0 {
				*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = nil
				return
			}
			p := reflect.New(elemType)
			p.Elem().SetInt(int64(src))
			*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = p.UnsafePointer()
		}
	}
	if srcType.Kind() == reflect.Int32 && elemType.Kind() == reflect.Int64 {
		return func(srcBase, dstBase unsafe.Pointer) {
			src := *(*int32)(addPtr(srcBase, srcOffset))
			if src == 0 {
				*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = nil
				return
			}
			p := reflect.New(elemType)
			p.Elem().SetInt(int64(src))
			*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = p.UnsafePointer()
		}
	}
	if srcType.Kind() == reflect.Int64 && elemType.Kind() == reflect.Int32 {
		return func(srcBase, dstBase unsafe.Pointer) {
			src := *(*int64)(addPtr(srcBase, srcOffset))
			if src == 0 {
				*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = nil
				return
			}
			p := reflect.New(elemType)
			p.Elem().SetInt(src)
			*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = p.UnsafePointer()
		}
	}

	// Case 2: Different struct types (StructA → *StructB)，使用 structFieldCache
	if srcType.Kind() == reflect.Struct && elemType.Kind() == reflect.Struct {
		subCache, ok := globalStructFieldCache.Load(structFieldTypePair{srcType, elemType})
		if !ok {
			subCache = buildStructFieldCache(srcType, elemType, true)
			globalStructFieldCache.Store(structFieldTypePair{srcType, elemType}, subCache)
		}
		sc := subCache.(*structFieldCache)
		fastFns := make([]fieldCopyFunc, len(sc.fastEntries))
		for i := range sc.fastEntries {
			fastFns[i] = sc.fastEntries[i].copyFunc
		}
		needSubConvert := len(sc.slowEntries) > 0

		if !needSubConvert && len(fastFns) > 0 {
			return func(srcBase, dstBase unsafe.Pointer) {
				srcAddr := addPtr(srcBase, srcOffset)
				dstPtrVal := reflect.New(elemType)
				dstPtr := dstPtrVal.UnsafePointer()
				for i := range fastFns {
					fastFns[i](srcAddr, dstPtr)
				}
				*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = dstPtr
			}
		}

		if len(fastFns) > 0 {
			return func(srcBase, dstBase unsafe.Pointer) {
				srcAddr := addPtr(srcBase, srcOffset)
				dstPtrVal := reflect.New(elemType)
				dstPtr := dstPtrVal.UnsafePointer()
				for i := range fastFns {
					fastFns[i](srcAddr, dstPtr)
				}
				applyStructSlowEntriesUnsafe(srcAddr, dstPtr, sc)
				*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = dstPtr
			}
		}
	}

	return nil
}

// makeFromPtrCopyFunc 生成 指针→值 的 unsafe 转换闭包（*T → T）
// 设计思想：解引用源指针，若 nil 则设置目标为零值，否则用 copy 拷贝值
// 仅当 srcType.Elem() == dstType（解引用后类型相同）时生成，否则返回 nil 走 reflect
func makeFromPtrCopyFunc(srcType, dstType reflect.Type, srcOffset, dstOffset uintptr) fieldCopyFunc {
	elemType := srcType.Elem()

	// Case 1: Same type after dereference (*T → T)，直接内存拷贝
	if dstType == elemType {
		elemSize := dstType.Size()
		return func(srcBase, dstBase unsafe.Pointer) {
			srcPtr := *(*unsafe.Pointer)(addPtr(srcBase, srcOffset))
			if srcPtr == nil {
				return
			}
			copy((*[1 << 30]byte)(addPtr(dstBase, dstOffset))[:elemSize],
				(*[1 << 30]byte)(srcPtr)[:elemSize])
		}
	}

	// Case 2: Different struct types (*StructA → StructB)，使用 structFieldCache
	if elemType.Kind() == reflect.Struct && dstType.Kind() == reflect.Struct {
		subCache, ok := globalStructFieldCache.Load(structFieldTypePair{elemType, dstType})
		if !ok {
			subCache = buildStructFieldCache(elemType, dstType, true)
			globalStructFieldCache.Store(structFieldTypePair{elemType, dstType}, subCache)
		}
		sc := subCache.(*structFieldCache)
		fastFns := make([]fieldCopyFunc, len(sc.fastEntries))
		for i := range sc.fastEntries {
			fastFns[i] = sc.fastEntries[i].copyFunc
		}
		needSubConvert := len(sc.slowEntries) > 0

		if !needSubConvert && len(fastFns) > 0 {
			return func(srcBase, dstBase unsafe.Pointer) {
				srcPtr := *(*unsafe.Pointer)(addPtr(srcBase, srcOffset))
				if srcPtr == nil {
					return
				}
				dstAddr := addPtr(dstBase, dstOffset)
				for i := range fastFns {
					fastFns[i](srcPtr, dstAddr)
				}
			}
		}

		if len(fastFns) > 0 {
			return func(srcBase, dstBase unsafe.Pointer) {
				srcPtr := *(*unsafe.Pointer)(addPtr(srcBase, srcOffset))
				if srcPtr == nil {
					return
				}
				dstAddr := addPtr(dstBase, dstOffset)
				for i := range fastFns {
					fastFns[i](srcPtr, dstAddr)
				}
				applyStructSlowEntriesUnsafe(srcPtr, dstAddr, sc)
			}
		}
	}

	return nil
}

// makeStringFallbackCopyFunc 生成 字符串命名类型转换的 unsafe 闭包
// string 底层是 {Data unsafe.Pointer, Len int}，16 字节可一次性拷贝
// 用于源/目标是不同命名类型的 string（如 MyString → string）
func makeStringFallbackCopyFunc(srcOffset, dstOffset uintptr) fieldCopyFunc {
	return func(srcBase, dstBase unsafe.Pointer) {
		srcStr := *(*string)(addPtr(srcBase, srcOffset))
		*(*string)(addPtr(dstBase, dstOffset)) = srcStr
	}
}

// makeDataWrapperCopyFunc 生成 DataWrapper[T] ↔ T / *T 的 unsafe 转换闭包
// DataWrapper[T] 是 sqlbuilder/types 等库中常见的模式：struct{ Data T } 或 struct{ Data *T }
// 支持四种方向：
//   - srcIsDW, !dstIsDW: DataWrapper[T] → T 或 DataWrapper[T] → *T（解包 Data 字段）
//   - dstIsDW, !srcIsDW: T → DataWrapper[T] 或 *T → DataWrapper[T]（包装到 Data 字段）
//
// 仅支持简单的指针拷贝场景（Data 字段是指针/值），复杂情况返回 nil 走 reflect 慢路径
func makeDataWrapperCopyFunc(srcType, dstType reflect.Type, srcOffset, dstOffset uintptr) fieldCopyFunc {
	_, srcInner, srcIsDW := extractDataWrapper(srcType)
	_, _, dstIsDW := extractDataWrapper(dstType)

	if srcIsDW && !dstIsDW {
		// DataWrapper[*T] → *T: Data 字段是指针，拷贝指针即可
		srcDataOffset := srcOffset + srcType.Field(0).Offset
		if srcInner.Kind() == reflect.Ptr && dstType.Kind() == reflect.Ptr && srcInner == dstType {
			return func(srcBase, dstBase unsafe.Pointer) {
				srcPtr := *(*unsafe.Pointer)(addPtr(srcBase, srcDataOffset))
				*(*unsafe.Pointer)(addPtr(dstBase, dstOffset)) = srcPtr
			}
		}
	}

	if dstIsDW && !srcIsDW {
		// *T → DataWrapper[*T]: 拷贝指针到 Data 字段
		dstDataOffset := dstOffset + dstType.Field(0).Offset
		if srcType.Kind() == reflect.Ptr {
			dstInner := dstType.Field(0).Type
			if dstInner.Kind() == reflect.Ptr && srcType == dstInner {
				return func(srcBase, dstBase unsafe.Pointer) {
					srcPtr := *(*unsafe.Pointer)(addPtr(srcBase, srcOffset))
					*(*unsafe.Pointer)(addPtr(dstBase, dstDataOffset)) = srcPtr
				}
			}
		}
	}

	return nil
}

// classifyField 根据源/目标字段的类型对，判断转换类型分类
// 这是构建 fast/slow 路径分层的关键决策函数，在 buildFieldCache 时对每个字段调用一次
// 返回值：(fieldKind, *wrapperConverter, wrapperDir)
// 决策优先级：
//  1. srcType == dstType → fieldSameType（最快，直接内存拷贝）
//  2. 时间戳/时间组合 → fieldTSToTime/fieldTimeToTS/fieldTimePtrToTS/fieldTSToTimePtr
//  3. Wrapper 类型匹配 → fieldWrapper
//  4. AssignableTo → fieldAssignable
//  5. ConvertibleTo + 整数 → fieldInteger
//  6. ConvertibleTo + 其他 → fieldConvertible
//  7. 两者都是 slice → fieldSlice
//  8. 两者都是 map → fieldMap
//  9. DataWrapper 匹配 → fieldDataWrapper
//  10. 指针→值 → fieldFromPtr
//  11. 值→指针 → fieldToPtr
//  12. 结构体 → fieldStruct/fieldStructPtr
//  13. 字符串 → fieldStringFallback
//  14. 默认 → fieldNoop
func classifyField(srcType, dstType reflect.Type, autoTime bool) (fieldKind, *wrapperConverter, int) {
	if srcType == dstType {
		return fieldSameType, nil, 0
	}

	srcKind := srcType.Kind()
	dstKind := dstType.Kind()

	if autoTime {
		if srcType == timestampPtrType && dstType == timeType {
			return fieldTSToTime, nil, 0
		}
		if srcType == timeType && dstType == timestampPtrType {
			return fieldTimeToTS, nil, 0
		}
		if srcType == timestampPtrType && dstType == timePtrType {
			return fieldTSToTimePtr, nil, 0
		}
		if srcType == timePtrType && dstType == timestampPtrType {
			return fieldTimePtrToTS, nil, 0
		}
	}

	if wc, dir := lookupWrapper(srcType, dstType); wc != nil {
		return fieldWrapper, wc, dir
	}

	if srcType.AssignableTo(dstType) {
		return fieldAssignable, nil, 0
	}

	if srcType.ConvertibleTo(dstType) {
		if IsIntegerType(srcType) && IsIntegerType(dstType) {
			return fieldInteger, nil, 0
		}
		// 可转换的结构体指针/值类型优先走 fieldStructPtr/fieldStruct
		// 这样可以利用 structFieldCache 进行 unsafe 快速转换，而非 reflect.Convert
		srcIsPtr, dstIsPtr := srcKind == reflect.Ptr, dstKind == reflect.Ptr
		if srcIsPtr && dstIsPtr {
			srcElem := srcType.Elem()
			dstElem := dstType.Elem()
			if srcElem.Kind() == reflect.Struct && dstElem.Kind() == reflect.Struct {
				return fieldStructPtr, nil, 0
			}
		} else if !srcIsPtr && !dstIsPtr && srcKind == reflect.Struct && dstKind == reflect.Struct {
			return fieldStruct, nil, 0
		}
		return fieldConvertible, nil, 0
	}

	// map[string]any ↔ *structpb.Struct 特殊转换（先于 Slice/Map 通用分类，避免落入 fieldToPtr/fieldFromPtr）
	if isMapStructPair(srcType, dstType) {
		return fieldMapStruct, nil, 0
	}

	if srcKind == reflect.Slice && dstKind == reflect.Slice {
		return fieldSlice, nil, 0
	}

	if srcKind == reflect.Map && dstKind == reflect.Map {
		return fieldMap, nil, 0
	}

	// DataWrapper[T] 检测：struct{Data T} ↔ T 或 struct{Data T} ↔ *T
	if isDataWrapperMatch(srcType, dstType) {
		return fieldDataWrapper, nil, 0
	}

	srcIsPtr, dstIsPtr := srcKind == reflect.Ptr, dstKind == reflect.Ptr
	if srcIsPtr && !dstIsPtr {
		return fieldFromPtr, nil, 0
	}
	if !srcIsPtr && dstIsPtr {
		return fieldToPtr, nil, 0
	}

	srcElem := DereferenceType(srcType)
	dstElem := DereferenceType(dstType)
	if srcElem.Kind() == reflect.Struct && dstElem.Kind() == reflect.Struct {
		if srcIsPtr && dstIsPtr {
			return fieldStructPtr, nil, 0
		}
		return fieldStruct, nil, 0
	}

	if srcKind == reflect.String && dstKind == reflect.String {
		return fieldStringFallback, nil, 0
	}

	return fieldNoop, nil, 0
}

// isDataWrapperMatch 检查 src↔dst 是否构成 DataWrapper[T] ↔ T 或 DataWrapper[T] ↔ *T 的关系
// DataWrapper[T] 是只有一个导出字段 Data T 的 struct
func isDataWrapperMatch(srcType, dstType reflect.Type) bool {
	if dwType, innerType, ok := extractDataWrapper(srcType); ok {
		if innerType == dstType || reflect.PointerTo(innerType) == dstType {
			return true
		}
		dstElem := dstType
		if dstType.Kind() == reflect.Ptr {
			dstElem = dstType.Elem()
		}
		if innerType == dstElem {
			return true
		}
		_ = dwType
	}
	if dwType, innerType, ok := extractDataWrapper(dstType); ok {
		if innerType == srcType || reflect.PointerTo(innerType) == srcType {
			return true
		}
		srcElem := srcType
		if srcType.Kind() == reflect.Ptr {
			srcElem = srcType.Elem()
		}
		if innerType == srcElem {
			return true
		}
		_ = dwType
	}
	return false
}

// extractDataWrapper 从类型中提取 DataWrapper[T] 的内嵌类型
// 返回 (DataWrapper 类型, Data 字段类型, 是否匹配)
func extractDataWrapper(t reflect.Type) (reflect.Type, reflect.Type, bool) {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, nil, false
	}
	if t.NumField() == 0 {
		return nil, nil, false
	}
	dataField, hasDataField := t.FieldByName("Data")
	if !hasDataField || !dataField.IsExported() {
		return nil, nil, false
	}
	return t, dataField.Type, true
}

// lookupWrapper 查找 wrapper 类型匹配
// 返回 (wrapperConverter, direction)
func lookupWrapper(srcType, dstType reflect.Type) (*wrapperConverter, int) {
	if wc, ok := wrapperByPtrType[srcType]; ok && dstType == wc.wrapperType {
		return wc, 0
	}
	if wc, ok := wrapperByWrapperType[srcType]; ok && dstType == wc.ptrType {
		return wc, 1
	}
	if wc, ok := wrapperByWrapperType[srcType]; ok && dstType == wc.valueType {
		return wc, 3
	}
	if wc, ok := wrapperByValueType[srcType]; ok && dstType == wc.wrapperType {
		return wc, 2
	}
	return nil, 0
}

// buildFieldCache 构建 PB↔Model 字段缓存，核心的预计算引擎
// 这是整个转换器性能的关键：将运行时类型判断和字段映射预先计算好，
// 后续每次转换只需按偏移量做 unsafe 指针操作
//
// 处理流程：
//  1. 构建目标字段名映射（支持 pbmo tag、json tag、FieldMapping）
//  2. 确定方向（Model→PB 用正向映射，PB→Model 用反向映射）
//  3. 遍历源类型字段，匹配目标字段，调用 classifyField 分类
//  4. 为每个字段生成 copyFunc（如果可以生成 unsafe 闭包）
//  5. 有 copyFunc 且无 Transformer 的字段 → fastEntries
//  6. 其余字段 → slowEntries
//  7. 构建 mergedCopyFunc 合并所有 fastEntries 的 copyFunc 为单次调用闭包
//  8. 预计算 hasTransformers 标志，避免每次调用 Count()
func buildFieldCache(srcType, dstType reflect.Type, srcIsModel bool, opts *Options, transformers *TransformerRegistry) *fieldCache {
	dstFields := make(map[string]reflect.StructField)
	for i := 0; i < dstType.NumField(); i++ {
		f := dstType.Field(i)
		if f.IsExported() {
			dstFields[f.Name] = f
		}
	}

	// srcToDst: src字段名 → dst字段名的映射
	// 由 tag map (model→pb) 和 FieldMapping (model→pb) 两个来源合并而成
	// 对于 PB→Model: src=PB, dst=Model, 需要 PB字段名→Model字段名
	// 对于 Model→PB: src=Model, dst=PB, 需要 Model字段名→PB字段名
	srcToDst := make(map[string]string)

	// 方向由调用方显式传入（BidiConverter 已知 Model/PB 类型），
	// 不再依赖 hasPbmoTag 推断，避免 Model 无 pbmo tag 时 FieldMapping 失效
	dstIsModel := !srcIsModel

	// 构建 modelToPbMap: Model字段名 → PB字段名
	modelToPbMap := make(map[string]string)
	if opts.TagMappingEnabled {
		// 确定哪个类型是 Model
		var modelType reflect.Type
		if srcIsModel && !dstIsModel {
			modelType = srcType
		} else if dstIsModel && !srcIsModel {
			modelType = dstType
		}
		// 跳过 PB 类型：PB 的 json tag 是 protoc 生成的 snake_case 名字，
		// 不是 Go 字段名，读它会污染 modelToPbMap，导致字段映射依赖 fallback
		// PB↔PB 转换时应直接用 Go 字段名匹配，或通过 FieldMapping 显式映射
		if modelType != nil && !isProtoMessage(modelType) {
			for i := 0; i < modelType.NumField(); i++ {
				field := modelType.Field(i)
				if !field.IsExported() {
					continue
				}
				if tag := field.Tag.Get("pbmo"); tag != "" && tag != "-" {
					modelToPbMap[field.Name] = tag
				} else if jtag := field.Tag.Get("json"); jtag != "" && jtag != "-" {
					comma := strings.Index(jtag, ",")
					if comma > 0 {
						modelToPbMap[field.Name] = jtag[:comma]
					} else {
						modelToPbMap[field.Name] = jtag
					}
				}
			}
		}
	}
	for modelField, pbField := range opts.FieldMapping {
		modelToPbMap[modelField] = pbField
	}

	if srcIsModel && !dstIsModel {
		// Model→PB 方向: src=Model, dst=PB, 直接用 modelToPbMap
		for modelField, pbField := range modelToPbMap {
			srcToDst[modelField] = pbField
		}
	} else if !srcIsModel && dstIsModel {
		// PB→Model 方向: src=PB, dst=Model, 需要反向映射
		for modelField, pbField := range modelToPbMap {
			srcToDst[pbField] = modelField
		}
	}

	autoTime := opts.AutoTimeConversion

	var fastEntries []fieldMappingEntry
	var slowEntries []fieldMappingEntry

	for i := 0; i < srcType.NumField(); i++ {
		srcField := srcType.Field(i)
		if !srcField.IsExported() {
			continue
		}

		// 查找目标字段名：先映射，后同名
		dstFieldName := srcToDst[srcField.Name]
		if dstFieldName == "" {
			dstFieldName = srcField.Name
		}

		dstField, ok := dstFields[dstFieldName]
		if !ok {
			for name, field := range dstFields {
				if strings.EqualFold(name, srcField.Name) {
					dstField = field
					dstFieldName = name
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}

		kind, wc, dir := classifyField(srcField.Type, dstField.Type, autoTime)

		hasTransform := transformers != nil && transformers.Has(srcField.Name)

		copyFn := makeFieldCopyFunc(kind, srcField.Type, dstField.Type,
			srcField.Offset, dstField.Offset, wc, dir)

		entry := fieldMappingEntry{
			srcIndex:     srcField.Index,
			dstIndex:     dstField.Index,
			srcType:      srcField.Type,
			dstType:      dstField.Type,
			kind:         kind,
			srcName:      srcField.Name,
			dstName:      dstField.Name,
			wrapperConv:  wc,
			wrapperDir:   dir,
			srcOffset:    srcField.Offset,
			dstOffset:    dstField.Offset,
			fieldSize:    srcField.Type.Size(),
			copyFunc:     copyFn,
			hasTransform: hasTransform,
		}

		if copyFn != nil && !hasTransform {
			fastEntries = append(fastEntries, entry)
		} else {
			slowEntries = append(slowEntries, entry)
		}
	}

	hasTransformersFlag := transformers != nil && transformers.Count() > 0

	mergedFn := buildMergedCopyFunc(fastEntries)

	return &fieldCache{
		srcType:         srcType,
		dstType:         dstType,
		fastEntries:     fastEntries,
		slowEntries:     slowEntries,
		autoTime:        autoTime,
		hasTransformers: hasTransformersFlag,
		mergedCopyFunc:  mergedFn,
	}
}

// buildMergedCopyFuncFromFns 将 copyFunc 列表合并为单个闭包
// 设计思想：N 个 fastEntries 意味着 N 次间接函数调用，合并为单个闭包后
// 只需一次调用，减少间接调用开销，并为编译器内联优化创造条件
// 通过 switch len 展开 1-8 个函数的直接调用，超过 8 个回退到循环
// buildMergedCopyFunc 和 buildStructMergedCopyFunc 共用此核心逻辑
// 性能影响：对于 Simple（4个 fast 字段）等小型结构体，约 5-10% 的额外提升
func buildMergedCopyFuncFromFns(fns []fieldCopyFunc) func(srcBase, dstBase unsafe.Pointer) {
	switch len(fns) {
	case 0:
		return nil
	case 1:
		f0 := fns[0]
		return func(srcBase, dstBase unsafe.Pointer) {
			f0(srcBase, dstBase)
		}
	case 2:
		f0, f1 := fns[0], fns[1]
		return func(srcBase, dstBase unsafe.Pointer) {
			f0(srcBase, dstBase)
			f1(srcBase, dstBase)
		}
	case 3:
		f0, f1, f2 := fns[0], fns[1], fns[2]
		return func(srcBase, dstBase unsafe.Pointer) {
			f0(srcBase, dstBase)
			f1(srcBase, dstBase)
			f2(srcBase, dstBase)
		}
	case 4:
		f0, f1, f2, f3 := fns[0], fns[1], fns[2], fns[3]
		return func(srcBase, dstBase unsafe.Pointer) {
			f0(srcBase, dstBase)
			f1(srcBase, dstBase)
			f2(srcBase, dstBase)
			f3(srcBase, dstBase)
		}
	case 5:
		f0, f1, f2, f3, f4 := fns[0], fns[1], fns[2], fns[3], fns[4]
		return func(srcBase, dstBase unsafe.Pointer) {
			f0(srcBase, dstBase)
			f1(srcBase, dstBase)
			f2(srcBase, dstBase)
			f3(srcBase, dstBase)
			f4(srcBase, dstBase)
		}
	case 6:
		f0, f1, f2, f3, f4, f5 := fns[0], fns[1], fns[2], fns[3], fns[4], fns[5]
		return func(srcBase, dstBase unsafe.Pointer) {
			f0(srcBase, dstBase)
			f1(srcBase, dstBase)
			f2(srcBase, dstBase)
			f3(srcBase, dstBase)
			f4(srcBase, dstBase)
			f5(srcBase, dstBase)
		}
	case 7:
		f0, f1, f2, f3, f4, f5, f6 := fns[0], fns[1], fns[2], fns[3], fns[4], fns[5], fns[6]
		return func(srcBase, dstBase unsafe.Pointer) {
			f0(srcBase, dstBase)
			f1(srcBase, dstBase)
			f2(srcBase, dstBase)
			f3(srcBase, dstBase)
			f4(srcBase, dstBase)
			f5(srcBase, dstBase)
			f6(srcBase, dstBase)
		}
	case 8:
		f0, f1, f2, f3, f4, f5, f6, f7 := fns[0], fns[1], fns[2], fns[3], fns[4], fns[5], fns[6], fns[7]
		return func(srcBase, dstBase unsafe.Pointer) {
			f0(srcBase, dstBase)
			f1(srcBase, dstBase)
			f2(srcBase, dstBase)
			f3(srcBase, dstBase)
			f4(srcBase, dstBase)
			f5(srcBase, dstBase)
			f6(srcBase, dstBase)
			f7(srcBase, dstBase)
		}
	}
	return func(srcBase, dstBase unsafe.Pointer) {
		for i := range fns {
			fns[i](srcBase, dstBase)
		}
	}
}

// buildMergedCopyFunc 将所有快速路径的 copyFunc 合并为单个闭包
// 提取 copyFunc 列表后委托给 buildMergedCopyFuncFromFns
func buildMergedCopyFunc(entries []fieldMappingEntry) func(srcBase, dstBase unsafe.Pointer) {
	fns := make([]fieldCopyFunc, len(entries))
	for i := range entries {
		fns[i] = entries[i].copyFunc
	}
	return buildMergedCopyFuncFromFns(fns)
}

// buildStructFieldCache 构建嵌套结构体的字段缓存
// 与 buildFieldCache 类似，但用于嵌套结构体（不包含 Transformer 和 tag 映射）
// 缓存通过 globalStructFieldCache 全局共享，避免每次递归转换时重建映射
func buildStructFieldCache(srcType, dstType reflect.Type, autoTime bool) *structFieldCache {
	var fastEntries []structFieldEntry
	var slowEntries []structFieldEntry

	dstFields := make(map[string]reflect.StructField)
	for i := 0; i < dstType.NumField(); i++ {
		f := dstType.Field(i)
		if f.IsExported() {
			dstFields[f.Name] = f
		}
	}

	for i := 0; i < srcType.NumField(); i++ {
		srcField := srcType.Field(i)
		if !srcField.IsExported() {
			continue
		}

		dstField, ok := dstFields[srcField.Name]
		if !ok {
			for name, field := range dstFields {
				if strings.EqualFold(name, srcField.Name) {
					dstField = field
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}

		kind, wc, dir := classifyField(srcField.Type, dstField.Type, autoTime)

		copyFn := makeFieldCopyFunc(kind, srcField.Type, dstField.Type,
			srcField.Offset, dstField.Offset, wc, dir)

		entry := structFieldEntry{
			srcIndex:    srcField.Index,
			dstIndex:    dstField.Index,
			srcType:     srcField.Type,
			dstType:     dstField.Type,
			kind:        kind,
			srcName:     srcField.Name,
			wrapperConv: wc,
			wrapperDir:  dir,
			srcOffset:   srcField.Offset,
			dstOffset:   dstField.Offset,
			fieldSize:   srcField.Type.Size(),
			copyFunc:    copyFn,
		}

		if copyFn != nil {
			fastEntries = append(fastEntries, entry)
		} else {
			slowEntries = append(slowEntries, entry)
		}
	}

	mergedFn := buildStructMergedCopyFunc(fastEntries)

	return &structFieldCache{
		srcType:     srcType,
		dstType:     dstType,
		fastEntries: fastEntries,
		slowEntries: slowEntries,
		autoTime:    autoTime,
		mergedFn:    mergedFn,
	}
}

// buildStructMergedCopyFunc 将嵌套结构体的快速路径 copyFunc 合并为单个闭包
// 与 buildMergedCopyFunc 类似，但用于 structFieldCache，委托给 buildMergedCopyFuncFromFns
func buildStructMergedCopyFunc(entries []structFieldEntry) func(srcBase, dstBase unsafe.Pointer) {
	fns := make([]fieldCopyFunc, len(entries))
	for i := range entries {
		fns[i] = entries[i].copyFunc
	}
	return buildMergedCopyFuncFromFns(fns)
}

// Converter 双向转换器接口
type Converter interface {
	ConvertPBToModel(pb, modelPtr interface{}) error
	ConvertModelToPB(model, pbPtr interface{}) error
	GetPBType() reflect.Type
	GetModelType() reflect.Type
}

// BidiConverter 双向转换器
type BidiConverter struct {
	pbType         reflect.Type
	modelType      reflect.Type
	options        *Options
	transformers   *TransformerRegistry
	pbToModelCache *fieldCache
	modelToPBCache *fieldCache
	pbToModelPtr   atomic.Pointer[fieldCache]
	modelToPBPtr   atomic.Pointer[fieldCache]
	structCache    map[structFieldTypePair]*structFieldCache
	mu             sync.RWMutex
}

// NewBidiConverter 创建双向转换器
func NewBidiConverter(pbType, modelType interface{}, opts ...Option) *BidiConverter {
	pbReflectType := reflect.TypeOf(pbType)
	modelReflectType := reflect.TypeOf(modelType)
	if pbReflectType.Kind() == reflect.Ptr {
		pbReflectType = pbReflectType.Elem()
	}
	if modelReflectType.Kind() == reflect.Ptr {
		modelReflectType = modelReflectType.Elem()
	}
	options := ApplyOptions(opts...)
	return &BidiConverter{
		pbType:       pbReflectType,
		modelType:    modelReflectType,
		options:      options,
		transformers: NewTransformerRegistry(),
		structCache:  make(map[structFieldTypePair]*structFieldCache),
	}
}

// WithAutoTimeConversion 设置自动时间转换
func (bc *BidiConverter) WithAutoTimeConversion(enabled bool) *BidiConverter {
	bc.options.AutoTimeConversion = enabled
	bc.invalidateCache()
	return bc
}

// WithFieldMapping 设置字段映射
func (bc *BidiConverter) WithFieldMapping(modelField, pbField string) *BidiConverter {
	bc.options.FieldMapping[modelField] = pbField
	bc.invalidateCache()
	return bc
}

// WithFieldMappings 批量设置字段映射
func (bc *BidiConverter) WithFieldMappings(mappings map[string]string) *BidiConverter {
	for k, v := range mappings {
		bc.options.FieldMapping[k] = v
	}
	bc.invalidateCache()
	return bc
}

// RegisterFieldMapping 注册字段映射
func (bc *BidiConverter) RegisterFieldMapping(mappings map[string]string) *BidiConverter {
	return bc.WithFieldMappings(mappings)
}

// WithValidation 设置是否启用校验
func (bc *BidiConverter) WithValidation(enabled bool) *BidiConverter {
	bc.options.ValidationEnabled = enabled
	return bc
}

// WithDesensitize 设置是否启用脱敏
func (bc *BidiConverter) WithDesensitize(enabled bool) *BidiConverter {
	bc.options.DesensitizeEnabled = enabled
	return bc
}

// WithSafeMode 设置是否启用安全模式
func (bc *BidiConverter) WithSafeMode(enabled bool) *BidiConverter {
	bc.options.SafeMode = enabled
	return bc
}

// WithTagName 设置 struct tag 名称
func (bc *BidiConverter) WithTagName(name string) *BidiConverter {
	bc.options.TagName = name
	bc.invalidateCache()
	return bc
}

// WithTagMapping 设置是否启用 struct tag 映射
func (bc *BidiConverter) WithTagMapping(enabled bool) *BidiConverter {
	bc.options.TagMappingEnabled = enabled
	bc.invalidateCache()
	return bc
}

// WithConcurrency 设置并发数
func (bc *BidiConverter) WithConcurrency(n int) *BidiConverter {
	bc.options.Concurrency = n
	return bc
}

// WithBatchSize 设置批处理大小
func (bc *BidiConverter) WithBatchSize(size int) *BidiConverter {
	bc.options.BatchSize = size
	return bc
}

// WithTimeout 设置超时时间
func (bc *BidiConverter) WithTimeout(timeout time.Duration) *BidiConverter {
	bc.options.Timeout = timeout
	return bc
}

// RegisterTransformer 注册字段转换器
func (bc *BidiConverter) RegisterTransformer(field string, fn TransformerFunc) *BidiConverter {
	bc.transformers.Register(field, fn)
	bc.invalidateCache()
	return bc
}

// Warmup 预热缓存，在注册完成后调用确保所有映射就绪
func (bc *BidiConverter) Warmup() *BidiConverter {
	bc.ensureFieldCache()
	return bc
}

// GetModelType 获取 Model 类型
func (bc *BidiConverter) GetModelType() reflect.Type {
	return bc.modelType
}

// GetPBType 获取 PB 类型
func (bc *BidiConverter) GetPBType() reflect.Type {
	return bc.pbType
}

// GetTransformers 获取转换器注册表
func (bc *BidiConverter) GetTransformers() *TransformerRegistry {
	return bc.transformers
}

// invalidateCache 清空缓存，下次调用时重建
func (bc *BidiConverter) invalidateCache() {
	bc.mu.Lock()
	bc.pbToModelCache = nil
	bc.modelToPBCache = nil
	bc.pbToModelPtr.Store(nil)
	bc.modelToPBPtr.Store(nil)
	bc.mu.Unlock()
}

// ensureFieldCache 确保字段缓存已构建
func (bc *BidiConverter) fieldCaches() (*fieldCache, *fieldCache) {
	pbToModel := bc.pbToModelPtr.Load()
	modelToPB := bc.modelToPBPtr.Load()
	if pbToModel != nil && modelToPB != nil {
		return pbToModel, modelToPB
	}

	bc.mu.Lock()
	defer bc.mu.Unlock()
	if bc.pbToModelCache == nil {
		bc.pbToModelCache = buildFieldCache(bc.pbType, bc.modelType, false, bc.options, bc.transformers)
	}
	if bc.modelToPBCache == nil {
		bc.modelToPBCache = buildFieldCache(bc.modelType, bc.pbType, true, bc.options, bc.transformers)
	}
	bc.pbToModelPtr.Store(bc.pbToModelCache)
	bc.modelToPBPtr.Store(bc.modelToPBCache)
	return bc.pbToModelCache, bc.modelToPBCache
}

func (bc *BidiConverter) pbToModelFieldCache() *fieldCache {
	cache, _ := bc.fieldCaches()
	return cache
}

func (bc *BidiConverter) modelToPBFieldCache() *fieldCache {
	_, cache := bc.fieldCaches()
	return cache
}

func (bc *BidiConverter) ensureFieldCache() {
	bc.fieldCaches()
}

// ConvertPBToModel 将 PB 消息转换为 Model（核心热路径）
// 入参校验后委托给 convertWithCache，三层加速策略详见 convertWithCache 注释
func (bc *BidiConverter) ConvertPBToModel(pb, modelPtr interface{}) error {
	if pb == nil || modelPtr == nil {
		return nil
	}

	pbVal := reflect.ValueOf(pb)
	modelVal := reflect.ValueOf(modelPtr)

	if modelVal.Kind() != reflect.Ptr || modelVal.IsNil() {
		return ErrMustBePointer
	}
	modelVal = modelVal.Elem()

	if pbVal.Kind() == reflect.Ptr {
		pbVal = pbVal.Elem()
	}

	return bc.convertWithCache(pbVal, modelVal, bc.pbToModelFieldCache(), pb)
}

// ConvertModelToPB 将 Model 转换为 PB 消息（核心热路径）
// 与 ConvertPBToModel 对称，方向为 Model → PB，委托给 convertWithCache
func (bc *BidiConverter) ConvertModelToPB(model, pbPtr interface{}) error {
	if model == nil || pbPtr == nil {
		return nil
	}

	modelVal := reflect.ValueOf(model)
	pbVal := reflect.ValueOf(pbPtr)

	if pbVal.Kind() != reflect.Ptr || pbVal.IsNil() {
		return ErrMustBePointer
	}
	pbVal = pbVal.Elem()

	if modelVal.Kind() == reflect.Ptr {
		if modelVal.IsNil() {
			return nil
		}
		modelVal = modelVal.Elem()
	}

	return bc.convertWithCache(modelVal, pbVal, bc.modelToPBFieldCache(), model)
}

// convertWithCache 执行 srcVal → dstVal 的转换，应用三层加速策略
// 这是 ConvertPBToModel 和 ConvertModelToPB 的共用核心逻辑
//
// 执行流程（三层加速策略）：
//
// 【第一层：mergedCopyFunc 快速路径】（最快，完全 unsafe）
//
//	条件：canFastPath && mergedCopyFunc != nil && !hasTransformers && slowEntries == 0
//	执行：单次 mergedCopyFunc(srcBase, dstBase) 调用，拷贝所有字段后直接返回
//	性能：接近原生手写代码（1.1-2x 开销）
//
// 【第二层：逐字段 copyFunc 快速路径】（次快，unsafe + 少量 reflect）
//
//	条件：canFastPath && fastEntries 存在，但 slowEntries 不为空或有 Transformer
//	执行：mergedCopyFunc 处理所有 fast 字段，然后逐个 slowEntry 用 reflect 处理
//	性能：比第一层慢约 50-100%，因为需要 FieldByIndex + convertFieldByKind
//
// 【第三层：reflect 回退路径】（最慢，但覆盖所有场景）
//
//	条件：值不可寻址（canFastPath=false）
//	执行：逐个 fastEntry 用 FieldByIndex + convertFieldByKind
//	      逐个 slowEntry 用 FieldByIndex + convertFieldByKind
//	性能：约 3-5x 原生（无 unsafe 优化）
//
// hasTransformers 标志在 buildFieldCache 时预计算，避免每次调用 transformers.Count()
//
// srcInterface 是 srcVal 对应的原始 interface{}，用于 ifaceDataPtr 路径
// （当 srcVal 不可寻址时，通过 interface{} 内部布局提取基地址）
func (bc *BidiConverter) convertWithCache(srcVal, dstVal reflect.Value, cache *fieldCache, srcInterface interface{}) error {
	// 如果 srcVal 不可寻址（值类型传入 interface{}），尝试通过 interface{} 内部布局获取基地址
	// 对于 struct 类型且大小 > ptrSize，interface{} 中的 data 指针指向堆上的副本
	// 对于小 struct（大小 <= ptrSize），data 可能直接存储值，不安全使用
	// 仅在类型与 cache 匹配时才走此路径，避免方向不匹配导致错误
	if !srcVal.CanAddr() && srcVal.Kind() == reflect.Struct && srcVal.Type() == cache.srcType {
		if srcVal.Type().Size() > uintptr(unsafe.Sizeof(uintptr(0))) {
			if srcBase := ifaceDataPtr(srcInterface); srcBase != nil {
				dstBase := unsafe.Pointer(dstVal.UnsafeAddr())
				if err := bc.applyFastPath(cache, srcBase, dstBase, srcVal, dstVal); err != nil {
					return err
				}
				return bc.applySlowEntries(cache, srcVal, dstVal)
			}
		}
	}

	canFastPath := srcVal.CanAddr() && dstVal.CanAddr()

	if canFastPath && cache.mergedCopyFunc != nil && !cache.hasTransformers {
		srcBase := unsafe.Pointer(srcVal.UnsafeAddr())
		dstBase := unsafe.Pointer(dstVal.UnsafeAddr())
		cache.mergedCopyFunc(srcBase, dstBase)

		if len(cache.slowEntries) == 0 {
			return nil
		}
	} else if canFastPath && len(cache.fastEntries) > 0 {
		srcBase := unsafe.Pointer(srcVal.UnsafeAddr())
		dstBase := unsafe.Pointer(dstVal.UnsafeAddr())
		if cache.mergedCopyFunc != nil {
			cache.mergedCopyFunc(srcBase, dstBase)
		} else {
			for i := range cache.fastEntries {
				cache.fastEntries[i].copyFunc(srcBase, dstBase)
			}
		}
	} else if len(cache.fastEntries) > 0 {
		for i := range cache.fastEntries {
			entry := &cache.fastEntries[i]
			srcField := srcVal.FieldByIndex(entry.srcIndex)
			dstField := dstVal.FieldByIndex(entry.dstIndex)
			if !srcField.IsValid() || !dstField.IsValid() || !dstField.CanSet() {
				continue
			}
			if err := convertFieldByKind(srcField, dstField, entry); err != nil {
				return NewConversionError("字段 %s 转换失败: %v", entry.srcName, err)
			}
		}
	}

	return bc.applySlowEntries(cache, srcVal, dstVal)
}

// convertFieldByKind 使用 reflect 回退路径转换字段（慢速路径）
// 这是所有无法生成 copyFunc 的字段类型的处理入口
// 每种 fieldKind 对应一种 reflect 操作，虽然比 unsafe 慢，但功能正确且全面
func convertFieldByKind(srcField, dstField reflect.Value, entry *fieldMappingEntry) error {
	if !srcField.CanInterface() || !dstField.CanSet() {
		return nil
	}

	kind := entry.kind

	switch kind {
	case fieldSameType, fieldAssignable:
		if srcField.Type().AssignableTo(dstField.Type()) {
			dstField.Set(srcField)
		} else {
			dstField.Set(srcField.Convert(dstField.Type()))
		}
	case fieldInteger, fieldConvertible:
		dstField.Set(srcField.Convert(dstField.Type()))
	case fieldTSToTime:
		if srcField.IsNil() {
			dstField.Set(reflect.Zero(dstField.Type()))
			return nil
		}
		ts, ok := srcField.Interface().(*timestamppb.Timestamp)
		if !ok {
			return NewConversionError("字段 %s: 期望 *timestamppb.Timestamp 类型", entry.srcName)
		}
		t := ts.AsTime()
		dstField.Set(reflect.ValueOf(t))
	case fieldTimeToTS:
		t, ok := srcField.Interface().(time.Time)
		if !ok {
			return NewConversionError("字段 %s: 期望 time.Time 类型", entry.srcName)
		}
		if t.IsZero() {
			dstField.Set(reflect.Zero(dstField.Type()))
			return nil
		}
		dstField.Set(reflect.ValueOf(timestamppb.New(t)))
	case fieldTSToTimePtr:
		if srcField.IsNil() {
			dstField.Set(reflect.Zero(dstField.Type()))
			return nil
		}
		ts, ok := srcField.Interface().(*timestamppb.Timestamp)
		if !ok {
			return NewConversionError("字段 %s: 期望 *timestamppb.Timestamp 类型", entry.srcName)
		}
		t := ts.AsTime()
		dstField.Set(reflect.ValueOf(&t))
	case fieldTimePtrToTS:
		tPtr, ok := srcField.Interface().(*time.Time)
		if !ok || tPtr == nil {
			dstField.Set(reflect.Zero(dstField.Type()))
			return nil
		}
		if tPtr.IsZero() {
			dstField.Set(reflect.Zero(dstField.Type()))
			return nil
		}
		dstField.Set(reflect.ValueOf(timestamppb.New(*tPtr)))
	case fieldWrapper:
		handled, err := tryConvertWrapper(srcField, dstField)
		if err != nil {
			return err
		}
		if !handled {
			return NewConversionError("字段 %s: wrapper 转换失败", entry.srcName)
		}
	case fieldSlice:
		return convertSlice(srcField, dstField)
	case fieldMap:
		return convertMap(srcField, dstField)
	case fieldMapStruct:
		return convertMapStruct(srcField, dstField)
	case fieldToPtr:
		convertToPtr(srcField, dstField)
	case fieldFromPtr:
		convertFromPtr(srcField, dstField)
	case fieldStruct:
		return convertStruct(srcField, dstField)
	case fieldStructPtr:
		return convertStructPtr(srcField, dstField)
	case fieldDataWrapper:
		return convertDataWrapper(srcField, dstField)
	case fieldStringFallback:
		dstField.SetString(srcField.String())
	case fieldNoop:
	}

	return nil
}

// convertSlice 转换切片类型（reflect 慢路径）
// 处理三种情况：
//  1. 元素类型相同 → 直接 Set（零拷贝）
//  2. 元素类型可直接转换 → MakeSlice + Convert 循环
//  3. 元素类型需要特殊处理 → MakeSlice + convertElement 递归
//
// 性能瓶颈：每次都通过 reflect.MakeSlice 分配 + Index/Set 操作
func convertSlice(srcField, dstField reflect.Value) error {
	srcLen := srcField.Len()
	if srcLen == 0 {
		if dstField.IsNil() {
			dstField.Set(reflect.MakeSlice(dstField.Type(), 0, 0))
		}
		return nil
	}

	srcElemType := srcField.Type().Elem()
	dstElemType := dstField.Type().Elem()

	if srcElemType == dstElemType {
		dstField.Set(srcField)
		return nil
	}

	if srcElemType.ConvertibleTo(dstElemType) {
		result := reflect.MakeSlice(dstField.Type(), srcLen, srcLen)
		for i := 0; i < srcLen; i++ {
			result.Index(i).Set(srcField.Index(i).Convert(dstElemType))
		}
		dstField.Set(result)
		return nil
	}

	dstField.Set(reflect.MakeSlice(dstField.Type(), srcLen, srcLen))
	for i := 0; i < srcLen; i++ {
		srcElem := srcField.Index(i)
		dstElem := dstField.Index(i)
		if err := convertElement(srcElem, dstElem); err != nil {
			return err
		}
	}
	return nil
}

// convertElement 转换单个切片/Map元素（reflect 慢路径）
// 支持：同类型、Assignable、Convertible、Wrapper、StructPtr
func convertElement(srcElem, dstElem reflect.Value) error {
	srcType := srcElem.Type()
	dstType := dstElem.Type()

	if srcType == dstType {
		dstElem.Set(srcElem)
		return nil
	}

	if srcType.AssignableTo(dstType) {
		dstElem.Set(srcElem)
		return nil
	}

	if srcType.ConvertibleTo(dstType) {
		dstElem.Set(srcElem.Convert(dstType))
		return nil
	}

	handled, err := tryConvertWrapper(srcElem, dstElem)
	if err != nil {
		return err
	}
	if handled {
		return nil
	}

	if srcType.Kind() != reflect.Ptr && dstType.Kind() == reflect.Ptr {
		convertToPtr(srcElem, dstElem)
		return nil
	}

	if srcType.Kind() == reflect.Ptr && dstType.Kind() != reflect.Ptr {
		convertFromPtr(srcElem, dstElem)
		return nil
	}

	if srcType.Kind() == reflect.Ptr && dstType.Kind() == reflect.Ptr {
		srcElemType := srcType.Elem()
		dstElemType := dstType.Elem()
		if srcElemType.Kind() == reflect.Struct && dstElemType.Kind() == reflect.Struct {
			if srcElem.IsNil() {
				dstElem.Set(reflect.Zero(dstType))
				return nil
			}
			return convertStructPtr(srcElem, dstElem)
		}
	}

	return nil
}

// convertMap 转换 map 类型（reflect 慢路径）
// 支持：key/value 类型 Convertible、string 兜底值转换
// nil map → zero value
func convertMap(srcField, dstField reflect.Value) error {
	if srcField.IsNil() {
		dstField.Set(reflect.Zero(dstField.Type()))
		return nil
	}

	dstMap := reflect.MakeMapWithSize(dstField.Type(), srcField.Len())

	iter := srcField.MapRange()
	for iter.Next() {
		srcKey := iter.Key()
		srcVal := iter.Value()

		dstKey := srcKey
		if srcKey.Type() != dstField.Type().Key() {
			if srcKey.Type().ConvertibleTo(dstField.Type().Key()) {
				dstKey = srcKey.Convert(dstField.Type().Key())
			}
		}

		dstVal := srcVal
		dstValType := dstField.Type().Elem()
		if srcVal.Type() != dstValType {
			if srcVal.Type().ConvertibleTo(dstValType) {
				dstVal = srcVal.Convert(dstValType)
			} else if dstValType.Kind() == reflect.String {
				dstVal = reflect.ValueOf(fmt.Sprint(srcVal.Interface())).Convert(dstValType)
			} else {
				dstVal = reflect.ValueOf(srcVal.Interface())
			}
		}

		dstMap.SetMapIndex(dstKey, dstVal)
	}

	dstField.Set(dstMap)
	return nil
}

// structPtrType *structpb.Struct 的反射类型缓存，避免重复反射
var structPtrType = reflect.TypeOf((*structpb.Struct)(nil))

// 常见 map 类型的反射缓存，用于 ConvertibleTo 快路径判断
var (
	mapStringSliceType = reflect.TypeOf(map[string][]string(nil))
	mapStringAnyType   = reflect.TypeOf(map[string]interface{}(nil))
)

// IsStructPtrType 判断是否为 *structpb.Struct 类型
func IsStructPtrType(t reflect.Type) bool {
	return t == structPtrType
}

// isMapStructPair 判断 src/dst 类型对是否为 map[string]any ↔ *structpb.Struct
// 要求 map 的 key 为 string 类型（与 structpb.Struct 的 key 语义一致）
func isMapStructPair(srcType, dstType reflect.Type) bool {
	// map[string]any → *structpb.Struct
	if srcType.Kind() == reflect.Map && dstType == structPtrType {
		return srcType.Key().Kind() == reflect.String
	}
	// *structpb.Struct → map[string]any
	if srcType == structPtrType && dstType.Kind() == reflect.Map {
		return dstType.Key().Kind() == reflect.String
	}
	return false
}

// convertMapStruct 处理 map[string]any ↔ *structpb.Struct 双向转换
// 方向由 srcField/dstField 的类型自动判断：
//   - src 为 Map、dst 为 *structpb.Struct → map → struct
//   - src 为 *structpb.Struct、dst 为 Map → struct → map
func convertMapStruct(srcField, dstField reflect.Value) error {
	// map → *structpb.Struct
	if srcField.Kind() == reflect.Map && dstField.Type() == structPtrType {
		if srcField.IsNil() {
			dstField.Set(reflect.Zero(dstField.Type()))
			return nil
		}

		// 快路径 1：map[string][]string（含 CompressedTextMap 等命名类型）
		// 直接构建 structpb.Struct，跳过 map[string]interface{} + []interface{} 中间层 + structpb.NewStruct 类型探测
		if srcField.Type().ConvertibleTo(mapStringSliceType) {
			m := srcField.Convert(mapStringSliceType).Interface().(map[string][]string)
			st := buildStructFromMapStringSlice(m)
			dstField.Set(reflect.ValueOf(st))
			return nil
		}

		// 快路径 2：map[string]interface{} — 先直接试 structpb.NewStruct（零归一化开销）
		// 失败时（值含 []string 等不兼容类型）才走 normalizeValue 逐项修复
		if srcField.Type().ConvertibleTo(mapStringAnyType) {
			m := srcField.Convert(mapStringAnyType).Interface().(map[string]interface{})
			st, err := structpb.NewStruct(m)
			if err == nil {
				dstField.Set(reflect.ValueOf(st))
				return nil
			}
			srcMap := make(map[string]interface{}, len(m))
			for k, v := range m {
				srcMap[k] = normalizeValue(v)
			}
			st, err = structpb.NewStruct(srcMap)
			if err != nil {
				return NewConversionError("map→struct 转换失败: %v", err)
			}
			dstField.Set(reflect.ValueOf(st))
			return nil
		}

		// 慢路径：任意 map 类型，逐元素 reflect
		srcMap := make(map[string]interface{}, srcField.Len())
		iter := srcField.MapRange()
		for iter.Next() {
			srcMap[iter.Key().String()] = normalizeValue(iter.Value().Interface())
		}
		st, err := structpb.NewStruct(srcMap)
		if err != nil {
			return NewConversionError("map→struct 转换失败: %v", err)
		}
		dstField.Set(reflect.ValueOf(st))
		return nil
	}

	// *structpb.Struct → map[string]any
	if srcField.Type() == structPtrType && dstField.Kind() == reflect.Map {
		if srcField.IsNil() {
			dstField.Set(reflect.Zero(dstField.Type()))
			return nil
		}
		st := srcField.Interface().(*structpb.Struct)
		// AsMap 返回 map[string]interface{}，通过 reflect 创建中间 Value 后复用 convertMap
		srcMapValue := reflect.ValueOf(st.AsMap())
		return convertMap(srcMapValue, dstField)
	}

	return nil
}

// buildStructFromMapStringSlice 直接从 map[string][]string 构建 *structpb.Struct
// 跳过 structpb.NewStruct 的 map[string]interface{} 中间层和类型探测
//
// 核心优化：批量预分配。原实现每个 Value/StringValue/ListValue 均单独 new，
// 对于 N 键 M 值产生约 4N+2M+2 次分配；本实现将同类对象合并到连续数组中，
// 通过 &array[i] 取地址复用，分配数降至常数级（~8 次），降幅约 85-90%
//
// 安全性：make([]T,n) 的底屽数组在堆上，&array[i] 存入 map 后由 GC 保持引用存活，
// 与 structpb.NewStringValue 内部 &Value{...} 语义完全等价
func buildStructFromMapStringSlice(m map[string][]string) *structpb.Struct {
	n := len(m)
	if n == 0 {
		return &structpb.Struct{Fields: map[string]*structpb.Value{}}
	}

	// 统计字符串值总数，用于预分配字符串相关数组
	totalStrings := 0
	for _, arr := range m {
		totalStrings += len(arr)
	}

	// 批量预分配：同类对象放入连续数组，消除逐元素 new 的堆分配
	// list Values 与 string Values 合并到同一 allValues 数组的不同区间，减少 1 次分配
	allValues := make([]structpb.Value, n+totalStrings)                // [0,n)=ListValue Value, [n,n+M)=StringValue Value
	listWrappers := make([]structpb.Value_ListValue, n)                // ListValue oneof wrapper
	stringWrappers := make([]structpb.Value_StringValue, totalStrings) // StringValue oneof wrapper
	listInnerObjs := make([]structpb.ListValue, n)                     // ListValue 内部对象
	valuePtrBacking := make([]*structpb.Value, totalStrings)           // 各 key 的 []*Value 共享同一 backing array

	fields := make(map[string]*structpb.Value, n)

	listIdx, strIdx, ptrIdx := 0, 0, 0
	for k, arr := range m {
		cnt := len(arr)
		// 填充本 key 对应的字符串 Value 指针（存放在 allValues 的 [n, n+M) 区间）
		for i, s := range arr {
			stringWrappers[strIdx].StringValue = s
			allValues[n+strIdx].Kind = &stringWrappers[strIdx]
			valuePtrBacking[ptrIdx+i] = &allValues[n+strIdx]
			strIdx++
		}
		// 组装 ListValue 并挂到 fields（ListValue Value 存放在 allValues 的 [0, n) 区间）
		listInnerObjs[listIdx].Values = valuePtrBacking[ptrIdx : ptrIdx+cnt]
		listWrappers[listIdx].ListValue = &listInnerObjs[listIdx]
		allValues[listIdx].Kind = &listWrappers[listIdx]
		fields[k] = &allValues[listIdx]

		ptrIdx += cnt
		listIdx++
	}

	return &structpb.Struct{Fields: fields}
}

// buildStructFromMapAny 从 map[string]interface{} 构建 *structpb.Struct，使用批量预分配
// 仅处理扁平 structpb 兼容类型（顶层与列表元素均为 string/bool/float64/nil），
// 嵌套 map/[]interface{} of 复杂类型等场景返回 (nil, error) 触发调用方回退 structpb.NewStruct
//
// 分配数从 structpb.NewStruct 的 ~3N+2M 降至常数级（~8 次），降幅约 70-85%
func buildStructFromMapAny(m map[string]interface{}) (*structpb.Struct, error) {
	n := len(m)
	if n == 0 {
		return &structpb.Struct{Fields: map[string]*structpb.Value{}}, nil
	}

	// 第一遍：按类型计数，同时检测是否全部为可快速处理的扁平类型
	var strCnt, boolCnt, numCnt, nullCnt, listCnt, listElemCnt int
	fast := true
	for _, v := range m {
		switch val := v.(type) {
		case string:
			strCnt++
		case bool:
			boolCnt++
		case float64:
			numCnt++
		case nil:
			nullCnt++
		case []interface{}:
			listCnt++
			listElemCnt += len(val)
			for _, elem := range val {
				switch elem.(type) {
				case string:
					strCnt++
				case bool:
					boolCnt++
				case float64:
					numCnt++
				case nil:
					nullCnt++
				default:
					fast = false
				}
			}
		default:
			fast = false
		}
	}
	if !fast {
		return nil, errMapAnyNotFlat
	}

	// 预分配各类 wrapper 与 Value 数组，同类对象连续存储
	// scalar Values 与 list Values 合并到同一 allValues 数组的不同区间，减少 1 次分配
	totalScalars := strCnt + boolCnt + numCnt + nullCnt
	allValues := make([]structpb.Value, totalScalars+listCnt) // [0,totalScalars)=标量 Value, [totalScalars,...)=ListValue Value
	stringWrappers := make([]structpb.Value_StringValue, strCnt)
	boolWrappers := make([]structpb.Value_BoolValue, boolCnt)
	numberWrappers := make([]structpb.Value_NumberValue, numCnt)
	nullWrappers := make([]structpb.Value_NullValue, nullCnt)
	listWrappers := make([]structpb.Value_ListValue, listCnt)
	listInnerObjs := make([]structpb.ListValue, listCnt)
	valuePtrBacking := make([]*structpb.Value, listElemCnt)

	fields := make(map[string]*structpb.Value, n)

	// 第二遍：填充
	strIdx, boolIdx, numIdx, nullIdx := 0, 0, 0, 0
	listIdx, ptrIdx := 0, 0
	scalarIdx := 0 // 标量 Values 的写入游标
	for k, v := range m {
		switch val := v.(type) {
		case string:
			stringWrappers[strIdx].StringValue = val
			allValues[scalarIdx].Kind = &stringWrappers[strIdx]
			fields[k] = &allValues[scalarIdx]
			strIdx++
			scalarIdx++
		case bool:
			boolWrappers[boolIdx].BoolValue = val
			allValues[scalarIdx].Kind = &boolWrappers[boolIdx]
			fields[k] = &allValues[scalarIdx]
			boolIdx++
			scalarIdx++
		case float64:
			numberWrappers[numIdx].NumberValue = val
			allValues[scalarIdx].Kind = &numberWrappers[numIdx]
			fields[k] = &allValues[scalarIdx]
			numIdx++
			scalarIdx++
		case nil:
			nullWrappers[nullIdx].NullValue = structpb.NullValue_NULL_VALUE
			allValues[scalarIdx].Kind = &nullWrappers[nullIdx]
			fields[k] = &allValues[scalarIdx]
			nullIdx++
			scalarIdx++
		case []interface{}:
			cnt := len(val)
			// 填充列表元素的标量 Value 指针
			for _, elem := range val {
				switch e := elem.(type) {
				case string:
					stringWrappers[strIdx].StringValue = e
					allValues[scalarIdx].Kind = &stringWrappers[strIdx]
					strIdx++
				case bool:
					boolWrappers[boolIdx].BoolValue = e
					allValues[scalarIdx].Kind = &boolWrappers[boolIdx]
					boolIdx++
				case float64:
					numberWrappers[numIdx].NumberValue = e
					allValues[scalarIdx].Kind = &numberWrappers[numIdx]
					numIdx++
				case nil:
					nullWrappers[nullIdx].NullValue = structpb.NullValue_NULL_VALUE
					allValues[scalarIdx].Kind = &nullWrappers[nullIdx]
					nullIdx++
				}
				valuePtrBacking[ptrIdx] = &allValues[scalarIdx]
				ptrIdx++
				scalarIdx++
			}
			// 组装 ListValue（ListValue Value 存放在 allValues 的 [totalScalars, ...) 区间）
			listInnerObjs[listIdx].Values = valuePtrBacking[ptrIdx-cnt : ptrIdx]
			listWrappers[listIdx].ListValue = &listInnerObjs[listIdx]
			allValues[totalScalars+listIdx].Kind = &listWrappers[listIdx]
			fields[k] = &allValues[totalScalars+listIdx]
			listIdx++
		}
	}

	return &structpb.Struct{Fields: fields}, nil
}

// errMapAnyNotFlat 标记 map 值含非扁平类型（嵌套 map/[]interface{} of 复杂类型等），
// 触发调用方回退到 normalizeValue + structpb.NewStruct 路径
var errMapAnyNotFlat = NewConversionError("map 值含非扁平类型，需回退归一化路径")

// normalizeValue 将 interface{} 归一化为 structpb.NewStruct/NewValue 兼容类型
// 优先 type-switch（零 reflect 开销），仅罕见类型走 reflect 兜底
// map[string][]string 分支使用单块连续 backing array 承载所有 []interface{} 切片，
// 将 N 次 slice 分配合并为 1 次，分配数从 ~2N+1 降至 ~N+2
func normalizeValue(v interface{}) interface{} {
	switch val := v.(type) {
	case nil:
		return nil
	case string, bool, int, int32, int64, float32, float64, []byte, []interface{}, map[string]interface{}:
		return val // structpb 原生兼容，零转换
	case []string:
		out := make([]interface{}, len(val))
		for i, s := range val {
			out[i] = s
		}
		return out
	case map[string][]string:
		// 单遍历 + 共享 backing array：用 append 累积所有字符串值到同一底层数组，
		// 各 key 对应的切片取 backing 的不同区间，减少 N-1 次 []interface{} 分配
		// 初始容量按每键约 2 个值估算，多数场景避免 append 扩容拷贝
		backing := make([]interface{}, 0, len(val)*2)
		out := make(map[string]interface{}, len(val))
		for k, arr := range val {
			start := len(backing)
			for _, s := range arr {
				backing = append(backing, s)
			}
			out[k] = backing[start:]
		}
		return out
	default:
		// reflect 兜底：处理 []int / []float64 / map[string]int 等罕见类型
		rv := reflect.ValueOf(v)
		if !rv.IsValid() {
			return nil
		}
		return normalizeValueReflect(rv)
	}
}

// normalizeValueReflect reflect 兜底路径，仅被 normalizeValue 的 default 分支调用
func normalizeValueReflect(v reflect.Value) interface{} {
	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return v.Bytes()
		}
		out := make([]interface{}, v.Len())
		for i := 0; i < v.Len(); i++ {
			out[i] = normalizeValueReflect(v.Index(i))
		}
		return out
	case reflect.Map:
		out := make(map[string]interface{}, v.Len())
		iter := v.MapRange()
		for iter.Next() {
			out[iter.Key().String()] = normalizeValueReflect(iter.Value())
		}
		return out
	default:
		return v.Interface()
	}
}

// convertToPtr 将值类型转换为指针类型（reflect 慢路径）
// T → *T：检查零值→nil，非零值→reflect.New + Set/Convert
func convertToPtr(srcField, dstField reflect.Value) {
	if srcField.IsZero() {
		dstField.Set(reflect.Zero(dstField.Type()))
		return
	}
	dstElemType := dstField.Type().Elem()
	if srcField.Type().ConvertibleTo(dstElemType) {
		ptr := reflect.New(dstElemType)
		ptr.Elem().Set(srcField.Convert(dstElemType))
		dstField.Set(ptr)
	} else if srcField.Type().AssignableTo(dstElemType) {
		ptr := reflect.New(dstElemType)
		ptr.Elem().Set(srcField)
		dstField.Set(ptr)
	} else {
		dstField.Set(reflect.Zero(dstField.Type()))
	}
}

// convertFromPtr 将指针类型转换为值类型（reflect 慢路径）
// *T → T：nil→零值，非nil→Elem()+Set/Convert
func convertFromPtr(srcField, dstField reflect.Value) {
	if srcField.IsNil() {
		dstField.Set(reflect.Zero(dstField.Type()))
		return
	}
	val := srcField.Elem()
	if val.Type().AssignableTo(dstField.Type()) {
		dstField.Set(val)
	} else if val.Type().ConvertibleTo(dstField.Type()) {
		dstField.Set(val.Convert(dstField.Type()))
	} else {
		dstField.Set(reflect.Zero(dstField.Type()))
	}
}

// applyStructFieldCacheReflect 在 reflect.Value 上应用 structFieldCache 的 fast+slow 转换
// 这是 convertStruct/convertStructWithCache/convertStructPtr 的共用逻辑
// CanAddr 时走 unsafe 快速路径（mergedFn 或 fastEntries copyFunc），否则全走 reflect
func applyStructFieldCacheReflect(srcVal, dstVal reflect.Value, sc *structFieldCache) {
	if srcVal.CanAddr() && dstVal.CanAddr() {
		srcBase := unsafe.Pointer(srcVal.UnsafeAddr())
		dstBase := unsafe.Pointer(dstVal.UnsafeAddr())
		if sc.mergedFn != nil {
			sc.mergedFn(srcBase, dstBase)
		} else {
			for i := range sc.fastEntries {
				sc.fastEntries[i].copyFunc(srcBase, dstBase)
			}
		}
		for i := range sc.slowEntries {
			entry := &sc.slowEntries[i]
			s := srcVal.FieldByIndex(entry.srcIndex)
			d := dstVal.FieldByIndex(entry.dstIndex)
			if !s.IsValid() || !d.IsValid() || !d.CanSet() {
				continue
			}
			convertStructFieldSlow(s, d, entry)
		}
		return
	}
	// 非 CanAddr 路径：fastEntries + slowEntries 都走 reflect convertStructFieldSlow
	for i := range sc.fastEntries {
		entry := &sc.fastEntries[i]
		s := srcVal.FieldByIndex(entry.srcIndex)
		d := dstVal.FieldByIndex(entry.dstIndex)
		if !s.IsValid() || !d.IsValid() || !d.CanSet() {
			continue
		}
		convertStructFieldSlow(s, d, entry)
	}
	for i := range sc.slowEntries {
		entry := &sc.slowEntries[i]
		s := srcVal.FieldByIndex(entry.srcIndex)
		d := dstVal.FieldByIndex(entry.dstIndex)
		if !s.IsValid() || !d.IsValid() || !d.CanSet() {
			continue
		}
		convertStructFieldSlow(s, d, entry)
	}
}

// applyStructSlowEntriesUnsafe 在 unsafe 指针基址上应用 structFieldCache 的 slowEntries
// 这是 make*CopyFunc 系列函数中处理嵌套结构体慢速字段的共用逻辑：
// 利用 sc.srcType/sc.dstType 构造 reflect.Value，再走 FieldByIndex + convertStructFieldSlow
// 调用方需保证 srcBase/dstBase 已指向与 sc.srcType/sc.dstType 对应的结构体实例
func applyStructSlowEntriesUnsafe(srcBase, dstBase unsafe.Pointer, sc *structFieldCache) {
	if len(sc.slowEntries) == 0 {
		return
	}
	srcVal := reflect.NewAt(sc.srcType, srcBase).Elem()
	dstVal := reflect.NewAt(sc.dstType, dstBase).Elem()
	for i := range sc.slowEntries {
		entry := &sc.slowEntries[i]
		s := srcVal.FieldByIndex(entry.srcIndex)
		d := dstVal.FieldByIndex(entry.dstIndex)
		if !s.IsValid() || !d.IsValid() {
			continue
		}
		convertStructFieldSlow(s, d, entry)
	}
}

// convertStruct 转换结构体（值类型）
// 优先级：
//  1. 查找 globalStructFieldCache 中的缓存 → 用 fast/slow 分层转换
//  2. 查找 globalRegistry 中的注册转换器 → ConvertModelToPB 递归
//  3. 无缓存 → buildStructFieldCache 构建并缓存 → convertStructWithCache
//  4. 最终回退 → convertStructFields（最慢，每次重建字段映射）
func convertStruct(srcField, dstField reflect.Value) error {
	srcType := srcField.Type()
	dstType := dstField.Type()

	subCache, cached := globalStructFieldCache.Load(structFieldTypePair{srcType, dstType})
	if cached {
		sc := subCache.(*structFieldCache)
		if srcField.CanAddr() && dstField.CanAddr() {
			applyStructFieldCacheReflect(srcField, dstField, sc)
			return nil
		}
	}

	converter, ok := findConverter(srcType, dstType)
	if ok && converter != nil {
		srcPtr := reflect.New(srcType)
		srcPtr.Elem().Set(srcField)
		dstPtr := reflect.New(dstType)
		if err := converter.ConvertModelToPB(srcPtr.Interface(), dstPtr.Interface()); err != nil {
			return err
		}
		dstField.Set(dstPtr.Elem())
		return nil
	}

	if !cached {
		subCache = buildStructFieldCache(srcType, dstType, true)
		globalStructFieldCache.Store(structFieldTypePair{srcType, dstType}, subCache)
		return convertStructWithCache(srcField, dstField, subCache.(*structFieldCache))
	}

	return convertStructFields(srcField, dstField)
}

// convertStructWithCache 使用缓存的字段映射转换结构体值
// 与 convertStruct 类似但直接使用预构建的 structFieldCache
// 委托给 applyStructFieldCacheReflect 处理 CanAddr 快速路径和 reflect 回退
func convertStructWithCache(srcField, dstField reflect.Value, sc *structFieldCache) error {
	applyStructFieldCacheReflect(srcField, dstField, sc)
	return nil
}

// convertStructPtr 转换结构体指针（*StructA → *StructB）
// 优先级：
//  1. 查找 globalStructFieldCache → 用 fast/slow 分层转换（unsafe 优先）
//  2. 查找 globalRegistry → ConvertModelToPB 递归
//  3. 构建 structFieldCache 并缓存 → 刷新后重试
//
// 关键优化：对嵌套结构体指针使用预计算的子缓存，避免每次递归的 reflect 开销
func convertStructPtr(srcField, dstField reflect.Value) error {
	if srcField.IsNil() {
		dstField.Set(reflect.Zero(dstField.Type()))
		return nil
	}

	srcElemType := srcField.Type().Elem()
	dstElemType := dstField.Type().Elem()

	subCache, cached := globalStructFieldCache.Load(structFieldTypePair{srcElemType, dstElemType})
	if !cached {
		// 未缓存时先尝试注册转换器，再构建缓存
		if converter, ok := findConverter(srcElemType, dstElemType); ok && converter != nil {
			dstPtr := reflect.New(dstElemType)
			if err := converter.ConvertModelToPB(srcField.Interface(), dstPtr.Interface()); err != nil {
				return err
			}
			dstField.Set(dstPtr)
			return nil
		}
		subCache = buildStructFieldCache(srcElemType, dstElemType, true)
		globalStructFieldCache.Store(structFieldTypePair{srcElemType, dstElemType}, subCache)
	}
	sc := subCache.(*structFieldCache)

	srcElem := srcField.Elem()
	dstPtr := reflect.New(dstElemType)
	dstElem := dstPtr.Elem()
	applyStructFieldCacheReflect(srcElem, dstElem, sc)
	dstField.Set(dstPtr)
	return nil
}

// convertStructFields 递归转换结构体字段（最慢的回退路径）
// 性能瓶颈：
//   - 每次调用都重建 srcFields map（O(N) 扫描）
//   - 使用 FieldByName 进行字段查找（O(N) 线性扫描）
//   - 对每个字段调用 convertFieldAuto 重新判断类型（重复 classifyField 的工作）
//
// 应尽量避免进入此路径，优先使用缓存的 convertStruct/convertStructPtr
func convertStructFields(srcVal, dstVal reflect.Value) error {
	srcType := srcVal.Type()
	dstType := dstVal.Type()

	srcFields := make(map[string]reflect.StructField)
	for i := 0; i < srcType.NumField(); i++ {
		f := srcType.Field(i)
		if f.IsExported() {
			srcFields[f.Name] = f
		}
	}

	for i := 0; i < dstType.NumField(); i++ {
		dstField := dstType.Field(i)
		if !dstField.IsExported() {
			continue
		}

		srcField, ok := srcFields[dstField.Name]
		if !ok {
			for name, field := range srcFields {
				if strings.EqualFold(name, dstField.Name) {
					srcField = field
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}

		srcFieldVal := srcVal.FieldByName(srcField.Name)
		dstFieldVal := dstVal.FieldByName(dstField.Name)
		if !srcFieldVal.IsValid() || !dstFieldVal.IsValid() || !dstFieldVal.CanSet() {
			continue
		}

		if err := convertFieldAuto(srcFieldVal, dstFieldVal); err != nil {
			return err
		}
	}

	return nil
}

// convertFieldAuto 自动推断并转换字段（convertStructFields 的递归回退）
// 对每个字段重新执行类型判断逻辑，是最慢的路径
// 支持：同类型、Wrapper、时间戳/时间、Assignable、Convertible、Slice、Map、Ptr、Struct
func convertFieldAuto(srcField, dstField reflect.Value) error {
	srcType := srcField.Type()
	dstType := dstField.Type()

	if srcType == dstType {
		dstField.Set(srcField)
		return nil
	}

	handled, err := tryConvertWrapper(srcField, dstField)
	if err != nil {
		return err
	}
	if handled {
		return nil
	}

	if IsTimestampPtrType(srcType) && IsTimeType(dstType) {
		ts, ok := srcField.Interface().(*timestamppb.Timestamp)
		if !ok || ts == nil {
			dstField.Set(reflect.Zero(dstType))
		} else {
			dstField.Set(reflect.ValueOf(ts.AsTime()))
		}
		return nil
	}

	if IsTimeType(srcType) && IsTimestampPtrType(dstType) {
		t, ok := srcField.Interface().(time.Time)
		if !ok || t.IsZero() {
			dstField.Set(reflect.Zero(dstType))
		} else {
			dstField.Set(reflect.ValueOf(timestamppb.New(t)))
		}
		return nil
	}

	if srcType == timePtrType && dstType == timestampPtrType {
		tPtr := srcField.Interface().(*time.Time)
		if tPtr == nil || tPtr.IsZero() {
			dstField.Set(reflect.Zero(dstType))
		} else {
			dstField.Set(reflect.ValueOf(timestamppb.New(*tPtr)))
		}
		return nil
	}

	if srcType == timestampPtrType && dstType == timePtrType {
		ts, ok := srcField.Interface().(*timestamppb.Timestamp)
		if !ok || ts == nil {
			dstField.Set(reflect.Zero(dstType))
		} else {
			t := ts.AsTime()
			dstField.Set(reflect.ValueOf(&t))
		}
		return nil
	}

	if srcType.AssignableTo(dstType) {
		dstField.Set(srcField)
		return nil
	}

	if srcType.ConvertibleTo(dstType) {
		dstField.Set(srcField.Convert(dstType))
		return nil
	}

	if srcType.Kind() == reflect.Slice && dstType.Kind() == reflect.Slice {
		return convertSlice(srcField, dstField)
	}

	if srcType.Kind() == reflect.Map && dstType.Kind() == reflect.Map {
		return convertMap(srcField, dstField)
	}

	// map[string]any ↔ *structpb.Struct 特殊转换
	if isMapStructPair(srcType, dstType) {
		return convertMapStruct(srcField, dstField)
	}

	if srcType.Kind() == reflect.Ptr && dstType.Kind() != reflect.Ptr {
		convertFromPtr(srcField, dstField)
		return nil
	}

	if srcType.Kind() != reflect.Ptr && dstType.Kind() == reflect.Ptr {
		convertToPtr(srcField, dstField)
		return nil
	}

	if srcType.Kind() == reflect.Ptr && dstType.Kind() == reflect.Ptr {
		if srcType.Elem().Kind() == reflect.Struct && dstType.Elem().Kind() == reflect.Struct {
			return convertStructPtr(srcField, dstField)
		}
	}

	if srcType.Kind() == reflect.Struct && dstType.Kind() == reflect.Struct {
		return convertStruct(srcField, dstField)
	}

	return nil
}

// convertDataWrapper 转换 DataWrapper[T] ↔ T 或 DataWrapper[T] ↔ *T
// DataWrapper 模式常见于 sqlbuilder/types 等库：type DataWrapper[T] struct{ Data T }
// 支持四种方向：
//   - DataWrapper[T](struct) → T(value)：解包 Data 字段，可能需要 Convert
//   - DataWrapper[T](struct) → *T(ptr)：解包 Data 字段，直接取指针
//   - T(value) → DataWrapper[T](struct)：包装到 Data 字段，可能需要 Convert
//   - *T(ptr) → DataWrapper[T](struct)：设置 Data 字段为指针
func convertDataWrapper(srcField, dstField reflect.Value) error {
	// 情况 1&2: src 是 DataWrapper[T] (struct 有 Data 字段), dst 是 T 或 *T
	if srcField.Kind() == reflect.Struct {
		srcDataField := srcField.FieldByName("Data")
		if !srcDataField.IsValid() {
			return convertFieldAuto(srcField, dstField)
		}

		// DataWrapper → *T
		if dstField.Kind() == reflect.Ptr {
			if srcDataField.IsNil() {
				dstField.Set(reflect.Zero(dstField.Type()))
				return nil
			}
			dstField.Set(srcDataField)
			return nil
		}

		// DataWrapper → T
		if srcDataField.Kind() == reflect.Ptr {
			if srcDataField.IsNil() {
				dstField.Set(reflect.Zero(dstField.Type()))
				return nil
			}
			dstField.Set(srcDataField.Elem().Convert(dstField.Type()))
			return nil
		}
		dstField.Set(srcDataField.Convert(dstField.Type()))
		return nil
	}

	// 情况 3&4: dst 是 DataWrapper[T], src 是 T 或 *T
	if dstField.Kind() == reflect.Struct {
		dstDataField := dstField.FieldByName("Data")
		if !dstDataField.IsValid() {
			return convertFieldAuto(srcField, dstField)
		}

		// *T → DataWrapper[T]
		if srcField.Kind() == reflect.Ptr {
			dstDataField.Set(srcField)
			return nil
		}

		// T → DataWrapper[T]
		dstDataField.Set(srcField.Convert(dstDataField.Type()))
		return nil
	}

	return convertFieldAuto(srcField, dstField)
}

// findConverter 查找已注册的结构体转换器（从全局注册表和缓存）
// 查找顺序：
//  1. converterCache（sync.Map，最快）
//  2. globalRegistry（按类型对查找）
//
// 找到后缓存到 converterCache，后续查找更快速
func findConverter(srcType, dstType reflect.Type) (*BidiConverter, bool) {
	key1 := typePair{srcType, dstType}
	if c, ok := converterCache.Load(key1); ok {
		return c.(*BidiConverter), true
	}

	key2 := typePair{dstType, srcType}
	if c, ok := converterCache.Load(key2); ok {
		return c.(*BidiConverter), true
	}

	if c, err := globalRegistry.Lookup(srcType, dstType); err == nil {
		converterCache.Store(key1, c)
		return c, true
	}

	if c, err := globalRegistry.Lookup(dstType, srcType); err == nil {
		converterCache.Store(key1, c)
		return c, true
	}

	return nil, false
}
