/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2023-09-26 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2023-09-26 00:00:00
 * @FilePath: \go-pbmo\testmodels.go
 * @Description: 测试用模型定义 - 仅用于测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// TestPB 测试用 PB 结构体
type TestPB struct {
	Id     uint64
	Name   string
	Email  string
	Age    int32
	Score  float64
	Active bool
	Tags   []string
}

// TestModel 测试用 Model 结构体
type TestModel struct {
	ID     uint64 `pbmo:"Id"`
	Name   string
	Email  string `desensitize:"email"`
	Age    int
	Score  float64
	Active bool
	Tags   []string
}

// TestPBWithMapping 测试用 PB 结构体（需要字段映射）
type TestPBWithMapping struct {
	ClientId  uint64
	UserName  string
	UserEmail string
}

// TestModelWithMapping 测试用 Model 结构体（需要字段映射）
type TestModelWithMapping struct {
	ID    uint64 `pbmo:"ClientId"`
	Name  string `pbmo:"UserName"`
	Email string `pbmo:"UserEmail"`
}

// TestSimplePB 简单测试 PB
type TestSimplePB struct {
	Value string
	Count int32
}

// TestSimpleModel 简单测试 Model
type TestSimpleModel struct {
	Value string
	Count int32
}

// TestNestPB 嵌套测试 PB
type TestNestPB struct {
	Name   string
	Detail *TestSimplePB
}

// TestNestModel 嵌套测试 Model
type TestNestModel struct {
	Name   string
	Detail *TestSimpleModel
}

// TestAllTypesPB 全类型测试 PB
type TestAllTypesPB struct {
	IntVal    int32
	Int64Val  int64
	UintVal   uint32
	Uint64Val uint64
	FloatVal  float32
	DoubleVal float64
	BoolVal   bool
	StrVal    string
	BytesVal  []byte
}

// TestAllTypesModel 全类型测试 Model
type TestAllTypesModel struct {
	IntVal    int32
	Int64Val  int64
	UintVal   uint32
	Uint64Val uint64
	FloatVal  float32
	DoubleVal float64
	BoolVal   bool
	StrVal    string
	BytesVal  []byte
}

// TestEmptyPB 空结构体 PB
type TestEmptyPB struct{}

// TestEmptyModel 空结构体 Model
type TestEmptyModel struct{}

// TestSingleFieldPB 单字段 PB
type TestSingleFieldPB struct {
	Name string
}

// TestSingleFieldModel 单字段 Model
type TestSingleFieldModel struct {
	Name string
}

// TestNamedSlicePB 命名切片测试 PB（使用 []string）
type TestNamedSlicePB struct {
	Name  string
	Tags  []string
	Items []string
}

// TestStringSlice 命名字符串切片类型
type TestStringSlice []string

// TestNamedSliceModel 命名切片测试 Model（使用命名切片类型）
type TestNamedSliceModel struct {
	Name  string
	Tags  TestStringSlice
	Items TestStringSlice
}

// TestInnerPB 内嵌结构体 PB
type TestInnerPB struct {
	Label string
	Count int32
}

// TestInnerModel 内嵌结构体 Model
type TestInnerModel struct {
	Label string
	Count int32
}

// TestNestedAutoPB 嵌套自动转换 PB
type TestNestedAutoPB struct {
	Name  string
	Inner *TestInnerPB
}

// TestNestedAutoModel 嵌套自动转换 Model
type TestNestedAutoModel struct {
	Name  string
	Inner *TestInnerModel
}

// TestTimeZeroPB 时间零值测试 PB
type TestTimeZeroPB struct {
	Name      string
	CreatedAt *timestamppb.Timestamp
	UpdatedAt *timestamppb.Timestamp
}

// TestTimeZeroModel 时间零值测试 Model
type TestTimeZeroModel struct {
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TestTimePtrPB 时间指针测试 PB
type TestTimePtrPB struct {
	Name        string
	ScheduledAt *timestamppb.Timestamp
	ReleasedAt  *timestamppb.Timestamp
}

// TestTimePtrModel 时间指针测试 Model
type TestTimePtrModel struct {
	Name        string
	ScheduledAt *time.Time
	ReleasedAt  *time.Time
}

// TestWrapperFieldPB Wrapper 字段测试 PB
type TestWrapperFieldPB struct {
	Name   string
	MinVal *wrapperspb.Int32Value
	MaxVal *wrapperspb.Int32Value
}

// TestWrapperFieldModel Wrapper 字段测试 Model
type TestWrapperFieldModel struct {
	Name   string
	MinVal *int32
	MaxVal *int32
}
