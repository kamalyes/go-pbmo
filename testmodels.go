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

	"google.golang.org/protobuf/runtime/protoimpl"
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

type TestPBUpdatesMsg struct {
	state         protoimpl.MessageState  `protogen:"open.v1"`
	Status        int32                   `protobuf:"varint,1,opt,name=status,proto3" json:"status,omitempty"`
	HostStatus    int32                   `protobuf:"varint,2,opt,name=host_status,json=hostStatus,proto3" json:"host_status,omitempty"`
	TenantId      string                  `protobuf:"bytes,3,opt,name=tenant_id,json=tenantId,proto3" json:"tenant_id,omitempty"`
	RegionCode    string                  `protobuf:"bytes,4,opt,name=region_code,json=regionCode,proto3" json:"region_code,omitempty"`
	IsProxied     *wrapperspb.BoolValue   `protobuf:"varint,5,opt,name=is_proxied,json=isProxied,proto3" json:"is_proxied,omitempty"`
	DisplayName   *wrapperspb.StringValue `protobuf:"bytes,6,opt,name=display_name,json=displayName,proto3" json:"display_name,omitempty"`
	Priority      *wrapperspb.Int32Value  `protobuf:"varint,7,opt,name=priority,json=priority,proto3" json:"priority,omitempty"`
	Score         *wrapperspb.DoubleValue `protobuf:"varint,8,opt,name=score,json=score,proto3" json:"score,omitempty"`
	Metadata      map[string]string       `protobuf:"bytes,9,rep,name=metadata,proto3" json:"metadata,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type TestPBUpdatesNoTag struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Name          string
	Count         int32
	Ignored       string `json:"-"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type TestPBUpdatesEmpty struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

type TestModelForUpdates struct {
	Name   string  `gorm:"column:name;type:varchar(255)" json:"name"`
	Status int     `gorm:"column:status;type:int" json:"status"`
	Score  float64 `gorm:"column:score;type:float" json:"score"`
	Active bool    `gorm:"column:active;type:boolean" json:"active"`
}

type TestModelJsonTag struct {
	Label  string `json:"label,omitempty"`
	Count  int    `json:"count,omitempty"`
	Secret string `json:"-"`
}

type TestModelAllTypes struct {
	Int       int     `gorm:"column:int" json:"int"`
	Int8      int8    `gorm:"column:int8" json:"int8"`
	Int16     int16   `gorm:"column:int16" json:"int16"`
	Int32     int32   `gorm:"column:int32" json:"int32"`
	Int64     int64   `gorm:"column:int64" json:"int64"`
	Uint      uint    `gorm:"column:uint" json:"uint"`
	Uint8     uint8   `gorm:"column:uint8" json:"uint8"`
	Uint16    uint16  `gorm:"column:uint16" json:"uint16"`
	Uint32    uint32  `gorm:"column:uint32" json:"uint32"`
	Uint64    uint64  `gorm:"column:uint64" json:"uint64"`
	Float32   float32 `gorm:"column:float32" json:"float32"`
	Float64   float64 `gorm:"column:float64" json:"float64"`
	Bool      bool    `gorm:"column:bool" json:"bool"`
	String    string  `gorm:"column:string" json:"string"`
	ByteSlice []byte  `gorm:"column:byte_slice" json:"byte_slice"`
	PtrInt    *int    `gorm:"column:ptr_int" json:"ptr_int"`
	PtrString *string `gorm:"column:ptr_string" json:"ptr_string"`
	PtrBool   *bool   `gorm:"column:ptr_bool" json:"ptr_bool"`
}

type TestModelUnexported struct {
	Name   string `gorm:"column:name" json:"name"`
	secret string
}
