module github.com/kamalyes/go-pbmo

go 1.25.0

// 核心依赖只有 protobuf 与 go-toolbox/pkg/desensitize
//
// protobuf 是本库的主题，无法也不该去掉
//
// go-toolbox/pkg/desensitize 用于在 PB ↔ Model 转换中**单遍**脱敏：脱敏不是转换之后再跑
// 一遍反射遍历，而是编织进既有的 unsafe 偏移拷贝计划——带 desensitize tag 的
// 字段在计划构建期就解析好脱敏器，转换时顺着同一次拷贝把值写成脱敏后的结果
//
// 两条关键保证：
//   - 只写目标端，绝不回写源对象，调用方的数据不会被就地改坏
//   - 无 tag 的类型零代价：不装包装闭包，拷贝计划与未引入脱敏时逐字节相同
//
// 由 WithDesensitize 开关控制，默认关闭：不显式开启就与引入脱敏前完全一致
//
// [EN] Core dependencies are protobuf and go-toolbox/pkg/desensitize. Masking is woven into
// the existing unsafe offset-based copy plan rather than run as a second
// reflection pass: fields carrying a desensitize tag have their desensitizer
// resolved once while the plan is built, and the same copy writes the masked
// value. Two guarantees matter — only the destination is written, so the
// caller's source object is never mutated in place, and a type with no tags
// costs nothing, as no wrapper is installed and the plan is byte-for-byte what
// it was before masking existed. Controlled by WithDesensitize, off by
// default, so an unmodified caller behaves exactly as before masking existed
require google.golang.org/protobuf v1.36.11

require (
	github.com/kamalyes/go-toolbox v0.18.1
	github.com/stretchr/testify v1.12.1
)

require go.yaml.in/yaml/v3 v3.0.5 // indirect
