# 开发指引

项目约束见 [AGENTS.md](AGENTS.md)，工具安装、运行配置和部署维护见 [README.md](README.md)。
本文说明当前代码入口、常用修改方法和验证方式。

## 修改流程

1. 从下表找到所属业务和公共模块，核对请求、响应、action、租户范围及并发版本。
2. 沿路由、Svc、数据库模型和前端 feature 修改完整链路，复用现有鉴权、事务和审计入口。
3. 涉及结构或契约时，同步修改初始化 SQL、Swagger、Zod 模型和配置示例。
4. 按[本地验证](#本地验证)执行对应检查，在变更说明中记录最终行为、验证结果及未验证范围。

## 代码入口

| 内容 | 入口 |
| --- | --- |
| API、worker、beat 命令 | [internal/cmd](src/api/internal/cmd/) |
| HTTP 路由、中间件和 MCP | [internal/httpd](src/api/internal/httpd/) |
| action 目录与业务鉴权 | [actions.go](src/api/internal/apps/access/accessConstants/actions.go)、[accessUtils](src/api/internal/apps/access/accessUtils/) |
| 业务模型、请求和事务 | [internal/apps](src/api/internal/apps/) 中各 app 的 Models、Routers、Svc |
| 第三方协议适配 | [internal/apis](src/api/internal/apis/) |
| 配置与共享资源 | [config](src/api/internal/config/)、[db](src/api/internal/db/)、[logger](src/api/internal/logger/)、[tracing](src/api/internal/tracing/) |
| 数据库结构 | [0001_init.sql](src/api/support-files/sql/0001_init.sql) |
| 前端页面与导航 | [router.tsx](src/web/src/app/router.tsx)、[pages](src/web/src/pages/)、[features](src/web/src/features/) |
| 公共组件与前端逻辑 | [components](src/web/src/components/)、[lib](src/web/src/lib/) |
| 构建与检查 | [根 Makefile](Makefile)、[后端 Makefile](src/api/Makefile)、[前端 package.json](src/web/package.json) |

## 后端实现

### 日志与追踪

业务日志使用 `logger.Logger.WithContext(ctx)`，由
[logger.go](src/api/internal/logger/logger.go) 的全局钩子从有效 OpenTelemetry 上下文补充
`trace` 和 `span_id`，分别关联当前追踪与跨度；无有效追踪上下文时省略这两个字段。
请求日志与组件异常日志复用同一钩子。
追踪资源在 [tracing.go](src/api/internal/tracing/tracing.go) 初始化，服务名优先读取非空
`OTEL_SERVICE_NAME`，未设置或为空时使用 `config.Config.App.Name`，不改写应用配置。

### 字段、接口与事务

新增资源可从相近业务的 Constants 和 Models 开始。例如：
[studentConstants](src/api/internal/apps/student/studentConstants/) 定义学生字段目录，
[projectModels](src/api/internal/apps/project/projectModels/) 分别组织存储、请求和响应模型。

数据库维护的时间字段使用只读标记：

```go
CreatedAt time.Time `gorm:"->;autoCreateTime:false"`
UpdatedAt time.Time `gorm:"->;autoUpdateTime:false"`
```

写入后在原事务内回读，再生成响应和审计快照。
接口接入按以下顺序进行：

1. 在 [actions.go](src/api/internal/apps/access/accessConstants/actions.go) 登记范围、方法和完整路径。
2. 在 [routers.go](src/api/internal/httpd/routers.go) 对应的 Echo Group 注册路由和中间件。
3. 在所属 Routers 提取参数并调用 Svc，使用 [response](src/api/internal/response/) 返回结果。
4. 在 Svc 通过 `accessUtils.Workspace` 执行业务事务，以 `principal.Workspace.ID` 限定租户；额外权限使用 `accessUtils.Require`。
5. 补全 Swagger 注释，并连接前端 API、Zod 模型和导航权限。

系统管理和自助事务分别使用 `System`、`Session`。
[authorization.go](src/api/internal/apps/access/accessUtils/authorization.go) 负责身份复核、锁和审计组合；
查询与写入锁的选择由 action 目录的 `ReadOnly` 判断。
后台事务使用 `LockWorkspace`，独立只读业务事务可参考 `ReadWorkspace`。

### 审计与删除

业务通过 [auditSvc/write.go](src/api/internal/apps/audit/auditSvc/write.go) 的 `Write`、`Failure`
保存 `auditModels.Event`，由审计模块转换成 `audit_events` 记录。
普通快照从固定字段目录提取，敏感快照沿用同一模块的加密流程。
解密关联上下文通过 `SensitiveAssociatedData` 恢复编码，不直接使用数据库 JSON 文本。

[query.go](src/api/internal/apps/audit/auditSvc/query.go) 组织列表、统计、筛选目录和详情：
列表在标量条件下计数、选取复合键，再关联读取当前页的安全字段；详情读取完整普通快照。
[login.go](src/api/internal/apps/audit/auditSvc/login.go) 的 `LatestLogins`、`LoginTrend`
供账号管理读取登录汇总。前端审计入口在 [audit-page.tsx](src/web/src/pages/audit-page.tsx)，
JSON 差异使用独立 Worker，长整数解析使用 `lossless-json`。

修改删除逻辑时，从初始化 SQL 的外键关系查出全部受影响记录，再组织原事务中的
权限与版本复核、下游快照、敏感明细和对象清理信息，最后执行主记录删除。
私有对象的状态流转和提交后清理参考
[attachmentSvc](src/api/internal/apps/attachment/attachmentSvc/)。

### 异步任务

API 侧先在业务事务中保存任务所需状态和操作审计，由 app 的 Task 编码载荷，
再到 [worker.go](src/api/internal/crond/worker/worker.go) 注册处理函数。
任务执行时根据业务编号读取最新状态。
入队载荷通过 `trace_context` 传递标准 Trace Context，
[worker/tracing.go](src/api/internal/crond/worker/tracing.go) 在任务日志和参数校验前统一恢复父追踪，
为每次执行创建消费者跨度；处理函数直接使用传入上下文，不重复恢复或创建同名跨度。
任务失败记录异常和错误状态，异常退出在结束跨度后继续交给队列恢复及重试。
后台事务与提交后日志可参考 [cpcpersonSvc](src/api/internal/apps/cpcperson/cpcpersonSvc/)。
进程运行和超时、重试配置见 [后台任务](README.md#后台任务)。

### SQL 维护

[0001_init.sql](src/api/support-files/sql/0001_init.sql) 保存当前完整结构，
包括 `audit_events`、UTC 分区维护过程和每日 Event。
修改表结构时直接编辑对应的 `CREATE TABLE` 定义，不在文件尾追加结构迁移或数据回填。
初始化执行方式和分区维护见 [数据库与审计分区](README.md#数据库与审计分区)。

静态核对时对照 Models 和查询，逐项检查字段类型、默认值、主键、索引列顺序、
复合租户外键、级联动作及 CHECK；同时核对建表依赖顺序。
与数据库维护时间有关的响应和快照，再检查事务内回读位置。
明确要求数据库验收时，使用独立空库执行整份基线，并验证分区维护。

### 查询与执行计划

先从实际 GORM 查询及参数生成代表性 SQL，将 EXPLAIN 文件放到系统临时目录。
对同一条件分别检查列表、总数、统计、导出和关联读取，覆盖存在匹配记录的高低选择性条件、
多选、时间边界和深分页。记录所选分区、访问类型、索引及其使用列、预估扫描行数、
覆盖索引、临时表和 filesort，并说明结果对应的数据规模与参数范围。

索引定义从基线核对，查询入口例如：

| 场景 | 查询入口 | 重点核对 |
| --- | --- | --- |
| 空间授权用户与候选 | [accessSvc/users.go](src/api/internal/apps/access/accessSvc/users.go) | `idx_users_access_search` 与 `access_permissions` 复合主键；筛选、总数和选页使用相同授权条件 |
| 审计列表、统计与详情 | [auditSvc/query.go](src/api/internal/apps/audit/auditSvc/query.go) | UTC 分区裁剪、标量筛选索引、当前页复合键回读和详情唯一索引 |
| 最近登录与登录趋势 | [auditSvc/login.go](src/api/internal/apps/audit/auditSvc/login.go) | `idx_audit_events_login`、`idx_audit_events_login_time` |
| 申报流程同步与排名 | [projectSvc](src/api/internal/apps/project/projectSvc/) | 空间、项目、状态组合条件及学生关联 |
| 申报历史正文 | [application_versions.go](src/api/internal/apps/project/projectSvc/application_versions.go) | 操作、空间、对象编号及成功结果条件 |

索引定义只说明可选访问路径，实际覆盖情况以本次 EXPLAIN 为准。
业务表出现 `ALL` 时继续调整；分页派生表有限行物化需要与业务全表扫描分别判断。
精确计数和聚合仍与匹配范围有关，不能仅凭覆盖索引推断固定开销。

### 工作空间 MCP

先完成原 GET 路由、action 和 Swagger，再在
[mcp_registrations.go](src/api/internal/httpd/mcp_registrations.go) 添加工具名称和路径。
[mcp_catalog.go](src/api/internal/httpd/mcp_catalog.go) 校验登记并生成模型，
[mcp_arguments.go](src/api/internal/httpd/mcp_arguments.go) 转换参数，
[mcp_dispatch.go](src/api/internal/httpd/mcp_dispatch.go) 复用原 HTTP 处理链路。
客户端连接方式见 [API 与探针](README.md#api-与探针)。

调试时先执行 `initialize` 和 `tools/list`，调用 `get_workspaces` 获取可访问空间，
再把 `workspace_id` 显式传给业务查询。多选参数使用数组。
相关验证入口为 [mcp_test.go](src/api/internal/httpd/mcp_test.go)，
覆盖工具范围、参数边界、身份隔离、来源检查、响应限制和审计上下文。

## 前端实现

### API、筛选与缓存

在所属 `features` 中维护 API 方法和 Zod 模型，统一调用 [api.ts](src/web/src/lib/api.ts)。
查询键加入工作空间、业务对象及适用的学生身份；URL 状态通过
[navigation-tabs.ts](src/web/src/lib/navigation-tabs.ts) 恢复。

[useFilterForm](src/web/src/lib/filter-form.ts) 接收当前已提交条件，保存文本草稿，
通过 `changeFilters` 仅合并本次选项或日期变化。搜索和重置提交完整条件；
`hasUnappliedChanges` 交给公共筛选面板提示待搜索内容。学生页面在更多筛选中修改状态和专业。
学生信息和党建人员页面展示可移除的已应用条件，并为更多筛选提供条件计数。
已应用条件包含学生的默认在籍状态和党建人员的默认在编状态。
党建人员页面移除时间线条件时保留其他待搜索修改。
[filterParams](src/web/src/lib/filter-params.ts) 生成重复多选参数。
列表、统计和导出接到同一份已提交条件，条件变化时重置分页。
按钮 loading 和请求后缓存刷新通过
[useApiAction/useApiMutation](src/web/src/lib/api-action.ts) 连接。

### 公共组件

| 内容 | 入口 |
| --- | --- |
| 语义颜色、尺寸、字体及系统外观 | [index.css](src/web/src/index.css) |
| 输入、自动增高文本及文件选择 | [input.tsx](src/web/src/components/ui/input.tsx)、[auto-textarea.tsx](src/web/src/components/ui/auto-textarea.tsx)、[file-upload-button.tsx](src/web/src/components/ui/file-upload-button.tsx) |
| 下拉、日期及日期时间 | [search-select.tsx](src/web/src/components/ui/search-select.tsx)、[date-picker.tsx](src/web/src/components/ui/date-picker.tsx)、[date-time-picker.tsx](src/web/src/components/ui/date-time-picker.tsx) |
| 筛选、统计、表格及分页 | [filter-panel.tsx](src/web/src/components/filter-panel.tsx)、[metric-card.tsx](src/web/src/components/metric-card.tsx)、[management-table.tsx](src/web/src/components/management-table.tsx)、[management-pagination.tsx](src/web/src/components/management-pagination.tsx) |
| 表格行操作及更多操作 | [table-action-button.tsx](src/web/src/components/table-action-button.tsx)、[table-actions-menu.tsx](src/web/src/components/table-actions-menu.tsx)，共用 `table-action` 按钮样式 |
| Drawer、底部分区及提示 | [drawer.tsx](src/web/src/components/ui/drawer.tsx)、[drawer-tabs.tsx](src/web/src/components/ui/drawer-tabs.tsx)、[tooltip.tsx](src/web/src/components/ui/tooltip.tsx) |
| 桌面和移动导航 | [side-navigation-layout.tsx](src/web/src/components/side-navigation-layout.tsx)、[mobile-navigation.tsx](src/web/src/components/mobile-navigation.tsx) |
| 表格列、空值、时间和身份 | [table-columns.ts](src/web/src/lib/table-columns.ts)、[record-time-columns.tsx](src/web/src/lib/record-time-columns.tsx)、[format.ts](src/web/src/lib/format.ts) |
| 图表提示和数量轴 | [chart-tooltip-table.tsx](src/web/src/components/chart-tooltip-table.tsx)、[chart-count-tick.tsx](src/web/src/components/chart-count-tick.tsx) |

表单与详情使用 Drawer 内容滚动模式，全屏导航使用 `scrollMode="viewport"`。
标题宽度不足时整行省略，通过公共 Tooltip 查看完整内容。包含对象标识的副标题使用
`truncateDescription` 启用同样的行为。
Drawer 默认在底部提供「关闭」。通过 `footerDismiss` 自定义取消或返回操作，
刷新和重试放入 `footerRefresh`，业务操作放入 `footer`，组件按此顺序渲染。
[通知抽屉](src/web/src/features/notification/notification-center.tsx)的范围按钮与 `DrawerTabs`
共用 [NavigationButtonGroup](src/web/src/components/ui/navigation-button-group.tsx) 的分组外观、悬停和选中高亮。
右上角使用无边框 X 图标关闭按钮，可通过 `closeLabel` 自定义无障碍名称。底部关闭按钮使用纯文字。
并列分区通过 `DrawerTabs`、`footerNavigation` 接到底部，
切换时保留原表单状态。导航修改同时连接 `features/*/navigation.ts`，并核对
[cdn-urls.ts](src/web/scripts/cdn-urls.ts) 及其覆盖验证。
关闭和取消按钮统一使用公共 `Button` 的 `cancel` 样式，启用及悬停时背景透明。

学生信息列表将姓名固定在左侧首列，学号紧随其后。有详情权限时，点击姓名打开学生详情抽屉。
学生档案的编辑与详情通过 [studentRecordTabs](src/web/src/features/student/fields.ts)
共用学籍资料、联系住宿、敏感信息三个分区。敏感分区仅向有处理权限的用户开放，
进入详情的敏感分区时自动读取并展示原始值。
学生服务的个人信息由 [学生档案接口](src/api/internal/apps/student/studentSvc/profile.go) 提供，
仅返回本人编号、学号、姓名、年级、状态、学历、专业、班级和班内任职。

业务表格通过 `ManagementTable` 的 `primaryColumn` 指定首列及详情操作，统一响应式宽度、
溢出提示和叠在文字上的加载指示器。奖项管理将项目固定在左侧首列，使用
`年份 · 项目名称` 展示，点击打开项目详情抽屉。
获奖学生将姓名固定在左侧首列，学号放在第二列。点击姓名打开该条获奖记录的参与评分详情。
填报管理和学生服务的我的审核将姓名固定在左侧首列，学号放在第二列。
我的申报将 `年份 · 项目名称` 固定在左侧首列，三个视图均通过首列打开对应申报详情。
新建申报与申报详情共用底部的主要信息、审批流程和流转记录分区，切换时保留表单草稿。
学生信息、奖项管理、获奖学生和填报管理的行删除入口统一放在更多操作菜单中。
毕业去向将姓名固定在左侧首列，有编辑权限时点击姓名打开绑定或编辑抽屉，清除入口放在更多操作菜单中。
党建人员视图将姓名固定在左侧首列，有详情权限时点击姓名打开党建人员详情。
时间线视图将姓名固定在左侧首列，点击打开当前节点详情，操作按详情、编辑、时间线排列。
学生专业表、住宿信息表和支部党员表使用公共 `Table` 的 `pinFirstColumn` 固定首个可见列。
党支部简称和时间线模板名称固定在各自表格的左侧首列，点击打开详情。
党支部、时间线模板及学生和党建月度工作通过更多操作菜单删除，模板克隆也放在该菜单中。
系统管理的用户表将 ID 固定在左侧首列，点击打开编辑抽屉。撤销会话和删除放在更多操作菜单中。
工作空间表固定名称列，点击打开编辑抽屉，删除入口放在更多操作菜单中。描述单独成列，超宽时省略并显示溢出提示。
权限管理固定用户列，点击打开权限配置抽屉。审计日志固定 ID 列，点击打开审计详情，桌面端在右侧固定操作列。

下拉的排序、分组和置顶入口由 [select-options.ts](src/web/src/lib/select-options.ts) 处理；
使用 `defaultValue` 连接字段重置目标。日期与日期时间共用
[CalendarPanel](src/web/src/components/ui/calendar-panel.tsx)，弹层确认后再提交草稿。
长文本结合 `min-w-0`、`wrap-anywhere`、`DetailValue` 和公共 Tooltip 处理。

### 表格文件与附件

后端工作簿从 [spreadsheet](src/api/internal/spreadsheet/) 接入，结合业务 Constants 生成固定字段和说明页。
前端预检、工作簿处理与保存分别使用 [file-upload.ts](src/web/src/lib/file-upload.ts)、
[spreadsheet.ts](src/web/src/lib/spreadsheet.ts) 和 [download.ts](src/web/src/lib/download.ts)。

附件业务从 [attachmentSvc](src/api/internal/apps/attachment/attachmentSvc/) 进入，
COS 和 STS 协议位于 [tencentcos](src/api/internal/apis/tencentcos/)。
[object.go](src/api/internal/apps/attachment/attachmentUtils/object.go) 为新对象键统一添加 `cos.prefix`。
上传凭据、确认、下载、删除和克隆来源均使用已登记的完整对象键。
[cos.go](src/api/internal/storage/cos.go) 提供共享对象复制与删除，
[cdn.go](src/api/internal/storage/cdn.go) 生成签名下载地址，并在 Redis 配置后复用缓存签名。
前端 [cos-upload.ts](src/web/src/lib/cos-upload.ts) 统一申请、COS 简单直传和完成确认，
申请响应不可写入查询缓存或浏览器存储。上传失败保留 pending 并占用名额，沿用显式删除。
[ApplicationDrawer](src/web/src/features/project/application-drawer.tsx) 在有未保存内容时，
通过确认抽屉说明保存范围，再由公共文件按钮选择文件并立即执行保存、上传；
内容未修改时直接上传。取消确认或选文件不保存，保存后的上传失败保留已保存内容并明确提示。
[Vite](src/web/vite.config.ts) 从 COS SDK 发布包的 `index.js` 构建，使用
[pnpm 配置](src/web/pnpm-workspace.yaml) 固定的 XML 解析器版本。
头像入口为 [avatar.go](src/api/internal/apps/student/studentSvc/avatar.go)，
待完成记录保存在 `student_avatar_uploads`，完成前保留旧头像。前端使用公共文件按钮，
预览由 [file-preview.ts](src/web/src/lib/file-preview.ts) 识别类型，
[attachment-preview.ts](src/web/src/lib/attachment-preview.ts) 按需加载 Flyfish 渲染器。
每次打开预览时调用下载接口完成鉴权和审计，再按 CDN 响应缓存头读取正文。
预览与下载使用同一个 File。分页 Word、PowerPoint 和 PDF 默认适应整页，连续排版的旧版 Word 适应宽度。Excel 按视口宽度适度缩小，上限为原始比例。图片按视口缩放。
Vite 将 Flyfish 运行资源发布到 `dist/extra-assets/vendor`，资源清单位于 `dist/extra-assets`。
项目汇集的预览许可文件输出到 `dist/extra-assets/licenses/file-viewer`。
修改解析器时检查各格式的实际渲染、分页及工作表切换，
并检查构建产物中的 Worker、WASM、字体及第三方许可，验证超时、请求中止和资源释放。

## 业务修改入口

| 业务 | 后端 | 前端 |
| --- | --- | --- |
| 账号、会话、API Token 和登录 | [user](src/api/internal/apps/user/) | [user](src/web/src/features/user/)、[student-service](src/web/src/features/student-service/) |
| 工作空间与授权 | [workspace](src/api/internal/apps/workspace/)、[access](src/api/internal/apps/access/) | [workspace](src/web/src/features/workspace/)、[access](src/web/src/features/access/) |
| 学生档案与毕业去向 | [student](src/api/internal/apps/student/)、[graduation](src/api/internal/apps/graduation/) | [student](src/web/src/features/student/)、[graduation](src/web/src/features/graduation/) |
| 项目、获奖及申报 | [project](src/api/internal/apps/project/) | [project](src/web/src/features/project/) |
| 月度工作 | [monthlywork](src/api/internal/apps/monthlywork/) | [monthly-work](src/web/src/features/monthly-work/) |
| 学生与党建仪表 | [studentdashboard](src/api/internal/apps/studentdashboard/)、[cpcdashboard](src/api/internal/apps/cpcdashboard/) | [student-dashboard](src/web/src/features/student-dashboard/)、[cpc-dashboard](src/web/src/features/cpc-dashboard/) |
| 党支部、人员及时间线 | [partybranch](src/api/internal/apps/partybranch/)、[cpcperson](src/api/internal/apps/cpcperson/)、[cpctimeline](src/api/internal/apps/cpctimeline/) | [party-branch](src/web/src/features/party-branch/)、[cpc-person](src/web/src/features/cpc-person/)、[cpc-timeline](src/web/src/features/cpc-timeline/) |
| 通知、邮件回调与审计 | [notification](src/api/internal/apps/notification/)、[email](src/api/internal/apps/email/)、[audit](src/api/internal/apps/audit/) | [notification](src/web/src/features/notification/)、[audit-page.tsx](src/web/src/pages/audit-page.tsx) |

账号认证同时检查中间件身份、会话事务和
[email_cache.go](src/api/internal/apps/user/userSvc/email_cache.go) 的验证码原子操作。
前端发送与校验入口在 [login-page.tsx](src/web/src/pages/login-page.tsx)、
[email-login-verification.tsx](src/web/src/features/user/email-login-verification.tsx)，
六格输入复用 [verification-code-input.tsx](src/web/src/components/ui/verification-code-input.tsx)。

申报变更重点连接 `projectSvc` 的 `application_reset.go`、`application_editing.go`、
`application_delete.go` 和 `application_versions.go`，分别处理流程重建、编辑权限、删除和审计正文还原。
分数计算共用 `score_calculation.go`、`score_formula.go`，排名由 `ranking_enqueue.go` 接到
`projectTask/ranking.go` 和 `projectSvc/ranking.go`；失败通知位于 `ranking_notifications.go`。
前端对应 `application-drawer.tsx`、`score-calculation-editor.tsx`、`score-calculation-detail.tsx`
及 `application-content-versions.tsx`。修改时联动原事务、Task 和配置读取位置。
[项目表单](src/web/src/features/project/project-editor.tsx)的新建和编辑共用底部分区，
包含主要信息、申报设置和分数计算。
项目详情的自行申报与总分计算共用状态标签，排名维度逐项展示为标签。

党支部规则变更沿候选范围、人员归属和后台重排检查；时间线变更沿模板、人员节点、异常、
筛选、导出及转移映射检查。转移预览、发起及审核仅以来源时间线异常阻断，目标异常在预览中
展示并按实际状态保存。通知从业务事务连接 notification 模块，再由前端
[notification/navigation.ts](src/web/src/features/notification/navigation.ts) 处理已读和对象定位。
通知卡片右下角按需显示查看详情，未读通知额外显示标为已读。
时间线详情和转移预览提供 `person_name` 与 `student_no`，通知直达审核时也使用接口返回的
姓名和学号生成抽屉页头。
[时间线模板表单](src/web/src/features/cpc-timeline/template-editor.tsx)从底部切换节点配置与转移配置。
模板名称和说明在节点配置中编辑。转移映射面板填满可用高度，并在并排布局中分别滚动两侧。

### 首页日历与节日

后端查询入口为 [homeSvc/calendar.go](src/api/internal/apps/home/homeSvc/calendar.go)，
前端日期计算位于 [features/home/calendar.ts](src/web/src/features/home/calendar.ts) 和
[lib/calendar.ts](src/web/src/lib/calendar.ts)。资料维护见 [API 与探针](README.md#api-与探针)。

节日背景的风格参考 [国庆节插图](src/web/public/extra-assets/imgs/holidays/国庆节.png)，
生成提示词位于 [holiday-illustrations-2027-prompts.txt](docs/holiday-illustrations-2027-prompts.txt)。
资源按节日名称连接，修改后检查中央留白、正文裁切、窄屏和跨日刷新。

## 配置修改

后端字段从 [internal/config](src/api/internal/config/) 的定义、解析和校验进入，
同步更新 [config.yaml](src/api/config.yaml)。前端连接
[lib/config.ts](src/web/src/lib/config.ts) 与 [public/config.js](src/web/public/config.js)。
[vite.config.ts](src/web/vite.config.ts) 读取构建环境变量并更新配置文件名和 HTML 引用。
部署字段共用 [地址与邮箱校验](src/web/src/lib/deployment-config.ts)，环境变量及默认值见 [运行配置](README.md#运行配置)。

涉及分页、文件、表单或选项上限时，核对前后端对应参数。
上传提示由 [file-upload.ts](src/web/src/lib/file-upload.ts) 的 `uploadHint`
读取 `fileContentTypeGroups` 和白名单生成；变更 MIME 时同时更新分组与校验。

## 本地验证

仅文档或 SQL 整理时，核对本地链接、脚本及配置引用、结构差异，并执行 `git diff --check`。
前端代码变更在根目录执行完整检查：

```sh
pnpm --dir src/web check
```

`check` 依次执行 `check:static` 和 `test`，包括 Prettier、ESLint、TypeScript、生产构建及 Vitest。
排查失败时可单独执行 `format:check`、`typecheck`、`lint`、`build`、`test`。

后端代码变更执行静态检查；接口变化时先生成 Swagger：

```sh
make -C src/api swagger
make -C src/api swagger-check format-check lint build
```

收到明确的后端动态测试要求后，再执行 `make -C src/api test` 或指定测试包。
根目录 `make check` 还会执行后端动态测试，不用作默认后端静态检查入口。
当前 [CI](.github/workflows/ci.yml) 的前端矩阵执行 `check`、`test`，汇总为
`Web quality and build`；后端执行 `lint`、`build`。

### 数据库集成验收

明确要求集成验收时，按 README 准备独立空库、DML 运行账号、测试 YAML 和虚构数据，
由维护账号执行一次完整 `0001_init.sql`。测试包的数据库名称后缀如下：

| 业务 | 测试库后缀 | Go 测试包 |
| --- | --- | --- |
| 账号与会话 | `_user_test` | `./internal/apps/user/userSvc` |
| 工作空间和授权 | `_access_test` | `./internal/apps/access/accessSvc` |
| 学生信息 | `_student_test` | `./internal/apps/student/studentSvc` |
| 毕业去向 | `_graduation_test` | `./internal/apps/graduation/graduationSvc` |
| 评奖评优 | `_project_test` | `./internal/apps/project/projectSvc` |
| 月度工作 | `_monthly_work_test` | `./internal/apps/monthlywork/monthlyworkSvc` |
| 个人通知 | `_notification_test` | `./internal/apps/notification/notificationSvc` |
| 党建人员与时间线 | `_cpc_person_test` | `./internal/apps/cpcperson/cpcpersonSvc` |

在 `src/api` 中运行对应包：

```sh
go test -race <测试包> -args --config <测试配置文件>
```

登录入口使用同一客户端 IP 连续发送请求，验证每个请求都按原有业务规则处理。
附件单元验证使用虚构 COS 和 STS；项目集成验收在本机临时 HTTP 服务中运行官方 SDK，不使用真实存储凭据。
权限、事务和删除变更检查跨租户、撤权复核、并发版本、删除保护及审计失败回滚。
查询变更同时完成[执行计划验证](#查询与执行计划)。
分区验收入口为 [check-audit-partitions.py](src/api/scripts/check-audit-partitions.py)，
用于独立容器中的 `_user_test` 数据库，需先运行账号集成用例生成虚构审计记录。
