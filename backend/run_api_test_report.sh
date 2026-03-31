#!/usr/bin/env sh
# 运行 cmd/server 的 API 测试并生成带时间戳的 Markdown 报告（见 test-results/）。
set -eu
cd "$(dirname "$0")"
go test -count=1 ./...
go run ./cmd/testreport
