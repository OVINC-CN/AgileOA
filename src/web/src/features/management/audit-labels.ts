const objectNames: Record<string, string> = {
  mcp: 'MCP 连接',
  user: '用户',
  user_session: '用户会话',
  user_api_token: '用户 API Token',
  user_email_login: '邮件登录',
  audit: '审计记录',
  workspace: '工作空间',
  access_permission: '操作权限',
  student: '学生',
  graduation: '毕业去向',
  project: '评奖评优项目',
  project_participation: '项目参与记录',
  project_application: '评奖评优申报',
  project_approval: '项目审批配置',
  project_detail: '项目明细',
  monthly_work: '月度工作',
  notification: '个人通知',
  attachment: '附件',
  cpc_person: '党建人员',
  cpc_dashboard: '党建数据仪表',
  cpc_party_branch: '党支部',
  cpc_timeline_template: '时间线模板',
  cpc_person_timeline_node: '人员时间线节点',
};

export function auditObjectLabel(value: string) {
  return objectNames[value] ?? value;
}

export function auditWorkspaceLabel(
  id: string | null,
  options: { value: string; label: string }[] = [],
) {
  if (id === null || id === 'system') return '系统';
  return options.find((option) => option.value === id)?.label ?? id;
}

export function auditIdentity(snapshot: Record<string, unknown>) {
  return {
    name: typeof snapshot['display_name'] === 'string' ? snapshot['display_name'] : '',
    username: typeof snapshot['username'] === 'string' ? snapshot['username'] : '',
  };
}
