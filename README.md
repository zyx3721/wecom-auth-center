<div align="center">

<h1>wecom-auth-center</h1>

<p><b>企业微信统一认证中心</b> — 一个企微应用 · N 个业务域名的扫码登录</p>

<p><b>简体中文</b> · <a href="README.en.md">English</a></p>

企业微信自建应用的可信域名与授权回调域配不了几个，业务系统却有一堆域名。
wecom-auth-center 把企业微信 OAuth 收口到一个回调域名：各业务系统带着 `app` 标识跳进来，
扫码完成后凭一次性 `ticket` 跳回去，各自建立自己的会话。
**企微 `secret` 只存在这一处；新增业务系统不用再去企微后台改任何配置。**

<p>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white&labelColor=1f2937" alt="Go 1.26+"></a>
  <img src="https://img.shields.io/badge/state%20%2F%20ticket-%E4%B8%80%E6%AC%A1%E6%80%A7-059669?labelColor=1f2937" alt="一次性 state/ticket">
  <img src="https://img.shields.io/badge/%E9%83%A8%E7%BD%B2-%E5%8D%95%E4%BA%8C%E8%BF%9B%E5%88%B6-1f2937?labelColor=1f2937" alt="单二进制部署">
  <img src="https://img.shields.io/badge/%E5%A4%96%E9%83%A8%E4%BE%9D%E8%B5%96-%E4%BB%85%20yaml.v3-3B82F6?labelColor=1f2937" alt="外部依赖仅 yaml.v3">
</p>

<p>
  <b><a href="#解决什么问题">解决什么问题</a></b> ·
  <a href="#怎么工作">怎么工作</a> ·
  <a href="#技术栈">技术栈</a> ·
  <a href="#快速开始">快速开始</a> ·
  <a href="#部署">部署</a> ·
  <a href="#http-接口">接口</a> ·
  <a href="#数据与安全">安全</a> ·
  <a href="#常见问题">常见问题</a> ·
  <a href="#项目结构">项目结构</a> ·
  <a href="#文档">文档</a>
</p>

</div>

---

多个内部系统（OA、IT 工单、CMDB……）都要「企业微信扫码登录」，但企微后台每应用的授权回调域只能按域名白名单逐个配置，还要域名归属校验。
wecom-auth-center 采用业界成熟的 CAS 式统一认证模式：企业微信只信任 `auth.example.com` 一个域名，
由本服务统一接收 OAuth 回调、用 `state` 反查来源系统、用一次性 `ticket` 把登录结果转发给各业务系统。
Go 编写，无外部数据库，`go build` 出单文件即可部署；内置 mock 模式，不连企业微信也能本地走通全流程。

## 解决什么问题

- **多域名统一回调** — 企业微信后台只配 `auth.example.com` 一个回调域名，业务系统数量不再受企微可信域名额度约束。
- **业务系统零企微配置** — 新系统接入只需在认证中心白名单加一行（`app` 标识 + 域名 + `app_secret`），企微后台从此不动。
- **`secret` 单点保管** — 企微 `corpid`/`secret` 只存在认证中心配置文件；业务系统只拿一个用于 verify 签名的独立密钥，泄露影响面小。
- **一次性凭证防重放** — `state`（5 分钟）与 `ticket`（60 秒）均为 128 位随机值、取出即删；verify 校验 HMAC-SHA256 签名与时间偏差，恒定时间比较。
- **防 Open Redirect** — `app` → 域名映射只存在于服务端配置，请求参数永远不接受完整 URL，`redirect` 仅允许站内相对路径。
- **开箱可演练** — `wecom.mock: true` 时用本地模拟扫码页替代真实企微，浏览器点两下即可观察完整登录链路。
- **生产就绪中间件** — 按 IP 限流、请求日志、panic 恢复、健康检查、优雅退出，均为内置。

**它不是** IAM / 用户管理系统：不存密码、不管权限、不维护组织架构。企业微信回答「这个人是谁」，本服务回答「这次登录来自哪个系统」，各业务系统自己回答「这个人在我这里有什权限」。

同一件事，两种做法对比：

| 场景 | 各系统直连企微 | 统一认证中心 |
| --- | --- | --- |
| 新增业务系统 | 企微后台改域名配置 + 域名校验 | 认证中心白名单加一行 |
| 回调域数量 | 受企微每应用可信域名额度约束 | 无限制（企微只认识 auth 一个域名） |
| 企微 `secret` 分发 | 每个系统各持一份 | 只在认证中心，单点保管 |
| 域名归属校验 | 每个域名都要放 `WW_verify` 文件 | 只有 auth 域名需要 |
| 登录逻辑 | 各系统各写一遍 OAuth | 各系统只写一个 ticket 校验路由 |

## 怎么工作

```text
   oa.example.com      it.example.com      cmdb.example.com
        │                    │                    │
        │ ① 302 /login?app=oa（未登录时跳转）      │
        └──────────┬─────────┴────────────────────┘
                   ▼
      ┌──────────────────────────┐  ② 302 扫码页  ┌────────────┐
      │   auth.example.com       │ ────────────▶ │  企业微信   │
      │   wecom-auth-center      │ ◀──────────── │  用户扫码   │
      └───────────┬──────────────┘  ③ 回调 code   └────────────┘
                  │ ④ code 换 userid，发一次性 ticket
                  │ ⑤ 302 /sso/login?ticket=xxx（只回白名单域名）
                  ▼
   业务系统后端 POST /api/verify 换 userid → 建立自己的会话
```

- **`state` 定向** — `/login` 生成随机 state 并登记来源 `app`，企微回调原样带回；取出即删，天然防 CSRF 与重放。
- **`ticket` 交接** — 认证中心用 code 从企微换出 `userid` 后签发 60 秒一次性 ticket，302 带回业务系统。
- **`verify` 收口** — 业务系统后端持 `app_secret` 签名调用 `/api/verify` 换取身份，凭证用后即焚；浏览器不接触 verify。
- **会话归属** — 认证中心不派发全局会话；每个业务系统验证 ticket 后建立自己的 Session/JWT，登出互不影响。
- **数据放哪** — state/ticket 均为秒级短时效数据，默认存进程内存；多实例部署配置 `store.driver: redis` 切换共享存储，安全事件可开启独立审计文件留痕。

## 技术栈

| 层 | 选型 |
| --- | --- |
| 语言 | Go 1.26+（仅标准库 `net/http`，Go 1.22+ 方法路由） |
| 外部依赖 | [gopkg.in/yaml.v3](https://github.com/go-yaml/yaml)（唯一第三方依赖） |
| 授权入口 | 企业微信 wwlogin 扫码（PC）/ 网页授权 `snsapi_base`（企微内 H5），配置切换 |
| 存储 | 进程内存（默认）或 Redis（`SET EX`+`GETDEL`），配置切换，多实例部署选 Redis |
| 部署形态 | 单二进制 + systemd，或多阶段 Docker 镜像 |

## 快速开始

本地演练只需要 **Go 1.26+**，不需要企业微信账号：

```bash
git clone <仓库地址> wecom-auth-center && cd wecom-auth-center/server
cp config.example.yaml config.yaml
```

把 `config.yaml` 里的 `wecom.mock` 改为 `true`（mock 模式不校验 corpid/secret），然后：

```bash
go run ./cmd/server -config config.yaml
```

浏览器打开 `http://127.0.0.1:8700/login?app=oa`，进入模拟扫码页点击「模拟扫码成功」，
即可看到 302 跳回 `https://oa.example.com/sso/login?ticket=...` 的完整链路；
用 `docs/client-integration.md` 中的签名算法对 ticket 调 `POST /api/verify` 可换取 `{"userid":"mockuser"}`。

跑测试：

```bash
go test ./...
```

## 部署

只保留两种方式：**Docker 部署**（推荐）与 **Release 二进制部署**。两者都需要先在企业微信后台完成域名与应用配置，清单见 [docs/wecom-setup.md](docs/wecom-setup.md)；配置文件全部字段的说明见 [docs/manual.md](docs/manual.md)。

### 方式一：Docker 部署

镜像由 GitHub CI 在发版时自动构建并同步推送到**阿里云镜像仓库**与 **Docker Hub**（多架构 amd64/arm64，也可在阿里云容器镜像服务控制台手动构建），镜像内只含二进制与静态资源，配置文件从宿主机挂载。无需拉取仓库，一条命令即可拉起：

```bash
docker run -d \
  --name wecom-auth-center \
  --restart unless-stopped \
  -p 127.0.0.1:8700:8700 \
  -v /opt/wecom-auth-center/config.yaml:/app/config.yaml:ro \
  registry.cn-shenzhen.aliyuncs.com/zyx3721/wecom-auth-center:latest
```

> Windows PowerShell 下把 `-v` 的源路径改写为 `${PWD}/config.yaml` 形式。

启动前先在宿主机准备 `config.yaml`（从 `server/config.example.yaml` 复制修改，`wecom.mock` 置为 `false`，填入 corpid/secret 与业务系统白名单；文件内含 `secret`，权限建议 600）。每个字段说明见 [docs/manual.md](docs/manual.md) 第五章。

> 可选增强：多实例部署时将 `store.driver` 改为 `redis`，进行中的登录流程跨实例共享；需要安全审计时开启 `audit.enabled`，Docker 下审计文件必须落在挂载的可写目录（如追加 `-v /opt/wecom-auth-center/audit:/app/audit` 并设 `audit.path`），否则服务会因审计文件无法打开而拒绝启动。详见 [docs/manual.md](docs/manual.md) 第五章与 8.1 节。

服务管理：

```bash
docker ps --filter name=wecom-auth-center        # 查看运行状态
docker logs -f wecom-auth-center                 # 查看实时日志
docker exec wecom-auth-center /app/wecom-auth-center -v  # 查看当前版本
docker restart wecom-auth-center                 # 重启容器
docker stop wecom-auth-center                    # 停止
docker stop wecom-auth-center && docker rm wecom-auth-center  # 停止并删除容器

# 升级到新镜像：拉取后，用开头那条 docker run 重新创建容器
docker pull registry.cn-shenzhen.aliyuncs.com/zyx3721/wecom-auth-center:latest

# 海外或阿里云拉取不便时，可用 Docker Hub 同步发布的镜像（把上面命令的镜像名换成）
docker pull zyx3721/wecom-auth-center:latest
```

**访问**

- 健康检查：`http://your-host:8700/healthz`
- 登录入口（由业务系统跳转，也可手工测试）：`http://your-host:8700/login?app=oa`

生产环境必须经 Nginx 收口做 HTTPS（企微可信域名硬性要求），并把配置中 `server.trust_proxy` 设为 `true` 以获取真实客户端 IP；含 HTTP 自动跳转、HTTPS 与 HSTS 的完整反代示例见 [docs/manual.md](docs/manual.md) 第四章。

### 方式二：Release 二进制部署

前往 [GitHub Releases](https://github.com/zyx3721/wecom-auth-center/releases) 页面，按自己的操作系统与 CPU 架构下载对应压缩包，再按下面步骤校验、解压、配置、启动。

**下载哪个包**

| 你的机器 | 下载文件 |
| --- | --- |
| Linux x86_64 | `wecom-auth-center_<版本>_linux_amd64.tar.gz` |
| Linux ARM64（鲲鹏、飞腾等） | `wecom-auth-center_<版本>_linux_arm64.tar.gz` |
| macOS Intel 芯片 | `wecom-auth-center_<版本>_darwin_amd64.tar.gz` |
| macOS Apple 芯片 | `wecom-auth-center_<版本>_darwin_arm64.tar.gz` |
| Windows x86_64 | `wecom-auth-center_<版本>_windows_amd64.zip` |
| Windows ARM64 | `wecom-auth-center_<版本>_windows_arm64.zip` |
| 校验和 | `SHA256SUMS` |

包内是 `wecom-auth-center` 可执行文件（Windows 为 `.exe`）、`config.example.yaml` 与接入文档。二进制无外部运行时依赖，下载后可直接运行。

**1. 校验下载**

```bash
VERSION=1.0.0
mkdir -p /opt/wecom-auth-center && cd /opt/wecom-auth-center
sha256sum -c SHA256SUMS
```

**2. 解压**

```bash
tar -xzf wecom-auth-center_${VERSION}_linux_amd64.tar.gz --strip-components=1
```

**3. 配置并启动**

```bash
cp config.example.yaml config.yaml
vim config.yaml    # 填入 corpid/secret 与业务系统白名单，mock 置 false；权限 600
./wecom-auth-center -config config.yaml
```

执行 `./wecom-auth-center -v`（或 `--version`）可查看当前版本信息。

默认监听 `:8700`。需要常驻时交给 systemd：

```ini
# /etc/systemd/system/wecom-auth-center.service
[Unit]
Description=wecom-auth-center (企业微信统一认证中心)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=wecom-auth
Group=wecom-auth
WorkingDirectory=/opt/wecom-auth-center
ExecStart=/opt/wecom-auth-center/wecom-auth-center -config /opt/wecom-auth-center/config.yaml
Restart=on-failure
RestartSec=3
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/wecom-auth-center
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

```bash
systemctl daemon-reload && systemctl enable --now wecom-auth-center
```

**4. 用 Nginx 收口**

`deploy/nginx.conf.example` 是可直接复用的完整示例（HTTP 跳转、HTTPS、HSTS、`X-Real-IP` 传递）；逐步讲解与排障见 [docs/manual.md](docs/manual.md) 第四章。

**5. 访问**

同 Docker 方式：健康检查 `/healthz`、登录入口 `/login?app=oa`、域名校验文件 `/WW_verify_xxxxxxxx.txt`。

## HTTP 接口

| 方法 | 路径 | 调用方 | 说明 |
| --- | --- | --- | --- |
| GET | `/login?app=oa&redirect=/path` | 业务系统跳转用户 | 校验白名单 → 登记 state → 302 企微扫码页 |
| GET | `/callback?code=&state=` | 企业微信 | 消费 state → code 换 userid → 发 ticket → 302 回业务系统 |
| POST | `/api/verify` | 业务系统后端 | 签名校验 → ticket 一次性消费 → 返回 `{userid, name}` |
| GET | `/healthz` | 探活 | 返回 `ok` |

verify 签名算法（业务系统侧生成，认证中心恒定时间比较）：

```text
sign = hex( HMAC-SHA256( key = app_secret, message = app + "\n" + ticket + "\n" + ts ) )
```

错误码：`invalid_app` / `invalid_sign` / `expired_ts` / `invalid_ticket`，统一 401/400 + JSON。完整时序与字段说明见 [docs/architecture.md](docs/architecture.md)，业务系统接入步骤见 [docs/client-integration.md](docs/client-integration.md)。

## 数据与安全

```text
企微 secret 只存认证中心 config.yaml（600 权限，git 已排除）
        +
state / ticket 128 位随机、一次性消费、短时效
        +
verify 双因子：HMAC-SHA256 签名 + 时间戳偏差 ≤ 60s
        +
白名单之外一律拒绝：app 不在册 400，redirect 仅站内路径
```

- **全站 HTTPS** — 企微可信域名硬性要求；反代开启 HSTS（nginx 示例已含）。
- **密钥最小分发** — 业务系统只持有自己的 `app_secret`，且仅用于 verify 签名，不等于企微凭据。
- **限流** — `/login` 默认 30 次/分钟/IP，`/api/verify` 120 次/分钟/IP，可在配置调整。
- **审计日志** — 可选开启独立 JSON 行审计文件：登录发起、ticket 签发与兑换成功，以及 state 失败、ticket 重放、签名失败等安全事件全量留痕；state 校验失败、ticket 重放、签名失败同时有 Warn 级运行日志。
- **重启影响** — 默认内存存储下重启会丢弃进行中的登录流程，用户重扫即可；多实例部署切换 Redis 后不受单实例重启影响。

## 常见问题

**为什么不让企微直接配多个回调域名？**

企微每应用的可信域名/授权回调域数量有限且需逐一域名归属校验；域名多了之后每次新增系统都要走企微后台变更。统一回调域名后，企微配置一次不再动，扩容动作收敛到认证中心白名单。

**state 是什么，为什么不能省？**

CSRF 防护 + 来源定位：回调时只有持有对应 state 的认证中心才知道该把用户送回哪个系统。省掉 state 就只能信任请求参数里的 redirect 地址，等于开门揖盗（Open Redirect）。

**ticket 被人截获怎么办？**

ticket 60 秒过期、验证一次即焚、且 verify 需要业务系统的 `app_secret` 签名；截获者几乎无法在窗口内完成签名调用。请确保业务系统到认证中心走 HTTPS。

**怎么新增一个业务系统？**

企微后台不用动。三步：认证中心 `config.yaml` 的 `apps:` 下登记 `app` 标识、域名、`app_secret` 并重启；业务系统加一个 `/sso/login` 路由做 verify；把未登录跳转指向 `/login?app=新标识`。详见 [docs/client-integration.md](docs/client-integration.md)。

**企业微信内打开的 H5 也支持吗？**

支持。配置 `wecom.mode: inside` 时 `/login` 改走企微网页授权（`snsapi_base`），回调链路完全一致；PC 扫码场景保持默认 `qrcode`。

**业务系统怎么区分扫码进来的是哪个用户？**

verify 返回企微 `userid`（企业成员账号）。与本地账号的映射由业务系统决定：推荐管理员预建绑定（用户表加 `wecom_userid` 字段），或 `userid == 用户名` 时首登自动绑定。

**忘记某个系统的 `app_secret` 怎么办？**

认证中心 `config.yaml` 里就在 `apps.<app>.app_secret`，改一个新值重启即生效（旧值立即失效）。

**可以做成一次扫码、多系统通行吗？**

可以，是规划中的阶段四：认证中心为浏览器下发自身会话 Cookie 后，已登录用户再访问 `/login` 免扫码直接发 ticket。当前版本每个系统首次均需扫码。

其余部署细节、配置字段说明与排障手册见 [docs/manual.md](docs/manual.md)。

## 项目结构

```text
wecom-auth-center/
├── server/                     Go 服务
│   ├── cmd/server/             服务入口（配置加载、优雅退出）
│   ├── internal/
│   │   ├── audit/              安全审计事件独立落盘（JSON 行）
│   │   ├── config/             YAML 配置加载与强校验
│   │   ├── handler/            /login /callback /api/verify /healthz 与路由装配
│   │   ├── service/            wecom.go（token 缓存/换取身份/mock）、sso.go（state/ticket）
│   │   ├── store/              短时效存储接口与内存/Redis 实现
│   │   └── middleware/         请求日志、panic 恢复、IP 限流
│   ├── web/static/             企业微信域名校验文件等静态资源
│   ├── config.example.yaml     配置模板（config.yaml 已被 git 排除）
│   └── go.mod
├── deploy/
│   ├── Dockerfile              多阶段构建（CI 推送阿里云镜像仓库）
│   ├── docker-compose.yml      Compose 部署示例（配置/审计目录挂载）
│   ├── nginx.conf.example      auth 域名反代 + HTTPS + HSTS
│   └── systemd/wecom-auth-center.service
├── .github/
│   └── workflows/ci.yml        测试门禁 + tag 多平台发布 + 阿里云镜像推送
├── docs/
│   ├── PLAN.md                 分阶段实施计划与当前状态
│   ├── architecture.md         接口定义、数据设计、安全设计、完整时序
│   ├── wecom-setup.md          企业微信后台/域名/证书配置清单
│   ├── client-integration.md   业务系统接入指南
│   └── manual.md               完整手册（配置参考、接口调试、反代与排障）
├── scripts/
│   └── run.sh                  服务器启停管理脚本（start/stop/restart）
├── README.md                   中文说明（本文件）
└── README.en.md                English
```

## 文档

| 先看这个 | 再往下 |
| --- | --- |
| [快速开始](#快速开始) | mock 模式本地演练，不连企微走通全流程 |
| [部署](#部署) | Docker 与 Release 二进制两条路径、环境变量与反向代理 |
| [HTTP 接口](#http-接口) | 四个接口的调用方与签名算法 |
| [docs/manual.md](docs/manual.md) | 完整手册：配置参考、接口调试、Nginx/HTTPS 示例、升级与排障 |
| [docs/architecture.md](docs/architecture.md) | 完整时序图、数据结构、安全设计 |
| [docs/wecom-setup.md](docs/wecom-setup.md) | 企微后台配置、域名归属校验清单 |
| [docs/client-integration.md](docs/client-integration.md) | 业务系统接入步骤与代码示例 |
| [English README](README.en.md) | 同样的内容，英文版 |

## 版本历史

| 版本 | 发布日期 | 更新日志 |
| --- | --- | --- |
| v1.1.0 | 2026-09-19 | [verchanglog/v1.1.0.md](verchanglog/v1.1.0.md) |
| v1.0.1 | 2026-09-19 | [verchanglog/v1.0.1.md](verchanglog/v1.0.1.md) |
| v1.0.0 | 2026-09-18 | [verchanglog/v1.0.0.md](verchanglog/v1.0.0.md) |

各版本的构建产物与发布说明见 [GitHub Releases](https://github.com/zyx3721/wecom-auth-center/releases)。
