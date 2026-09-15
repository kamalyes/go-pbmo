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
	"sync/atomic"
	"unsafe"
)

// typePair 类型对，用于缓存 key
type typePair [2]reflect.Type

// converterCache 转换器缓存：typePair -> *BidiConverter
var converterCache sync.Map

// entrySlotCount 热路径缓存槽位数，按 typePair 哈希分片降低多类型对互驱逐
const entrySlotCount = 8

// entrySlot 热路径缓存槽位，按缓存行对齐避免相邻槽位伪共享
type entrySlot struct {
	_     [128]byte
	entry atomic.Pointer[cachedConverter]
}

// lastEntries 最近使用的转换条目槽组
var lastEntries [entrySlotCount]entrySlot

// cachedConverter 转换器热路径缓存条目
// 一次原子读同时取得转换器与双向字段缓存，稳态零分配
// gen 记录构建时的缓存代数，与转换器当前代数不一致时回源刷新
type cachedConverter struct {
	key       fastKey
	converter *BidiConverter
	gen       uint64
	pbToModel *fieldCache
	modelToPB *fieldCache
}

// fastKey 热路径槽位 key，取类型对底层 rtype 数据指针组合
// reflect.Type 底层 *rtype 指针唯一标识类型，等值比较退化为两次纯指针比较
// 避免 typePair 接口等值比较引入的 ifaceeq 间接调用开销
type fastKey struct {
	model uintptr
	pb    uintptr
}

// derefType 指针类型解引用，与注册中心 key 语义保持一致
func derefType(t reflect.Type) reflect.Type {
	if t.Kind() == reflect.Ptr {
		return t.Elem()
	}
	return t
}

// fastKeyOf 提取类型对的 rtype 指针、规范 typePair 与槽位下标
// 哈希与 key 构造共用一次接口数据字提取，热路径零多余反射调用
func fastKeyOf[M any, P any]() (fastKey, typePair, int) {
	mt := derefType(reflect.TypeFor[M]())
	pt := derefType(reflect.TypeFor[P]())
	mk := uintptr(ifaceDataPtr(mt))
	pk := uintptr(ifaceDataPtr(pt))
	return fastKey{mk, pk}, typePair{mt, pt}, int((mk*31 + pk) & (entrySlotCount - 1))
}

// newCachedConverter 确保字段缓存已构建并生成热路径条目
func newCachedConverter(key fastKey, c *BidiConverter) *cachedConverter {
	pbToModel, modelToPB := c.fieldCaches()
	return &cachedConverter{
		key:       key,
		converter: c,
		gen:       c.gen.Load(),
		pbToModel: pbToModel,
		modelToPB: modelToPB,
	}
}

// getOrInitCachedConverter 获取或初始化热路径缓存条目
// 命中槽位仅一次原子读；未命中从 converterCache 取转换器后重建条目
func getOrInitCachedConverter[M any, P any]() *cachedConverter {
	key, tp, idx := fastKeyOf[M, P]()
	slot := &lastEntries[idx]
	if e := slot.entry.Load(); e != nil && e.key == key && e.gen == e.converter.gen.Load() {
		return e
	}
	var c *BidiConverter
	if v, ok := converterCache.Load(tp); ok {
		c = v.(*BidiConverter)
	} else {
		actual, _ := converterCache.LoadOrStore(tp, NewBidiConverter(new(P), new(M)).WithAutoTimeConversion(true))
		c = actual.(*BidiConverter)
	}
	e := newCachedConverter(key, c)
	slot.entry.Store(e)
	return e
}

// lookupCachedConverter 按运行时元素类型查找热路径缓存条目
// 槽位命中零装箱；未命中回源 converterCache，仍无则返回 nil 不创建
// 用于元素类型与类型参数不一致的批量入口（如 SafeToPBs 的指针切片）
func lookupCachedConverter[P any](modelType reflect.Type) *cachedConverter {
	pt := reflect.TypeFor[P]()
	key := fastKey{uintptr(ifaceDataPtr(modelType)), uintptr(ifaceDataPtr(pt))}
	idx := int((key.model*31 + key.pb) & (entrySlotCount - 1))
	slot := &lastEntries[idx]
	if e := slot.entry.Load(); e != nil && e.key == key && e.gen == e.converter.gen.Load() {
		return e
	}
	var c *BidiConverter
	if v, ok := converterCache.Load(typePair{modelType, pt}); ok {
		c = v.(*BidiConverter)
	} else {
		return nil
	}
	e := newCachedConverter(key, c)
	slot.entry.Store(e)
	return e
}

// getOrInitConverter 获取或初始化转换器
func getOrInitConverter[M any, P any]() *BidiConverter {
	return getOrInitCachedConverter[M, P]().converter
}

// Register[M, P] 注册 Model-PB 转换对（使用默认配置）
func Register[M any, P any]() *BidiConverter {
	return getOrInitConverter[M, P]()
}

// RegisterWith[M, P] 注册 Model-PB 转换对（自定义配置）
func RegisterWith[M any, P any](opts ...Option) *BidiConverter {
	key, tp, idx := fastKeyOf[M, P]()
	c := NewBidiConverter(new(P), new(M), opts...)
	converterCache.Store(tp, c)
	lastEntries[idx].entry.Store(newCachedConverter(key, c))
	return c
}

// ToPB 将 Model 转换为 PB 消息，nil 输入返回 nil
func ToPB[M any, P any](m *M) (*P, error) {
	if m == nil {
		return nil, nil
	}
	e := getOrInitCachedConverter[M, P]()
	cache := e.modelToPB
	info := new(P)
	if cache.mergedCopyFunc != nil && len(cache.slowEntries) == 0 {
		cache.mergedCopyFunc(unsafe.Pointer(m), unsafe.Pointer(info))
		return info, nil
	}
	return info, e.converter.convertModelToPBPtrCached(unsafe.Pointer(m), unsafe.Pointer(info), cache)
}

// FromPB 将 PB 消息转换为 Model，nil 输入返回 nil
func FromPB[P any, M any](pb *P) (*M, error) {
	if pb == nil {
		return nil, nil
	}
	e := getOrInitCachedConverter[M, P]()
	cache := e.pbToModel
	model := new(M)
	if cache.mergedCopyFunc != nil && len(cache.slowEntries) == 0 {
		cache.mergedCopyFunc(unsafe.Pointer(pb), unsafe.Pointer(model))
		return model, nil
	}
	return model, e.converter.convertPBToModelPtrCached(unsafe.Pointer(pb), unsafe.Pointer(model), cache)
}

// ToPBs 批量将 Model 指针切片转换为 PB 切片
func ToPBs[M any, P any](models []*M) ([]*P, error) {
	pbs := make([]*P, len(models))
	if len(models) == 0 {
		return pbs, nil
	}
	e := getOrInitCachedConverter[M, P]()
	cache := e.modelToPB
	values := make([]P, len(models))
	if cache.mergedCopyFunc != nil && len(cache.slowEntries) == 0 {
		for i, m := range models {
			p := &values[i]
			if m != nil {
				cache.mergedCopyFunc(unsafe.Pointer(m), unsafe.Pointer(p))
			}
			pbs[i] = p
		}
		return pbs, nil
	}
	for i, m := range models {
		p := &values[i]
		if err := e.converter.convertModelToPBPtrCached(unsafe.Pointer(m), unsafe.Pointer(p), cache); err != nil {
			return nil, NewBatchError("元素 %d: %v", i, err)
		}
		pbs[i] = p
	}
	return pbs, nil
}

// FromPBs 批量将 PB 切片转换为 Model 指针切片
// nil 元素会被跳过，对应的 model 保持零值
func FromPBs[P any, M any](pbs []*P) ([]*M, error) {
	models := make([]*M, len(pbs))
	if len(pbs) == 0 {
		return models, nil
	}
	e := getOrInitCachedConverter[M, P]()
	cache := e.pbToModel
	values := make([]M, len(pbs))
	if cache.mergedCopyFunc != nil && len(cache.slowEntries) == 0 {
		for i, pb := range pbs {
			m := &values[i]
			if pb != nil {
				cache.mergedCopyFunc(unsafe.Pointer(pb), unsafe.Pointer(m))
			}
			models[i] = m
		}
		return models, nil
	}
	for i, pb := range pbs {
		m := &values[i]
		if pb == nil {
			models[i] = m
			continue
		}
		if err := e.converter.convertPBToModelPtrCached(unsafe.Pointer(pb), unsafe.Pointer(m), cache); err != nil {
			return nil, NewBatchError("元素 %d: %v", i, err)
		}
		models[i] = m
	}
	return models, nil
}

// SafeToPBs 安全批量将 Model 切片转换为 PB 切片，不因单个失败而中断
// 自动支持 []M（值切片）和 []*M（指针切片），nil 指针元素标记为失败
func SafeToPBs[M any, P any](models []M) ([]*P, *BatchResult) {
	pbs := make([]*P, len(models))
	result := &BatchResult{Results: make([]BatchItem, 0, len(models))}
	if len(models) == 0 {
		return pbs, result
	}
	values := make([]P, len(models))

	mType := reflect.TypeOf(models).Elem()

	if mType.Kind() == reflect.Ptr {
		// 槽位命中零装箱；未命中回源 converterCache 后仍走惰性创建回退
		var c *BidiConverter
		var cache *fieldCache
		if e := lookupCachedConverter[P](mType.Elem()); e != nil {
			c = e.converter
			cache = e.modelToPB
		} else {
			c = getOrInitConverter[M, P]()
			cache = c.modelToPBFieldCache()
		}
		pureFast := cache.mergedCopyFunc != nil && len(cache.slowEntries) == 0
		// 切片数据指针按元素大小步进，避免逐元素 reflect.ValueOf 构造
		base := unsafe.SliceData(models)
		elemSize := unsafe.Sizeof(*base)
		for i := range models {
			item := BatchItem{Index: i}
			// 元素为指针类型，槽位数据即元素指针值
			srcPtr := *(*unsafe.Pointer)(addPtr(unsafe.Pointer(base), uintptr(i)*elemSize))
			if srcPtr == nil {
				item.Success = false
				item.Error = NewConversionError("元素 %d: 输入为 nil", i)
				result.FailureCount++
				result.Results = append(result.Results, item)
				continue
			}
			p := &values[i]
			var err error
			if pureFast {
				cache.mergedCopyFunc(srcPtr, unsafe.Pointer(p))
			} else {
				err = c.convertModelToPBPtrCached(srcPtr, unsafe.Pointer(p), cache)
			}
			if err != nil {
				item.Success = false
				item.Error = err
				result.FailureCount++
			} else {
				item.Success = true
				item.Value = p
				pbs[i] = p
				result.SuccessCount++
			}
			result.Results = append(result.Results, item)
		}
	} else {
		c := getOrInitConverter[M, P]()
		cache := c.modelToPBFieldCache()
		pureFast := cache.mergedCopyFunc != nil && len(cache.slowEntries) == 0
		for i := range models {
			item := BatchItem{Index: i}
			p := &values[i]
			var err error
			if pureFast {
				cache.mergedCopyFunc(unsafe.Pointer(&models[i]), unsafe.Pointer(p))
			} else {
				err = c.convertModelToPBPtrCached(unsafe.Pointer(&models[i]), unsafe.Pointer(p), cache)
			}
			if err != nil {
				item.Success = false
				item.Error = err
				result.FailureCount++
			} else {
				item.Success = true
				item.Value = p
				pbs[i] = p
				result.SuccessCount++
			}
			result.Results = append(result.Results, item)
		}
	}

	return pbs, result
}

// SafeFromPBs 安全批量将 PB 切片转换为 Model 切片，不因单个失败而中断
// 接受 []*P（指针切片），nil 元素标记为失败
func SafeFromPBs[P any, M any](pbs []*P) ([]*M, *BatchResult) {
	models := make([]*M, len(pbs))
	result := &BatchResult{Results: make([]BatchItem, 0, len(pbs))}
	if len(pbs) == 0 {
		return models, result
	}
	values := make([]M, len(pbs))
	c := getOrInitConverter[M, P]()
	cache := c.pbToModelFieldCache()
	pureFast := cache.mergedCopyFunc != nil && len(cache.slowEntries) == 0

	for i, pb := range pbs {
		item := BatchItem{Index: i}
		if pb == nil {
			item.Success = false
			item.Error = NewConversionError("元素 %d: 输入为 nil", i)
			result.FailureCount++
			result.Results = append(result.Results, item)
			continue
		}
		m := &values[i]
		var err error
		if pureFast {
			cache.mergedCopyFunc(unsafe.Pointer(pb), unsafe.Pointer(m))
		} else {
			err = c.convertPBToModelPtrCached(unsafe.Pointer(pb), unsafe.Pointer(m), cache)
		}
		if err != nil {
			item.Success = false
			item.Error = err
			result.FailureCount++
		} else {
			item.Success = true
			item.Value = m
			models[i] = m
			result.SuccessCount++
		}
		result.Results = append(result.Results, item)
	}

	return models, result
}

// ConverterFor 获取已注册的转换器
func ConverterFor[M any, P any]() *BidiConverter {
	return getOrInitConverter[M, P]()
}

// PBToUpdates 将 PB 消息转换为 map[string]interface{}
// 遍历 proto 字段，使用 protobuf json tag 名作为 key（snake_case）
// 自动跳过零值、protobuf 内部字段，自动解包 wrapper 类型
// 字段 key 与偏移按类型缓存于更新计划，热路径零 tag 解析
func PBToUpdates[P any](pb *P) map[string]interface{} {
	if pb == nil {
		return map[string]interface{}{}
	}
	return buildUpdatesFromPlan(getUpdatesPlan[P](false), unsafe.Pointer(pb))
}

// ModelToUpdates 将 Model 结构体转换为 map[string]interface{}
// 使用 gorm column tag 或 json tag 作为 key，跳过零值字段和不可导出字段
// 字段 key 与偏移按类型缓存于更新计划，热路径零 tag 解析
func ModelToUpdates[M any](m *M) map[string]interface{} {
	if m == nil {
		return map[string]interface{}{}
	}
	return buildUpdatesFromPlan(getUpdatesPlan[M](true), unsafe.Pointer(m))
}

// buildUpdatesFromPlan 按预构建计划输出更新字段
// 空值跳过语义与旧反射实现一致：nil 指针、空字符串、零值、空集合不输出
func buildUpdatesFromPlan(plan *updatesPlan, base unsafe.Pointer) map[string]interface{} {
	updates := make(map[string]interface{}, len(plan.fields))
	for i := range plan.fields {
		entry := &plan.fields[i]
		if entry.empty(base) {
			continue
		}
		updates[entry.key] = entry.emit(base)
	}
	return updates
}
