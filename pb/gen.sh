#!/bin/bash
# 生成 Agently 的 gRPC / grpc-gateway 代码及 OpenAPI 文档。
#
# 依赖（均已在开发机安装）：
#   - protoc
#   - protoc-gen-go
#   - protoc-gen-go-grpc
#   - protoc-gen-grpc-gateway
#   - protoc-gen-openapiv2   # 从 proto 自动生成 OpenAPI(swagger) 文档
#
# 用法：在项目根目录执行  bash pb/gen.sh
set -euo pipefail

# 切到项目根目录（脚本位于 pb/ 下）
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# 确保能找到 go 安装的 protoc 插件
export PATH="$PATH:$(go env GOPATH)/bin"

# 生成产物统一输出到 pb/gen，pb/proto 只存放原生 .proto
mkdir -p pb/gen

protoc \
  -I pb/api \
  -I pb/proto \
  --go_out=pb/gen --go_opt=paths=source_relative \
  --go-grpc_out=pb/gen --go-grpc_opt=paths=source_relative \
  --grpc-gateway_out=pb/gen --grpc-gateway_opt=paths=source_relative \
  --openapiv2_out=pb/gen \
  --openapiv2_opt=logtostderr=true,json_names_for_fields=false \
  pb/proto/agently.proto

echo "proto 代码生成完成 -> pb/gen/（含 agently.swagger.json OpenAPI 文档）"
