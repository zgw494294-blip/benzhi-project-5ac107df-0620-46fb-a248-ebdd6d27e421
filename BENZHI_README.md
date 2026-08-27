# BENZHI_README

基于 Go 实现的StageCaption Web 项目，一款后端服务，StageCaption 是面向剧场无障碍服务团队的字幕排演与播出锁版工作台。

## 项目说明
- 项目：benzhi-project-5ac107df-0620-46fb-a248-ebdd6d27e421
- 项目用途：StageCaption 是面向剧场无障碍服务团队的字幕排演与播出锁版工作台。
- Go 工具链：`golang:1.23.0`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/server -selfcheck -addr=127.0.0.1:19081
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-5ac107df-0620-46fb-a248-ebdd6d27e421-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-5ac107df-0620-46fb-a248-ebdd6d27e421-arm64 linux/arm64
docker run -it benzhi-project-5ac107df-0620-46fb-a248-ebdd6d27e421-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -selfcheck -addr=127.0.0.1:19081`
