# go-pbmo Skill

高性能 Protocol Buffer ↔ Model 双向转换库的优化与维护技能。

## 触发条件

当用户输入 `/pbmo` 时激活此技能。

## 项目信息

- **模块路径**: `github.com/kamalyes/go-pbmo`
- **Go 版本**: 1.25+
- **核心依赖**: `github.com/kamalyes/go-toolbox`

## 核心架构

### 泛型 API（推荐，优先使用）

位于 `generic.go`，利用 `reflect.Type` 做 key + `sync.Map` 缓存：

| 函数 | 签名 | 说明 |
|------|------|------|
| `Register` | `Register[M, P]() *BidiConverter` | 注册转换对（默认配置） |
| `RegisterWith` | `RegisterWith[M, P](opts...) *BidiConverter` | 注册转换对（自定义配置） |
| `ToPB` | `ToPB[M, P](m *M) (*P, error)` | Model → PB |
| `FromPB` | `FromPB[P, M](pb *P) (*M, error)` | PB → Model |
| `ToPBs` | `ToPBs[M, P](models []*M) ([]*P, error)` | 批量 Model → PB，遇错即停 |
| `FromPBs` | `FromPBs[P, M](pbs []*P) ([]*M, error)` | 批量 PB → Model，遇错即停 |
| `SafeToPBs` | `SafeToPBs[M, P](models []*M) ([]*P, *BatchResult)` | 安全批量 Model → PB |
| `SafeFromPBs` | `SafeFromPBs[P, M](pbs []*P) ([]*M, *BatchResult)` | 安全批量 PB → Model |
| `ConverterFor` | `ConverterFor[M, P]() *BidiConverter` | 获取已注册的转换器 |

### 传统 API（已 Deprecated，保留但不再推荐）

- `BidiConverter.BatchConvertPBToModel` → 用 `FromPBs` 替代
- `BidiConverter.BatchConvertModelToPB` → 用 `ToPBs` 替代
- `BidiConverter.SafeBatchConvertPBToModel` → 用 `SafeFromPBs` 替代
- `BidiConverter.SafeBatchConvertModelToPB` → 用 `SafeToPBs` 替代
- `RegisterConverter` / `MustRegisterConverter` → 用 `Register[M, P]()` 替代
- `GetConverter` → 用 `ConverterFor[M, P]()` 替代
- `ConvertPBToModel` / `ConvertModelToPB` → 用 `FromPB` / `ToPB` 替代

### 文件结构

| 文件 | 职责 |
|------|------|
| `generic.go` | 泛型便捷函数（Register, ToPB, FromPB, ToPBs, FromPBs, SafeToPBs, SafeFromPBs, ConverterFor） |
| `converter.go` | BidiConverter 核心双向转换器 |
| `batch.go` | 批量转换（Deprecated，保留兼容） |
| `registry.go` | Registry 注册中心 + 全局便捷函数（Deprecated，保留兼容） |
| `safe.go` | SafeConverter 安全转换器 |
| `desensitize.go` | DesensitizeConverter 脱敏转换器 |
| `enum.go` | EnumMapper / GenericEnumMapper 枚举映射 |
| `transform.go` | TransformerRegistry 字段转换器 |
| `validate.go` | Validator 参数校验 |
| `time.go` | 时间转换工具 |
| `option.go` | Functional Options 选项模式 |
| `errors.go` | 类型化错误体系 |
| `helpers.go` | 反射工具函数 |
| `testmodels.go` | 测试用模型定义 |

## 编码规范

### 代码风格

- 不添加注释（除非用户明确要求）
- 遵循 Go 命名惯例
- 泛型参数命名：`M` = Model, `P` = PB
- `FromPB` 的泛型参数顺序为 `[P, M]`（先 PB 后 Model），因为调用时更直观：`FromPB[UserPB, UserModel](pb)`

### 新增功能原则

1. 优先使用泛型 API，避免 `interface{}` + reflect 的方式
2. 新增泛型函数放在 `generic.go`
3. 保持与现有 API 命名风格一致
4. 泛型批量函数返回 `[]*T` 而非 `interface{}`
5. 安全版本函数返回 `(*T, *BatchResult)` 元组

### 测试规范

- 测试文件与源文件同目录
- 泛型 API 测试放在 `generic_test.go` 和 `generic_batch_test.go`
- 测试类型定义在 `testmodels.go`
- 使用 `github.com/stretchr/testify/assert`
- 覆盖场景：正常值、零值、nil、空切片、大切片、字段映射、tag 映射

## 典型用法示例

### 基础转换

```go
pbmo.Register[UserModel, UserPB]()

pb, _ := pbmo.ToPB[UserModel, UserPB](&UserModel{ID: 1, Name: "张三"})
model, _ := pbmo.FromPB[UserPB, UserModel](&UserPB{Id: 2, Name: "李四"})
```

### 批量转换

```go
models := []*UserModel{{ID: 1, Name: "a"}, {ID: 2, Name: "b"}}
pbs, _ := pbmo.ToPBs[UserModel, UserPB](models)
```

### 安全批量转换

```go
pbs, result := pbmo.SafeToPBs[UserModel, UserPB](models)
fmt.Printf("成功: %d, 失败: %d\n", result.SuccessCount, result.FailureCount)
```

### 自定义配置

```go
pbmo.RegisterWith[UserModel, UserPB](
    pbmo.WithAutoTimeConversion(true),
    pbmo.WithFieldMapping("ID", "Id"),
    pbmo.WithDesensitize(true),
)
```

### 获取转换器进行高级操作

```go
c := pbmo.ConverterFor[UserModel, UserPB]()
c.RegisterTransformer("Name", func(v interface{}) interface{} {
    return strings.ToUpper(v.(string))
})
```

## 迁移指南

当用户需要从旧 API 迁移时，参考以下对照表：

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
