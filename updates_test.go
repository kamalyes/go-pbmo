/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-04-23 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-23 20:53:30
 * @FilePath: \go-pbmo\updates_test.go
 * @Description: UpdatesBuilder 测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"testing"
)

func TestNewUpdates(t *testing.T) {
	b := NewUpdates()
	assert.NotNil(t, b)
	assert.Equal(t, 0, b.Len())
	assert.True(t, b.IsEmpty())
}

func TestUpdatesBuilderSet(t *testing.T) {
	b := NewUpdates().Set("name", "test")
	assert.Equal(t, 1, b.Len())
	assert.True(t, b.Has("name"))

	result := b.Build()
	assert.Equal(t, "test", result["name"])
}

func TestUpdatesBuilderSetIf(t *testing.T) {
	b := NewUpdates().
		SetIf(true, "key1", "val1").
		SetIf(false, "key2", "val2")

	result := b.Build()
	assert.Equal(t, "val1", result["key1"])
	_, ok := result["key2"]
	assert.False(t, ok)
}

func TestUpdatesBuilderSetIfNotEmpty(t *testing.T) {
	b := NewUpdates().
		SetIfNotEmpty("name", "hello").
		SetIfNotEmpty("empty", "").
		SetIfNotEmpty("space", " ")

	result := b.Build()
	assert.Equal(t, "hello", result["name"])
	assert.Equal(t, " ", result["space"])
	_, ok := result["empty"]
	assert.False(t, ok)
}

func TestUpdatesBuilderSetIfNotNil(t *testing.T) {
	var val interface{} = "value"
	var nilVal interface{} = nil

	b := NewUpdates().
		SetIfNotNil("key1", val).
		SetIfNotNil("key2", nilVal).
		SetIfNotNil("key3", nil)

	result := b.Build()
	assert.Equal(t, "value", result["key1"])
	_, ok := result["key2"]
	assert.False(t, ok)
	_, ok = result["key3"]
	assert.False(t, ok)
}

func TestUpdatesBuilderSetIfNotZero(t *testing.T) {
	var zeroInt int
	var zeroStr string
	var zeroBool bool

	b := NewUpdates().
		SetIfNotZero("int1", 42).
		SetIfNotZero("int2", zeroInt).
		SetIfNotZero("str1", "hello").
		SetIfNotZero("str2", zeroStr).
		SetIfNotZero("bool1", true).
		SetIfNotZero("bool2", zeroBool).
		SetIfNotZero("nil", nil)

	result := b.Build()
	assert.Equal(t, 42, result["int1"])
	assert.Equal(t, "hello", result["str1"])
	assert.Equal(t, true, result["bool1"])

	for _, key := range []string{"int2", "str2", "bool2", "nil"} {
		_, ok := result[key]
		assert.False(t, ok, "key=%s should not exist", key)
	}
}

func TestUpdatesBuilderSetIfNotZeroPointer(t *testing.T) {
	val := 100
	var nilPtr *int

	b := NewUpdates().
		SetIfNotZero("ptr1", &val).
		SetIfNotZero("ptr2", nilPtr)

	result := b.Build()
	assert.NotNil(t, result["ptr1"])
	_, ok := result["ptr2"]
	assert.False(t, ok)
}

func TestUpdatesBuilderSetStringVal(t *testing.T) {
	b := NewUpdates().
		SetStringVal("name", wrapperspb.String("hello")).
		SetStringVal("empty", wrapperspb.String("")).
		SetStringVal("nil", nil)

	result := b.Build()
	assert.Equal(t, "hello", result["name"])
	_, ok := result["empty"]
	assert.False(t, ok)
	_, ok = result["nil"]
	assert.False(t, ok)
}

func TestUpdatesBuilderSetStringValAny(t *testing.T) {
	b := NewUpdates().
		SetStringValAny("name", wrapperspb.String("hello")).
		SetStringValAny("empty", wrapperspb.String("")).
		SetStringValAny("nil", nil)

	result := b.Build()
	assert.Equal(t, "hello", result["name"])
	assert.Equal(t, "", result["empty"])
	_, ok := result["nil"]
	assert.False(t, ok)
}

func TestUpdatesBuilderSetInt32Val(t *testing.T) {
	b := NewUpdates().
		SetInt32Val("sort", wrapperspb.Int32(5)).
		SetInt32Val("zero", wrapperspb.Int32(0)).
		SetInt32Val("nil", nil)

	result := b.Build()
	assert.Equal(t, int32(5), result["sort"])
	assert.Equal(t, int32(0), result["zero"])
	_, ok := result["nil"]
	assert.False(t, ok)
}

func TestUpdatesBuilderSetInt64Val(t *testing.T) {
	b := NewUpdates().
		SetInt64Val("id", wrapperspb.Int64(999)).
		SetInt64Val("nil", nil)

	result := b.Build()
	assert.Equal(t, int64(999), result["id"])
	_, ok := result["nil"]
	assert.False(t, ok)
}

func TestUpdatesBuilderSetBoolVal(t *testing.T) {
	b := NewUpdates().
		SetBoolVal("flag", wrapperspb.Bool(true)).
		SetBoolVal("falseval", wrapperspb.Bool(false)).
		SetBoolVal("nil", nil)

	result := b.Build()
	assert.Equal(t, true, result["flag"])
	assert.Equal(t, false, result["falseval"])
	_, ok := result["nil"]
	assert.False(t, ok)
}

func TestUpdatesBuilderSetFloatVal(t *testing.T) {
	b := NewUpdates().
		SetFloatVal("score", wrapperspb.Float(3.14)).
		SetFloatVal("nil", nil)

	result := b.Build()
	assert.InDelta(t, float32(3.14), result["score"], 0.001)
	_, ok := result["nil"]
	assert.False(t, ok)
}

func TestUpdatesBuilderSetDoubleVal(t *testing.T) {
	b := NewUpdates().
		SetDoubleVal("price", wrapperspb.Double(99.99)).
		SetDoubleVal("nil", nil)

	result := b.Build()
	assert.InDelta(t, 99.99, result["price"], 0.001)
	_, ok := result["nil"]
	assert.False(t, ok)
}

func TestUpdatesBuilderDelete(t *testing.T) {
	b := NewUpdates().Set("name", "test")
	assert.True(t, b.Has("name"))

	b.Delete("name")
	assert.False(t, b.Has("name"))
}

func TestUpdatesBuilderClear(t *testing.T) {
	b := NewUpdates().Set("a", 1).Set("b", 2)
	assert.Equal(t, 2, b.Len())

	b.Clear()
	assert.Equal(t, 0, b.Len())
	assert.True(t, b.IsEmpty())
}

func TestUpdatesBuilderMerge(t *testing.T) {
	b1 := NewUpdates().Set("a", 1).Set("b", 2)
	b2 := NewUpdates().Set("b", 20).Set("c", 30)

	b1.Merge(b2)
	result := b1.Build()
	assert.Equal(t, 1, result["a"])
	assert.Equal(t, 20, result["b"])
	assert.Equal(t, 30, result["c"])
}

func TestUpdatesBuilderMergeNil(t *testing.T) {
	b := NewUpdates().Set("a", 1)
	b.Merge(nil)
	assert.Equal(t, 1, b.Len())
}

func TestUpdatesBuilderString(t *testing.T) {
	b := NewUpdates().Set("name", "test").Set("age", 18)
	s := b.String()
	assert.Contains(t, s, "name=test")
	assert.Contains(t, s, "age=18")
}

func TestUpdatesBuilderChainedCalls(t *testing.T) {
	result := NewUpdates().
		SetStringVal("name", wrapperspb.String("kronos")).
		SetInt32Val("sort", wrapperspb.Int32(1)).
		SetBoolVal("active", wrapperspb.Bool(true)).
		SetIfNotEmpty("desc", "a platform").
		SetIfNotZero("count", 42).
		Build()

	assert.Equal(t, 5, len(result))
	assert.Equal(t, "kronos", result["name"])
	assert.Equal(t, int32(1), result["sort"])
	assert.Equal(t, true, result["active"])
	assert.Equal(t, "a platform", result["desc"])
	assert.Equal(t, 42, result["count"])
}

func TestUpdatesBuilderRealWorldScenario(t *testing.T) {
	result := NewUpdates().
		SetStringVal("name", wrapperspb.String("new-name")).
		SetStringVal("icon", wrapperspb.String("")).
		SetInt32Val("sortorder", wrapperspb.Int32(10)).
		SetIfNotNil("config", `{"theme":"dark"}`).
		Build()

	assert.Equal(t, 3, len(result))
	assert.Equal(t, "new-name", result["name"])
	assert.Equal(t, int32(10), result["sortorder"])
	assert.Equal(t, `{"theme":"dark"}`, result["config"])
	_, ok := result["icon"]
	assert.False(t, ok)
}

func TestIsZeroValueInternal(t *testing.T) {
	assert.True(t, isZeroValue(nil))
	assert.True(t, isZeroValue(0))
	assert.True(t, isZeroValue(""))
	assert.True(t, isZeroValue(false))
	assert.True(t, isZeroValue(int32(0)))

	var p *int
	assert.True(t, isZeroValue(p))

	assert.False(t, isZeroValue(1))
	assert.False(t, isZeroValue("x"))
	assert.False(t, isZeroValue(true))

	v := 42
	assert.False(t, isZeroValue(&v))
}
