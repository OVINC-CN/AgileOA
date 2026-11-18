package httpd

// #nosec G101 -- 固定 HTTP 路径模板，不是认证凭据。
const mcpWorkspaceQueryPrefix = "/v1/workspaces/{workspace_id}"

// mcpQueryRegistrations 是供人工维护的 MCP 查询开放清单，未登记的接口默认不开放。
func mcpQueryRegistrations() []mcpQueryRegistration {
	return []mcpQueryRegistration{
		// 工作空间发现与上下文。
		{
			Name: "get_workspaces",
			Path: "/v1/workspaces",
		},
		{
			Name: "get_workspace",
			Path: mcpWorkspaceQueryPrefix,
		},

		// 学生信息。
		{
			Name: "get_workspace_students",
			Path: mcpWorkspaceQueryPrefix + "/students",
		},
		{
			Name: "get_workspace_students_fields",
			Path: mcpWorkspaceQueryPrefix + "/students/fields",
		},
		{
			Name: "get_workspace_students_options",
			Path: mcpWorkspaceQueryPrefix + "/students/options",
		},
		{
			Name: "get_workspace_students_sensitive_options",
			Path: mcpWorkspaceQueryPrefix + "/students/sensitive-options",
		},
		{
			Name: "get_workspace_students_statistics",
			Path: mcpWorkspaceQueryPrefix + "/students/statistics",
		},
		{
			Name: "get_workspace_students_by_id",
			Path: mcpWorkspaceQueryPrefix + "/students/{id}",
		},
		{
			Name: "get_workspace_students_by_id_sensitive",
			Path: mcpWorkspaceQueryPrefix + "/students/{id}/sensitive",
		},

		// 学生数据仪表。
		{
			Name: "get_workspace_student_dashboard",
			Path: mcpWorkspaceQueryPrefix + "/student-dashboard",
		},
		{
			Name: "get_workspace_student_dashboard_accommodation",
			Path: mcpWorkspaceQueryPrefix + "/student-dashboard/accommodation",
		},
		{
			Name: "get_workspace_student_dashboard_accommodation_filter_options",
			Path: mcpWorkspaceQueryPrefix + "/student-dashboard/accommodation/filter-options",
		},
		{
			Name: "get_workspace_student_dashboard_fields",
			Path: mcpWorkspaceQueryPrefix + "/student-dashboard/fields",
		},

		// 毕业去向。
		{
			Name: "get_workspace_graduation_destinations",
			Path: mcpWorkspaceQueryPrefix + "/graduation-destinations",
		},
		{
			Name: "get_workspace_graduation_destinations_fields",
			Path: mcpWorkspaceQueryPrefix + "/graduation-destinations/fields",
		},
		{
			Name: "get_workspace_graduation_destinations_filter_options",
			Path: mcpWorkspaceQueryPrefix + "/graduation-destinations/filter-options",
		},
		{
			Name: "get_workspace_graduation_destinations_statistics",
			Path: mcpWorkspaceQueryPrefix + "/graduation-destinations/statistics",
		},
		{
			Name: "get_workspace_graduation_destinations_statistics_filter_options",
			Path: mcpWorkspaceQueryPrefix + "/graduation-destinations/statistics/filter-options",
		},

		// 评奖评优及参与记录。
		{
			Name: "get_workspace_project_participations",
			Path: mcpWorkspaceQueryPrefix + "/project-participations",
		},
		{
			Name: "get_workspace_project_participations_detail_names",
			Path: mcpWorkspaceQueryPrefix + "/project-participations/detail-names",
		},
		{
			Name: "get_workspace_project_participations_filter_options",
			Path: mcpWorkspaceQueryPrefix + "/project-participations/filter-options",
		},
		{
			Name: "get_workspace_project_participations_students",
			Path: mcpWorkspaceQueryPrefix + "/project-participations/students",
		},
		{
			Name: "get_workspace_project_participations_by_id",
			Path: mcpWorkspaceQueryPrefix + "/project-participations/{id}",
		},
		{
			Name: "get_workspace_projects",
			Path: mcpWorkspaceQueryPrefix + "/projects",
		},
		{
			Name: "get_workspace_projects_fields",
			Path: mcpWorkspaceQueryPrefix + "/projects/fields",
		},
		{
			Name: "get_workspace_projects_filter_options",
			Path: mcpWorkspaceQueryPrefix + "/projects/filter-options",
		},
		{
			Name: "get_workspace_projects_options",
			Path: mcpWorkspaceQueryPrefix + "/projects/options",
		},
		{
			Name: "get_workspace_projects_statistics",
			Path: mcpWorkspaceQueryPrefix + "/projects/statistics",
		},

		// 学生月度工作。
		{
			Name: "get_workspace_monthly_work",
			Path: mcpWorkspaceQueryPrefix + "/monthly-work",
		},
		{
			Name: "get_workspace_monthly_work_fields",
			Path: mcpWorkspaceQueryPrefix + "/monthly-work/fields",
		},
		{
			Name: "get_workspace_monthly_work_filter_options",
			Path: mcpWorkspaceQueryPrefix + "/monthly-work/filter-options",
		},
		{
			Name: "get_workspace_monthly_work_owners",
			Path: mcpWorkspaceQueryPrefix + "/monthly-work/owners",
		},

		// 附件元数据。
		{
			Name: "get_workspace_attachments_cpc_monthly_work",
			Path: mcpWorkspaceQueryPrefix + "/attachments/cpc-monthly-work",
		},
		{
			Name: "get_workspace_attachments_monthly_work",
			Path: mcpWorkspaceQueryPrefix + "/attachments/monthly-work",
		},
		{
			Name: "get_workspace_attachments_project",
			Path: mcpWorkspaceQueryPrefix + "/attachments/project",
		},

		// 党建数据仪表。
		{
			Name: "get_workspace_cpc_dashboard",
			Path: mcpWorkspaceQueryPrefix + "/cpc-dashboard",
		},
		{
			Name: "get_workspace_cpc_dashboard_fields",
			Path: mcpWorkspaceQueryPrefix + "/cpc-dashboard/fields",
		},

		// 党支部。
		{
			Name: "get_workspace_cpc_party_branches",
			Path: mcpWorkspaceQueryPrefix + "/cpc-party-branches",
		},
		{
			Name: "get_workspace_cpc_party_branches_fields",
			Path: mcpWorkspaceQueryPrefix + "/cpc-party-branches/fields",
		},
		{
			Name: "get_workspace_cpc_party_branches_secretaries",
			Path: mcpWorkspaceQueryPrefix + "/cpc-party-branches/secretaries",
		},
		{
			Name: "get_workspace_cpc_party_branches_by_id_admission_rule",
			Path: mcpWorkspaceQueryPrefix + "/cpc-party-branches/{id}/admission-rule",
		},

		// 时间线模板。
		{
			Name: "get_workspace_cpc_timeline_templates",
			Path: mcpWorkspaceQueryPrefix + "/cpc-timeline-templates",
		},
		{
			Name: "get_workspace_cpc_timeline_templates_fields",
			Path: mcpWorkspaceQueryPrefix + "/cpc-timeline-templates/fields",
		},
		{
			Name: "get_workspace_cpc_timeline_templates_transfer_targets_create",
			Path: mcpWorkspaceQueryPrefix + "/cpc-timeline-templates/transfer-targets/create",
		},
		{
			Name: "get_workspace_cpc_timeline_templates_transfer_targets_update",
			Path: mcpWorkspaceQueryPrefix + "/cpc-timeline-templates/transfer-targets/update",
		},
		{
			Name: "get_workspace_cpc_timeline_templates_by_id",
			Path: mcpWorkspaceQueryPrefix + "/cpc-timeline-templates/{id}",
		},
		{
			Name: "get_workspace_cpc_timeline_templates_by_template_id_nodes_by_node_id_attachments",
			Path: mcpWorkspaceQueryPrefix + "/cpc-timeline-templates/{template_id}/nodes/{node_id}/attachments",
		},

		// 党建人员及时间线。
		{
			Name: "get_workspace_cpc_people",
			Path: mcpWorkspaceQueryPrefix + "/cpc-people",
		},
		{
			Name: "get_workspace_cpc_people_branches",
			Path: mcpWorkspaceQueryPrefix + "/cpc-people/branches",
		},
		{
			Name: "get_workspace_cpc_people_fields",
			Path: mcpWorkspaceQueryPrefix + "/cpc-people/fields",
		},
		{
			Name: "get_workspace_cpc_people_options",
			Path: mcpWorkspaceQueryPrefix + "/cpc-people/options",
		},
		{
			Name: "get_workspace_cpc_people_statistics",
			Path: mcpWorkspaceQueryPrefix + "/cpc-people/statistics",
		},
		{
			Name: "get_workspace_cpc_people_student_options",
			Path: mcpWorkspaceQueryPrefix + "/cpc-people/student-options",
		},
		{
			Name: "get_workspace_cpc_people_timeline_fields",
			Path: mcpWorkspaceQueryPrefix + "/cpc-people/timeline-fields",
		},
		{
			Name: "get_workspace_cpc_people_timeline_templates_create",
			Path: mcpWorkspaceQueryPrefix + "/cpc-people/timeline-templates/create",
		},
		{
			Name: "get_workspace_cpc_people_timeline_templates_import",
			Path: mcpWorkspaceQueryPrefix + "/cpc-people/timeline-templates/import",
		},
		{
			Name: "get_workspace_cpc_people_timeline_templates_list",
			Path: mcpWorkspaceQueryPrefix + "/cpc-people/timeline-templates/list",
		},
		{
			Name: "get_workspace_cpc_people_timeline_templates_update",
			Path: mcpWorkspaceQueryPrefix + "/cpc-people/timeline-templates/update",
		},
		{
			Name: "get_workspace_cpc_people_timeline_view_by_template_id",
			Path: mcpWorkspaceQueryPrefix + "/cpc-people/timeline-view/{template_id}",
		},
		{
			Name: "get_workspace_cpc_people_timeline_view_by_template_id_nodes_by_node_id",
			Path: mcpWorkspaceQueryPrefix + "/cpc-people/timeline-view/{template_id}/nodes/{node_id}",
		},
		{
			Name: "get_workspace_cpc_people_timeline_view_by_template_id_nodes_by_node_id_statistics",
			Path: mcpWorkspaceQueryPrefix + "/cpc-people/timeline-view/{template_id}/nodes/{node_id}/statistics",
		},
		{
			Name: "get_workspace_cpc_people_by_id",
			Path: mcpWorkspaceQueryPrefix + "/cpc-people/{id}",
		},
		{
			Name: "get_workspace_cpc_people_by_id_timeline",
			Path: mcpWorkspaceQueryPrefix + "/cpc-people/{id}/timeline",
		},
		{
			Name: "get_workspace_cpc_people_by_id_timeline_edit",
			Path: mcpWorkspaceQueryPrefix + "/cpc-people/{id}/timeline/edit",
		},
		{
			Name: "get_workspace_cpc_people_by_id_timeline_review",
			Path: mcpWorkspaceQueryPrefix + "/cpc-people/{id}/timeline/review",
		},
		{
			Name: "get_workspace_cpc_people_by_id_timeline_transfer_options",
			Path: mcpWorkspaceQueryPrefix + "/cpc-people/{id}/timeline/transfer-options",
		},
		{
			Name: "get_workspace_cpc_people_by_id_timeline_transfer_review",
			Path: mcpWorkspaceQueryPrefix + "/cpc-people/{id}/timeline/transfer-review",
		},

		// 党建月度工作。
		{
			Name: "get_workspace_cpc_monthly_work",
			Path: mcpWorkspaceQueryPrefix + "/cpc-monthly-work",
		},
		{
			Name: "get_workspace_cpc_monthly_work_fields",
			Path: mcpWorkspaceQueryPrefix + "/cpc-monthly-work/fields",
		},
		{
			Name: "get_workspace_cpc_monthly_work_filter_options",
			Path: mcpWorkspaceQueryPrefix + "/cpc-monthly-work/filter-options",
		},
		{
			Name: "get_workspace_cpc_monthly_work_owners",
			Path: mcpWorkspaceQueryPrefix + "/cpc-monthly-work/owners",
		},
	}
}
