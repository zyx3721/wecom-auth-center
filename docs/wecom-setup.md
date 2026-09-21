# 企业微信后台与域名配置清单

阶段〇的操作手册。全部完成后才进入编码阶段。

## 1. 前置条件

- [ ] 回调域名（如 `auth.example.com`）已确定，且主域已 ICP 备案（企业微信硬性要求）。
- [ ] DNS 记录已生效：`auth.example.com` → 目标服务器。
- [ ] HTTPS 证书已签发并部署到 Nginx，`https://auth.example.com` 可访问。
- [ ] 管理员权限：企业微信管理后台（work.weixin.qq.com）的管理员或应用管理员。

## 2. 企业微信管理后台配置

1. **我的企业 → 企业信息**：记录 `CorpID`（corpid）。
2. **应用管理 → 应用 → 自建**：新建应用（如「统一登录」）或复用现有应用，记录 `AgentId` 与 `Secret`。
   - Secret 只显示一次，妥善保存到服务器本地配置（勿进 git）。
   - 应用可见范围决定哪些成员可以扫码登录，按需配置。
3. **应用详情 → 开发者接口 → 网页授权及 JS-SDK**：
   - 配置「可信域名」为 `auth.example.com`（不带 https:// 前缀）。
   - 按提示完成**域名归属验证**：下载 `WW_verify_xxxxxxxx.txt`，放到认证中心静态目录 `server/web/static/`，保证 `https://auth.example.com/WW_verify_xxxxxxxx.txt` 可直接访问后再点验证。
4. **授权回调域**：在同一开发者接口区域配置为 `auth.example.com`。扫码登录（wwlogin）的 `redirect_uri` 必须位于该域名下。
5. 记录扫码登录入口 URL 格式（新入口为 `login.work.weixin.qq.com/wwlogin/sso/login`），与 `docs/architecture.md` 中 `/login` 的 302 地址保持一致。
6. **邮箱与员工编码说明**：
   - **邮箱**：成员邮箱/企业邮箱受企微平台限制，2022-06-20 后新建的自建应用在扫码登录链路无法获取（`user/get` 不返回该类敏感字段）；如确有需要，只能经企微内 H5 的 `snsapi_privateinfo` 手工授权获取，本服务暂未实现。
   - **成员扩展属性**：员工编码（工号）无标准字段，需管理员在 **通讯录 → 成员详情 → 扩展属性** 为成员维护自定义字段（认证中心默认取字段名「员工编码」，可用 `job_number_extattr` 调整；字段名不匹配时返回空串不报错）。
   - **部门完整路径**：部门显示为沿 `parentid` 向上追溯的完整层级路径；要求应用「可见范围」覆盖成员所属部门及其**全部上级部门**，否则路径止于可见范围内的最上级（服务日志记 `部门信息查询失败 errcode=60003` Warn）。需完整路径时请将可见范围扩大到整个组织树。

## 3. 业务系统登记

在认证中心配置文件中为每个业务系统登记（示例）：

```yaml
apps:
  oa:
    domain: https://oa.example.com
    callback_path: /sso/login
    app_secret: "<openssl rand -hex 32 生成>"
  itdb:
    domain: https://it.example.com
    callback_path: /sso/login
    app_secret: "<...>"
```

每个系统的 `app_secret` 通过安全渠道（不经过聊天工具明文）交给对应系统管理员。

## 4. 完成检查

- [ ] `https://auth.example.com/WW_verify_xxxxxxxx.txt` 返回 200。
- [ ] 企业微信后台可信域名、授权回调域均显示验证通过。
- [ ] corpid / agentid / secret 已存入服务器本地配置（非 git、非文档）。
- [ ] 各业务系统 `app` 白名单与 `app_secret` 已生成并登记。
- [ ] （开启 `fetch_profile` 时）应用已授予通讯录读取权限，可见范围覆盖需登录的成员及其部门的全部上级（获取完整部门路径的前提）。
