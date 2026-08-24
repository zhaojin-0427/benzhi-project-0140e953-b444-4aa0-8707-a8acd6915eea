# BENZHI_README

基于 Go 实现的specimen-custody-gate HTTP API 项目，一款后端服务，已完整实现自然保护地科研样本从采集建批、许可与封签核验、顺序交接、到站差异整改到冻结清单并签发可验证入库凭证的监管 HTTP 服务。

## 项目说明
- 项目：benzhi-project-0140e953-b444-4aa0-8707-a8acd6915eea
- 项目用途：已完整实现自然保护地科研样本从采集建批、许可与封签核验、顺序交接、到站差异整改到冻结清单并签发可验证入库凭证的监管 HTTP 服务。
- Go 工具链：`golang:1.22`
- 前端工具链：无

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/server -selfcheck -addr=127.0.0.1:19081
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-0140e953-b444-4aa0-8707-a8acd6915eea-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-0140e953-b444-4aa0-8707-a8acd6915eea-arm64 linux/arm64
docker run -it benzhi-project-0140e953-b444-4aa0-8707-a8acd6915eea-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -selfcheck -addr=127.0.0.1:19081`
