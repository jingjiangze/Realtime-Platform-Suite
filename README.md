# realtime-platform-foundation

一个**与产品无关**的实时业务框架：即时消息、对象存储、数据集镜像三层都通过接口抽象，
业务项目通过 Go module 引用它，框架本身不含产品名、品牌素材、业务规则或任何密钥。

## 为什么有这个仓

把「IM 底座 + 文件存储 + 数据集镜像」做成可替换的接口，是为了让业务项目在任何一家服务商
变更时只换适配器、不改业务代码。这个判断来自 2026-11 的一次技术审计，结论写在
`docs/decisions/`：

- `0001-storage-not-dataset.md` — 用户文件必须走对象存储，数据集只能做公开素材镜像/冷备；
- `0002-im-provider.md` — IM 通过 `chat.Provider` 抽象，首选自托管，不绑定 SaaS；
- `0004-hot-patch.md` — 客户端版本通道与增量补丁的接口契约（只给协议，不给实现）。

## 目录

```
cmd/server            入口（装配 Provider，默认内存实现）
internal/chat         IM 抽象 + 内存实现（测试/本地）
internal/storage      对象存储抽象（预签名、分片、续传会话）
internal/dataset      数据集抽象（只做镜像/冷备，禁止存用户文件）
internal/httpapi      最小 HTTP 面（healthz + 演示端点 + 版本通道）
internal/config       环境变量配置
examples/web          最小前端示例（原生 JS，无构建步骤）
deploy/               Dockerfile + docker compose 模板
docs/decisions/       架构决策记录（ADR）
```

## 快速开始

```bash
# 只跑 API（内存 IM，无外部依赖）
go run ./cmd/server
# 或用构建入口：make build / make test / make docker
```

构建链路、CI 步骤与私有产品仓接入方式见 [`BUILD.md`](BUILD.md)。

curl -s localhost:8080/healthz
# {"status":"ok","version":"dev"}

curl -s -X POST localhost:8080/v1/demo/chat/send \
  -H 'content-type: application/json' \
  -d '{"convId":"c1","senderId":"u1","text":"hello","clientMsgId":"m1"}'

# 前端示例
cd examples/web && python -m http.server 5173
```

## 接口约定

`chat.Provider` 要求实现方保证：

1. `SendText` / `SendImage` 幂等 —— 同一 `clientMsgID` 重复调用不产生重复消息；
2. `GetHistory` 的 `Seq` 单调递增，游标续页不丢消息；
3. 订阅断开时不得静默丢消息（至少返回错误）。

`storage.Provider` 要求：

1. 客户端只拿签名 URL，永不接触云端凭据；
2. 私有对象必须签名访问；
3. 分片上传的续传状态外置（多数云 SDK 不提供 checkpoint）。

`dataset.Provider` 只提供镜像/冷备能力，实现方应把批量写合并成一次提交。

## 版本通道

`GET /v1/app/version?platform=android|ios|web&appVersion=<当前版本>` 返回该平台的最新版与
最低支持版；低于最低支持版时 `force=true`。配置按平台独立（`APP_<PLATFORM>_LATEST` /
`APP_<PLATFORM>_MIN_SUPPORTED`），缺项回落到通用 `APP_*`。增量补丁的完整契约见 ADR-0004。

## 边界

- 本仓只提供接口与内存实现，**不含**生产适配器（自托管 IM、对象存储 SDK、数据集客户端），
  也不含认证、业务规则、部署密钥；
- 使用方在自己的私有仓库实现 `chat.Provider` / `storage.Provider` / `dataset.Provider`，
  通过依赖注入装配，不修改本仓。

## 许可

代码以 **GPL-3.0-or-later** 发布，全文见 `LICENSE`。
本仓不含任何第三方美术素材；若将来引入，须在 `NOTICE` 中单独划界。