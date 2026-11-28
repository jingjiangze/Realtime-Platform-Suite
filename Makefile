# realtime-platform-foundation 构建入口
#
# 公开仓只负责「框架 + 构建链路」：编译、测试、镜像都在这里；
# 产品项目（UI / 业务 / 部署配置）放在私有仓，通过 Go module 引用本仓。

GO ?= go
BIN ?= bin/server

.PHONY: build test vet fmt docker compose clean

## 构建框架服务
build:
	$(GO) build -o $(BIN) ./cmd/server

## 全量测试（竞态检测）
test:
	$(GO) test ./... -race -count=1

## 静态检查
vet:
	$(GO) vet ./...

## 格式化（提交前自检，CI 也会校验）
fmt:
	gofmt -w .

## 构建容器镜像（通用模板，不含产品配置）
docker:
	docker build -f deploy/Dockerfile -t realtime-platform-foundation .

## 本地单机起服务（内存实现，无外部依赖）
compose:
	cd deploy && docker compose up -d

clean:
	rm -rf bin
