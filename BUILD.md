# 构建说明

本仓是**框架仓**：只放接口抽象、内存实现、最小 HTTP 面与构建链路，
不包含任何产品 UI、业务规则、域名或密钥。

## 本地构建

```bash
make build        # 编译服务（内存实现，零外部依赖）
make test         # go test ./... -race
make vet && make fmt
make docker       # 容器镜像
make compose      # docker compose 起服务
```

直接用 Go 也可以：

```bash
go build -o bin/server ./cmd/server
go run ./cmd/server
```

## CI 构建链

`.github/workflows/ci.yml`（push / PR / 手动触发）：

1. `go mod tidy`
2. `gofmt -l .` 检查格式
3. `go vet ./...`
4. `go test ./... -race -count=1`
5. `go build ./...`

## 私有产品仓怎么接

产品仓持有全部业务文件（客户端 UI、业务后端实现、部署配置），通过 Go module 引用本仓：

```bash
go get github.com/jingjiangze/realtime-platform-foundation@vX.Y.Z
```

然后按依赖注入装配自己的实现：

- `chat.Provider` → 自托管 IM / SaaS 适配
- `storage.Provider` → 对象存储（客户端只拿签名 URL）
- `dataset.Provider` → 公开素材镜像 / 冷备
- 认证、限流、审计等放在产品仓，不在本仓

框架接口契约见 `internal/{chat,storage,dataset}` 包注释与 `docs/decisions/`。
