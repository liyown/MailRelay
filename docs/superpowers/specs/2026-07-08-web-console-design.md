# MailRelay Web Console 设计规格

## 目标

为 MailRelay 增加一个真实可用、单机部署、面向 B 端运维场景的 Web 控制台。控制台使用选定的 “Warm Operations” 视觉方向，提供状态观察、Command 管理、配置编辑、Queue/死信恢复、执行审计和运行日志，不改变 MailRelay “预声明、受限、可审计的邮件命令协议”定位。

## 产品边界

- 控制台是现有 CLI 和 SQLite 运维能力的可视化入口，不新增任意远程执行能力。
- v0.1 只面向单实例、单管理员、本机或受信反向代理部署。
- 默认监听 `127.0.0.1`；非回环地址必须显式配置。
- 不提供多租户、RBAC、SSO、组织管理、可视化 Workflow 编排器或任意 SQL 查询。
- WebUI 不直接访问 SQLite 或 YAML；所有数据和修改都经过 Go API。

## 技术架构

### 前端

新建 `console/` React SPA：

- Vite + React + TypeScript
- shadcn/ui + Radix UI
- Tailwind CSS
- TanStack Query 与 TanStack Table
- React Hook Form + Zod
- Recharts
- Lucide React

生产构建生成静态资源，由 Go `embed` 打包进 MailRelay 二进制。开发模式下 Vite 代理 `/api` 到本地 Go 服务。控制台不需要 Node.js 运行时。

### 后端

新增 `internal/web`，职责限定为：

- 提供静态控制台资源。
- 提供版本化 JSON API `/api/v1`。
- 将现有 Store、Config 和 Runtime 能力适配为只读查询或明确的管理动作。
- 统一鉴权、CSRF、请求大小、超时、错误分类和审计。

运行时继续由 `internal/app.Runtime` 持有 Store、Router 与配置。Web 层通过窄接口调用，不复制 CLI 业务逻辑，不允许 Handler 暴露 HTTP 路由。

### 启动方式

- `mailrelay run` 可依据 `web.enabled` 同时启动控制台。
- 新增 `mailrelay console`，只启动管理控制台与 API，适合独立排障。
- 默认地址 `127.0.0.1:8787`。
- `web.public_url` 仅用于展示链接，不改变监听边界。

配置新增：

```yaml
web:
  enabled: false
  address: 127.0.0.1:8787
  public_url: http://127.0.0.1:8787
  session_secret: ${MAILRELAY_WEB_SESSION_SECRET}
  admin_password_hash: ${MAILRELAY_WEB_ADMIN_PASSWORD_HASH}
  session_ttl: 8h
```

`session_secret` 与 `admin_password_hash` 都是敏感字段；API 只返回 `configured: true/false`，保存其他配置时保留原引用。

## 安全模型

- 首次启用要求 `web.session_secret` 使用环境变量引用，禁止默认值。
- 登录使用本地管理员密码的 Argon2id 哈希；配置中不保存明文密码。
- 登录成功后使用 `HttpOnly`、`SameSite=Strict`、本地非 TLS 时不标记 `Secure` 的短期 Session Cookie。
- 所有修改请求要求 CSRF Token，且只接受 JSON。
- 登录、配置保存、replay 和运行控制均写入管理审计事件。
- API 永不返回邮箱密码、Command Token、API Key、Webhook Secret、MCP 凭据或完整邮件正文。
- 配置响应仅返回是否已设置、环境变量引用名及可编辑的非敏感字段。
- 日志和错误继续使用既有安全分类，不透传供应商响应。
- 默认禁用跨域；不提供通配 CORS。

## 信息架构

### 仪表盘

- KPI：24 小时执行量、成功率、P95 耗时、活跃 Handler。
- 趋势图：执行量与成功率，可切换 24 小时、7 天、30 天。
- 运行健康：IMAP、SMTP、SQLite、Queue Worker、配置状态。
- Queue/回复：pending、running、dead、done。
- 最近事件：错误分类、Command、发生时间和恢复入口。
- 最近执行：可跳转到执行详情。

### Command

- 列表显示名称、Handler、成熟度、参数数、最后执行和启用状态。
- 支持按名称、Handler、成熟度筛选。
- 创建/编辑使用分区表单：基本信息、参数、Handler 配置、示例。
- Handler 切换时只呈现对应 schema，不显示无关字段。
- 保存前调用服务端校验；成功后使用原子配置替换和热重载。
- 删除前显示依赖关系，存在 Workflow/Queue 引用时禁止删除。

### 执行记录

- 服务端分页、时间范围、Command、Handler、状态和错误分类筛选。
- 详情抽屉显示请求 ID、脱敏参数、耗时、摘要和关联运行事件。
- 不显示原始邮件正文、Token 或供应商响应。

### Queue 与死信

- Tabs：Queue、Reply Outbox、Dead Letter。
- 显示状态、尝试次数、下次执行、lease、目标 Command 和安全失败分类。
- Replay 使用确认对话框；批量 replay 首期不提供。
- dead 项可跳转到相关执行和事件。

### 运行日志

- 展示结构化 `runtime_events`，不是读取任意系统日志文件。
- 支持 severity、phase、Command、时间范围筛选和自动刷新。
- 错误详情仅展示安全摘要。

### 邮箱与安全

- IMAP/SMTP 地址、用户名、From、Mailbox、轮询间隔。
- 密码只显示“已设置/未设置”和环境变量引用，不回显值。
- 发件人 allowlist、HTTP host allowlist、可执行根目录。
- 提供只读连接检查；不会发送测试邮件，除非用户明确点击并再次确认。

### 系统设置

- Command timeout、重试次数、退避、配置热重载、Catalog 通知。
- Web 监听、Session 生命周期和管理员密码更新。
- 版本、数据库路径、Catalog Hash、构建信息。

## API

### 只读

- `GET /api/v1/session`
- `GET /api/v1/dashboard?range=24h`
- `GET /api/v1/health`
- `GET /api/v1/commands`
- `GET /api/v1/commands/{name}`
- `GET /api/v1/executions`
- `GET /api/v1/executions/{id}`
- `GET /api/v1/jobs`
- `GET /api/v1/replies`
- `GET /api/v1/events`
- `GET /api/v1/config`
- `GET /api/v1/system`

### 修改

- `POST /api/v1/login`
- `POST /api/v1/logout`
- `POST /api/v1/config/validate`
- `PUT /api/v1/config`
- `POST /api/v1/jobs/{id}/replay`
- `POST /api/v1/replies/{id}/replay`
- `POST /api/v1/mail/check`

所有列表使用 `cursor` 或稳定 ID 分页，不使用不稳定的前端切片。API 错误格式统一为：

```json
{
  "error": {
    "code": "invalid_configuration",
    "message": "配置校验失败",
    "fields": {"commands.0.config.url": "Host 未加入 allowlist"},
    "request_id": "req_..."
  }
}
```

## 配置保存流程

1. 前端本地 Zod 校验即时反馈。
2. 服务端解析结构化配置并运行完整 `Config.Validate`。
3. 服务端生成 YAML 到同目录临时文件，权限固定为 `0600`。
4. `fsync` 临时文件并原子 rename。
5. Runtime 使用现有 parse/validate/build/atomic-swap 热重载。
6. 若热重载失败，恢复原文件并记录管理审计事件。
7. API 返回新 Catalog Hash 和配置版本。

前端不直接编辑原始 YAML。首期提供只读 YAML 预览，便于运维审查。

## 视觉系统

### 色彩

- 页面背景：`#FAF7F0`
- 主表面：`#FFFDF8`
- 主文字：`#1A1714`
- 次文字：`#6B6358`
- 分隔线：`#E4DCCB`
- 品牌/主操作：`#B33210`
- 品牌 hover：`#91290D`
- 成功：`#218739`
- 警告：`#C87A0A`
- 错误：`#C53A2A`
- 信息：`#3568A8`

状态色只用于 Badge、图标、图表和小面积提示，不铺满整张卡片。

### 字体与密度

- UI 字体：Encode Sans Variable。
- 数字与 ID：系统等宽字体。
- 页面标题 24/32，区块标题 16/24，正文 14/22，辅助信息 12/18。
- 控件高度统一为 36px；主要按钮 40px。
- 表格行高 44px；紧凑日志行高 36px。

### 间距与形状

- 4px 基础网格，常用间距 8/12/16/24/32。
- Sidebar 248px，Header 64px，内容最大宽度不设固定上限。
- 页面边距桌面 24px，超宽屏 32px。
- 控件圆角 8px，面板圆角 10px，Badge 圆角 999px。
- 面板使用 1px 边框，阴影只用于浮层、Popover、Dialog 和 Drawer。

### 统一组件规则

- 页面均使用 `PageHeader`、`FilterBar`、`DataTable`、`StatusBadge`、`EmptyState`、`ErrorState`、`ConfirmDialog`。
- 表单使用固定 label、description、error 顺序；必填标记与错误颜色一致。
- Toast 只反馈已完成动作；校验错误就地显示。
- 删除、replay、密码更新使用 Dialog；详情使用右侧 Drawer。
- Skeleton 尺寸匹配最终内容，避免页面跳动。
- 图表共享同一 tooltip、legend、网格、时间格式和状态色映射。
- 空状态提供原因和唯一主行动，不使用装饰性插画。

## 响应式

- `>=1280px`：完整 Sidebar 和多列仪表盘。
- `768–1279px`：可折叠 Sidebar，图表两列转一列。
- `<768px`：Sidebar 变 Drawer，表格提供关键列和详情抽屉；配置表单单列。
- 不尝试在手机上完整编辑复杂 Command 参数；显示提示并保留基础操作。

## 错误与恢复

- 网络断开时保留最近成功数据并显示“数据可能过期”。
- API 401 统一跳转登录；403 展示权限错误；409 展示配置版本冲突。
- 修改配置使用版本号实现乐观并发控制。
- replay 失败不移除列表项，刷新其最新状态并显示安全错误。
- 图表无数据与加载失败分开呈现。

## 测试与验收

### Go

- API Handler、鉴权、Session、CSRF、脱敏和请求限制单元测试。
- SQLite 查询与分页集成测试。
- 配置 validate/save/rollback 测试。
- Replay 和审计事件测试。
- `go test ./...`、`go test -race ./...`、`go vet ./...`。

### 前端

- Vitest + Testing Library：表单、筛选、错误状态和危险操作。
- MSW：API success/error/slow/offline 状态。
- Playwright：登录、仪表盘、Command 编辑、配置冲突、Queue replay。
- axe：关键页面无严重可访问性问题。
- 视觉 QA 对照选定的 Warm Operations 稿，桌面视口 `1440x1024`。

## 分阶段交付

### Phase 1：只读运营台

登录、Shell、仪表盘、执行记录、Queue/Reply、运行事件和系统信息。

### Phase 2：恢复操作

Queue/Reply replay、邮箱连接检查、管理审计和确认流程。

### Phase 3：配置管理

结构化配置表单、Command CRUD、服务端校验、原子保存与回滚。

Phase 1 必须独立可运行和可验证；后续阶段不能以静态 mock 代替真实 API。
