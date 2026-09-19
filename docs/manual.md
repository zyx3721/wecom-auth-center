# wecom-auth-center — 完整手册

> **文档说明**：本文档是项目的完整手册，承载配置文件全字段参考、HTTP 接口调试示例、Nginx/HTTPS 部署细节、业务系统接入联调与排障。项目概览、功能说明与部署入口请回到 [README.md](../README.md)；部署方式以 README 给出的 **Docker 部署** 与 **Release 二进制部署** 两种为准。
>
> 相关文档分工：[architecture.md](architecture.md) 讲设计与安全模型，[wecom-setup.md](wecom-setup.md) 讲企业微信后台配置，[client-integration.md](client-integration.md) 讲业务系统接入，本文讲部署、配置与运维。

## 目录

- [一、项目介绍](#一项目介绍)
- [二、本地开发快速启动](#二本地开发快速启动)
- [三、Docker 部署](#三docker-部署)
- [四、生产环境部署](#四生产环境部署)
- [五、配置参考](#五配置参考)
- [六、HTTP 接口与调试](#六http-接口与调试)
- [七、业务系统接入联调](#七业务系统接入联调)
- [八、运维与排障](#八运维与排障)
- [九、安全建议](#九安全建议)
- [十、常见问题](#十常见问题)

# 一、项目介绍

## 1.1 项目简介

wecom-auth-center 是企业微信统一认证中心：解决「一个企业微信自建应用要给多个业务域名提供扫码登录」的问题。企业微信后台只配置 `auth.example.com` 一个回调域名，本服务统一接收 OAuth 回调，用一次性 `state` 反查来源系统，用一次性 `ticket` 把登录结果转发给各业务系统；业务系统后端持独立密钥签名调用 `/api/verify` 换取用户身份后，建立自己的会话。

采用 CAS 式 SSO 模式，Go 编写，无外部数据库，编译产物为单二进制；内置 mock 模式，不连接企业微信即可本地演练全流程。

## 1.2 核心机制

| 机制 | 说明 |
| --- | --- |
| `state` | 128 位随机值，`/login` 生成时登记来源 `app`，企微回调原样带回；取出即删，TTL 300s（默认），防 CSRF 与重放 |
| `ticket` | 128 位随机值，回调验证通过后签发，携带 `userid`；取出即焚，TTL 60s（默认），绑定 `app` |
| `verify` | 业务系统后端专用接口，HMAC-SHA256 签名 + 时间偏差 ≤ 60s 校验后返回身份 |
| 白名单 | `app` → 业务域名映射只存在服务端配置；`redirect` 参数仅接受站内相对路径 |
| `access_token` | corpid + secret 换取，进程内缓存（提前 5 分钟刷新，并发加锁） |

完整时序图与安全设计见 [architecture.md](architecture.md)。

## 1.3 数据与安全边界

- 企业微信 `secret` 只存认证中心 `config.yaml`；业务系统仅持有用于 verify 签名的独立 `app_secret`。
- state/ticket 均为秒级短时效数据，默认存进程内存；重启丢弃进行中的登录流程（用户重扫即可），多实例部署切换 Redis（`Store` 接口已预留）。
- 认证中心不维护用户会话、不存密码、不管权限；身份归属各业务系统自决。

## 1.4 技术栈

| 层 | 选型 |
| --- | --- |
| 语言 | Go 1.26+（标准库 `net/http`，Go 1.22+ 方法路由） |
| 外部依赖 | gopkg.in/yaml.v3（唯一） |
| 授权入口 | 企微 wwlogin 扫码（`qrcode`）/ 网页授权 `snsapi_base`（`inside`） |
| 存储 | 进程内存 + TTL；Redis 预留 |
| 部署 | 单二进制 + systemd / 多阶段 Docker 镜像 |

## 1.5 项目结构

```text
wecom-auth-center/
├── server/                     Go 服务
│   ├── cmd/server/             服务入口
│   ├── internal/config/        配置加载与校验
│   ├── internal/handler/       HTTP 接口与路由装配
│   ├── internal/service/       企微客户端（token/身份/mock）与 state/ticket
│   ├── internal/store/         短时效存储接口与内存/Redis 实现
│   ├── internal/audit/         安全审计事件独立落盘
│   ├── internal/middleware/    日志、恢复、限流
│   ├── web/static/             企微域名校验文件等静态资源
│   └── config.example.yaml     配置模板
├── deploy/                     Dockerfile、compose 示例、nginx 示例、systemd 单元
├── scripts/                    服务器启停管理脚本（start/stop/restart）
├── .github/workflows/ci.yml    测试门禁 + tag 发布 + 阿里云镜像推送
└── docs/                       架构、企微配置、接入指南与本手册
```

# 二、本地开发快速启动

## 2.1 环境要求

- Go 1.25+
- 本地演练不需要企业微信账号（使用 mock 模式）

## 2.2 克隆与配置

```bash
git clone https://github.com/zyx3721/wecom-auth-center.git
cd wecom-auth-center/server
cp config.example.yaml config.yaml
```

mock 演练最小配置（其余字段保持模板默认即可）：

```yaml
server:
  listen: "127.0.0.1:8700"
  external_url: "http://127.0.0.1:8700"
  static_dir: "web/static"

wecom:
  mode: "qrcode"
  mock: true        # 关键：不访问企微真实接口，corpid/secret 可留空

apps:
  oa:
    domain: "https://oa.example.com"
    callback_path: "/sso/login"
    app_secret: "0123456789abcdef0123456789abcdef"
```

## 2.3 启动与演练

```bash
go run ./cmd/server -config config.yaml
```

浏览器打开 `http://127.0.0.1:8700/login?app=oa`：

1. 302 到「模拟扫码页」→ 点击「模拟扫码成功」；
2. `/callback` 消费 state、签发 ticket，302 跳回 `https://oa.example.com/sso/login?ticket=...`（业务系统不存在，浏览器打不开属正常，关注 URL 即可）；
3. 按[第六章](#六http-接口与调试)的 curl 示例用 ticket 调 `/api/verify`，返回 `{"userid":"mockuser","name":"模拟用户"}`。

## 2.4 测试与常用命令

```bash
cd server
go test ./...          # 全量单元测试
go test -race ./...    # 带竞态检测
go vet ./...
gofmt -l .             # 无输出即格式合规
```

# 三、Docker 部署

## 3.1 部署目录结构

```text
/opt/wecom-auth-center/
├── config.yaml      # 从 server/config.example.yaml 复制修改，权限 600
└── web/static/      # 可选：WW_verify 域名校验文件（镜像已内置静态目录，也可挂载覆盖）
```

## 3.2 准备配置文件

```bash
mkdir -p /opt/wecom-auth-center
cp server/config.example.yaml /opt/wecom-auth-center/config.yaml
vim /opt/wecom-auth-center/config.yaml
chmod 600 /opt/wecom-auth-center/config.yaml
```

必改项：`wecom.mock: false`、`wecom.corpid`、`wecom.agentid`、`wecom.secret`、`server.external_url`（改为 `https://auth.example.com`）、`apps:` 白名单。全字段说明见[第五章](#五配置参考)。

## 3.3 启动服务

镜像由 GitHub CI 在打 `v*` tag 时自动构建并同步推送到阿里云镜像仓库与 Docker Hub（amd64/arm64 双架构），也可在阿里云容器镜像服务控制台手动构建：

```bash
docker run -d \
  --name wecom-auth-center \
  --restart unless-stopped \
  -p 127.0.0.1:8700:8700 \
  -v /opt/wecom-auth-center/config.yaml:/app/config.yaml:ro \
  registry.cn-shenzhen.aliyuncs.com/zyx3721/wecom-auth-center:latest
```

`-p 127.0.0.1:8700:8700` 只绑回环，由宿主机 Nginx 反代对外；如需临时直连调试可改 `-p 8700:8700`。

也可使用 Compose 部署：复制 `deploy/docker-compose.yml` 并按需调整挂载路径，`docker compose up -d` 启动，`docker compose pull && docker compose up -d` 升级；审计目录挂载示例见该文件注释。

## 3.4 服务管理与升级

```bash
docker ps --filter name=wecom-auth-center        # 状态
docker logs -f wecom-auth-center                 # 日志
docker restart wecom-auth-center                 # 重启
docker stop wecom-auth-center && docker rm wecom-auth-center   # 删除容器（配置在宿主机，不受影响）
docker pull registry.cn-shenzhen.aliyuncs.com/zyx3721/wecom-auth-center:latest  # 升级：拉新镜像后重新 run
```

# 四、生产环境部署

## 4.1 获取二进制

从 [GitHub Releases](https://github.com/zyx3721/wecom-auth-center/releases) 下载对应平台压缩包（下载与校验步骤见 [README.md](../README.md) 部署章节），解压到 `/opt/wecom-auth-center/`；或从源码构建：

```bash
cd server
CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/wecom-auth-center ./cmd/server
```

## 4.2 配置

同[第三章 3.2](#32-准备配置文件)，`config.yaml` 放 `/opt/wecom-auth-center/`，权限 600。

## 4.3 systemd 常驻

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
useradd -r -s /usr/sbin/nologin wecom-auth
chown -R wecom-auth:wecom-auth /opt/wecom-auth-center
systemctl daemon-reload && systemctl enable --now wecom-auth-center
systemctl status wecom-auth-center
```

## 4.4 Nginx 收口

企微可信域名要求 HTTPS，必须反代收口。完整示例即 [deploy/nginx.conf.example](../deploy/nginx.conf.example)：

- HTTP 80 全部 301 到 HTTPS；
- HTTPS 443 配证书与 `Strict-Transport-Security`；
- `location /` 反代 `http://127.0.0.1:8700`，传递 `X-Real-IP`、`X-Forwarded-For`、`X-Forwarded-Proto`。

```nginx
# HTTP 全部跳转 HTTPS
server {
    listen 80;
    server_name auth.example.com;
    return 301 https://$host$request_uri;
}

server {
    # listen 443 ssl http2;  # Nginx 1.25 以下版本写法
    listen 443 ssl;
    http2 on;
    server_name auth.example.com;

    ssl_certificate     /usr/local/nginx/ssl/auth.example.com.fullchain.pem;
    ssl_certificate_key /usr/local/nginx/ssl/auth.example.com.key;
    
    ssl_protocols              TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers  on;
    ssl_ciphers                ECDHE-RSA-AES256-GCM-SHA512:DHE-RSA-AES256-GCM-SHA512:ECDHE-RSA-AES256-GCM-SHA384:DHE-RSA-AES256-GCM-SHA384;
    ssl_session_timeout        10m;
    ssl_session_cache          shared:SSL:10m;
    ssl_session_tickets        off;
    
    access_log /usr/local/nginx/logs/auth-access.log;
    error_log /usr/local/nginx/logs/auth-error.log warn;
    
    # HSTS
    add_header Strict-Transport-Security "max-age=31536000" always;

    # 企业微信域名归属校验文件与静态资源也经此入口
    location / {
        proxy_pass http://127.0.0.1:8700;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 30s;
    }
}
```

反代生效后，配置文件中 `server.trust_proxy: true`，限流与日志才能取到真实客户端 IP（否则全部记为 127.0.0.1，限流会误伤）。

## 4.5 域名校验文件与企微后台

把企微后台下发的 `WW_verify_xxxxxxxx.txt` 放入 `server/web/static/`（Docker 挂载或二进制部署直接放部署目录的 `web/static/`），确认 `https://auth.example.com/WW_verify_xxxxxxxx.txt` 可访问后，再回企微后台完成可信域名与授权回调域验证。完整清单见 [wecom-setup.md](wecom-setup.md)。

# 五、配置参考

`config.yaml` 全部字段（`server/config.example.yaml` 为模板）：

## 5.1 server

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `listen` | `:8700` | HTTP 监听地址；容器内保持默认，反代场景建议 `127.0.0.1:8700` |
| `external_url` | 无（必填） | 对外访问地址，用于拼接 `redirect_uri`；不能以 `/` 结尾 |
| `static_dir` | `web/static` | 静态目录（域名校验文件），相对工作目录 |
| `trust_proxy` | `false` | 反代后设 `true`，从 `X-Real-IP` 取客户端 IP |

## 5.2 wecom

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `corpid` | 无（非 mock 必填） | 企业 ID（我的企业 → 企业信息） |
| `agentid` | 无（非 mock 必填） | 自建应用 AgentId |
| `secret` | 无（非 mock 必填） | 自建应用 Secret，泄露后在企微后台重置 |
| `mode` | `qrcode` | `qrcode`=PC 扫码（wwlogin）；`inside`=企微内 H5 网页授权 |
| `mock` | `false` | `true` 时不访问企微接口，`/login` 进入本地模拟扫码页，`userid` 固定为 `mockuser` |
| `fetch_name` | `false` | `true` 时额外调通讯录 `user/get` 补全姓名（需应用有通讯录读取权限）；取不到不阻断登录 |

## 5.3 ttl

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `state` | `300s` | state 有效期 |
| `ticket` | `60s` | ticket 有效期 |
| `verify_ts_skew` | `60s` | verify 时间戳允许偏差 |

## 5.4 apps（业务系统白名单）

每个系统一个键，键名即 `app` 标识：

| 字段 | 说明 |
| --- | --- |
| `domain` | 业务系统对外域名，`https://` 或 `http://` 开头（生产推荐 HTTPS；HTTP 仅建议内网，ticket 将明文传输，启动时有 Warn 提示），不带尾斜杠 |
| `callback_path` | 登录成功后的回调路径，约定统一为 `/sso/login` |
| `app_secret` | verify 签名密钥，≥32 位（`openssl rand -hex 32` 生成）；更换后重启即生效，旧值立即失效 |

## 5.5 ratelimit

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `login_per_minute` | `30` | `/login` 每 IP 每分钟上限 |
| `verify_per_minute` | `120` | `/api/verify` 每 IP 每分钟上限 |

## 5.6 store

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `driver` | `memory` | 存储驱动：`memory` 进程内存（单实例）；`redis` 多实例共享（要求 Redis 6.2+ 支持 `GETDEL`），启动时 Ping 校验连通，失败拒绝启动 |
| `redis.addr` | — | Redis 地址，`driver: redis` 时必填 |
| `redis.password` | 空 | Redis 密码，无则留空 |
| `redis.db` | `0` | Redis 逻辑库编号 |

state/ticket 以 JSON 存于 `wecom-auth-center:state:*` 与 `wecom-auth-center:ticket:*` 键，TTL 与 `ttl` 配置一致；消费即 `GETDEL` 删除。Redis 读取故障按凭证不存在处理（fail closed），对应请求失败用户重试即可。数据均为秒级时效，Redis 无需持久化配置。

## 5.7 audit

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `enabled` | `false` | 开启后安全审计事件独立落盘为 JSON 行文件 |
| `path` | `audit.log` | 审计文件路径；Docker 部署须指向挂载的可写目录（镜像内以非 root 运行） |

开启后记录事件：`login_start`（登录发起）、`ticket_issue`（ticket 签发）、`verify_ok`（兑换成功）、`state_reject`（state 校验失败）、`verify_app_reject`（白名单外 app）、`verify_sign_reject`（签名失败）、`verify_ts_reject`（时间戳超差）、`ticket_reject`（ticket 重放/过期/不存在）、`ticket_mismatch`（ticket 归属不匹配）。文件打开失败时服务拒绝启动，避免安全事件失录；轮转建议交给系统 logrotate。

## 5.8 status

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `enabled` | `false` | 开启后提供 `/status` 监控页与 `/api/status` 数据接口 |
| `token` | — | 访问令牌，`enabled: true` 时必填（建议 `openssl rand -hex 16` 生成），以 `?token=` 方式校验 |
| `data_path` | `status-metrics.json` | 统计持久化文件，每 60 秒原子落盘、启动时加载，重启后历史保留；Docker 部署需挂载可写目录 |

监控页展示：近 7 天登录发起/登录成功、今日登录发起/兑换成功 4 张统计卡，近 7 天趋势图（页面内自绘 SVG，无外部图表库），今日拒绝事件明细（state 失败/签名失败/ticket 拒绝等），版本与构建信息、运行时长、存储驱动与 Redis 在线状态，30 秒自动刷新。统计按日分桶保留 7 天，与审计事件同名同点位采集。

「最近登录」面板逐条展示成功扫码登录的流水（时间、企微账号 userid、姓名、来源应用、登录发起 IP，最多保留 50 条并随统计文件持久化）。IP 为发起登录时浏览器来源地址（反代后取 `X-Real-IP`）；开启 `fetch_name` 后才有姓名。该面板含用户账号信息，请妥善保管访问令牌，必要时在 Nginx 对 `/status`、`/api/status` 追加来源限制。

# 六、HTTP 接口与调试

## 6.1 接口清单

| 方法 | 路径 | 调用方 | 说明 |
| --- | --- | --- | --- |
| GET | `/login?app=oa&redirect=/path` | 业务系统 302 用户 | 白名单校验 → 登记 state → 302 企微授权页（mock 模式 302 模拟扫码页） |
| GET | `/callback?code=&state=` | 企业微信 | 消费 state → code 换 userid → 签发 ticket → 302 回 `app.callback_path` |
| POST | `/api/verify` | 业务系统后端 | 签名/时间偏差校验 → ticket 一次性消费 → `{userid, name}` |
| GET | `/status?token=` | 管理员浏览器 | 监控页（`status.enabled` 开启后可用） |
| GET | `/api/status?token=` | 监控页 | 统计、运行信息与存储健康（Token 保护） |
| GET | `/healthz` | 探活 | `ok` |

## 6.2 verify 签名算法

```text
sign = hex( HMAC-SHA256( key = app_secret, message = app + "\n" + ticket + "\n" + ts ) )
```

`ts` 为 Unix 秒。请求体：

```json
{"app":"oa","ticket":"<ticket>","ts":1726650000,"sign":"<hex>"}
```

错误码：`invalid_app`（400/401，app 不在白名单）、`invalid_body`（400，JSON 非法）、`invalid_sign`（401，签名错误）、`expired_ts`（401，时间偏差超限）、`invalid_ticket`（401，ticket 不存在/过期/已用/归属不符）。

## 6.3 curl 调试示例

拿到 302 回业务系统 URL 中的 ticket 后（bash）：

```bash
TICKET="<从 callback 302 的 Location 中提取>"
APP="oa"
SECRET="<该 app 的 app_secret>"
TS=$(date +%s)
SIGN=$(printf '%s\n%s\n%s' "$APP" "$TICKET" "$TS" | openssl dgst -sha256 -hmac "$SECRET" -hex | sed 's/^.*= //')

curl -s -X POST https://auth.example.com/api/verify \
  -H "Content-Type: application/json" \
  -d "{\"app\":\"$APP\",\"ticket\":\"$TICKET\",\"ts\":$TS,\"sign\":\"$SIGN\"}"
# {"name":"...","userid":"zhangsan"}

# 同一 ticket 再发一次 → {"error":"invalid_ticket"}（一次性生效）
```

# 七、业务系统接入联调

接入步骤与代码示例见 [client-integration.md](client-integration.md)，本章只讲端到端联调顺序：

1. **登记** — 认证中心 `config.yaml` 的 `apps:` 增加条目，生成 `app_secret` 并通过安全渠道交给业务系统负责人，重启服务生效。
2. **业务系统改造** — 添加 `/sso/login` 路由（verify → 匹配本地账号 → 建会话），未登录 302 指向 `https://auth.example.com/login?app=<标识>&redirect=<当前路径>`。
3. **联调** — 业务系统登录页发起跳转 → 企微扫码 → 落回业务系统并建立会话。
4. **验收清单** — 见 client-integration.md 第五节：伪造 ticket 拒绝、ticket 复用拒绝、未绑定账号友好提示、登出后重新发起流程。

常见联调问题：

- 落回业务系统后 verify 报 `invalid_ticket`：多为业务系统与认证中心服务器**时钟偏差超过 60s**，先校时（NTP）。
- `invalid_sign`：签名串必须是 `app + "\n" + ticket + "\n" + ts` 三段拼接，确认无多余空行且密钥与认证中心登记一致。

# 八、运维与排障

## 8.1 日志

服务输出 JSON 结构化日志（slog）：每个请求记录方法、路径、状态、耗时与客户端 IP；安全事件（state 校验失败、ticket 重放、verify 签名失败）有独立 Warn 日志，可按 `msg` 字段接入采集告警。

开启 `audit.enabled` 后另有独立审计文件（JSON 行，事件名在 `msg` 字段），完整记录登录发起、ticket 签发/兑换成功与全部拒绝类安全事件，与运行日志互不干扰，建议单独采集长期留存。文件需落在服务账号可写目录；Docker 部署示例：

```bash
# 挂载可写目录并在配置中指向它
docker run ... -v /opt/wecom-auth-center/audit:/app/audit ... 
# config.yaml: audit.enabled: true, audit.path: "/app/audit/audit.log"
```

## 8.2 故障对照表

| 现象 | 可能原因 | 处理 |
| --- | --- | --- |
| 企微扫码后提示 redirect_uri 域名与配置不一致 | 企微后台「授权回调域」与 `external_url` 域名不一致 | 改企微后台配置或 `external_url`，两边域名必须一致 |
| 域名归属校验不通过 | `WW_verify_xxxxxxxx.txt` 不可访问或放错目录 | 确认 `https://auth域名/WW_verify_xxx.txt` 返回 200，文件名未改名 |
| `/callback` 一直 403 | state 过期（超过 300s）、重放或跨浏览器跳转 | 重新发起登录；检查是否有代理剥 query |
| verify `invalid_sign` | 签名串拼接错误或密钥不一致 | 按第 6.2 节算法核对；确认用的是该 app 当前密钥 |
| verify `expired_ts` | 服务器时钟偏差 > 60s | 两端配置 NTP 校时 |
| verify `invalid_ticket` 但流程刚走完 | ticket 已被消费一次（重放），或超过 60s | ticket 一次性，重新发起登录 |
| 限流误伤（全部请求 429） | 反代后未开 `trust_proxy`，所有请求按 127.0.0.1 计数 | 配置 `server.trust_proxy: true` 并确认 Nginx 传 `X-Real-IP` |
| 重启后进行中的登录失败 | 内存存储不持久化 | 属预期行为，重扫即可；多实例需求切 Redis |

## 8.3 升级

1. 备份 `config.yaml`；
2. Docker：`docker pull` 新镜像后重新 `docker run`；二进制：替换可执行文件后 `systemctl restart wecom-auth-center`；
3. 执行 `./wecom-auth-center -v`（Docker 用 `docker exec <容器名> /app/wecom-auth-center -v`）确认版本已更新，该参数同样支持 `--version` 写法；
4. 观察 `/healthz` 与日志恢复正常。

## 8.4 监控页

开启 `status.enabled` 并配置 `status.token` 后，浏览器访问 `https://auth域名/status?token=你的令牌` 查看运行状态：近 7 天与今日的登录发起、ticket 签发、兑换成功统计，今日拒绝事件（state 失败 / 签名失败 / ticket 拒绝），最近登录流水（企微账号、姓名、来源应用与登录发起 IP），版本与构建信息、运行时长、存储驱动与 Redis 在线状态，每 30 秒自动刷新。

- 数据接口为同令牌的 `/api/status?token=`，可供脚本采集；令牌错误返回 403，`status.enabled: false` 时按 404 处理；
- 统计与审计事件同名同点位采集，按日分桶保留 7 天，每 60 秒落盘至 `status.data_path`，重启自动恢复；
- Docker 部署时 `data_path` 须落在挂载的可写目录，例如追加 `-v /opt/wecom-auth-center/data:/app/data` 并设 `data_path: "/app/data/status-metrics.json"`；
- 页面仅含聚合计数与运行信息，不含密钥与用户身份；如需限制访问范围，可在 Nginx 反代上对 `/status` 与 `/api/status` 加来源 IP 或 Basic Auth 控制。

# 九、安全建议

- **最小暴露** — 服务只绑 `127.0.0.1`，对外一律经 Nginx + HTTPS（企微可信域名硬性要求）。
- **密钥治理** — `config.yaml` 权限 600、属主为运行账号；`secret`/`app_secret` 不进 git、不出现在日志与工单；企微 `secret` 疑似泄露立即后台重置。
- **校时** — 服务两端（认证中心与业务系统）都要 NTP 校时，verify 时间戳防重放依赖时钟基本准确。
- **限流与审计** — 生产环境按需收紧 `ratelimit`；定期检索 Warn 级安全日志。
- **最小授权** — 企微应用「可见范围」只加需要扫码登录的成员；未开启 `fetch_name` 时无需通讯录读取权限。

# 十、常见问题

**可以部署多个实例做高可用吗？**

当前内置存储为进程内存，多实例会导致 state/ticket 不共享、登录随机失败。高可用需求出现时实现 `Store` 的 Redis 版（接口已预留，见 [PLAN.md](PLAN.md) 阶段三），切换配置即可，接口层无需改动。

**换认证域名（auth 域名迁移）怎么办？**

改 `server.external_url`、DNS 与证书，企微后台的可信域名/授权回调域改为新域名并重新做域名校验；业务系统侧无需变更（它们只认 `/login?app=` 的完整地址，统一改 DNS 即可，前提是域名不变仅换服务器）。跨域名迁移时业务系统的跳转地址要同步更新。

**为什么 verify 不做成无签名的公开接口？**

ticket 本身出现在 URL 中，可能经浏览器历史、代理日志泄露；签名保证「只有登记过的业务系统后端」能兑换身份，浏览器即使拿到 ticket 也无法使用。

**业务系统如何下线？**

从 `apps:` 删除对应条目并重启：该系统的 `/login?app=` 立即返回 400，存量 ticket 60s 内自然过期，无需其他清理。

**企微接口报错如何定位？**

服务日志中企微错误以 `errcode=xxx errmsg=xxx` 记录（gettoken/getuserinfo），对照[企业微信官方错误码文档](https://developer.work.weixin.qq.com/document/path/90313)排查；常见 `40029` 为 code 无效或已使用，`60020` 为出口 IP 未加入企微后台「企业可信 IP」。
