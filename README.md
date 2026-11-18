<p align="center">
  <img src="src/web/public/agile.png" alt="敏捷办公图标" height="24">
</p>

<h2 align="center">Agile OA · 敏捷办公</h2>

<p align="center">
  <a href="https://github.com/OVINC-CN/AgileOA/actions/workflows/ci.yml"><img src="https://github.com/OVINC-CN/AgileOA/actions/workflows/ci.yml/badge.svg?branch=main" alt="CI 状态"></a>
</p>

<p align="center">
  <a href="#功能">功能</a> ·
  <a href="#快速开始">快速开始</a> ·
  <a href="#部署">部署</a> ·
  <a href="CONTRIBUTING.md">编程指引</a>
</p>

Agile OA 是面向学生工作与党建工作的多工作空间管理系统，使用 Go API 和 React Web 界面。系统管理员管理账号、工作空间和授权，用户按所在工作空间的授权使用各项功能。

## 功能

- **学生信息**：维护学生档案、头像和校外住宿明细，支持筛选、统计及 XLSX 导入导出。敏感档案独立授权。
- **评奖评优**：管理年度奖项、获奖学生、评分明细和项目附件，查看历年人数统计。
- **毕业去向**：按学生维护毕业年份和去向，按专业或去向类型查看人数及就业率。
- **月度工作**：学生工作与党建工作各自记录工作、负责人、参与人、外部链接和附件，参与人支持多选，负责或参与的记录进入“我的工作”，支持时间轴浏览与批量导入，数据及权限分别隔离。
- **系统管理**：管理本地账号、工作空间和逐项授权，查询系统审计日志。

## 快速开始

准备 MySQL 8.4，并安装 [mise](https://mise.jdx.dev/) 以使用仓库固定的 Go、Node.js 和 pnpm 版本。版本以 [mise.toml](mise.toml) 为准。

克隆仓库，安装工具和依赖，再复制一份独立的后端配置。

```sh
git clone https://github.com/OVINC-CN/AgileOA.git
cd AgileOA
mise install
mise exec -- make setup
cp src/api/config.yaml src/api/config.local.yaml
```

1. 创建一个使用 `utf8mb4` 的空库，由数据库维护账号执行完整 [0001_init.sql](src/api/support-files/sql/0001_init.sql)，并按[数据库与审计分区](#数据库与审计分区)启用审计分区维护。
2. 在 `src/api/config.local.yaml` 中填写 `mysql.dsn`，使用只有 DML 权限的运行账号。全新开发库可通过 `user.bootstrap_password` 设置初始管理员临时密码，长度为 12 至 72 字节。首次登录需修改密码，管理员初始化完成后可清空该配置。
3. 在 `src/web/public/config.js` 中将 `apiBaseURL` 设为 `http://127.0.0.1:8080`，使本地前端连接下面启动的 API。

在两个终端分别启动后端和前端。

```sh
cd src/api
mise exec -- go run . api --config config.local.yaml
```

```sh
mise exec -- make web
```

打开 [本地 Web 界面](http://127.0.0.1:5173) 登录。首次使用时，在系统管理中创建工作空间，为账号授予进入空间的 `access_workspace` 以及所需的业务操作。

## 部署

### 构建产物

在仓库根目录运行 `mise exec -- make build`。前端静态文件位于 `src/web/dist`，后端二进制位于 `src/api/bin/agile-workspace`。

Docker 工作流在推送 Git tag 时构建并发布 `linux/amd64` 后端镜像，标签沿用 Git tag。镜像默认启动 API，部署时将配置以只读方式挂载到 `/app/config.yaml`，并确保容器用户 `65532:65532` 可读。容器内可用 `app.addr: 0.0.0.0:8080` 监听端口。 [Docker 工作流](.github/workflows/docker.yml)

### 运行配置

| 配置                                       | 部署要点                                                                                                                                          |
| ------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------- |
| [后端 config.yaml](src/api/config.yaml)    | 通过 `--config` 指定部署配置。后端在启动时读取 YAML，修改后需重启。生产环境使用 HTTPS 并启用 `user.cookie_secure`。                               |
| [前端 config.js](src/web/public/config.js) | 浏览器加载的公开配置，构建产物为 `config.<hash>.js`。调整配置后重新构建，同步发布 HTML 和配置文件。前后端的分页、附件和表单预校验上限应保持一致。 |

生产构建从环境变量 `API_BASE_URL` 读取 API 基础地址，未设置或为空时使用页面同源地址。
`CONTACT_EMAIL` 可覆盖前端配置中的 `contactEmail`，供用户协议及隐私协议显示。
未设置时沿用 `public/config.js` 的邮箱，示例值为 `contact@example.com`。部署时请填写实际接收请求的邮箱。

```sh
API_BASE_URL=https://api.example.com CONTACT_EMAIL=privacy@example.com mise exec -- pnpm --dir src/web build
```

构建会将 `passwordLoginEnabled` 设为 `false`，按最终配置生成 `config.<hash>.js` 并更新 HTML 引用。部署时先上传配置及其他静态资源，再更新 HTML，并保留旧哈希资源供缓存页面使用。

密码登录由后端 YAML 的 `user.password_login_enabled` 和前端 `config.js` 的 `passwordLoginEnabled` 控制，开发示例均为 `true`，前端构建产物默认为 `false`，后端部署 YAML 按需配置，前后端开关应保持一致。关闭后端开关会拒绝新的密码登录；关闭前端开关会隐藏密码登录及用户密码设置入口，新建用户仅使用邮件登录，编辑已有用户时保留其密码设置。默认会话与 Cookie 有效期为 30 天，由 `user.session_ttl: 720h` 配置，适用于组织账号和学生账号的新会话；已有会话保留原到期时间。

静态服务器将前端页面路由回退到 `index.html`。同源部署时，将 `apiBaseURL` 留空并把 `/v1`、`/health` 和 `/ready` 转发到 Go API。跨源部署使用同站地址，并在 `http.cors_allow_origins` 中配置前端的完整来源，以支持会话 Cookie。

数据库初始化、审计分区、登录密钥、后台进程及文件服务分别按下述说明准备。

### 数据库与审计分区

[0001_init.sql](src/api/support-files/sql/0001_init.sql) 是全新、空的 `utf8mb4` 数据库的完整初始化脚本，
包含全部业务表、标准审计表、索引、约束及分区维护对象；新安装只执行这一份 SQL。
由数据库维护账号人工执行，遇错立即停止核对，不使用忽略错误的执行选项。
应用运行账号仅需 DML 权限，不自动建表、迁移或创建分区。
已有数据库不得重跑基线；调整已有库前需核对目标结构与基线的差异，仓库不保留历史迁移及回填脚本。

审计只使用 `audit_events`，按请求 `start_time` 所属 UTC 日期分区，
初始化时创建 UTC 当天起的 30 个每日分区。
脚本安装 `maintain_audit_events_partitions` 过程和每日 UTC 00:05 执行的
`maintain_audit_events_partitions_daily` Event，持续补齐未来 30 天。
数据库连接及维护连接使用 UTC。启用 MySQL 的 `event_scheduler`，
保留脚本定义者的 ALTER、EXECUTE、EVENT 权限，应用运行账号无需这些权限。
可通过以下语句检查及补齐分区：

```sql
SHOW VARIABLES LIKE 'event_scheduler';
CALL maintain_audit_events_partitions();
SELECT EVENT_NAME, STATUS, TIME_ZONE, LAST_EXECUTED
FROM information_schema.EVENTS
WHERE EVENT_SCHEMA = DATABASE() AND EVENT_NAME = 'maintain_audit_events_partitions_daily';
```

分区维护不删除历史记录，没有无限兜底分区；请求开始时刻所属 UTC 分区缺失会阻断审计写入。
过期会话可由维护人员按需通过 DML 清理。

审计详情在浏览器独立 Worker 中生成 JSON 行差异，前端 `config.js` 的 `auditDiffTimeoutMs`
控制计算上限，默认 2000 毫秒。达到上限仍可切换查看完整前后 JSON。

### 登录与加密密钥

敏感审计使用 `audit.snapshot_keys` 中独立的 32 字节 Base64 密钥，`audit.snapshot_key_id` 选择当前密钥，轮换后保留历史密钥以读取旧快照。API Token 使用另一个独立的 `user.api_token_key`；可用 `openssl rand -base64 32` 生成，填写在私有部署 YAML。更换 Token 密钥前先撤销现有 Token，再重新生成。

启用邮件登录时，配置 Redis，并按 [config.yaml](src/api/config.yaml) 填写 `user.email_login` 的有效期、发送间隔及 `ses` 凭据、地域、发信地址、模板编号和超时，再开启 `enabled`。发信账号需要 `ses:SendEmail` 权限。地域与发信地址、模板一致，发信名称可空且不含冒号、尖括号或换行。

将 [登录邮件模板](docs/login-email-v3.html) 注册为已审核模板，并将 `ses.template_id` 更新为该模板编号。模板只保留 `{{LOGIN_CODE}}` 和 `{{TTL_MINUTES}}`，验证码为 6 位数字字符串，包含前导零；邮件不再包含登录链接或回跳地址。`user.email_login.max_attempts` 必须配置为 1 至 10，示例为 5；达到错误次数上限后须重新获取验证码。组织账号和学生在原登录页面发送、输入验证码并登录，重发成功后旧验证码失效。随机校验标识由发送接口返回，校验时连同验证码放入请求体；二者只保存在当前页面内存，不写入 URL、浏览器存储、日志、追踪或审计。`/login/email` 兼容入口仅返回登录页。

腾讯云验证码通过 `user.captcha` 配置，控制台启用 CaptchaAppId 强制校验和一次一密。反向代理环境配套设置 `http.trusted_proxy_cidrs`。OTLP gRPC 的主机与端口、TLS 模式及认证头分别由 `trace.endpoint`、`trace.insecure`、`trace.headers` 配置。

追踪始终开启，`trace.report_enabled: true` 时才向接收端上报。
追踪服务名优先使用非空环境变量 `OTEL_SERVICE_NAME`，未设置或为空时回退 YAML 的 `app.name`；
部署环境及其余运行参数仍从 YAML 读取。API 和 worker 可分别设置服务名，以便在追踪平台区分进程。

### 后台任务

当前 worker 处理党建人员党支部重排、站内通知投递、项目草稿自动提交、申报排名重排及排名失败通知。使用这些功能时配置 Redis 并启动 worker：

```sh
cd src/api
mise exec -- go run . worker --config config.local.yaml
```

部署二进制或镜像时使用相同的 `worker --config <部署配置>` 参数。项目保存若携带自行申报截止时间，会提交截止后 1 分钟执行的任务；超时和重试由 `project.auto_submit_timeout`、`project.auto_submit_max_retries` 控制。通知投递使用 `notification.delivery_timeout`、`notification.delivery_max_retries`，党支部重排使用 `cpc_person` 的重算参数。

worker 上报追踪时，其配置文件也需开启 `trace.report_enabled` 并填写接收端参数。
可在 worker 进程设置 `OTEL_SERVICE_NAME=agile-workspace-worker`；每次任务执行创建消费者跨度，
沿用入队时的 Trace ID，任务日志和数据库、Redis 等组件操作关联当前执行跨度。

项目公式计算的上限由后端 `project.score_*` 与前端 `projectScore*` 配套配置。
保存奖项、末级审批完成及删除已通过填报会按配置提交排名任务，
超时和重试由 `project.ranking_timeout`、`project.ranking_max_retries` 控制。
排名最终失败后通知当前空间拥有奖项编辑权限的启用账号，重新保存奖项可重试。

beat 当前无周期业务任务，无需启动；后续启用周期任务时，同一队列运行一个 beat。Redis 的 `maint_notifications_mode` 默认 `disabled`，需要维护通知时再按服务端支持情况选择 `auto` 或 `enabled`。

### 邮件回调

在部署 YAML 中将 `email.callback_sign` 设为随机密钥，并在腾讯云的回调地址末尾填写
`/v1/callback/email?sign=<URL 编码后的密钥>`。API 收到 POST 请求时逐字匹配解码后的签名。
配置留空、签名缺失、重复或不匹配时，API 返回 401。

```yaml
email:
  callback_sign: "" # 启用回调时填写随机密钥
  abnormal_events: [dropped, bounce, spamreport, unsubscribe]
```

可选事件为 delivered、dropped、bounce、open、click、spamreport、unsubscribe、deferred，值区分大小写。配置为空数组时继续保存回调，但不新增黑名单；调整配置不会删除已有黑名单。回调和永久禁发邮箱表均已包含在初始化脚本中。

### 对象存储与 CDN

启用附件时在 `cos` 配置腾讯云私有桶的 `bucket`（含 APPID）、`region`、
`secret_id`、`secret_key`、`enabled` 和 `upload_credential_ttl`，并配置 CDN 签名。
运行账号需要申请 STS 的 `GetFederationToken` 权限，及指定桶内的 `PutObject`、
`HeadObject`、`DeleteObject`；模板克隆还需来源对象的 `GetObject`。
COS、STS 使用腾讯云官方 SDK 和固定 HTTPS 端点，配置的 `request_timeout` 控制请求超时。

浏览器先向业务上传接口提交文件名、准确字节数和声明媒体类型，获取仅允许本对象
`PutObject` 的短时 STS，使用 COS JavaScript SDK 直接上传正式路径，最后调用同一路径的
`/complete` 接口确认。API 只通过 `HEAD` 核对实际大小和声明类型，随后提交 `ready` 及审计。
申请与确认均沿用原上传 action 和业务权限；临时凭据只在本次操作的内存中保留。
失败的 `pending` 和 `deleting` 附件继续占用名额，由有权限的用户显式删除；
头像失败上传可在学生编辑抽屉中删除。STS 到期不会自动清理记录，无需增加 worker 或 beat 任务。

在 COS 控制台为网页来源配置 CORS：允许部署站点的 Origin、`PUT` 方法以及
`Authorization`、`Content-Type`、`Content-Disposition`、`x-cos-security-token` 请求头，
并暴露 `ETag`。桶保持私有，上传路径使用服务端生成的十位随机目录；本流程不配置临时目录生命周期。
使用 EdgeOne 鉴权方式 A 时，使参数名、签名密钥和回源路径与 `cdn` 配置一致。
附件下载接口返回原始文件名 `filename` 和签名链接 `url`，链接有效期由 EdgeOne 鉴权规则设置。
配置 Redis 后，附件下载和头像链接按 `cdn.signing_cache_ttl` 缓存签名，默认 `10m`，命中时不续期。
缓存时间须短于 [EdgeOne 鉴权有效期](https://edgeone.ai/zh/document/46164)，给浏览器请求留出时间。
Redis 未配置或缓存访问失败时按需签发。

全新数据库执行 [0001_init.sql](src/api/support-files/sql/0001_init.sql)。
已有旧版 S3 附件结构的数据库单独执行
[0002_cos_direct_upload.sql](src/api/support-files/sql/0002_cos_direct_upload.sql)，
再部署本版本；该文件将对象键列改为 `object_key`、增加附件声明类型和头像直传记录表，
已有附件及头像的对象键和文件内容保持原值，已有库不得重跑初始化脚本。

`cos.max_upload_bytes` 与前端 `fileMaxUploadBytes`、业务导入字节上限保持一致，默认 5242880；允许类型通过 `cos.allowed_content_types` 与前端 `fileAllowedContentTypes` 配套配置。Office 预览需要完整发布构建目录中的 WASM、Worker 资源，并正确提供 JavaScript 与 WASM 内容类型，处理时限由 `filePreviewTimeoutMs` 控制。

前端上传提示通过 [config.js](src/web/public/config.js) 的 `fileContentTypeGroups` 配置一对多分组，例如 `Word: ['application/msword', 'application/vnd.openxmlformats-officedocument.wordprocessingml.document']`、`PPT: ['application/vnd.ms-powerpoint', 'application/vnd.openxmlformats-officedocument.presentationml.presentation']`。`fileAllowedContentTypes` 中每项（含通配符）都必须出现在一个分组内，同一类型不能重复归组。提示按分组配置顺序展示命中白名单的组名，每组只显示一次，例如“支持 PDF、图片、Word、Excel、PPT”。更新已有部署配置时需补充分组，再重新构建发布；上传校验继续使用 MIME 白名单。

CDN 文件读取需要允许前端来源的普通 CORS 请求。业务 API 的下载鉴权使用带 Cookie 的 CORS，
签名链接响应使用 `Cache-Control: no-store`。浏览器按 CDN 响应的缓存头读取文件正文。
例如，CDN 返回 `Cache-Control: max-age=600` 时，同一签名 URL 的正文可在缓存新鲜期内复用。
签名路径在验签前保持原样，部署验收可检查正常、篡改和过期签名。

### 前端 CDN 刷新

前端构建完成后，在仓库根目录生成刷新清单：

```sh
cdn_manifest=$(mktemp)
mise exec -- make cdn-urls CDN_ORIGIN=https://workspace.example.com > "$cdn_manifest"
```

`CDN_ORIGIN` 是前端协议及域名，`CDN_WEB_DIST` 默认 `src/web/dist`。清单逐行包含固定页面、重定向和本次构建的静态资源 URL，可提交 CDN 刷新。

页面 HTML 的缓存键忽略查询参数，兼容入口 `/login/email` 回源到 `index.html` 并禁止 HTML 缓存，立即替换历史项返回验证码登录页，访问日志不保存旧链接中的原始登录码。页面 GET 响应提供与当前 `index.html` 一致的 ETag，前端通过 `pageUpdateCheckIntervalMs` 和 `pageUpdateCheckTimeoutMs` 检查版本，发现变化后提示刷新。发布图标时一并上传 `apple-touch-icon.png`、Manifest 图标及 `site.webmanifest`。

### API 与探针

- `/v1/mcp` 提供工作空间只读 MCP 查询，复用现有账号、权限和 API。客户端仅使用 API Token，并为 POST 配置 `X-CSRF-Protection: 1`；其他受保护 API 仅使用 Cookie。 [MCP 接入](CONTRIBUTING.md#工作空间-mcp)
- `GET /health` 检查进程存活，`GET /ready` 检查服务就绪。
- 业务 API 位于 `/v1`。开启 `app.debug` 后可在 `/v1/swagger/index.html` 浏览接口。 [Swagger 定义](src/api/internal/httpd/docs/swagger.yaml)
- 填报管理删除需在工作空间授权中单独授予 `project.application.delete`，并保留 `project.application.list`；既有查看权限不会自动获得删除能力。
- `GET /v1/home/calendar?date=YYYY-MM-DD` 无需登录，提供首页当日及下一个已录入节日。初始化已包含日历表，日期资料按 [导入模板](docs/calendar-holidays-import.sql) 人工维护，也可使用 [2026、2027 年资料](docs/calendar-holidays-2026-2027.sql)。 [日历代码入口](CONTRIBUTING.md#首页日历与节日)
- 后台任务及进程启动见 [后台任务](#后台任务)。学生档案与用户账号独立维护。

## 贡献

开发流程、业务模块接入和验证方法见 [CONTRIBUTING.md](CONTRIBUTING.md)。代码、接口和前端交互遵循 [AGENTS.md](AGENTS.md)。

## 许可证

项目源码采用 [MIT License](LICENSE)。随附的第三方组件与素材适用各自的许可。
