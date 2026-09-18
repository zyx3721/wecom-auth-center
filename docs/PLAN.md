# 分阶段实施计划

当前状态：**阶段〇未做（按皇上指示跳过）、阶段一已完成并通过本地 mock 端到端演练、阶段三已完成（Redis 存储与审计日志；限流参数化随阶段一交付，Prometheus 为可选项未实施）**。按阶段推进，每阶段结束有明确验证点。

## 阶段一：认证中心 MVP（约 1–2 天）—— ✅ 已完成

任务：

1. 初始化 `server/`：Go module、`config.example.yaml`（corpid / agentid / secret / app 白名单）、`config.yaml` 进 `.gitignore`。
2. 实现 `store.MemoryStore`（state/ticket 存取即删 + TTL）与 `Store` 接口。
3. 实现 `service/wecom.go`：`gettoken`（缓存 + 提前刷新 + 并发锁）、`getuserinfo`；`fetch_name` 可选补全姓名。
4. 实现 `service/sso.go`：state / ticket 生成与原子消费。
5. 实现 handler：`GET /login`、`GET /callback`、`POST /api/verify`（含 HMAC 签名校验）、`GET /healthz`、静态文件（域名校验文件、错误页）；mock 模式含本地模拟扫码页。
6. IP 限流中间件（固定窗口，参数可配置）、请求日志、panic 恢复、优雅退出。

验证结果（`go test ./...` 全部通过 + mock 端到端演练）：

- 单元测试：state/ticket 生命周期（过期、重放拒绝、原子消费、并发仅一胜）、白名单拒绝、redirect 清洗、HMAC 签名校验、时间戳偏差拒绝、配置校验、限流窗口。
- 端到端演练（mock 模式 curl 全流程）：`/login` 302 → 模拟扫码页 → `/callback` 消费 state → 302 回 `https://oa.example.com/sso/login?ticket=...&redirect=%2Fconsole`；state 重放 403；verify 正确签名 200 返回 userid；ticket 重放 401 `invalid_ticket`；错误签名 401 `invalid_sign`；白名单外 app 400。

## 阶段〇：配置准备（约 0.5 天，无代码）

| 任务 | 说明 |
| --- | --- |
| 确定回调域名 | 如 `auth.example.com`，需已 ICP 备案；新增 DNS A/CNAME 记录指向目标服务器 |
| HTTPS 证书 | 签发/配置证书（可用现有 CertFlow 流程），确认 Nginx 全站 HTTPS |
| 企业微信后台 | 管理后台创建（或复用）自建应用，记录 `CorpID`、`AgentId`、`Secret`；配置「网页授权及 JS-SDK 可信域名」与「授权回调域」为回调域名，下载域名校验文件 `WW_verify_xxxxxxxx.txt` |
| 梳理业务系统清单 | 确定第一批接入的系统与域名（如 oa / it / cmdb），为每个系统确定 `app` 标识、回调域名、生成 `app_secret` |

详细操作步骤见 [wecom-setup.md](wecom-setup.md)。

**验证**：`https://auth.example.com/WW_verify_xxxxxxxx.txt` 可通过 HTTPS 访问；企业微信后台域名校验显示通过。

## 阶段二：首个业务系统接入 + 上线（约 1 天）

任务：

1. 选定首个接入系统（建议 itdb，Go 后端改动最小），按 [client-integration.md](client-integration.md) 添加 `/sso/login` 路由：verify ticket → 匹配本地账号 → 建立会话。
2. 前端 401 未登录处理改为跳转 `auth.example.com/login?app=itdb`。
3. 部署认证中心：`deploy/nginx.conf.example` 反代 + HTTPS + `systemd` 服务单元；放置 `WW_verify` 文件。
4. 企业微信后台配置完成后真实扫码联调。

验证方式：

- 真实扫码登录走通全流程；篡改 ticket / 复用 ticket / 过期 ticket 均被拒绝；verify 错误签名返回 401。
- 浏览器验证登录后会话正常、退出后重新发起流程正常。

## 阶段三：加固（约 1 天）—— ✅ 已完成（Prometheus 监控为可选项未实施）

任务：

1. `store.RedisStore`（`SET ... EX` + `GETDEL`），配置切换，为多实例部署留出能力。✅
2. 审计日志结构化输出（state 失败 / ticket 重放 / 签名失败单独标记）。✅
3. 限流参数化；`/api/verify` 增加失败次数监控指标（Prometheus 文本接口可选）。✅ 限流参数化随阶段一交付；Prometheus 未实施，留作后续可选。
4. 部署文档完善：备份与恢复仅涉及配置文件；发布流程（单二进制 + systemd）。✅ 随 v1.0.0 文档与发布流水线交付。

验证结果（`go test ./...` 七包全绿 + mock 端到端演练）：

- 单元测试：Redis 实现（miniredis）覆盖往返存取、取出即删（重放拒绝）、TTL 过期、数据损坏与宕机 fail closed；审计包覆盖 JSON 行落盘与 nil 安全；配置覆盖 driver 校验（非法驱动、redis 缺地址拒绝，redis + audit 合法加载）。
- 端到端演练（mock 模式 + 审计开启）：login → 模拟扫码 → callback → verify 全链路审计文件依次落盘 `login_start` / `ticket_issue` / `verify_ok`；ticket 重放 `ticket_reject`、错误签名 `verify_sign_reject` 均有审计与 Warn 运行日志。
- fail-fast 演练：审计文件不可写时拒绝启动；`store.driver: redis` 且 Redis 不可达时启动即报「Redis 连接失败」退出（exit 1）。

## 阶段四（可选）：多系统单点登录

1. 认证中心为浏览器下发自身会话 Cookie（HttpOnly + Secure，域 `auth.example.com`）。
2. 已登录用户再次访问 `/login?app=xx` 免扫码，直接发 ticket 302 返回。
3. 各业务系统接入后即获得「一次扫码，多系统通行」；登出暂不做跨系统广播。

## 风险与对策

| 风险 | 对策 |
| --- | --- |
| 域名未备案 / 校验文件不通过 → 企业微信后台配置被拒 | 阶段〇先完成验证再写代码顺序不变；备选：换用已备案子域 |
| `secret` 泄露 | 只存服务器本地配置文件（600 权限），不进 git、不进日志；泄露立即在后台重置 |
| 内存存储重启丢登录流程 | 用户重扫即可；多实例需求出现时切 Redis |
| 企业微信接口变更（新旧扫码入口并存） | `service/wecom.go` 独立封装，仅此一层接触外部 API |
| 业务系统 userid 与本地账号映射不一致 | 接入前先定映射策略（管理员预建 / 首登绑定），见 client-integration.md |
