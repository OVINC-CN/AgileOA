package accessConstants

import (
	"net/http"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
)

// 系统管理与用户业务权限分开，任何系统身份都不隐含 workspace 操作。
const (
	HomeCalendar              = "home.calendar"
	EmailCallbackReceive      = "email.callback.receive"
	UserBootstrap             = "user.bootstrap"
	UserLogin                 = "user.login"
	UserLoginEmailSend        = "user.login_email.send"
	StudentLoginEmailSend     = "student.login_email.send"
	StudentServiceRead        = "student.service.read"
	UserInfo                  = "user.info"
	UserLogout                = "user.logout"
	UserChangePassword        = "user.password.change"
	NotificationList          = "notification.list"
	NotificationUnread        = "notification.unread"
	NotificationRead          = "notification.read"
	StudentNotificationList   = "student.notification.list"
	StudentNotificationUnread = "student.notification.unread"
	StudentNotificationRead   = "student.notification.read"
	UserCreate                = "user.create"
	UserSyncStudents          = "user.sync_students"
	UserList                  = "user.list"
	UserStatistics            = "user.statistics"
	UserUpdate                = "user.update"
	UserDelete                = "user.delete"
	UserRevokeSessions        = "user.sessions.revoke"
	MCPAccess                 = "mcp.access"
	// #nosec G101 -- 固定权限操作标识，不是认证凭据。
	UserAPITokenRead = "user.api_token.read"
	// #nosec G101 -- 固定权限操作标识，不是认证凭据。
	UserAPITokenWrite = "user.api_token.write"
	// #nosec G101 -- 固定权限操作标识，不是认证凭据。
	UserAPITokenDelete              = "user.api_token.delete"
	AuditRead                       = "audit.read"
	AuditDetail                     = "audit.detail"
	AuditStatistics                 = "audit.statistics"
	AuditExport                     = "audit.export"
	AuditSensitiveRead              = "audit.sensitive.read"
	AuditSensitiveExport            = "audit.sensitive.export"
	WorkspaceList                   = "workspace.list"
	WorkspaceEnum                   = "workspace.enum"
	WorkspaceCreate                 = "workspace.create"
	WorkspaceDetail                 = "workspace.detail"
	WorkspaceUpdate                 = "workspace.update"
	WorkspaceDelete                 = "workspace.delete"
	AccessWorkspace                 = "access_workspace"
	AccessCatalog                   = "access.catalog"
	AccessUserList                  = "access.user.list"
	AccessActionsRead               = "access.actions.read"
	AccessActionsUpdate             = "access.actions.update"
	AccessMyWorkspaces              = "access.my_workspaces"
	StudentSensitiveHandle          = "student.sensitive.handle"
	StudentList                     = "student.list"
	StudentDashboardList            = "student.dashboard.list"
	StudentDashboardExport          = "student.dashboard.export"
	GraduationList                  = "graduation.list"
	GraduationUpdate                = "graduation.update"
	GraduationDelete                = "graduation.delete"
	GraduationStatistics            = "graduation.statistics"
	GraduationImport                = "graduation.import"
	GraduationExport                = "graduation.export"
	ProjectList                     = "project.list"
	ProjectCreate                   = "project.create"
	ProjectUpdate                   = "project.update"
	ProjectDelete                   = "project.delete"
	ProjectStatistics               = "project.statistics"
	ProjectImport                   = "project.import"
	ProjectExport                   = "project.export"
	ParticipationList               = "project.participation.list"
	ParticipationCreate             = "project.participation.create"
	ParticipationUpdate             = "project.participation.update"
	ParticipationDelete             = "project.participation.delete"
	ParticipationImport             = "project.participation.import"
	ParticipationExport             = "project.participation.export"
	ProjectAttachmentRead           = "project.attachment.read"
	ProjectAttachmentUpload         = "project.attachment.upload"
	ProjectAttachmentDelete         = "project.attachment.delete"
	MonthlyWorkList                 = "monthly_work.list"
	MonthlyWorkCreate               = "monthly_work.create"
	MonthlyWorkUpdate               = "monthly_work.update"
	MonthlyWorkDelete               = "monthly_work.delete"
	MonthlyWorkImport               = "monthly_work.import"
	MonthlyWorkAttachmentRead       = "monthly_work.attachment.read"
	MonthlyWorkAttachmentUpload     = "monthly_work.attachment.upload"
	MonthlyWorkAttachmentDelete     = "monthly_work.attachment.delete"
	CPCMonthlyWorkList              = "cpc.monthly_work.list"
	CPCMonthlyWorkCreate            = "cpc.monthly_work.create"
	CPCMonthlyWorkUpdate            = "cpc.monthly_work.update"
	CPCMonthlyWorkDelete            = "cpc.monthly_work.delete"
	CPCMonthlyWorkImport            = "cpc.monthly_work.import"
	CPCMonthlyWorkAttachmentRead    = "cpc.monthly_work.attachment.read"
	CPCMonthlyWorkAttachmentUpload  = "cpc.monthly_work.attachment.upload"
	CPCMonthlyWorkAttachmentDelete  = "cpc.monthly_work.attachment.delete"
	CPCDashboardList                = "cpc.dashboard.list"
	CPCDashboardExport              = "cpc.dashboard.export"
	CPCPartyBranchList              = "cpc.party_branch.list"
	CPCPartyBranchCreate            = "cpc.party_branch.create"
	CPCPartyBranchUpdate            = "cpc.party_branch.update"
	CPCPartyBranchDelete            = "cpc.party_branch.delete"
	CPCPartyBranchImport            = "cpc.party_branch.import"
	CPCPartyBranchExport            = "cpc.party_branch.export"
	CPCTimelineList                 = "cpc.timeline_template.list"
	CPCTimelineDetail               = "cpc.timeline_template.detail"
	CPCTimelineCreate               = "cpc.timeline_template.create"
	CPCTimelineUpdate               = "cpc.timeline_template.update"
	CPCTimelineDelete               = "cpc.timeline_template.delete"
	CPCTimelineImport               = "cpc.timeline_template.import"
	CPCTimelineAttachmentRead       = "cpc.timeline_template.attachment.read"
	CPCTimelineAttachmentUpload     = "cpc.timeline_template.attachment.upload"
	CPCTimelineAttachmentDelete     = "cpc.timeline_template.attachment.delete"
	CPCPersonList                   = "cpc.person.list"
	CPCPersonDetail                 = "cpc.person.detail"
	CPCPersonCreate                 = "cpc.person.create"
	CPCPersonUpdate                 = "cpc.person.update"
	CPCPersonTimelineTransferReview = "cpc.person.timeline_transfer.review"
	CPCPersonDelete                 = "cpc.person.delete"
	CPCPersonImport                 = "cpc.person.import"
	CPCPersonExport                 = "cpc.person.export"
	CPCPersonStatistics             = "cpc.person.statistics"
	CPCPersonRecalculate            = "cpc.person.branch_recalculate"
	StudentDetail                   = "student.detail"
	StudentCreate                   = "student.create"
	StudentUpdate                   = "student.update"
	StudentDelete                   = "student.delete"
	StudentStatistics               = "student.statistics"
	StudentImport                   = "student.import"
	StudentExport                   = "student.export"
	ScopePublic                     = "public"
	ScopeSession                    = "session"
	ScopeSystem                     = "system"
	ScopeWorkspace                  = "workspace"
)

// 目录只在代码中维护；新增业务必须逐项登记，查询与写入不互相推导。
var catalog = []accessModels.Action{
	definition(
		EmailCallbackReceive, "接收邮件回调", "接收腾讯云邮件事件并按配置拉黑邮箱",
		"email", ScopePublic,
		endpoint("POST", "/v1/callback/email"),
	),
	definition(
		ProjectApplicationAttachmentUpload,
		"上传申报子项附件",
		"具有查看填报管理权限的当前节点处理人可上传子项附件",
		"project",
		ScopeSession,
		endpoint("POST", "/v1/workspaces/:workspace_id/project-applications/:id/attachments"),
		endpoint(
			"POST",
			"/v1/workspaces/:workspace_id/project-applications/:id/attachments/complete",
		),
	),
	definition(
		ProjectApplicationAttachmentDelete,
		"删除申报子项附件",
		"具有查看填报管理权限的当前节点处理人可删除子项附件",
		"project",
		ScopeSession,
		endpoint(
			"DELETE",
			"/v1/workspaces/:workspace_id/project-applications/:id/attachments/:file_id",
		),
	),
	definition(
		StudentApplicationAttachmentUpload,
		"上传申报子项附件",
		"仅当前节点处理人可修改申报子项私有附件，无需额外授权",
		"student_service",
		ScopeSession,
		endpoint(
			"POST",
			"/v1/workspaces/:workspace_id/student-service/project-applications/:id/attachments",
		),
		endpoint(
			"POST",
			"/v1/workspaces/:workspace_id/student-service/project-applications/:id/attachments/complete",
		),
	),
	definition(
		StudentApplicationAttachmentDelete,
		"删除申报子项附件",
		"仅当前节点处理人可修改申报子项私有附件，无需额外授权",
		"student_service",
		ScopeSession,
		endpoint(
			"DELETE",
			"/v1/workspaces/:workspace_id/student-service/project-applications/:id/attachments/:file_id",
		),
	),
	definition(
		StudentReviewAttachmentUpload,
		"上传申报子项附件",
		"仅当前节点处理人可修改申报子项私有附件，无需额外授权",
		"student_service",
		ScopeSession,
		endpoint(
			"POST",
			"/v1/workspaces/:workspace_id/student-service/project-reviews/:id/attachments",
		),
		endpoint(
			"POST",
			"/v1/workspaces/:workspace_id/student-service/project-reviews/:id/attachments/complete",
		),
	),
	definition(
		StudentReviewAttachmentDelete,
		"删除申报子项附件",
		"仅当前节点处理人可修改申报子项私有附件，无需额外授权",
		"student_service",
		ScopeSession,
		endpoint(
			"DELETE",
			"/v1/workspaces/:workspace_id/student-service/project-reviews/:id/attachments/:file_id",
		),
	),

	definition(
		ProjectApplicationList,
		"查看填报管理",
		"查看当前空间全部申报单及流转历史",
		"project",
		ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/project-applications"),
		endpoint(
			"GET",
			"/v1/workspaces/:workspace_id/project-applications/:id/attachments/:file_id/download",
		),
		endpoint("GET", "/v1/workspaces/:workspace_id/project-applications/filter-options"),
		endpoint("GET", "/v1/workspaces/:workspace_id/project-applications/:id"),
		endpoint("GET", "/v1/workspaces/:workspace_id/project-applications/:id/versions"),
	),
	definition(
		ProjectApplicationExport,
		"导出填报明细",
		"须同时具备查看填报管理权限，按当前筛选导出申报及评分子项",
		"project",
		ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/project-applications/export"),
	),
	definition(
		ProjectApplicationDelete,
		"删除填报",
		"须同时具备查看填报管理权限，按版本硬删除申报、评分、审批及附件，保留审计",
		"project",
		ScopeWorkspace,
		endpoint("DELETE", "/v1/workspaces/:workspace_id/project-applications/:id"),
	),
	definition(
		ProjectReviewUpdate, "修改审核内容", "具有查看填报管理权限的当前节点处理人可修改内容",
		"project", ScopeSession,
		endpoint("PUT", "/v1/workspaces/:workspace_id/project-applications/:id"),
	),
	definition(
		ProjectReviewApprove, "通过审核", "具有查看填报管理权限的当前节点处理人可通过，全部通过后保留原表",
		"project", ScopeSession,
		endpoint("POST", "/v1/workspaces/:workspace_id/project-applications/:id/approve"),
	),
	definition(
		ProjectReviewReject, "驳回审核", "具有查看填报管理权限的当前节点处理人可驳回，直接退回申报人",
		"project", ScopeSession,
		endpoint("POST", "/v1/workspaces/:workspace_id/project-applications/:id/reject"),
	),
	definition(
		StudentProjectList, "查看可申报奖项", "学生仅查看绑定空间允许自行申报的项目",
		"student_service", ScopeSession,
		endpoint("GET", "/v1/workspaces/:workspace_id/student-service/projects"),
	),
	definition(
		StudentApplicationList,
		"查看我的申报",
		"学生仅查看本人申报及流转历史",
		"student_service",
		ScopeSession,
		endpoint("GET", "/v1/workspaces/:workspace_id/student-service/project-applications"),
		endpoint(
			"GET",
			"/v1/workspaces/:workspace_id/student-service/project-applications/:id/attachments/:file_id/download",
		),
		endpoint(
			"GET",
			"/v1/workspaces/:workspace_id/student-service/project-applications/filter-options",
		),
		endpoint("GET", "/v1/workspaces/:workspace_id/student-service/project-applications/:id"),
		endpoint(
			"GET",
			"/v1/workspaces/:workspace_id/student-service/project-applications/:id/versions",
		),
	),
	definition(
		StudentApplicationCreate, "提交奖项申报", "学生以本人身份提交允许自行申报的项目",
		"student_service", ScopeSession,
		endpoint("POST", "/v1/workspaces/:workspace_id/student-service/project-applications"),
	),
	definition(
		StudentApplicationUpdate, "修改退回申报", "仅申报人节点可修改本人退回申报",
		"student_service", ScopeSession,
		endpoint("PUT", "/v1/workspaces/:workspace_id/student-service/project-applications/:id"),
	),
	definition(
		StudentApplicationSubmit,
		"重新提交申报",
		"申报人修改后送往下一级审批",
		"student_service",
		ScopeSession,
		endpoint(
			"POST",
			"/v1/workspaces/:workspace_id/student-service/project-applications/:id/submit",
		),
	),
	definition(
		StudentReviewList,
		"查看学生我的审核",
		"仅查看明确指派本人或本班所选任职的节点",
		"student_service",
		ScopeSession,
		endpoint("GET", "/v1/workspaces/:workspace_id/student-service/project-reviews"),
		endpoint(
			"GET",
			"/v1/workspaces/:workspace_id/student-service/project-reviews/:id/attachments/:file_id/download",
		),
		endpoint(
			"GET",
			"/v1/workspaces/:workspace_id/student-service/project-reviews/filter-options",
		),
		endpoint("GET", "/v1/workspaces/:workspace_id/student-service/project-reviews/:id"),
		endpoint(
			"GET",
			"/v1/workspaces/:workspace_id/student-service/project-reviews/:id/versions",
		),
	),
	definition(
		StudentReviewUpdate, "修改学生审核内容", "仅当前节点的处理学生可修改申报内容",
		"student_service", ScopeSession,
		endpoint("PUT", "/v1/workspaces/:workspace_id/student-service/project-reviews/:id"),
	),
	definition(
		StudentReviewApprove,
		"通过学生审核",
		"通过当前指派节点，全部通过后保留在原申报表",
		"student_service",
		ScopeSession,
		endpoint(
			"POST",
			"/v1/workspaces/:workspace_id/student-service/project-reviews/:id/approve",
		),
	),
	definition(
		StudentReviewReject, "驳回学生审核", "直接退回申报人，重新提交后从第一级审批",
		"student_service", ScopeSession,
		endpoint("POST", "/v1/workspaces/:workspace_id/student-service/project-reviews/:id/reject"),
	),

	definition(
		MCPAccess, "访问工作空间 MCP", "建立只读协议连接，业务查询仍分别检查工作空间权限",
		"mcp", ScopeSession,
		endpoint("POST", "/v1/mcp"),
		endpoint("GET", "/v1/mcp"),
		endpoint("DELETE", "/v1/mcp"),
	),
	definition(
		StudentDashboardList,
		"查看学生数据仪表",
		"查看当前空间在籍学生的专业、班长及住宿人数汇总",
		"student_dashboard",
		ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/student-dashboard"),
		endpoint("GET", "/v1/workspaces/:workspace_id/student-dashboard/fields"),
		endpoint("GET", "/v1/workspaces/:workspace_id/student-dashboard/accommodation"),
		endpoint(
			"GET",
			"/v1/workspaces/:workspace_id/student-dashboard/accommodation/filter-options",
		),
	),
	definition(
		StudentDashboardExport, "导出学生数据仪表", "独立导出当前空间在籍学生的专业表、住宿表及人数合计",
		"student_dashboard", ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/student-dashboard/export"),
		endpoint("GET", "/v1/workspaces/:workspace_id/student-dashboard/accommodation/export"),
	),
	definition(
		HomeCalendar, "查看首页节日", "无需登录，查看当日节假日与后续假期首日，不包含补班安排",
		"home", ScopePublic,
		endpoint("GET", "/v1/home/calendar"),
	),
	definition(
		CPCDashboardList, "查看党建数据仪表", "查看当前空间全部支部的在编党员数量、支委及导师汇总",
		"cpc_dashboard", ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-dashboard"),
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-dashboard/fields"),
	),
	definition(
		CPCDashboardExport, "下载党建数据仪表", "独立下载当前空间全部支部汇总及合计的 XLSX 表格",
		"cpc_dashboard", ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-dashboard/export"),
	),
	definition(
		CPCPersonRecalculate, "历史后台重算党建人员归属", "仅供查询历史审计，不提供 HTTP 接口或人工授权",
		"cpc_person", ScopeSystem,
	),
	definition(
		CPCPersonStatistics, "查看党建人员统计", "按人员读取范围及名单筛选统计发展阶段和时间线节点人数",
		"cpc_person", ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-people/statistics"),
		endpoint(
			"GET",
			"/v1/workspaces/:workspace_id/cpc-people/timeline-view/:template_id/nodes/:node_id/statistics",
		),
	),
	definition(
		CPCPersonList, "查看党建人员名单", "按书记支部范围查看名单及选项；查看党支部权限仅扩大读取范围",
		"cpc_person", ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-people"),
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-people/fields"),
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-people/timeline-fields"),
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-people/timeline-templates/list"),
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-people/timeline-view/:template_id"),
		endpoint(
			"GET",
			"/v1/workspaces/:workspace_id/cpc-people/timeline-view/:template_id/nodes/:node_id",
		),
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-people/options"),
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-people/branches"),
	),
	definition(
		CPCPersonDetail,
		"查看党建人员详情",
		"查看范围内人员详情，学生基础信息只读",
		"cpc_person",
		ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-people/:id"),
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-people/:id/timeline"),
		endpoint(
			"GET",
			"/v1/workspaces/:workspace_id/cpc-people/:person_id/timeline/nodes/:node_id/attachments/:file_id/download",
		),
		endpoint(
			"GET",
			"/v1/workspaces/:workspace_id/cpc-people/:person_id/timeline/nodes/:node_id/template-attachments/:file_id/download",
		),
	),
	definition(
		CPCPersonCreate, "新增党建人员", "关联已有学生；编辑党支部权限仅扩大新增范围",
		"cpc_person", ScopeWorkspace,
		endpoint("POST", "/v1/workspaces/:workspace_id/cpc-people"),
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-people/student-options"),
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-people/timeline-templates/create"),
	),
	definition(
		CPCPersonUpdate,
		"编辑党建人员",
		"复核原支部及目标支部，编辑党支部权限仅扩大编辑范围",
		"cpc_person",
		ScopeWorkspace,
		endpoint("PUT", "/v1/workspaces/:workspace_id/cpc-people/:id"),
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-people/timeline-templates/update"),
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-people/:id/timeline/edit"),
		endpoint("POST", "/v1/workspaces/:workspace_id/cpc-people/:id/timeline/bind"),
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-people/:id/timeline/transfer-options"),
		endpoint("POST", "/v1/workspaces/:workspace_id/cpc-people/:id/timeline/transfer-preview"),
		endpoint("POST", "/v1/workspaces/:workspace_id/cpc-people/:id/timeline/transfer"),
		endpoint("POST", "/v1/workspaces/:workspace_id/cpc-people/:id/timeline/transfer-cancel"),
		endpoint("PUT", "/v1/workspaces/:workspace_id/cpc-people/:id/timeline/nodes/:node_id"),
		endpoint(
			"POST",
			"/v1/workspaces/:workspace_id/cpc-people/:id/timeline/nodes/:node_id/reset",
		),
		endpoint(
			"POST",
			"/v1/workspaces/:workspace_id/cpc-people/:id/timeline/nodes/:node_id/skip",
		),
		endpoint(
			"POST",
			"/v1/workspaces/:workspace_id/cpc-people/:person_id/timeline/nodes/:node_id/attachments",
		),
		endpoint(
			"POST",
			"/v1/workspaces/:workspace_id/cpc-people/:person_id/timeline/nodes/:node_id/attachments/complete",
		),
		endpoint(
			"DELETE",
			"/v1/workspaces/:workspace_id/attachments/cpc-person-timeline-nodes/:id",
		),
		endpoint(
			"GET",
			"/v1/workspaces/:workspace_id/cpc-people/:person_id/timeline/nodes/:node_id/attachments/:file_id/edit/download",
		),
		endpoint(
			"GET",
			"/v1/workspaces/:workspace_id/cpc-people/:person_id/timeline/nodes/:node_id/template-attachments/:file_id/edit/download",
		),
	),
	definition(
		CPCPersonTimelineTransferReview,
		"审核时间线转移",
		"审核通过或驳回人员时间线转移，编辑党支部权限仅扩大审核范围",
		"cpc_person",
		ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-people/:id/timeline/review"),
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-people/:id/timeline/transfer-review"),
		endpoint("POST", "/v1/workspaces/:workspace_id/cpc-people/:id/timeline/transfer-review"),
		endpoint("POST", "/v1/workspaces/:workspace_id/cpc-people/:id/timeline/transfer-reject"),
	),
	definition(
		CPCPersonDelete, "删除党建人员", "硬删除党建记录，删除党支部权限仅扩大删除范围",
		"cpc_person", ScopeWorkspace,
		endpoint("DELETE", "/v1/workspaces/:workspace_id/cpc-people/:id"),
	),
	definition(
		CPCPersonImport, "导入党建人员", "下载模板并整批新增范围内人员，不覆盖学生基础信息",
		"cpc_person", ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-people/template"),
		endpoint("POST", "/v1/workspaces/:workspace_id/cpc-people/import"),
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-people/timeline-templates/import"),
	),
	definition(
		CPCPersonExport, "导出党建人员", "按已提交筛选导出读取范围内人员，导出审计提交后返回",
		"cpc_person", ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-people/export"),
	),
	definition(
		CPCPartyBranchList,
		"查看党支部",
		"查看当前工作空间的党支部、准入规则及可选书记",
		"cpc_party_branch",
		ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-party-branches"),
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-party-branches/fields"),
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-party-branches/secretaries"),
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-party-branches/:id/admission-rule"),
	),
	definition(
		CPCTimelineList, "查看时间线模板列表", "查看当前工作空间的时间线模板",
		"cpc_timeline_template", ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-timeline-templates"),
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-timeline-templates/fields"),
	),
	definition(
		CPCTimelineDetail, "查看时间线模板详情", "查看完整节点、自定义字段及非阻断提示规则",
		"cpc_timeline_template", ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-timeline-templates/:id"),
	),
	definition(
		CPCTimelineCreate,
		"新增时间线模板",
		"新增或克隆模板及完整节点定义，克隆时复制节点附件，不创建人员时间线",
		"cpc_timeline_template",
		ScopeWorkspace,
		endpoint("POST", "/v1/workspaces/:workspace_id/cpc-timeline-templates"),
		endpoint("POST", "/v1/workspaces/:workspace_id/cpc-timeline-templates/:id/clone"),
		endpoint(
			"GET",
			"/v1/workspaces/:workspace_id/cpc-timeline-templates/transfer-targets/create",
		),
	),
	definition(
		CPCTimelineUpdate,
		"修改时间线模板",
		"按版本整体保存模板、节点顺序、字段和提示规则",
		"cpc_timeline_template",
		ScopeWorkspace,
		endpoint("PUT", "/v1/workspaces/:workspace_id/cpc-timeline-templates/:id"),
		endpoint(
			"GET",
			"/v1/workspaces/:workspace_id/cpc-timeline-templates/transfer-targets/update",
		),
	),
	definition(
		CPCTimelineDelete, "删除时间线模板", "按版本硬删除无业务引用的模板，保留审计",
		"cpc_timeline_template", ScopeWorkspace,
		endpoint("DELETE", "/v1/workspaces/:workspace_id/cpc-timeline-templates/:id"),
	),
	definition(
		CPCTimelineImport, "导入时间线模板", "下载通用 JSON 导入格式并新增模板，不覆盖已有记录",
		"cpc_timeline_template", ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-timeline-templates/template"),
		endpoint("POST", "/v1/workspaces/:workspace_id/cpc-timeline-templates/import"),
	),
	definition(
		CPCTimelineAttachmentRead,
		"读取时间线节点附件",
		"查询当前空间节点附件并按需下载，读取完成审计后返回",
		"cpc_timeline_template",
		ScopeWorkspace,
		endpoint(
			"GET",
			"/v1/workspaces/:workspace_id/cpc-timeline-templates/:template_id/nodes/:node_id/attachments",
		),
		endpoint("GET", "/v1/workspaces/:workspace_id/attachments/cpc-timeline-nodes/:id/download"),
	),
	definition(
		CPCTimelineAttachmentUpload,
		"上传时间线节点附件",
		"为已保存的当前空间模板节点上传私有附件",
		"cpc_timeline_template",
		ScopeWorkspace,
		endpoint(
			"POST",
			"/v1/workspaces/:workspace_id/cpc-timeline-templates/:template_id/nodes/:node_id/attachments",
		),
		endpoint(
			"POST",
			"/v1/workspaces/:workspace_id/cpc-timeline-templates/:template_id/nodes/:node_id/attachments/complete",
		),
	),
	definition(
		CPCTimelineAttachmentDelete, "删除时间线节点附件", "删除时间线节点附件",
		"cpc_timeline_template", ScopeWorkspace,
		endpoint("DELETE", "/v1/workspaces/:workspace_id/attachments/cpc-timeline-nodes/:id"),
	),
	definition(
		CPCPartyBranchCreate, "新增党支部", "新增党支部并可选关联当前空间可用用户为书记",
		"cpc_party_branch", ScopeWorkspace,
		endpoint("POST", "/v1/workspaces/:workspace_id/cpc-party-branches"),
	),
	definition(
		CPCPartyBranchUpdate, "修改党支部", "按版本修改党支部全称、简称、书记和准入规则，或手动重排人员",
		"cpc_party_branch", ScopeWorkspace,
		endpoint("PUT", "/v1/workspaces/:workspace_id/cpc-party-branches/:id"),
		endpoint("PUT", "/v1/workspaces/:workspace_id/cpc-party-branches/:id/admission-rule"),
		endpoint("POST", "/v1/workspaces/:workspace_id/cpc-party-branches/reorder-people"),
	),
	definition(
		CPCPartyBranchDelete, "删除党支部", "按版本硬删除无业务引用的党支部，保留审计",
		"cpc_party_branch", ScopeWorkspace,
		endpoint("DELETE", "/v1/workspaces/:workspace_id/cpc-party-branches/:id"),
	),
	definition(
		CPCPartyBranchImport, "导入党支部", "下载固定 XLSX 模板并整批新增党支部，书记按账号匹配",
		"cpc_party_branch", ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-party-branches/template"),
		endpoint("POST", "/v1/workspaces/:workspace_id/cpc-party-branches/import"),
	),
	definition(
		CPCPartyBranchExport, "导出党支部", "按已提交搜索条件导出当前空间党支部，审计提交后返回",
		"cpc_party_branch", ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-party-branches/export"),
	),
	definition(
		MonthlyWorkList,
		"查看月度工作",
		"按年份和月份查看本人负责或参与的工作",
		"monthly_work",
		ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/monthly-work"),
		endpoint("GET", "/v1/workspaces/:workspace_id/monthly-work/fields"),
		endpoint("GET", "/v1/workspaces/:workspace_id/monthly-work/filter-options"),
		endpoint("GET", "/v1/workspaces/:workspace_id/monthly-work/owners"),
	),
	definition(
		MonthlyWorkCreate,
		"新增月度工作",
		"记录日期、工作项目、负责人、详细说明及多条外部链接",
		"monthly_work",
		ScopeWorkspace,
		endpoint("POST", "/v1/workspaces/:workspace_id/monthly-work"),
	),
	definition(MonthlyWorkUpdate, "修改月度工作", "仅负责人可按版本修改或转交工作", "monthly_work", ScopeWorkspace,
		endpoint("PUT", "/v1/workspaces/:workspace_id/monthly-work/:id"),
	),
	definition(MonthlyWorkDelete, "删除月度工作", "仅负责人可删除工作，保留审计", "monthly_work", ScopeWorkspace,
		endpoint("DELETE", "/v1/workspaces/:workspace_id/monthly-work/:id"),
	),
	definition(MonthlyWorkImport, "导入月度工作", "下载固定 XLSX 模板并整批新增工作记录", "monthly_work", ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/monthly-work/template"),
		endpoint("POST", "/v1/workspaces/:workspace_id/monthly-work/import"),
	),
	definition(
		MonthlyWorkAttachmentRead,
		"读取月度工作附件",
		"查看和下载本人负责或参与的工作附件",
		"monthly_work",
		ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/attachments/monthly-work"),
		endpoint("GET", "/v1/workspaces/:workspace_id/attachments/monthly-work/:id/download"),
	),
	definition(
		MonthlyWorkAttachmentUpload,
		"上传月度工作附件",
		"仅负责人可上传及确认工作附件",
		"monthly_work",
		ScopeWorkspace,
		endpoint("POST", "/v1/workspaces/:workspace_id/monthly-work/:work_id/attachments"),
		endpoint("POST", "/v1/workspaces/:workspace_id/monthly-work/:work_id/attachments/complete"),
	),
	definition(
		MonthlyWorkAttachmentDelete,
		"删除月度工作附件",
		"仅负责人可删除工作附件，可重试未完成的删除",
		"monthly_work",
		ScopeWorkspace,
		endpoint("DELETE", "/v1/workspaces/:workspace_id/attachments/monthly-work/:id"),
	),
	definition(
		CPCMonthlyWorkList,
		"查看党建月度工作",
		"按年份和月份查看本人负责或参与的工作",
		"cpc_monthly_work",
		ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-monthly-work"),
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-monthly-work/fields"),
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-monthly-work/filter-options"),
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-monthly-work/owners"),
	),
	definition(
		CPCMonthlyWorkCreate,
		"新增党建月度工作",
		"记录日期、工作项目、负责人、详细说明及多条外部链接",
		"cpc_monthly_work",
		ScopeWorkspace,
		endpoint("POST", "/v1/workspaces/:workspace_id/cpc-monthly-work"),
	),
	definition(
		CPCMonthlyWorkUpdate,
		"修改党建月度工作",
		"仅负责人可按版本修改或转交工作",
		"cpc_monthly_work",
		ScopeWorkspace,
		endpoint("PUT", "/v1/workspaces/:workspace_id/cpc-monthly-work/:id"),
	),
	definition(
		CPCMonthlyWorkDelete,
		"删除党建月度工作",
		"仅负责人可删除工作，保留审计",
		"cpc_monthly_work",
		ScopeWorkspace,
		endpoint("DELETE", "/v1/workspaces/:workspace_id/cpc-monthly-work/:id"),
	),
	definition(
		CPCMonthlyWorkImport,
		"导入党建月度工作",
		"下载固定 XLSX 模板并整批新增工作记录",
		"cpc_monthly_work",
		ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/cpc-monthly-work/template"),
		endpoint("POST", "/v1/workspaces/:workspace_id/cpc-monthly-work/import"),
	),
	definition(
		CPCMonthlyWorkAttachmentRead,
		"读取党建月度工作附件",
		"查看和下载本人负责或参与的工作附件",
		"cpc_monthly_work",
		ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/attachments/cpc-monthly-work"),
		endpoint("GET", "/v1/workspaces/:workspace_id/attachments/cpc-monthly-work/:id/download"),
	),
	definition(
		CPCMonthlyWorkAttachmentUpload,
		"上传党建月度工作附件",
		"仅负责人可上传及确认工作附件",
		"cpc_monthly_work",
		ScopeWorkspace,
		endpoint("POST", "/v1/workspaces/:workspace_id/cpc-monthly-work/:work_id/attachments"),
		endpoint(
			"POST",
			"/v1/workspaces/:workspace_id/cpc-monthly-work/:work_id/attachments/complete",
		),
	),
	definition(
		CPCMonthlyWorkAttachmentDelete,
		"删除党建月度工作附件",
		"仅负责人可删除工作附件，可重试未完成的删除",
		"cpc_monthly_work",
		ScopeWorkspace,
		endpoint("DELETE", "/v1/workspaces/:workspace_id/attachments/cpc-monthly-work/:id"),
	),
	definition(GraduationList, "查看毕业去向", "查看和筛选学生毕业去向", "graduation", ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/graduation-destinations"),
		endpoint("GET", "/v1/workspaces/:workspace_id/graduation-destinations/filter-options"),
		endpoint("GET", "/v1/workspaces/:workspace_id/graduation-destinations/fields"),
	),
	definition(GraduationUpdate, "维护毕业去向", "为学生绑定或修改唯一毕业去向", "graduation", ScopeWorkspace,
		endpoint("PUT", "/v1/workspaces/:workspace_id/graduation-destinations/:id"),
	),
	definition(GraduationDelete, "清除毕业去向", "清除学生的毕业去向", "graduation", ScopeWorkspace,
		endpoint("DELETE", "/v1/workspaces/:workspace_id/graduation-destinations/:id"),
	),
	definition(
		GraduationStatistics,
		"查看毕业去向统计",
		"按专业或去向类型统计人数及就业率",
		"graduation",
		ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/graduation-destinations/statistics"),
		endpoint(
			"GET",
			"/v1/workspaces/:workspace_id/graduation-destinations/statistics/filter-options",
		),
	),
	definition(GraduationImport, "导入毕业去向", "下载模板并按学号整批新增去向绑定", "graduation", ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/graduation-destinations/template"),
		endpoint("POST", "/v1/workspaces/:workspace_id/graduation-destinations/import"),
	),
	definition(GraduationExport, "导出毕业去向", "按已提交筛选导出全部毕业去向，审计提交后返回", "graduation", ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/graduation-destinations/export"),
	),
	definition(ProjectList, "查看评优项目", "查看和筛选评优项目", "project", ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/projects"),
		endpoint("GET", "/v1/workspaces/:workspace_id/projects/options"),
		endpoint("GET", "/v1/workspaces/:workspace_id/projects/filter-options"),
		endpoint("GET", "/v1/workspaces/:workspace_id/projects/approval-options"),
		endpoint("GET", "/v1/workspaces/:workspace_id/projects/fields"),
	),
	definition(ProjectCreate, "创建评优项目", "按项目归属年份创建项目", "project", ScopeWorkspace,
		endpoint("POST", "/v1/workspaces/:workspace_id/projects"),
		endpoint("POST", "/v1/workspaces/:workspace_id/projects/score-preview"),
	),
	definition(ProjectUpdate, "修改评优项目", "按版本修改项目名称、年份、说明及子项配置", "project", ScopeWorkspace,
		endpoint("PUT", "/v1/workspaces/:workspace_id/projects/:id"),
		endpoint("POST", "/v1/workspaces/:workspace_id/projects/:id/score-preview"),
	),
	definition(ProjectDelete, "删除评优项目", "仅硬删除无参与记录、子项配置及附件的项目", "project", ScopeWorkspace,
		endpoint("DELETE", "/v1/workspaces/:workspace_id/projects/:id"),
	),
	definition(ProjectStatistics, "查看项目年度人数", "按项目归属年份和名称统计参加人数", "project", ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/projects/statistics"),
	),
	definition(ProjectImport, "导入评优项目", "生成项目模板并整批新增项目", "project", ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/projects/template"),
		endpoint("POST", "/v1/workspaces/:workspace_id/projects/import"),
	),
	definition(ProjectExport, "导出评优项目", "按已提交筛选导出完整项目表格，审计提交后返回", "project", ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/projects/export"),
	),
	definition(ParticipationList, "查看项目参与记录", "查看参与记录及当前空间学生选择项", "project", ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/project-participations"),
		endpoint("GET", "/v1/workspaces/:workspace_id/project-participations/students"),
		endpoint("GET", "/v1/workspaces/:workspace_id/project-participations/filter-options"),
		endpoint("GET", "/v1/workspaces/:workspace_id/project-participations/detail-names"),
		endpoint("GET", "/v1/workspaces/:workspace_id/project-participations/:id"),
	),
	definition(ParticipationCreate, "绑定学生项目", "为当前空间学生新增参与记录及评分子项", "project", ScopeWorkspace,
		endpoint("POST", "/v1/workspaces/:workspace_id/project-participations"),
	),
	definition(ParticipationUpdate, "修改项目评分", "独立维护排名、总分及子项分数", "project", ScopeWorkspace,
		endpoint("PUT", "/v1/workspaces/:workspace_id/project-participations/:id"),
	),
	definition(ParticipationDelete, "删除项目参与记录", "删除已清空评分子项的参与记录", "project", ScopeWorkspace,
		endpoint("DELETE", "/v1/workspaces/:workspace_id/project-participations/:id"),
	),
	definition(ParticipationImport, "导入项目参与记录", "生成模板并整批绑定学生、项目及评分子项", "project", ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/project-participations/template"),
		endpoint("POST", "/v1/workspaces/:workspace_id/project-participations/import"),
	),
	definition(
		ParticipationExport,
		"导出获奖学生",
		"按已提交筛选导出获奖学生或含评分子项列的完整明细，审计提交后返回",
		"project",
		ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/project-participations/export"),
	),
	definition(ProjectAttachmentRead, "查看项目附件", "查看和下载项目附件", "project", ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/attachments/project"),
		endpoint("GET", "/v1/workspaces/:workspace_id/attachments/:id/download"),
	),
	definition(
		ProjectAttachmentUpload,
		"上传项目附件",
		"上传普通项目资料附件",
		"project",
		ScopeWorkspace,
		endpoint("POST", "/v1/workspaces/:workspace_id/projects/:project_id/attachments"),
		endpoint("POST", "/v1/workspaces/:workspace_id/projects/:project_id/attachments/complete"),
	),
	definition(
		ProjectAttachmentDelete,
		"删除项目附件",
		"删除项目附件及存储对象，可重试未完成的存储操作",
		"project",
		ScopeWorkspace,
		endpoint("DELETE", "/v1/workspaces/:workspace_id/attachments/:id"),
	),
	definition(
		StudentSensitiveHandle,
		"处理敏感信息",
		"查看、编辑和筛选学生敏感信息；导入和导出需同时拥有对应权限",
		"student",
		ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/students/:id/sensitive"),
		endpoint("GET", "/v1/workspaces/:workspace_id/students/sensitive-options"),
		endpoint("PUT", "/v1/workspaces/:workspace_id/students/:id/sensitive"),
	),

	definition(
		StudentList,
		"查看学生明细表",
		"查看基础信息明细及基础字段枚举，不输出敏感字段；敏感字段筛选额外授权",
		"student",
		ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/students"),
		endpoint("GET", "/v1/workspaces/:workspace_id/students/options"),
		endpoint("GET", "/v1/workspaces/:workspace_id/students/fields"),
	),
	definition(StudentDetail, "查看学生详情", "查看基础详情、头像及校外住宿明细，不授予学生敏感信息权限", "student", ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/students/:id"),
	),
	definition(StudentCreate, "添加学生", "新增基础档案，不自动授予其他操作", "student", ScopeWorkspace,
		endpoint("POST", "/v1/workspaces/:workspace_id/students"),
	),
	definition(StudentUpdate, "修改学生", "修改基础档案、头像和校外住宿明细，不授予学生敏感信息权限", "student", ScopeWorkspace,
		endpoint("PUT", "/v1/workspaces/:workspace_id/students/:id"),
		endpoint("POST", "/v1/workspaces/:workspace_id/students/:id/avatar"),
		endpoint("POST", "/v1/workspaces/:workspace_id/students/:id/avatar/complete"),
		endpoint("DELETE", "/v1/workspaces/:workspace_id/students/:id/avatar-uploads/:upload_id"),
	),
	definition(StudentDelete, "删除学生", "删除学生档案及隐私信息", "student", ScopeWorkspace,
		endpoint("DELETE", "/v1/workspaces/:workspace_id/students/:id"),
	),
	definition(
		StudentStatistics,
		"查看学生统计",
		"与明细共用筛选；敏感字段筛选额外授权，其他统计不检查处理敏感信息权限",
		"student",
		ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/students/statistics"),
	),
	definition(StudentImport, "导入学生", "模板新增，任一错误整批回滚；敏感写入额外授权", "student", ScopeWorkspace,
		endpoint("POST", "/v1/workspaces/:workspace_id/students/import"),
		endpoint("GET", "/v1/workspaces/:workspace_id/students/template"),
	),
	definition(
		StudentExport,
		"导出学生",
		"默认导出基础字段；敏感字段筛选及导出额外授权",
		"student",
		ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id/students/export"),
	),

	definition(UserBootstrap, "初始化管理员", "运维初始化全局管理员，不授予用户业务权限", "user", ScopeSystem),
	// 保留已停用任务的标识，用于历史审计筛选和中文显示。
	definition(
		UserSyncStudents,
		"同步学生账号",
		"仅用于查询历史学生账号同步审计，当前版本已停止执行",
		"user",
		ScopeSystem,
	),
	definition(
		UserLogin,
		"登录",
		"获取登录验证码，使用账号口令或一次性邮件验证码建立会话",
		"user",
		ScopePublic,
		endpoint("POST", "/v1/login"),
		endpoint("POST", "/v1/login-captcha"),
		endpoint("POST", "/v1/login-email-verify"),
	),
	definition(
		UserLoginEmailSend,
		"发送登录邮件",
		"用户名与已登记邮箱匹配后发送一次性数字验证码",
		"user",
		ScopePublic,
		endpoint("POST", "/v1/login-email"),
	),
	definition(
		StudentLoginEmailSend, "发送学生登录邮件", "空间、学号、邮箱与在籍状态匹配后发送数字验证码",
		"user", ScopePublic,
		endpoint("POST", "/v1/student-login-email"),
	),
	definition(
		StudentServiceRead, "查看个人信息", "在籍学生查看绑定空间内本人的非敏感档案，无需配置操作授权",
		"user", ScopeSession,
		endpoint("GET", "/v1/workspaces/:workspace_id/student-service"),
	),
	definition(
		UserInfo,
		"查看当前用户",
		"查看当前启用账号的基本信息",
		"user",
		ScopeSession,
		endpoint("GET", "/v1/user-info"),
	),
	definition(
		NotificationList, "查看我的通知", "仅分页查看当前登录账号收到的通知",
		"notification", ScopeSession,
		endpoint("GET", "/v1/notifications"),
	),
	definition(
		StudentNotificationList, "查看学生通知", "仅查看绑定空间内当前学生收到的通知",
		"notification", ScopeSession,
		endpoint("GET", "/v1/workspaces/:workspace_id/student-service/notifications"),
	),
	definition(
		StudentNotificationUnread, "查看学生未读通知数量", "仅统计绑定空间内当前学生的未读通知",
		"notification", ScopeSession,
		endpoint("GET", "/v1/workspaces/:workspace_id/student-service/notifications/unread"),
	),
	definition(
		StudentNotificationRead, "标记学生通知为已读", "仅将绑定空间内当前学生明确指定的通知标为已读",
		"notification", ScopeSession,
		endpoint("POST", "/v1/workspaces/:workspace_id/student-service/notifications/read"),
	),
	definition(
		NotificationUnread, "查看我的未读通知数量", "仅统计当前登录账号的未读通知",
		"notification", ScopeSession,
		endpoint("GET", "/v1/notifications/unread"),
	),
	definition(
		NotificationRead, "标记我的通知为已读", "仅将当前登录账号明确指定的通知标为已读",
		"notification", ScopeSession,
		endpoint("POST", "/v1/notifications/read"),
	),
	definition(
		UserLogout,
		"退出登录",
		"撤销当前账号的当前会话",
		"user",
		ScopeSession,
		endpoint("POST", "/v1/logout"),
	),
	definition(
		UserChangePassword,
		"修改临时密码",
		"待改密账号验证当前密码后设置新密码",
		"user",
		ScopeSession,
		endpoint("POST", "/v1/change-password"),
	),
	definition(
		UserCreate,
		"添加用户",
		"系统管理员添加普通账号",
		"user",
		ScopeSystem,
		endpoint("POST", "/v1/system/users"),
	),
	definition(
		UserList,
		"查看用户列表",
		"系统管理员查询全局账号目录",
		"user",
		ScopeSystem,
		endpoint("GET", "/v1/system/users"),
	),
	definition(
		UserStatistics,
		"查看用户统计",
		"系统管理员查询全局账号统计",
		"user",
		ScopeSystem,
		endpoint("GET", "/v1/system/users/statistics"),
	),
	definition(
		UserUpdate,
		"更新用户",
		"系统管理员更新账号状态及基本信息",
		"user",
		ScopeSystem,
		endpoint("PUT", "/v1/system/users/:id"),
	),
	definition(
		UserDelete,
		"删除用户",
		"系统管理员硬删除无引用的账号",
		"user",
		ScopeSystem,
		endpoint("DELETE", "/v1/system/users/:id"),
	),
	definition(
		UserRevokeSessions,
		"撤销用户会话",
		"撤销全部登录会话、API Token 及待用验证码",
		"user",
		ScopeSystem,
		endpoint("DELETE", "/v1/system/users/:id/sessions"),
	),
	definition(
		UserAPITokenRead, "查看用户 API Token", "系统管理员查看账号 API Token，支持再次查看",
		"user", ScopeSystem,
		endpoint("GET", "/v1/system/users/:id/api-token"),
	),
	definition(
		UserAPITokenWrite, "配置用户 API Token", "系统管理员生成或重置账号 API Token",
		"user", ScopeSystem,
		endpoint("PUT", "/v1/system/users/:id/api-token"),
	),
	definition(
		UserAPITokenDelete, "撤销用户 API Token", "系统管理员删除账号 API Token",
		"user", ScopeSystem,
		endpoint("DELETE", "/v1/system/users/:id/api-token"),
	),
	definition(
		AuditRead,
		"查询审计日志",
		"系统管理员查看普通审计目录",
		"audit",
		ScopeSystem,
		endpoint("GET", "/v1/system/audit/logs"),
	),
	definition(
		AuditDetail,
		"查看审计详情",
		"查看操作记录详情",
		"audit",
		ScopeSystem,
		endpoint("GET", "/v1/system/audit/logs/:id"),
	),
	definition(
		AuditStatistics,
		"查看审计统计",
		"系统管理员查询普通审计统计",
		"audit",
		ScopeSystem,
		endpoint("GET", "/v1/system/audit/statistics"),
	),
	definition(AuditExport, "导出审计", "保留操作，尚未提供接口且不能授予用户", "audit", ScopeSystem),
	definition(AuditSensitiveRead, "查看敏感快照", "保留操作，尚未提供接口且不能授予用户", "audit", ScopeSystem),
	definition(AuditSensitiveExport, "导出敏感快照", "保留操作，尚未提供接口且不能授予用户", "audit", ScopeSystem),
	definition(
		WorkspaceList,
		"查询工作空间",
		"系统管理员查询全部工作空间",
		"workspace",
		ScopeSystem,
		endpoint("GET", "/v1/system/workspaces"),
	),
	definition(
		WorkspaceCreate,
		"创建工作空间",
		"系统管理员初始化工作空间，不自动授予任何操作",
		"workspace",
		ScopeSystem,
		endpoint("POST", "/v1/system/workspaces"),
	),
	definition(
		WorkspaceEnum,
		"查询工作空间枚举",
		"系统管理员一次读取完整工作空间编号、名称和状态，用于本地搜索选择器",
		"workspace",
		ScopeSystem,
		endpoint("GET", "/v1/system/workspaces/enum"),
	),
	definition(
		WorkspaceDetail,
		"查看工作空间详情",
		"系统管理员查看工作空间管理信息",
		"workspace",
		ScopeSystem,
		endpoint("GET", "/v1/system/workspaces/:workspace_id"),
	),
	definition(
		WorkspaceUpdate,
		"修改工作空间",
		"系统管理员修改名称、说明及启用状态",
		"workspace",
		ScopeSystem,
		endpoint("PUT", "/v1/system/workspaces/:workspace_id"),
	),
	definition(
		WorkspaceDelete,
		"删除工作空间",
		"系统管理员硬删除没有权限或业务引用的工作空间",
		"workspace",
		ScopeSystem,
		endpoint("DELETE", "/v1/system/workspaces/:workspace_id"),
	),
	definition(
		AccessWorkspace,
		"进入工作空间",
		"进入当前工作空间；其他业务操作仍需分别授权",
		"workspace",
		ScopeWorkspace,
		endpoint("GET", "/v1/workspaces/:workspace_id"),
	),
	definition(
		AccessCatalog,
		"查看操作目录",
		"系统管理员查看代码登记的操作及接口",
		"access",
		ScopeSystem,
		endpoint("GET", "/v1/system/access/actions"),
	),
	definition(
		AccessUserList,
		"查询用户权限",
		"系统管理员查看指定工作空间的用户授权目录",
		"access",
		ScopeSystem,
		endpoint("GET", "/v1/system/access/workspaces/:workspace_id/users"),
	),
	definition(
		AccessActionsRead,
		"查看用户权限",
		"查看指定用户的操作权限",
		"access",
		ScopeSystem,
		endpoint("GET", "/v1/system/access/workspaces/:workspace_id/users/:user_id/actions"),
	),
	definition(
		AccessActionsUpdate,
		"设置用户权限",
		"系统管理员逐项勾选当前空间的操作权限，空数组撤销全部权限",
		"access",
		ScopeSystem,
		endpoint("PUT", "/v1/system/access/workspaces/:workspace_id/users/:user_id/actions"),
	),
	definition(
		AccessMyWorkspaces,
		"查看我的工作空间",
		"查看可访问的工作空间",
		"access",
		ScopeSession,
		endpoint("GET", "/v1/workspaces"),
	),
}

var (
	actionsByValue    map[string]accessModels.Action
	actionsByEndpoint map[accessModels.Endpoint]accessModels.Action
	readOnlyActions   map[string]bool
)

func init() {
	// 固定目录只建立一次索引，避免每次鉴权复制全部操作及接口。
	actionsByValue = make(map[string]accessModels.Action, len(catalog))
	actionsByEndpoint = make(map[accessModels.Endpoint]accessModels.Action)
	readOnlyActions = make(map[string]bool, len(catalog))
	for _, action := range catalog {
		actionsByValue[action.Value] = action
		readOnly := len(action.Endpoints) > 0
		for _, endpoint := range action.Endpoints {
			actionsByEndpoint[endpoint] = action
			if endpoint.Method != http.MethodGet && endpoint.Method != http.MethodHead {
				readOnly = false
			}
		}
		readOnlyActions[action.Value] = readOnly
	}
}

// Catalog 返回目录副本，避免调用方修改全局权限定义。
func Catalog() []accessModels.Action {
	items := make([]accessModels.Action, len(catalog))
	for index, item := range catalog {
		items[index] = item
		items[index].Endpoints = append([]accessModels.Endpoint{}, item.Endpoints...)
	}
	return items
}

// Lookup 仅查询代码登记的稳定标识。
func Lookup(action string) (accessModels.Action, bool) {
	item, ok := actionsByValue[action]
	if ok {
		item.Endpoints = append([]accessModels.Endpoint{}, item.Endpoints...)
	}
	return item, ok
}

// Registered 检查审计操作是否显式登记，不代表已获得任何权限。
func Registered(action string) bool {
	_, ok := actionsByValue[action]
	return ok
}

// Grantable 仅允许用户获得工作空间范围的操作，拒绝系统管理和未知标识。
func Grantable(action string) bool {
	item, ok := actionsByValue[action]
	return ok && item.Scope == ScopeWorkspace
}

// ReadOnly 仅对全部接口均为查询的操作使用共享锁，混合、保留和未知操作保持排他锁。
func ReadOnly(action string) bool {
	return readOnlyActions[action]
}

// ForEndpoint 使用集中注册后的路由模板匹配操作，不根据 URL 前缀放行。
func ForEndpoint(method, path string) (accessModels.Action, bool) {
	item, ok := actionsByEndpoint[accessModels.Endpoint{Method: method, Path: path}]
	if ok {
		item.Endpoints = append([]accessModels.Endpoint{}, item.Endpoints...)
	}
	return item, ok
}

// Label 提供操作标识对应的中文显示名称，不参与身份或权限判断。
func Label(action string) string {
	if item, ok := actionsByValue[action]; ok {
		return item.Label
	}
	return action
}

func definition(
	value, label, description, group, scope string,
	endpoints ...accessModels.Endpoint,
) accessModels.Action {
	labels := map[string]string{
		"mcp":                   "工作空间 MCP",
		"home":                  "首页",
		"email":                 "邮件",
		"notification":          "个人通知",
		"user":                  "[系统管理] 用户管理",
		"audit":                 "[系统管理] 审计日志",
		"workspace":             "[系统管理] 工作空间",
		"access":                "[系统管理] 权限管理",
		"student":               "[学生工作] 学生信息",
		"student_dashboard":     "[学生工作] 数据仪表",
		"project":               "[学生工作] 评奖评优",
		"student_service":       "学生服务",
		"graduation":            "[学生工作] 毕业去向",
		"monthly_work":          "[学生工作] 月度工作",
		"cpc_monthly_work":      "[党建工作] 月度工作",
		"cpc_dashboard":         "[党建工作] 数据仪表",
		"cpc_party_branch":      "[党建工作] 党支部",
		"cpc_timeline_template": "[党建工作] 时间线模板",
		"cpc_person":            "[党建工作] 人员名单",
	}
	return accessModels.Action{
		Value:       value,
		Label:       label,
		Description: description,
		Group:       accessModels.Option{Value: group, Label: labels[group]},
		Scope:       scope,
		Endpoints:   append([]accessModels.Endpoint{}, endpoints...),
	}
}

func endpoint(method, path string) accessModels.Endpoint {
	return accessModels.Endpoint{Method: method, Path: path}
}
