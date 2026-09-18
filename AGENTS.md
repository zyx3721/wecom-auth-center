# wecom-auth-center 项目开发规范

本文档为 AI 助手提供项目开发指导，确保代码修改、提交和发布流程的一致性。

---

## 1. 代码修改规范

### 1.1 修改后无需构建

- 对 `server/` 下的代码进行修改后，**无需立即执行构建命令**
- 开发环境下优先依赖 `go test`、`go vet` 等针对性验证确认修改效果
- 仅在以下情况需要构建：
  - 准备部署到生产环境
  - 创建 Docker 镜像
  - 验证交叉编译（发布前冒烟）

### 1.2 避免的操作

- ❌ 修改后立即执行 `go build`（`go test` 与 `go vet` 不受限）
- ❌ 修改后立即执行 `docker build`
- ❌ 所有修改的文件，如果本身为 `utf8` 编码格式，请不要修改为其他格式，如 `utf8-bom`
- ❌ 不要随便清空整个代码文件
- ❌ 禁止将中文改为乱码形式，如 `??`、`\u` 等
- ❌ 禁止把 `server/config.yaml`（含 corpid/secret）加入 git 或复制进文档、日志、镜像层

### 1.3 代码超出行数拆分规范

对代码文件需进行处理：

- 修改文件前，如果目标文件已超过 `600` 行，必须先评估是否需要拆分
- 如果本次修改会让文件超过 `600` 行，必须优先拆分后再继续实现，除非拆分会明显扩大风险
- 如果暂不拆分，必须在最终回复中说明原因、风险和后续建议
- 拆分必须遵循当前目录分层：
  - 服务入口放在 `server/cmd/server/`，配置加载与校验放在 `server/internal/config/`
  - HTTP 处理放在 `server/internal/handler/`：`router.go` 只保留路由装配与中间件串联；业务处理按接口拆文件（`login.go` 含 /login 与 /callback、`verify.go`、`misc.go` 辅助与静态页）
  - 业务逻辑放在 `server/internal/service/`：企微对接只允许在 `wecom.go`（真实客户端与 mock 客户端都在此），state/ticket 生命周期在 `sso.go`
  - 短时效存储放在 `server/internal/store/`：接口定义与实现分离，内存实现与未来 Redis 实现各自独立文件
  - 横切能力放在 `server/internal/middleware/`：日志、恢复、限流
- 拆分时必须保持包边界和调用方向清晰：`handler` 可调用 `service` 与 `store`（经 `service` 封装），`service` 依赖 `store` 接口；`middleware` 不反向依赖 `handler`；不得形成循环依赖
- 重复判断逻辑抽取规范：
  - 客户端 IP 提取统一使用 `middleware.RealIP`，不得在 handler 内重复实现
  - JSON 响应统一走 `writeJSON`，verify 错误统一走 `writeVerifyError`
  - 抽取重复逻辑时必须保持接口契约与日志语义不变；若抽取会改变行为，必须同步更新 `README.md`、`README.en.md`、`docs/manual.md`、`docs/architecture.md` 与验证用例
- 文件移动或目录层级调整后，必须同步更新 import、`README.md` 与 `docs/manual.md` 的「项目结构」、`README.en.md` 的「Repository layout」，并执行至少一次 `go test ./...`

### 1.4 接口与文档同步规范

对 HTTP 接口进行**增/删/改**操作时，必须同步更新：

- `README.md` 的「HTTP 接口」章节：接口清单表与签名算法
- `docs/manual.md` 的《六、HTTP 接口与调试》与《五、配置参考》（涉及配置项时）
- `docs/architecture.md` 的接口定义、数据设计与时序图
- `docs/client-integration.md`：涉及业务系统侧对接方式变化时
- `README.en.md` 的「HTTP API」章节：当本次变更涉及该章节已列出的内容时同步更新
- 接口对应单元测试（`internal/handler/handler_test.go` 等）
- 文档与代码需保持**同一变更批次完成**，禁止出现“代码已改、文档未更”的状态；如果当前目录是 Git 仓库，则应放在同一提交内

### 1.5 日志输出规范

- 服务端日志（slog）统一使用中文描述
- 禁止在语句末尾添加任何标点符号（如。、！、？等）
- 安全事件（state 失败、ticket 重放、签名失败）必须记录 Warn 及以上级别，字段便于检索（app、remote 等）
- 示例：✅ `"颁发 ticket"` ❌ `"颁发 ticket。"`

### 1.6 文档与项目结构同步规范

当前文档分工如下，本节及其它章节提到的文档均指这些文件：

- `README.md`：中文主文档，承载项目概览、核心机制、部署方式（只保留 Docker 与 Release 二进制两条路径）、HTTP 接口摘要、常见问题与项目结构
- `README.en.md`：`README.md` 的英文版本，章节结构与锚点保持一一对应
- `docs/manual.md`：完整手册，承载配置全字段参考、接口调试示例、Nginx/HTTPS 部署细节、接入联调与排障；不设版本历史章节，版本记录只在 `README.md` 维护
- `docs/architecture.md`：设计与安全模型（时序、数据结构、安全设计），跟随代码事实更新
- `docs/wecom-setup.md`：企业微信后台与域名配置清单
- `docs/client-integration.md`：业务系统接入指南
- `docs/PLAN.md`：分阶段实施计划与当前状态

- 当项目发生以下结构变更时，必须同步更新 `README.md` 与 `docs/manual.md` 文档中的「项目结构」章节，并同步 `README.en.md` 的「Repository layout」章节：
  - 新增/删除代码文件、目录
  - 移动/重命名文件或目录
  - 调整模块划分、重构目录层级
- 新增或调整部署方式时，`README.md` 只保留 Docker 部署与 Release 二进制部署两条路径，其余过程化步骤补充到 `docs/manual.md`
- 项目结构的文档描述需遵循以下规则：
  - 需准确反映当前代码仓库的目录层级与模块划分，与实际代码结构保持完全一致
  - 目录层级描述原则上不超过三级，特殊场景如需要明确了解三级下的代码文件，可添加

### 1.7 CodeGraph 同步规范

- 如果项目根目录存在 `.codegraph/` 或已确认当前项目启用了 CodeGraph，优先依赖 `codegraph serve --mcp` 默认启用的文件监听自动同步机制；不要把每次代码、文档或目录结构变更后手动执行 `codegraph sync` 作为常规硬性步骤
- 在以下场景必须先检查 `codegraph status` 或 `codegraph_status`，若出现 `Pending Changes`、陈旧索引提示或查询结果明显缺失，再在项目根目录执行 `codegraph sync`：
  - MCP Server 未运行、使用了 `codegraph serve --mcp --no-watch`，或文件监听疑似未生效
  - 刚完成分支切换、批量生成、批量移动/删除文件，或外部工具一次性改动大量源码
  - 需要使用 `codegraph query`、`codegraph callers`、`codegraph impact` 等 CLI 查询确认最新代码结构
  - 本次变更后继续使用 `codegraph_context`、`codegraph_search`、`codegraph_trace` 等 CodeGraph 工具，且工具返回陈旧提示或与本地文件内容不一致
- 如果 `codegraph sync` 执行失败或 CodeGraph 服务连接异常，需改用本地 `rg`、文件读取和测试继续完成任务，并在最终回复中说明同步或查询失败情况

### 1.8 注释规范

**只允许以下三类注释，其余位置一律不写：**

1. 顶格的顶层声明说明：顶层函数、方法、类型、常量声明正上方的一句话说明（Go 惯例以声明名开头，如 `// ConsumeTicket 消费 ticket...`）；
2. 文件顶部（package/import 之前或紧后）的模块职责说明，一行以内；
3. 配置模板与部署示例文件（`config.example.yaml`、`deploy/` 下示例）中的字段说明注释。

**禁止写注释的位置（写了即违规，提交前必须删除）：**

- 函数/方法体内任何位置的注释（包括复杂流程旁、分支旁、赋值旁）；
- 结构体字面量内部的注释；
- 代码行尾注释。

**执行口径：**

- 判断标准：注释是否紧贴某个顶层声明的上方且只此一句；位于函数体、对象字面量、表达式内部即为违规；
- 复杂逻辑优先通过拆函数与命名自解释；确需说明时上移为对应顶层声明说明的一句话；
- SQL 字符串、正则等字符串内部的文字属于业务内容，不受本约束；
- 新增代码遵循同一口径；修改既有代码时，顺带按此口径清理所在文件内的违规注释。

---

## 2. Git 提交规范

### 2.1 Git 仓库前置条件

执行任何 Git 提交、Tag、推送、GitHub Release 操作前，必须先确认当前目录或父目录存在 `.git`：

- 如果当前项目不是 Git 仓库：
  - 不执行 `git status`、`git add`、`git commit`、`git tag`、`git push`、`gh release`
  - 不主动执行 `git init`
  - 仅完成文件修改、验证和结果说明
  - 最终回复中说明“当前目录不是 Git 仓库，已跳过提交/发布相关步骤”
  - 只有用户明确要求初始化仓库时，才可以执行 `git init`
- 如果当前项目是 Git 仓库：
  - 按下方提交流程执行
  - 保留用户已有改动，不回滚无关变化
  - 禁止使用 `git add .`，必须逐文件添加

### 2.2 提交流程

修改完成后的标准流程：

```bash
# 1. 查看修改状态
git status

# 2. 逐文件添加到暂存区（禁止使用 git add .）
git add <file1>
git add <file2>

# 3. 提交（遵循 Conventional Commits 规范）
git commit -m "type: 简要描述"

# 4. 仅提交到本地，不推送到远程
# ❌ 不要执行 git push
```

### 2.3 提交类型（Conventional Commits）

| 类型 | 说明 | 示例 |
|:---:|:---:|:---:|
| `feat` | 新功能 | `feat: 支持企微内 H5 网页授权模式` |
| `fix` | Bug 修复 | `fix: 修复 state 过期判断边界问题` |
| `docs` | 文档更新 | `docs: 更新接入指南签名示例` |
| `style` | 代码格式 | `style: gofmt 统一格式` |
| `refactor` | 重构 | `refactor: 抽取 IP 提取到 middleware` |
| `perf` | 性能优化 | `perf: 优化内存存储过期清理` |
| `test` | 测试 | `test: 补充 verify 时间偏差用例` |
| `build` | 构建系统 | `build: 更新依赖版本` |
| `ci` | CI/CD | `ci: 更新镜像推送目标` |
| `chore` | 杂项 | `chore: 清理无用代码` |

### 2.4 提交注意事项

- ✅ 每次提交只包含一个逻辑变更
- ✅ 提交信息使用简体中文
- ✅ 提交后**不推送到远程**（除非明确要求）

---

## 3. 版本发布规范

### 3.0 发布前置条件

- 只有用户明确要求发布版本时，才执行本章节流程
- 执行提交、Tag、推送和 GitHub Release 前，必须满足“2.1 Git 仓库前置条件”
- 如果当前项目尚未初始化为 Git 仓库：
  - 可根据用户要求准备版本日志、`README.md`、`README.en.md`、`docs/manual.md` 等文件内容
  - 跳过提交、Tag、推送和 GitHub Release
  - 最终回复说明“当前目录不是 Git 仓库，已跳过发布中的 Git 与 GitHub Release 步骤”
- 如果当前项目是 Git 仓库但缺少远程仓库、GitHub CLI、登录状态或发布权限，应停止对应远程步骤并说明缺失条件，不做绕过

### 3.1 发布前准备

在准备发布新版本时，必须完成以下步骤：

1. **创建版本日志文档**
   - 在 `verchanglog/` 目录下创建新文档
   - 文件命名格式：`vX.X.X.md`（如 `v1.0.0.md`）
   - 若已存在历史版本文档，**严格遵循现有版本文档的格式结构**
   - 若 `verchanglog/` 目录或历史版本文档不存在，首次发布时先创建目录并使用本文档的版本日志模板

2. **更新文档**
   - 更新前，请勿修改文档内的整体结构
   - `README.md`：**版本历史只在本文件维护**，首次发布时在文末建立「版本历史」表（版本 / 发布日期 / 更新日志链接），新版本追加一行；如有接口变更同步「HTTP 接口」章节，如有新代码文件同步「项目结构」章节
   - `README.en.md`：同步「Releases」版本表（与 `README.md` 版本历史保持一致），以及「HTTP API」「Repository layout」章节
   - `docs/manual.md`：不设版本历史章节，发版时无需改动；仅在接口变更时同步《六、HTTP 接口与调试》、配置项变更时同步《五、配置参考》、代码文件变动时同步《1.5 项目结构》

3. **提交所有变更**
   - 仅当当前项目是 Git 仓库时执行
   - 如果当前项目不是 Git 仓库，跳过本步骤并在最终回复说明
   ```bash
   git add verchanglog/vX.X.X.md
   git add README.md
   git add README.en.md
   git add docs/manual.md   # 仅当本次同步过该文档时
   git commit -m "docs: 新增 vX.X.X 版本日志"
   git push origin main
   ```

### 3.2 版本日志文档格式

**必须严格遵循以下结构**：

```markdown
# wecom-auth-center vX.X.X 版本更新日志

**发布日期**: YYYY-MM-DD  
**提交数量**: XX 次提交  
**版本类型**: 版本类型说明

---

## 📋 版本概述

（简要说明本版本的主要变更）

---

## ✨ 新增特性

（列出新增的功能特性）

---

## 🐛 Bug 修复

（列出修复的问题）

---

## 🚀 性能优化

（列出性能优化项）

---

## 🔧 重构优化

（列出代码重构和架构优化）

---

## 📚 文档更新

（列出文档变更）

---

## 🔄 API 变更

（列出 HTTP 接口变更）

---

## 🔄 升级说明

（提供配置迁移与注意事项）

---

## 👥 贡献者

- Jerion (416685476@qq.com)

---

## 📄 许可证

MIT License

---

**完整提交记录**: XX 次提交  
**开发周期**: YYYY-MM-DD 至 YYYY-MM-DD (X 天)
```

### 3.3 创建 Git Tag

仅当当前项目是 Git 仓库，且版本日志与 `README.md`（含 `README.en.md`）等发布文件已提交后执行。

```bash
# 1. 创建带注释的标签
git tag -a vX.X.X -m "Release vX.X.X - 版本简要说明

（包含核心特性、技术优化、提交数量等）

详细更新日志: verchanglog/vX.X.X.md"

# 2. 推送标签到远程（推送 tag 将触发 CI 构建镜像与 Release）
git push origin vX.X.X
```

### 3.4 创建 GitHub Release

正常情况下推送 `v*` tag 后由 CI（`.github/workflows/ci.yml` 的 release job）自动完成：多平台二进制 + `SHA256SUMS` + 阿里云与 Docker Hub 镜像推送 + Release 创建。仅当 CI 不可用需手动发布时，使用 GitHub CLI 并**严格遵循以下格式结构**：

```bash
gh release create vX.X.X --title "vX.X.X" --notes "
## 🎉 wecom-auth-center vX.X.X 发布

### 📋 构建信息
- **版本**: vX.X.X
- **Commit**: <commit-hash>
- **构建时间**: YYYY-MM-DDTHH:MM:SSZ

### 📝 更新内容
- type: 提交说明 (commit-hash)
- type: 提交说明 (commit-hash)
...（列出最近 10 条比较主要的提交）

### 🐳 Docker 镜像
\`\`\`bash
# 阿里云镜像仓库（国内推荐）
docker pull registry.cn-shenzhen.aliyuncs.com/zyx3721/wecom-auth-center:X.X.X
# Docker Hub
docker pull zyx3721/wecom-auth-center:X.X.X
\`\`\`

### 📦 详细更新日志
完整的功能说明和升级指南请查看: [verchanglog/vX.X.X.md](https://github.com/zyx3721/wecom-auth-center/blob/main/verchanglog/vX.X.X.md)
"
```

### 3.5 Release 格式要点

- ✅ 标题仅为版本号：`vX.X.X`
- ✅ 包含构建信息（版本、Commit、构建时间）
- ✅ 列出最近 10 条提交记录（包含 commit hash）
- ✅ 提供阿里云与 Docker Hub 镜像拉取命令
- ✅ 链接到详细的 changelog 文档
- ✅ 使用相同的 emoji 图标（🎉📋📝🐳📦）

---

## 4. 版本号规范

遵循语义化版本（Semantic Versioning）：`主版本号.次版本号.修订号`

- **主版本号**：不兼容的 API 修改
- **次版本号**：向下兼容的功能性新增
- **修订号**：向下兼容的问题修正

示例：
- `v1.0.0` → `v1.0.1`（Bug 修复）
- `v1.0.1` → `v1.1.0`（新增功能，如 Redis 存储支持）
- `v1.1.0` → `v2.0.0`（破坏性变更，如 verify 签名算法更换）

---

## 5. 参考示例

### 5.1 版本日志参考

- 如果存在 `verchanglog/` 下历史版本文档，优先参考其格式
- 如果不存在历史版本文档，使用本文档“3.2 版本日志文档格式”作为首次版本日志模板

### 5.2 Git Tag 参考

仅当当前项目是 Git 仓库且已存在对应标签时执行：

```bash
# 查看现有标签
git tag -l
git show v1.0.0
```

### 5.3 GitHub Release 参考

- 推送 tag 后由 CI 自动创建；手动发布的 Release 严格按“3.4 创建 GitHub Release”格式

---

## 6. 检查清单

### 6.1 代码修改检查清单

- [ ] 代码修改完成，`go test ./...` 通过
- [ ] 涉及接口/配置变更时，`README.md`、`README.en.md`、`docs/manual.md`、`docs/architecture.md` 已同步
- [ ] 如果当前项目是 Git 仓库，逐文件提交到 Git
- [ ] 如果执行了提交，提交信息符合规范
- [ ] 如果当前项目不是 Git 仓库，最终回复说明已跳过 Git 提交
- [ ] **未推送到远程**（除非明确要求或正在执行版本发布流程）

### 6.2 版本发布检查清单

- [ ] 确认用户明确要求发布版本
- [ ] 确认当前项目是 Git 仓库；如果不是，仅准备文件并跳过 Git/Release 步骤
- [ ] 创建版本日志文档（`verchanglog/vX.X.X.md`）
- [ ] 版本日志格式与现有版本保持一致
- [ ] 更新 `README.md`「版本历史」并同步 `README.en.md`「Releases」；`docs/manual.md` 仅在接口、配置或项目结构变更时同步
- [ ] 如果当前项目是 Git 仓库，提交并推送所有变更
- [ ] 如果当前项目是 Git 仓库，创建 Git Tag（格式与现有标签一致）
- [ ] 如果当前项目配置了远程仓库，推送 Tag 到远程（触发 CI 自动构建镜像与 Release）
- [ ] 确认 CI 的 release job 成功：多平台产物、SHA256SUMS、阿里云与 Docker Hub 镜像推送、GitHub Release
- [ ] Release 标题仅为版本号
- [ ] Release 内容包含构建信息、提交列表、Docker 镜像、详细日志链接
- [ ] 验证 Release 页面显示正常

---

## 7. 常见问题

### Q1: 为什么修改后不需要构建？
**A**: 验证以 `go test` 与 `go vet` 为准，`go run` 即可本地演练。仅在部署生产环境或发布版本时才需要构建。

### Q2: 为什么提交后不推送？
**A**: 保持本地提交历史的灵活性，避免频繁推送未完成的工作。仅在完成一个完整功能或准备发布时才推送。

### Q3: 如何确保 Release 格式一致？
**A**: 正式发布优先由 CI 在 tag 推送后自动完成；如果需要手动发布且有历史 Release，严格参考历史 Release 页面复制其结构和格式，仅替换版本号、提交记录等变量内容；尚无历史 Release 时按本文档模板准备内容。

### Q4: 版本日志文档必须包含哪些章节？
**A**: 必须包含：版本概述、新增特性、Bug 修复、性能优化、重构优化、文档更新、API 变更、升级说明、贡献者、许可证。

### Q5: 当前目录还不是 Git 仓库怎么办？
**A**: 不主动初始化仓库，不执行提交、Tag、推送或 GitHub Release；完成代码和文档修改后，在最终回复说明已跳过 Git/发布相关步骤。只有用户明确要求初始化仓库时，才执行 `git init`。

### Q6: 改了 verify 签名算要注意什么？
**A**: 这是不兼容变更：所有已接入业务系统要同步改造，版本号必须升主版本，并在 `docs/manual.md`、`docs/client-integration.md` 与 `docs/architecture.md` 同一提交内更新。

---

## 8. 联系方式

如有疑问，请联系：
- **作者**: Jerion
- **邮箱**: 416685476@qq.com
