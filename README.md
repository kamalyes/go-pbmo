<div align="center">

# 🔄 Go-PBMO

**高性能 Protocol Buffer ↔ Model 双向转换库**

*为 gRPC/REST 网关场景精心打造的 PB-Model 转换引擎，热路径零分配，批量/并发/map 字段场景反超原生代码*

<br>

[![稳定版本](https://img.shields.io/badge/版本-v1.0.0-blue.svg)](https://github.com/kamalyes/go-pbmo)
[![许可协议](https://img.shields.io/github/license/kamalyes/go-pbmo)](LICENSE)
[![Go 版本](https://img.shields.io/github/go-mod/go-version/kamalyes/go-pbmo)](https://golang.org/)
[![Go 报告卡](https://goreportcard.com/badge/github.com/kamalyes/go-pbmo)](https://goreportcard.com/report/github.com/kamalyes/go-pbmo)
[![文档](https://pkg.go.dev/badge/github.com/kamalyes/go-pbmo.svg)](https://pkg.go.dev/github.com/kamalyes/go-pbmo)

<br>

*[📖 快速开始](#快速开始)* · *[📚 详细文档](#详细文档)* · *[📊 性能基准](#性能基准)*

</div>

---

## ✨ 特性亮点

- 🚀 **高性能转换** - 稳态热路径零分配，unsafe 零拷贝 + 反射缓存 + 类型特化三引擎，批量/并发场景反超原生代码
- 🎯 **泛型便捷函数** - `ToPB` / `FromPB` / `ToPBs` / `FromPBs` 一行搞定，类型安全无需 reflect
- 🔄 **双向转换** - PB → Model 和 Model → PB 一致性保证
- 🗺️ **字段映射** - 支持 struct tag 和手动映射两种方式
- ⚡ **字段转换器** - 自定义字段级别转换逻辑
- ⏰ **自动时间转换** - `time.Time` ↔ `*timestamppb.Timestamp` 自动处理
- ✅ **参数校验** - 字段级校验规则（必填、长度、范围、正则、自定义）
- 📦 **批量转换** - 泛型批量 `ToPBs` / `FromPBs`，安全批量 `SafeToPBs` / `SafeFromPBs` 支持值切片与指针切片自动适配
- 🗂️ **注册中心** - `Register[M, P]()` 一行注册，`ConverterFor[M, P]()` 按需获取
- 🔢 **枚举映射** - 支持 int32 和泛型枚举映射
- 🎭 **脱敏转换** - 集成 go-toolbox/desensitize
- 🛡️ **安全转换** - 集成 go-toolbox/safe 避免 nil panic
- 🧩 **更新构建器** - `NewUpdates()` 链式构建局部更新，`PBToUpdates` / `ModelToUpdates` 一行输出 GORM Updates 字典
- 🎁 **WKT 包装类型** - `wrapperspb` 包装类型与原生值/指针双向互转工具
- ⚙️ **选项模式** - Functional Options + 链式调用灵活配置

## 🏗️ 架构概览

```mermaid
graph TB
    A["🔄 go-pbmo"] --> B["🎯 泛型 API"]
    A --> C["🚀 核心转换"]
    A --> D["⚡ 扩展能力"]
    A --> E["🏗️ 基础设施"]
    A --> F["🛠️ 集成工具"]

    B --> B1["Register[M, P]<br/>一行注册"]
    B --> B2["ToPB / FromPB<br/>单条转换"]
    B --> B3["ToPBs / FromPBs<br/>批量转换"]
    B --> B4["SafeToPBs / SafeFromPBs<br/>安全批量"]
    B --> B5["PBToUpdates / ModelToUpdates<br/>更新字典直转"]

    C --> C1["📦 BidiConverter<br/>双向转换器"]
    C --> C2["📚 Batch<br/>批量转换"]
    C --> C3["🏛️ Registry<br/>注册中心"]

    D --> D1["🔧 Transformer<br/>字段转换器"]
    D --> D2["✅ Validator<br/>参数校验"]
    D --> D3["🔢 EnumMapper<br/>枚举映射"]
    D --> D4["🎭 Desensitize<br/>脱敏转换"]
    D --> D5["🛡️ SafeConverter<br/>安全转换"]
    D --> D6["🎁 Wrapper 工具<br/>WKT 包装类型互转"]

    E --> E1["❌ Errors<br/>类型化错误"]
    E --> E2["⚙️ Options<br/>选项模式"]
    E --> E3["🔍 Helpers<br/>反射工具"]
    E --> E4["⏰ Time<br/>时间转换"]

    F --> F1["syncx<br/>并发安全Map"]
    F --> F2["errorx<br/>类型化错误"]
    F --> F3["safe<br/>安全访问"]
    F --> F4["desensitize<br/>数据脱敏"]

    style A fill:#4CAF50,color:#fff
    style B fill:#E91E63,color:#fff
    style C fill:#2196F3,color:#fff
    style D fill:#FF9800,color:#fff
    style E fill:#9C27B0,color:#fff
    style F fill:#607D8B,color:#fff
```

## 🚀 快速开始

### 安装

```bash
go get github.com/kamalyes/go-pbmo
```

### 泛型 API（推荐）

泛型 API 是最简洁的使用方式，利用 `reflect.Type` 做 key + `sync.Map` 缓存，消除样板代码：

```go
package main

import (
    "fmt"
    pbmo "github.com/kamalyes/go-pbmo"
)

type UserPB struct {
    Id    uint64
    Name  string
    Email string
    Age   int32
}

type UserModel struct {
    ID    uint64 `pbmo:"Id"`
    Name  string
    Email string
    Age   int
}

func main() {
    // 一行注册（自动缓存，无需重复调用）
    pbmo.Register[UserModel, UserPB]()

    // 单条转换
    pb, _ := pbmo.ToPB[UserModel, UserPB](&UserModel{ID: 1, Name: "张三", Age: 25})
    model, _ := pbmo.FromPB[UserPB, UserModel](&UserPB{Id: 2, Name: "李四", Age: 30})

    // 批量转换
    models := []*UserModel{{ID: 1, Name: "a"}, {ID: 2, Name: "b"}}
    pbs, _ := pbmo.ToPBs[UserModel, UserPB](models)

    // 安全批量转换（不因单个失败中断，支持 []M 值切片与 []*M 指针切片）
    pbs2, result := pbmo.SafeToPBs[UserModel, UserPB](models)
    fmt.Printf("成功: %d, 失败: %d\n", result.SuccessCount, result.FailureCount)
}
```

### 传统 API（仍可用）

> ⚠️ 传统 API 已标记为 **Deprecated**，建议迁移到泛型 API

```go
// 手动创建转换器
converter := pbmo.NewBidiConverter(UserPB{}, UserModel{})

// PB -> Model
var model UserModel
converter.ConvertPBToModel(pb, &model)

// Model -> PB
var pb2 UserPB
converter.ConvertModelToPB(model2, &pb2)

// 批量转换（已废弃，请用 ToPBs / FromPBs）
var models []UserModel
converter.BatchConvertPBToModel(pbs, &models) // Deprecated
```

### 带选项配置

```go
// 泛型方式
pbmo.RegisterWith[UserModel, UserPB](
    pbmo.WithAutoTimeConversion(true),
    pbmo.WithValidation(true),
    pbmo.WithFieldMapping("ID", "Id"),
    pbmo.WithDesensitize(true),
)

// 传统方式
converter := pbmo.NewBidiConverter(PB{}, Model{},
    pbmo.WithAutoTimeConversion(true),
    pbmo.WithValidation(true),
)
```

### 链式调用

```go
converter := pbmo.NewBidiConverter(UserPB{}, UserModel{}).
    WithAutoTimeConversion(true).
    WithValidation(true).
    WithFieldMapping("ID", "Id").
    WithDesensitize(true)
```

### 更新字段构建器

`NewUpdates()` 用于构建 `map[string]interface{}`，适合传给 GORM `Updates` 或仓储层局部更新方法。JSON 数组字段请使用 `SetJSONSlice`，避免把 Go slice 作为标量值写入 JSON 列。

```go
updates := pbmo.NewUpdates().
    SetStringVal("url", req.Url).
    SetInt32Val("sort_order", wrapperspb.Int32(req.SortOrder)).
    SetJSONSlice("display_pages", req.DisplayPages).
    Build()
```

`SetJSONSlice` 仅处理 slice/array，nil slice 会被跳过，非 nil 空切片会写入 `[]`。

### 更新字典直转

`PBToUpdates` / `ModelToUpdates` 一行将消息或结构体转为更新字典，自动跳过零值，适合直接传给 GORM `Updates`。字段 key 与偏移按类型缓存于更新计划，热路径零 tag 解析：

```go
// PB 按 protobuf json tag 名（snake_case）输出，自动解包 wrapper 类型
updates := pbmo.PBToUpdates(req)

// Model 按 gorm column tag 或 json tag 输出，跳过零值与不可导出字段
updates := pbmo.ModelToUpdates(&user)

db.Model(&User{}).Where("id = ?", 1).Updates(updates)
```

## 📋 泛型 API 速查表

| 函数 | 签名 | 说明 |
|------|------|------|
| `Register` | `Register[M, P]() *BidiConverter` | 注册转换对（默认配置） |
| `RegisterWith` | `RegisterWith[M, P](opts...) *BidiConverter` | 注册转换对（自定义配置） |
| `ToPB` | `ToPB[M, P](m *M) (*P, error)` | Model → PB |
| `FromPB` | `FromPB[P, M](pb *P) (*M, error)` | PB → Model |
| `ToPBs` | `ToPBs[M, P](models []*M) ([]*P, error)` | 批量 Model → PB，遇错即停 |
| `FromPBs` | `FromPBs[P, M](pbs []*P) ([]*M, error)` | 批量 PB → Model，遇错即停 |
| `SafeToPBs` | `SafeToPBs[M, P](models []M) ([]*P, *BatchResult)` | 安全批量 Model → PB，支持 `[]M` 与 `[]*M` 自动适配 |
| `SafeFromPBs` | `SafeFromPBs[P, M](pbs []*P) ([]*M, *BatchResult)` | 安全批量 PB → Model |
| `ConverterFor` | `ConverterFor[M, P]() *BidiConverter` | 获取已注册的转换器 |
| `PBToUpdates` | `PBToUpdates[P](pb *P) map[string]interface{}` | PB → 更新字典，protobuf json tag 作 key，跳过零值并解包 wrapper |
| `ModelToUpdates` | `ModelToUpdates[M](m *M) map[string]interface{}` | Model → 更新字典，gorm column/json tag 作 key，跳过零值 |

## 🔄 迁移指南

| 旧 API（Deprecated） | 新 API（推荐） |
|----------------------|---------------|
| `NewBidiConverter(PB{}, Model{})` + `RegisterConverter(c)` | `Register[Model, PB]()` |
| `converter.ConvertModelToPB(m, &pb)` | `ToPB[Model, PB](m)` |
| `converter.ConvertPBToModel(pb, &model)` | `FromPB[PB, Model](pb)` |
| `converter.BatchConvertModelToPB(models, &pbs)` | `ToPBs[Model, PB](models)` |
| `converter.BatchConvertPBToModel(pbs, &models)` | `FromPBs[PB, Model](pbs)` |
| `converter.SafeBatchConvertModelToPB(models, &pbs)` | `SafeToPBs[Model, PB](models)` |
| `converter.SafeBatchConvertPBToModel(pbs, &models)` | `SafeFromPBs[PB, Model](pbs)` |
| `GetConverter(pbType, modelType)` | `ConverterFor[Model, PB]()` |
| `ConvertPBToModel(pb, &model)` | `FromPB[PB, Model](pb)` |
| `ConvertModelToPB(model, &pb)` | `ToPB[Model, PB](model)` |

## 🧰 模块一览

| 模块 | 文件 | 功能描述 | 使用场景 |
|------|------|----------|----------|
| 🎯 泛型 API | [generic.go](generic.go) | 一行注册、一行转换 | **推荐首选** |
| 🔄 核心转换 | [converter.go](converter.go) | BidiConverter 双向转换 | PB ↔ Model 转换 |
| 📦 批量转换 | [batch.go](batch.go) | 批量转换、安全批量 | 列表数据转换 |
| 🗂️ 注册中心 | [registry.go](registry.go) | 转换器统一管理 | 多类型转换场景 |
| ⚡ 字段转换 | [transform.go](transform.go) | 字段级自定义转换 | 数据格式化、类型适配 |
| ✅ 参数校验 | [validate.go](validate.go) | 字段校验规则 | 数据验证 |
| 🔢 枚举映射 | [enum.go](enum.go) | int32/泛型枚举映射 | 枚举类型转换 |
| 🎭 脱敏转换 | [desensitize.go](desensitize.go) | 数据脱敏 | 隐私保护 |
| 🛡️ 安全转换 | [safe.go](safe.go) | nil 安全转换 | 防止 panic |
| ⏰ 时间转换 | [time.go](time.go) | Time ↔ Timestamp | 时间字段处理 |
| 🧩 更新构建器 | [updates.go](updates.go) | 链式构建局部更新字段 | GORM Updates、JSON 字段更新 |
| 📋 更新计划 | [updates_plan.go](updates_plan.go) | 更新字典字段计划缓存 | PBToUpdates / ModelToUpdates 热路径 |
| 🎁 包装类型 | [wrappers.go](wrappers.go) | WKT 包装类型互转工具 | wrapperspb ↔ 原生值/指针 |
| 🔧 辅助函数 | [helpers.go](helpers.go) | 反射工具、类型判断 | 内部使用 |
| ❌ 错误定义 | [errors.go](errors.go) | 类型化错误体系 | 错误处理 |
| ⚙️ 选项模式 | [option.go](option.go) | Functional Options | 配置管理 |

## 📊 性能基准

基于 `go test -bench=. -benchmem -benchtime=3s` 实测，与手写原生转换代码对比，完整数据见 [go-pbmo-benchmark](https://github.com/kamalyes/go-pbmo-benchmark)：

> **单位说明**: `ns` = 纳秒（10⁻⁹秒），`µs` = 微秒（10⁻⁶秒，1µs = 1000ns）；对比列 `PBMO/原生`，**< 1 表示 PBMO 反超**
> **测试环境**: Windows AMD64 · Go 1.25 · Intel i5-9300H @ 2.40GHz · 8 逻辑核

### 核心转换（PBMO vs 原生）

| 模型（结构特征） | 场景 | PBMO | 原生 | 对比 | 内存 | 分配 |
|------|------|------|------|------|------|------|
| Simple（8 字段平铺） | PB→Model | 434.2 ns | 155.3 ns | 2.80x | 48 B | 1 |
| Simple（8 字段平铺） | Model→PB | 775.1 ns | 259.3 ns | 2.99x | 48 B | 1 |
| Mapped（字段映射） | PB→Model | 177.9 ns | 134.5 ns | 1.32x | 48 B | 1 |
| Mapped（字段映射） | Model→PB | 178.9 ns | 135.7 ns | 1.32x | 48 B | 1 |
| Medium（含切片/时间） | PB→Model | **213.8 ns** | 339.1 ns | **0.63x** | 96 B | 1 |
| Full（多类型混合） | PB→Model | 798.1 ns | 585.4 ns | 1.36x | 184 B | 4 |
| Large（大结构） | PB→Model | 1426 ns | 1177 ns | 1.21x | 296 B | 3 |
| Huge30（30 字段） | PB→Model | 814.8 ns | 514.8 ns | 1.58x | 448 B | 1 |
| Huge100（100 字段） | PB→Model | 5205 ns | 3975 ns | 1.31x | 1328 B | 1 |
| DeepNested4（4 层嵌套） | PB→Model | 528.4 ns | 301.2 ns | 1.75x | 224 B | 3 |
| Enterprise（多层聚合） | PB→Model | 5784 ns | 1510 ns | 3.83x | 368 B | 1 |

### 类型特征场景（PBMO vs 原生）

| 场景 | PBMO | 原生 | 对比 | 说明 |
|------|------|------|------|------|
| Config Model→PB | **260.0 ns** | 3597 ns | **0.07x** | map 字段转换 PBMO 快 13.8x |
| Config PB→Model | 323.4 ns | 226.8 ns | 1.43x | map[string]any 写入 |
| ScoreBoard Model→PB | 713.0 ns | 386.6 ns | 1.84x | 计分板切片聚合 |
| Wrapper Model→PB | **857.9 ns** | 1161 ns | **0.74x** | WKT 包装类型 PBMO 快 1.4x |
| TimePtr Model→PB | **246.9 ns** | 251.8 ns | **0.98x** | *time.Time 与原生持平 |
| TimeZero PB→Model | 224.3 ns | 120.0 ns | 1.87x | 零值时间自动归零 |
| NamedSlice PB→Model | 162.6 ns | 133.1 ns | 1.22x | 类型别名切片 |
| Metadata PB→Model | 126.7 ns | 74.1 ns | 1.71x | 极小结构（24 B/1 alloc） |

### 更新字典（PBMO vs 原生）

| 场景 | PBMO | 原生 | 对比 | 内存 | 分配 |
|------|------|------|------|------|------|
| PBToUpdates | 1.87 µs | 973 ns | 1.92x | 744 B | 8 |
| ModelToUpdates | 1.47 µs | 1.06 µs | 1.39x | 840 B | 11 |
| UpdatesBuilder 链式构建 | **2.26 µs** | 3.34 µs | **0.68x** | 480 B | 11 |

> 💡 更新字典直转按类型缓存字段计划，热路径零 tag 解析、零 reflect.Value 构造；wrapper 解包与 nil 指针跳过按字段种类特化为直读比较

### 注册中心（PBMO vs 原生）

| 场景 | PBMO | 原生 | 对比 | 内存 | 分配 |
|------|------|------|------|------|------|
| Register 热路径 | **21.2 ns** | - | - | 0 B | 0 |
| ConverterFor | **23.0 ns** | - | - | 0 B | 0 |
| 批量注册 6 类型对轮换 | **151 ns** | 553 ns | **0.27x** | 0 B | 0 |
| Registry.Lookup | 74.1 ns | 35.1 ns | 2.11x | 0 B | 0 |

> 💡 Register/ConverterFor 命中 32 槽热路径缓存零分配，黄金比例散列保证多类型对不互相驱逐；批量注册场景比手写 map 反超 3.7x

### 并发与批量转换（PBMO vs 原生）

| 场景 | PBMO | 原生 | 对比 |
|------|------|------|------|
| Simple 并发 PB→Model | 125.0 ns | 78.5 ns | 1.59x |
| Wrapper 并发 PB→Model | **440.0 ns** | 4111 ns | **0.11x** |
| Outer 并发 PB→Model | **298.3 ns** | 543.9 ns | **0.55x** |
| DeepNested4 并发 PB→Model | 355.9 ns | 294.5 ns | 1.21x |
| Simple 批量 100 条 PB→Model | **13.9 µs** | 27.6 µs | **0.50x** |
| Simple 批量 1000 条 PB→Model | **211 µs** | 229 µs | **0.92x** |
| Simple BatchConvert 100 条 | **8.2 µs** | 9.5 µs | **0.86x** |
| DeepNested4 批量 100 条 | 44.4 µs | 32.3 µs | 1.38x |

> 💡 批量转换采用预分配 + unsafe 批量拷贝，100 条批量分配 2 次而原生需 101 次；并发场景 Wrapper 反超 9x（原生 Protobuf 反射有锁竞争）

### 内部组件

| 场景 | 耗时 | 内存 | 分配 |
|------|------|------|------|
| EnumMapper.Map | 73.7 ns | 0 B | 0 |
| GenericEnumMapper.Map | 74.3 ns | 0 B | 0 |
| AutoEnumConverter.Convert | 49.8 ns | 0 B | 0 |
| TransformerRegistry.Apply | 439.4 ns | 48 B | 3 |
| NewBidiConverter（构建期一次） | 728.6 ns | 256 B | 2 |

> 💡 平铺字段单对象场景原生手写赋值更快属物理极限（零反射零间接调用）；PBMO 的价值在批量/并发/map 字段/字段映射/更新字典场景的反超与全场景零样板代码

## 🧪 测试

```bash
# 运行单元测试
go test ./...

# 运行指定模块测试
go test -run TestConverter -v
```

## 📚 详细文档

| 编号 | 文档 | 说明 |
|------|------|------|
| 01 | [快速开始](docs/01.快速开始.md) | 安装、泛型 API、核心 API |
| 02 | [字段映射](docs/02.字段映射.md) | struct tag、手动映射、优先级 |
| 03 | [字段转换器](docs/03.字段转换器.md) | TransformerFunc、注册、常见场景 |
| 04 | [参数校验](docs/04.参数校验.md) | FieldRule、ValidationErrors、中间件集成 |
| 05 | [批量转换](docs/05.批量转换.md) | 泛型批量、安全批量、BatchResult |
| 06 | [注册中心](docs/06.注册中心.md) | Registry、泛型注册、多类型管理 |
| 07 | [枚举映射](docs/07.枚举映射.md) | EnumMapper、GenericEnumMapper、AutoEnumConverter |
| 08 | [脱敏转换](docs/08.脱敏转换.md) | DesensitizeConverter、脱敏 Tag、自定义脱敏器 |
| 09 | [安全转换](docs/09.安全转换.md) | SafeConverter、安全字段访问、SafeAccess |
| 10 | [时间转换](docs/10.时间转换.md) | 自动时间转换、手动转换函数 |
| 11 | [选项模式](docs/11.选项模式.md) | Functional Options、链式调用、常见组合 |
| 12 | [错误处理](docs/12.错误处理.md) | 类型化错误体系、错误判断、最佳实践 |

## 📄 许可证

Copyright (c) 2026 kamalyes. All Rights Reserved.
