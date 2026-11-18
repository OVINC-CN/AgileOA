package httpd

import (
	"github.com/labstack/echo/v4"
	echoSwagger "github.com/swaggo/echo-swagger"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessRouters"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentRouters"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditRouters"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcdashboard/cpcdashboardRouters"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonRouters"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpctimeline/cpctimelineRouters"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/email/emailRouters"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/graduation/graduationRouters"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/home/homeRouters"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/monthlywork/monthlyworkRouters"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationRouters"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/partybranch/partybranchRouters"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectRouters"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/student/studentRouters"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/studentdashboard/studentdashboardRouters"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userRouters"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/workspace/workspaceRouters"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
)

func serveHealthz(engine *echo.Echo) {
	// 文档和探针路由集中注册，不提供内部状态查询接口。
	if config.Config.App.Debug {
		engine.GET("/v1/swagger/*", echoSwagger.WrapHandler)
	}
	engine.GET("/health", homeRouters.Health)
	engine.GET("/ready", homeRouters.Ready)

	// 首页日历只读公开节日资料，允许未登录访问。
	home := engine.Group("/v1/home")
	home.GET("/calendar", homeRouters.Calendar)

	// 邮件回调独立接收云端事件，不使用账号会话或浏览器跨站写入标识。
	callback := engine.Group("/v1/callback")
	callback.POST("/email", emailRouters.Callback)

	// 账号路由仅注册路径和处理函数，鉴权与跨站保护由全局中间件统一处理。
	session := engine.Group("/v1")
	session.POST("/mcp", MCP)
	session.GET("/mcp", MCP)
	session.DELETE("/mcp", MCP)
	changeLimiter := passwordLimiter(accessConstants.UserChangePassword)
	emailSendLimiter := passwordLimiter(accessConstants.UserLoginEmailSend)
	studentEmailLimiter := passwordLimiter(accessConstants.StudentLoginEmailSend)
	emailVerifyLimiter := passwordLimiter(accessConstants.UserLogin)
	session.POST("/login-captcha", userRouters.LoginCaptcha, captchaIssueLimiter())
	session.POST("/login", userRouters.Login)
	session.POST("/login-email", userRouters.SendLoginEmail, emailSendLimiter)
	session.POST("/student-login-email", userRouters.SendStudentLoginEmail, studentEmailLimiter)
	session.POST("/login-email-verify", userRouters.LoginByEmail, emailVerifyLimiter)
	session.GET("/user-info", userRouters.Info)
	session.POST("/logout", userRouters.Logout)
	session.POST("/change-password", userRouters.ChangePassword, changeLimiter)

	// 通知是账号自助资源，接收人始终从当前会话确定。
	notifications := session.Group("/notifications")
	notifications.GET("", notificationRouters.List)
	notifications.GET("/unread", notificationRouters.Unread)
	notifications.POST("/read", notificationRouters.MarkRead)

	user := engine.Group("/v1/system/users")
	user.POST("", userRouters.Create)
	user.GET("", userRouters.List)
	user.GET("/statistics", userRouters.Statistics)
	user.PUT("/:id", userRouters.Update)
	user.DELETE("/:id", userRouters.Delete)
	user.DELETE("/:id/sessions", userRouters.RevokeSessions)
	user.GET("/:id/api-token", userRouters.APIToken)
	user.PUT("/:id/api-token", userRouters.ResetAPIToken)
	user.DELETE("/:id/api-token", userRouters.DeleteAPIToken)

	// 审计查询与详情保持独立 app，当前沿用系统管理员身份检查。
	audit := engine.Group("/v1/system/audit")
	audit.GET("/logs", auditRouters.List)
	audit.GET("/statistics", auditRouters.Statistics)
	audit.GET("/logs/:id", auditRouters.Detail)

	// 工作空间管理仅限系统管理员，进入空间单独检查用户操作授权。
	workspace := engine.Group("/v1/system/workspaces")
	workspace.GET("", workspaceRouters.List)
	workspace.GET("/enum", workspaceRouters.Enum)
	workspace.POST("", workspaceRouters.Create)
	workspace.GET("/:workspace_id", workspaceRouters.Detail)
	workspace.PUT("/:workspace_id", workspaceRouters.Update)
	workspace.DELETE("/:workspace_id", workspaceRouters.Delete)
	workspaces := engine.Group("/v1/workspaces")
	workspaces.GET("", accessRouters.MyWorkspaces)
	workspaces.GET("/:workspace_id", workspaceRouters.Enter)
	workspaces.GET("/:workspace_id/student-service", studentRouters.Profile)

	// 学生数据仪表独立授权，汇总和导出复用当前空间的学生基础字段。
	studentDashboard := workspaces.Group("/:workspace_id/student-dashboard")
	studentDashboard.GET("", studentdashboardRouters.Summary)
	studentDashboard.GET("/fields", studentdashboardRouters.Fields)
	studentDashboard.GET("/export", studentdashboardRouters.Export)
	accommodation := studentDashboard.Group("/accommodation")
	accommodation.GET("", studentdashboardRouters.Accommodation)
	accommodation.GET("/filter-options", studentdashboardRouters.AccommodationOptions)
	accommodation.GET("/export", studentdashboardRouters.ExportAccommodation)

	// 党建数据仪表独立授权，不依赖人员名单或党支部管理权限。
	cpcDashboard := workspaces.Group("/:workspace_id/cpc-dashboard")
	cpcDashboard.GET("", cpcdashboardRouters.Summary)
	cpcDashboard.GET("/fields", cpcdashboardRouters.Fields)
	cpcDashboard.GET("/export", cpcdashboardRouters.Export)

	// 学生业务全部位于工作空间下，权限由全局目录中间件统一检查。
	students := workspaces.Group("/:workspace_id/students")
	students.GET("", studentRouters.List)
	students.POST("", studentRouters.Create)
	students.GET("/statistics", studentRouters.Statistics)
	students.GET("/fields", studentRouters.Fields)
	students.GET("/options", studentRouters.Options)
	students.GET("/sensitive-options", studentRouters.SensitiveOptions)
	students.GET("/template", studentRouters.Template)
	students.POST("/import", studentRouters.Import)
	students.GET("/export", studentRouters.Export)
	students.GET("/:id", studentRouters.Detail)
	students.PUT("/:id", studentRouters.Update)
	students.POST("/:id/avatar", studentRouters.UploadAvatar)
	students.POST("/:id/avatar/complete", studentRouters.CompleteAvatar)
	students.DELETE("/:id/avatar-uploads/:upload_id", studentRouters.DeleteAvatarUpload)
	students.DELETE("/:id", studentRouters.Delete)
	students.GET("/:id/sensitive", studentRouters.ReadSensitive)
	students.PUT("/:id/sensitive", studentRouters.UpdateSensitive)

	// 毕业去向存储在学生宽表内，列表、维护、清除和统计分别授权。
	graduation := workspaces.Group("/:workspace_id/graduation-destinations")
	graduation.GET("", graduationRouters.List)
	graduation.GET("/statistics", graduationRouters.Statistics)
	graduation.GET("/filter-options", graduationRouters.Options)
	graduation.GET("/statistics/filter-options", graduationRouters.StatisticsOptions)
	graduation.GET("/fields", graduationRouters.Fields)
	graduation.GET("/template", graduationRouters.Template)
	graduation.POST("/import", graduationRouters.Import)
	graduation.GET("/export", graduationRouters.Export)
	graduation.PUT("/:id", graduationRouters.Save)
	graduation.DELETE("/:id", graduationRouters.Clear)

	// 评奖评优按项目归属年份维护，参与记录及附件全部受工作空间权限保护。
	projects := workspaces.Group("/:workspace_id/projects")
	projects.GET("", projectRouters.ListProjects)
	projects.POST("", projectRouters.CreateProject)
	projects.POST("/score-preview", projectRouters.PreviewNewProjectScore)
	projects.POST("/:id/score-preview", projectRouters.PreviewProjectScore)
	projects.GET("/statistics", projectRouters.Statistics)
	projects.GET("/fields", projectRouters.Fields)
	projects.GET("/options", projectRouters.ProjectOptions)
	projects.GET("/filter-options", projectRouters.ProjectFilterOptions)
	projects.GET("/template", projectRouters.ProjectTemplate)
	projects.POST("/import", projectRouters.ProjectImport)
	projects.GET("/export", projectRouters.ProjectExport)
	projects.PUT("/:id", projectRouters.UpdateProject)
	projects.DELETE("/:id", projectRouters.DeleteProject)
	participations := workspaces.Group("/:workspace_id/project-participations")
	participations.GET("", projectRouters.ListParticipations)
	participations.POST("", projectRouters.CreateParticipation)
	participations.GET("/students", projectRouters.StudentOptions)
	participations.GET("/filter-options", projectRouters.ParticipationFilterOptions)
	participations.GET("/detail-names", projectRouters.DetailNames)
	participations.GET("/:id", projectRouters.ParticipationDetail)
	participations.GET("/template", projectRouters.ParticipationTemplate)
	participations.POST("/import", projectRouters.ParticipationImport)
	participations.GET("/export", projectRouters.ParticipationExport)
	participations.PUT("/:id", projectRouters.UpdateParticipation)
	participations.DELETE("/:id", projectRouters.DeleteParticipation)
	attachments := workspaces.Group("/:workspace_id/attachments")
	attachments.GET("/project", attachmentRouters.List)
	projects.POST("/:project_id/attachments", attachmentRouters.Upload)
	projects.POST("/:project_id/attachments/complete", attachmentRouters.CompleteUpload)
	attachments.GET("/:id/download", attachmentRouters.Download)
	attachments.DELETE("/:id", attachmentRouters.Delete)

	// 填报管理须具备查看权限，流程明确指定的当前审批人才能处理节点。
	applications := workspaces.Group("/:workspace_id/project-applications")
	applications.GET("", projectRouters.ManagedApplications)
	applications.GET("/filter-options", projectRouters.ManagedApplicationProjects)
	applications.GET("/export", projectRouters.ManagedApplicationExport)
	applications.GET("/:id", projectRouters.ManagedApplicationsDetail)
	applications.GET("/:id/versions", projectRouters.ManagedApplicationVersions)
	applications.PUT("/:id", projectRouters.ManagedApplicationsUpdate)
	applications.DELETE("/:id", projectRouters.ManagedApplicationsDelete)
	applications.POST("/:id/approve", projectRouters.ManagedApplicationsApprove)
	applications.POST("/:id/reject", projectRouters.ManagedApplicationsReject)
	applicationFiles := applications.Group("/:id/attachments")
	applicationFiles.POST("", projectRouters.ManagedApplicationAttachmentUpload)
	applicationFiles.POST("/complete", projectRouters.ManagedApplicationAttachmentComplete)
	applicationFiles.DELETE("/:file_id", projectRouters.ManagedApplicationAttachmentDelete)
	applicationFiles.GET("/:file_id/download", projectRouters.ManagedApplicationAttachmentDownload)
	projects.GET("/approval-options", projectRouters.ApprovalOptions)
	studentService := workspaces.Group("/:workspace_id/student-service")
	studentService.GET("/projects", projectRouters.StudentProjects)
	studentNotifications := studentService.Group("/notifications")
	studentNotifications.GET("", notificationRouters.StudentList)
	studentNotifications.GET("/unread", notificationRouters.StudentUnread)
	studentNotifications.POST("/read", notificationRouters.StudentMarkRead)
	studentApplications := studentService.Group("/project-applications")
	studentApplications.GET("", projectRouters.MyApplications)
	studentApplications.GET("/filter-options", projectRouters.MyApplicationProjects)
	studentApplications.GET("/:id", projectRouters.MyApplicationsDetail)
	studentApplications.GET("/:id/versions", projectRouters.MyApplicationVersions)
	studentApplications.POST("", projectRouters.CreateApplication)
	studentApplications.PUT("/:id", projectRouters.MyApplicationsUpdate)
	studentApplications.POST("/:id/submit", projectRouters.MyApplicationsSubmit)
	myApplicationFiles := studentApplications.Group("/:id/attachments")
	myApplicationFiles.POST("", projectRouters.MyApplicationAttachmentUpload)
	myApplicationFiles.POST("/complete", projectRouters.MyApplicationAttachmentComplete)
	myApplicationFiles.DELETE("/:file_id", projectRouters.MyApplicationAttachmentDelete)
	myApplicationFiles.GET("/:file_id/download", projectRouters.MyApplicationAttachmentDownload)
	studentReviews := studentService.Group("/project-reviews")
	studentReviews.GET("", projectRouters.StudentReviews)
	studentReviews.GET("/filter-options", projectRouters.StudentReviewProjects)
	studentReviews.GET("/:id", projectRouters.StudentReviewsDetail)
	studentReviews.GET("/:id/versions", projectRouters.StudentReviewVersions)
	studentReviews.PUT("/:id", projectRouters.StudentReviewsUpdate)
	studentReviews.POST("/:id/approve", projectRouters.StudentReviewsApprove)
	studentReviews.POST("/:id/reject", projectRouters.StudentReviewsReject)
	studentReviewFiles := studentReviews.Group("/:id/attachments")
	studentReviewFiles.POST("", projectRouters.StudentReviewAttachmentUpload)
	studentReviewFiles.POST("/complete", projectRouters.StudentReviewAttachmentComplete)
	studentReviewFiles.DELETE("/:file_id", projectRouters.StudentReviewAttachmentDelete)
	studentReviewFiles.GET("/:file_id/download", projectRouters.StudentReviewAttachmentDownload)

	// 月度工作按日期维护，读写、导入和附件操作独立授权。
	monthlyWork := workspaces.Group("/:workspace_id/monthly-work")
	monthlyWork.GET("", monthlyworkRouters.List)
	monthlyWork.POST("", monthlyworkRouters.Create)
	monthlyWork.GET("/fields", monthlyworkRouters.Fields)
	monthlyWork.GET("/filter-options", monthlyworkRouters.FilterOptions)
	monthlyWork.GET("/owners", monthlyworkRouters.Owners)
	monthlyWork.GET("/template", monthlyworkRouters.Template)
	monthlyWork.POST("/import", monthlyworkRouters.Import)
	monthlyWork.PUT("/:id", monthlyworkRouters.Update)
	monthlyWork.DELETE("/:id", monthlyworkRouters.Delete)
	monthlyWork.POST("/:work_id/attachments", attachmentRouters.UploadMonthlyWork)
	monthlyWork.POST("/:work_id/attachments/complete", attachmentRouters.CompleteMonthlyWork)
	attachments.GET("/monthly-work", attachmentRouters.ListMonthlyWork)
	attachments.GET("/monthly-work/:id/download", attachmentRouters.DownloadMonthlyWork)
	attachments.DELETE("/monthly-work/:id", attachmentRouters.DeleteMonthlyWork)

	// 党建月度工作复用业务实现，类别和权限由固定路径隔离。
	cpcMonthlyWork := workspaces.Group("/:workspace_id/cpc-monthly-work")
	cpcMonthlyWork.GET("", monthlyworkRouters.List)
	cpcMonthlyWork.POST("", monthlyworkRouters.Create)
	cpcMonthlyWork.GET("/fields", monthlyworkRouters.Fields)
	cpcMonthlyWork.GET("/filter-options", monthlyworkRouters.FilterOptions)
	cpcMonthlyWork.GET("/owners", monthlyworkRouters.Owners)
	cpcMonthlyWork.GET("/template", monthlyworkRouters.Template)
	cpcMonthlyWork.POST("/import", monthlyworkRouters.Import)
	cpcMonthlyWork.PUT("/:id", monthlyworkRouters.Update)
	cpcMonthlyWork.DELETE("/:id", monthlyworkRouters.Delete)
	cpcMonthlyWork.POST("/:work_id/attachments", attachmentRouters.UploadMonthlyWork)
	cpcMonthlyWork.POST("/:work_id/attachments/complete", attachmentRouters.CompleteMonthlyWork)
	attachments.GET("/cpc-monthly-work", attachmentRouters.ListMonthlyWork)
	attachments.GET("/cpc-monthly-work/:id/download", attachmentRouters.DownloadMonthlyWork)
	attachments.DELETE("/cpc-monthly-work/:id", attachmentRouters.DeleteMonthlyWork)

	// 党建数据管理按空间隔离，党支部各操作独立授权。
	partyBranches := workspaces.Group("/:workspace_id/cpc-party-branches")
	partyBranches.GET("", partybranchRouters.List)
	partyBranches.POST("", partybranchRouters.Create)
	partyBranches.GET("/fields", partybranchRouters.Fields)
	partyBranches.GET("/secretaries", partybranchRouters.Secretaries)
	partyBranches.GET("/template", partybranchRouters.Template)
	partyBranches.POST("/import", partybranchRouters.Import)
	partyBranches.GET("/export", partybranchRouters.Export)
	partyBranches.POST("/reorder-people", partybranchRouters.ReorderPeople)
	partyBranches.PUT("/:id", partybranchRouters.Update)
	partyBranches.DELETE("/:id", partybranchRouters.Delete)
	partyBranches.GET("/:id/admission-rule", partybranchRouters.ReadAdmissionRule)
	partyBranches.PUT("/:id/admission-rule", partybranchRouters.SaveAdmissionRule)

	// 时间线模板独立授权，模板定义和修改审计在同一事务保存。
	timelineTemplates := workspaces.Group("/:workspace_id/cpc-timeline-templates")
	timelineTemplates.GET("", cpctimelineRouters.List)
	timelineTemplates.POST("", cpctimelineRouters.Create)
	timelineTemplates.GET("/fields", cpctimelineRouters.Fields)
	timelineTemplates.GET("/template", cpctimelineRouters.ImportTemplate)
	timelineTemplates.POST("/import", cpctimelineRouters.Import)
	timelineTemplates.GET("/transfer-targets/create", cpctimelineRouters.CreateTransferTargets)
	timelineTemplates.GET("/transfer-targets/update", cpctimelineRouters.UpdateTransferTargets)
	timelineTemplates.GET("/:id", cpctimelineRouters.Detail)
	timelineTemplates.POST("/:id/clone", cpctimelineRouters.Clone)
	timelineTemplates.PUT("/:id", cpctimelineRouters.Update)
	timelineTemplates.DELETE("/:id", cpctimelineRouters.Delete)
	timelineNodeFiles := timelineTemplates.Group("/:template_id/nodes/:node_id/attachments")
	timelineNodeFiles.POST("", attachmentRouters.UploadTimelineNode)
	timelineNodeFiles.POST("/complete", attachmentRouters.CompleteTimelineNode)
	timelineNodeFiles.GET("", attachmentRouters.ListTimelineNode)
	attachments.GET("/cpc-timeline-nodes/:id/download", attachmentRouters.DownloadTimelineNode)
	attachments.DELETE("/cpc-timeline-nodes/:id", attachmentRouters.DeleteTimelineNode)

	// 党建人员的每个操作独立授权，资源范围在所属服务中实时复核。
	cpcPeople := workspaces.Group("/:workspace_id/cpc-people")
	cpcPeople.GET("", cpcpersonRouters.List)
	cpcPeople.POST("", cpcpersonRouters.Create)
	cpcPeople.GET("/fields", cpcpersonRouters.Fields)
	cpcPeople.GET("/timeline-fields", cpcpersonRouters.TimelineFilterFields)
	cpcPeople.GET("/options", cpcpersonRouters.Options)
	cpcPeople.GET("/branches", cpcpersonRouters.Branches)
	cpcPeople.GET("/student-options", cpcpersonRouters.Candidates)
	cpcPeople.GET("/template", cpcpersonRouters.Template)
	cpcPeople.POST("/import", cpcpersonRouters.Import)
	cpcPeople.GET("/export", cpcpersonRouters.Export)
	cpcPeople.GET("/statistics", cpcpersonRouters.Statistics)
	cpcPeople.GET("/timeline-templates/create", cpcpersonRouters.CreateTimelineOptions)
	cpcPeople.GET("/timeline-templates/update", cpcpersonRouters.UpdateTimelineOptions)
	cpcPeople.GET("/timeline-templates/import", cpcpersonRouters.ImportTimelineOptions)
	cpcPeople.GET("/timeline-templates/list", cpcpersonRouters.ListTimelineOptions)
	personTimelineView := cpcPeople.Group("/timeline-view/:template_id")
	personTimelineView.GET("", cpcpersonRouters.TimelineView)
	personTimelineView.GET("/nodes/:node_id", cpcpersonRouters.TimelineViewList)
	personTimelineView.GET("/nodes/:node_id/statistics", cpcpersonRouters.TimelineViewStatistics)
	cpcPeople.GET("/:id/timeline", cpcpersonRouters.Timeline)
	cpcPeople.GET("/:id/timeline/edit", cpcpersonRouters.EditTimeline)
	cpcPeople.POST("/:id/timeline/bind", cpcpersonRouters.BindTimeline)
	cpcPeople.GET("/:id/timeline/transfer-options", cpcpersonRouters.TimelineTransferOptions)
	cpcPeople.POST("/:id/timeline/transfer-preview", cpcpersonRouters.PreviewTimelineTransfer)
	cpcPeople.POST("/:id/timeline/transfer", cpcpersonRouters.TransferTimeline)
	cpcPeople.GET("/:id/timeline/review", cpcpersonRouters.ReviewTimeline)
	cpcPeople.GET("/:id/timeline/transfer-review", cpcpersonRouters.PreviewTimelineTransferReview)
	cpcPeople.POST("/:id/timeline/transfer-review", cpcpersonRouters.ReviewTimelineTransfer)
	cpcPeople.POST("/:id/timeline/transfer-reject", cpcpersonRouters.RejectTimelineTransfer)
	cpcPeople.POST("/:id/timeline/transfer-cancel", cpcpersonRouters.CancelTimelineTransfer)
	cpcPeople.PUT("/:id/timeline/nodes/:node_id", cpcpersonRouters.SaveTimelineNode)
	cpcPeople.POST("/:id/timeline/nodes/:node_id/reset", cpcpersonRouters.ResetTimelineNode)
	cpcPeople.POST("/:id/timeline/nodes/:node_id/skip", cpcpersonRouters.SkipTimelineNode)
	personNodeFiles := cpcPeople.Group("/:person_id/timeline/nodes/:node_id")
	personNodeFiles.POST("/attachments", attachmentRouters.UploadPersonTimeline)
	personNodeFiles.POST("/attachments/complete", attachmentRouters.CompletePersonTimeline)
	personNodeFiles.GET("/attachments/:file_id/download", attachmentRouters.DownloadPersonTimeline)
	personNodeFiles.GET(
		"/template-attachments/:file_id/download",
		attachmentRouters.DownloadPersonTemplate,
	)
	personNodeFiles.GET(
		"/attachments/:file_id/edit/download",
		attachmentRouters.EditDownloadPersonTimeline,
	)
	personNodeFiles.GET(
		"/template-attachments/:file_id/edit/download",
		attachmentRouters.EditDownloadPersonTemplate,
	)
	attachments.DELETE("/cpc-person-timeline-nodes/:id", attachmentRouters.DeletePersonTimeline)
	cpcPeople.GET("/:id", cpcpersonRouters.Detail)
	cpcPeople.PUT("/:id", cpcpersonRouters.Update)
	cpcPeople.DELETE("/:id", cpcpersonRouters.Delete)

	// 权限管理仅限系统管理员，直接维护用户、操作、工作空间关系。
	access := engine.Group("/v1/system/access")
	access.GET("/actions", accessRouters.Catalog)
	permissions := access.Group("/workspaces/:workspace_id/users")
	permissions.GET("", accessRouters.Users)
	permissions.GET("/:user_id/actions", accessRouters.UserActions)
	permissions.PUT("/:user_id/actions", accessRouters.UpdateActions)
}
