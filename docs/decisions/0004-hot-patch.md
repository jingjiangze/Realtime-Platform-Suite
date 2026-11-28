# 0004 · 热补丁（真增量）机制

- 状态：已采纳（2026-11-28）
- 相关：`/v1/app/version`、`internal/config`、客户端 SDK 契约

## 背景与目标

用户明确要求：**「热更新就是热补丁——能安装新包的那种，或者一个小补丁重载」**。

这不是「WebView 加载远端页面」那种伪热更（改站点即生效，但无法改原生能力），而是：

1. **整包更新**：原生能力变化时换新 APK/IPA（应用商店或直链）；
2. **增量补丁**：只下发与当前版本差分的部分，客户端校验后重启重载，**不重新安装**。

两者都要有，且由同一个接口驱动。

## 接口契约

```
GET /v1/app/version?platform=android|ios|mp&appVersion=<当前版本>&userId=<可选>
```

响应字段：

| 字段 | 含义 |
|-|-|
| `platform` | 回显平台，便于排查串号 |
| `latest` / `minSupported` | 该平台的最新版与最低支持版 |
| `update` | 是否有新版（`appVersion < latest`） |
| `force` | 是否强制整包（`appVersion < minSupported`，此时不返回 patch） |
| `inRollout` | 灰度命中与否（`rollout < 100` 时返回） |
| `patch` | 增量补丁，**仅有新版且非强制且灰度命中时返回** |

`patch` 结构：

```json
{
  "from": "0.1.1",
  "to": "0.1.2",
  "url": "https://cdn.example.com/p-011-012.zip",
  "sha256": "…",
  "size": 184320,
  "signature": "…",
  "kind": "delta"   // delta=版本差分；full=通用补丁包
}
```

### 客户端决策顺序

```
1. force == true            → 强制整包（弹不可取消对话框 → 下载安装）
2. update == false          → 无需动作
3. inRollout == false       → 静默等待（不下补丁，避免灰度外用户被提前升级）
4. patch.kind == "delta"    → 下载增量 → 校验 sha256/signature → 应用 → 重启重载
5. patch.kind == "full"     → 下载全量补丁包 → 校验 → 应用 → 重启重载
6. 无 patch                 → 整包更新（引导下载，非强制）
```

### 版本匹配规则

服务端在 `Patches` 清单里找 `From == appVersion` 的条目；多条命中取 `To` 最大的那条。
客户端可链式升级（0.1.0 → 0.1.1 → 0.1.2 分两次拉），也可只提供跳版补丁（0.1.0 → 0.1.3 一次到位）。
**没有匹配补丁时不下发 patch，客户端走整包**——宁可慢，不下坏包。

## 配置方式

按平台独立，互不污染（历史 bug：iOS/小程序曾拿到 Android 的强制更新策略）。

```bash
# 通用兜底（所有平台默认继承）
APP_LATEST=0.1.1
APP_MIN_SUPPORTED=0.1.0

# 平台覆盖（键名大写）
APP_ANDROID_LATEST=0.1.3
APP_ANDROID_MIN_SUPPORTED=0.1.0
APP_ANDROID_ROLLOUT=30
APP_IOS_LATEST=1.0.0
APP_MP_LATEST=1.0.0

# 差分补丁清单：两种写法
# 1) 简写：from>to>url[,from>to>url[>sha256]]
APP_ANDROID_PATCHES=0.1.1>0.1.2>https://cdn/p1.zip,0.1.2>0.1.3>https://cdn/p2.zip
# 2) JSON（可带 size / signature）
APP_IOS_PATCHES=[{"from":"1.0.0","to":"1.0.1","url":"https://cdn/i.zip","sha256":"ff00","size":1024}]
```

坏配置（JSON 不合法、字段缺失）**静默丢弃并回落到整包**，不会让服务起不来，也不会下发坏补丁。

## 灰度

`rollout ∈ [0,100]`，按 `fnv32a(userId) % 100 < rollout` 分桶：

- 稳定：同一 userId 永远同桶（有测试保证）；
- 均匀：10000 个 id 落在 100 个桶，分布均匀（有测试保证）；
- **不传 userId 视为灰度外**（不下补丁）——避免匿名请求随机命中导致灰度失效。

实测：`rollout=30` 时 20 个 userId 命中 8 个。

## 各端落地差异（重要）

| 端 | 能否真热补丁 | 机制 |
|-|-|-|
| **Android 原生** | 能 | 补丁包替换 dex/资源 + 重启进程（Tinker 类方案）或 Flutter 的 Shorebird |
| **iOS 原生** | **不能下发可执行代码**（App Store 审核规则 3.3.2/2.5.2） | 只能 L1 远程配置 + JSON 驱动 UI + H5（WKWebView 内解释执行） |
| **微信小程序** | 能（官方机制） | 代码包发布即替换，可分阶段发布；另有 wgt 类方案（uni-app） |
| **WebView 壳** | 能，但只是换 H5 | 站点更新即生效，**改不了原生能力** |

**结论**：`patch` 通道对 Android 原生与小程序是「真补丁」；对 iOS 必须降级为「配置 + H5 资源」，
不能承诺 Dart/ObjC 代码热更。这条差异必须在产品沟通里说清，不能对 iOS 用户承诺同样的热更能力。

## 验证记录（2026-11-28）

| 场景 | 请求 | 实际响应 |
|-|-|-|
| 有差分补丁 | `appVersion=0.1.1`（清单含 0.1.1→0.1.2） | `patch.kind=delta, from=0.1.1, to=0.1.2` ✓ |
| 无匹配补丁 | `appVersion=0.1.0` | 无 `patch`，`update=true` → 走整包 ✓ |
| 已是最新 | `appVersion=0.1.3` | `update=false` ✓ |
| 强制更新 | `appVersion < minSupported` | `force=true` 且无 `patch` ✓ |
| 平台隔离 | `platform=ios` / `mp` | 各自版本，不受 Android 影响 ✓ |
| 灰度命中率 | `rollout=30`，20 个 userId | 8/20 命中 ✓ |
| 灰度稳定性 | 同一 id 重复 100 次 | 桶号不变 ✓ |

## 未做（PENDING）

- **补丁生成工具链**：目前服务端只负责「下发清单」，差分包如何生成（bsdiff / 目录对比 / Flutter asset 差量）尚未实现，取决于客户端选型。
- **补丁签名**：`signature` 字段已预留，客户端校验逻辑与服务端签名工具未做。
- **回滚**：客户端保留上一份可用版本 + 加载失败自动回退，属客户端职责，未实现。
- **iOS 的 L1 配置通道**：`GET /v1/app/config` 尚未实现（PLAN-v3 里已规划）。
