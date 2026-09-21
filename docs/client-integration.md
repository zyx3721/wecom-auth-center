# 业务系统接入指南

业务系统（如 itdb、kvm-manager）接入统一认证中心只需要三件事：登记信息、加一个回调路由、把未登录跳转指到认证中心。

## 1. 接入登记

向认证中心管理员提供：

- 业务系统标识 `app`（如 `itdb`）与对外域名（如 `https://it.example.com`）；
- 回调路径（约定统一为 `/sso/login`）。

管理员返回该系统的 `app_secret`（仅 verify 签名用）。

## 2. 后端添加 /sso/login 路由

以 Go（itdb 同栈）为例，核心逻辑约 40 行：

```go
func ssoLogin(w http.ResponseWriter, r *http.Request) {
    ticket := r.URL.Query().Get("ticket")
    redirect := sanitizeLocalPath(r.URL.Query().Get("redirect")) // 只允许以 / 开头的站内路径，默认 "/"

    ts := time.Now().Unix()
    sign := verifySign(appSecret, "itdb", ticket, ts)
    // sign = hex( HMAC-SHA256( key = app_secret, msg = app + "\n" + ticket + "\n" + ts ) )

    resp, err := http.Post("https://auth.example.com/api/verify", "application/json",
        strings.NewReader(fmt.Sprintf(`{"app":"itdb","ticket":%q,"ts":%d,"sign":%q}`,
            ticket, ts, sign)))
    // err 处理、解码 {"userid":..., "name":..., "email":..., "biz_mail":..., "job_number":...,
    //                   "alias":..., "departments":[{"id":..,"name":..}], "main_department":..}
    // 档案字段（email/job_number/departments 等）需认证中心开启 fetch_profile 才有值，未开启时为零值
    // verify 失败（401/非 200）→ 跳转认证中心 /login 重新走流程

    userid := parse(resp)
    localUser := matchLocalAccount(userid) // 见第 4 节映射策略
    if localUser == nil {
        http.Error(w, "该企业微信账号未绑定系统用户，请联系管理员", 403)
        return
    }
    issueSession(w, localUser) // 建立本系统自己的 Session/JWT Cookie
    http.Redirect(w, r, redirect, http.StatusFound)
}
```

要点：

- **verify 必须由后端发起**，`app_secret` 不进前端。
- ticket 一次性，验证失败不要重试同一 ticket，直接 302 到 `/login` 重走流程。
- 会话建立后，原有登录体系不变——本方案只替换「身份认证」环节，不动「权限」逻辑。

## 3. 前端 / 中间件改造

- 未登录拦截（401 中间件）改为：`302 → https://auth.example.com/login?app=itdb&redirect={当前路径}`。
- 可选：登录页保留账号密码入口，旁边加「企业微信扫码登录」按钮指向同一地址。

## 4. userid 与本地账号映射

企业微信返回的是企业微信 `userid`（如 `zhangsan`），需要映射到各系统本地用户，二选一：

| 策略 | 做法 | 适用 |
| --- | --- | --- |
| 预建绑定（推荐） | 用户表加 `wecom_userid` 字段，管理员在后台为现有账号绑定 | 账号体系已有、要求权限严格对应 |
| 首登自动关联 | `wecom_userid == 本地用户名` 时自动绑定，否则拒绝 | 内部系统、用户名与企业微信账号一致 |

## 5. 接入验收清单

- [ ] 扫码后能回到本系统并建立会话。
- [ ] 直接访问 `/sso/login?ticket=伪造值` 被拒并重新进入登录流程。
- [ ] 同一 ticket 提交第二次，verify 返回失败（一次性生效）。
- [ ] 未绑定账号的用户收到明确提示而非 500。
- [ ] 登出本系统后重新访问受保护页面，重新发起扫码流程。
