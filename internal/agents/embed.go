// Package agents 负责全部 Agent 的初始化、注册与编排
package agents

import "embed"

// EmbeddedPrompts 嵌入 agents 目录下所有子目录中的 prompt.yaml 文件
// 编译时会将这些文件打包进二进制文件，运行时无需依赖外部文件路径
//
//go:embed */prompt.yaml
var EmbeddedPrompts embed.FS
