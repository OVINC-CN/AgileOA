// 浏览器在加载页面时读取此文件，构建产物按最终配置内容添加哈希文件名。
// 调整配置后重新构建，同步发布 HTML 和对应配置文件。
// 文件内容公开可见，请勿存放凭据或密钥。
globalThis.AGILE_WORKSPACE_CONFIG = {
  // 日期时间的显示时区
  displayTimeZone: 'Asia/Shanghai',
  // 应用显示名称，同时用作浏览器标签页标题。
  appName: '敏捷办公',
  // 用户协议和隐私协议中的联系邮箱，部署时填写实际接收请求的地址。
  contactEmail: 'contact@example.com',
  // 页脚版权归属和年份，® 由页面自动添加。
  copyrightOwner: 'OVINC TECH',
  copyrightYear: 2026,
  // 密码登录及用户密码设置入口，与 YAML 的 user.password_login_enabled 保持一致。
  passwordLoginEnabled: true,

  // API 基础地址，留空字符串时使用当前页面的来源。
  apiBaseURL: 'https://api.ovinc.localhost',
  // 浏览器等待单次 API 请求的时限，单位为毫秒。
  apiTimeoutMs: 15000,
  // 顶部异常消息自动关闭前的显示时长，单位为毫秒。
  messageDurationMs: 5000,
  // 页面更新检测的随机间隔范围和单次请求时限，单位为毫秒。
  pageUpdateCheckIntervalMs: { min: 50000, max: 100000 },
  pageUpdateCheckTimeoutMs: 15000,
  // 查询结果在缓存中保持新鲜的时长，单位为毫秒。
  queryStaleTimeMs: 30000,
  // 查询失败后的额外重试次数，填 0 则不重试。
  queryRetryCount: 1,
  // 已登录账号的通知刷新随机间隔范围，单位为毫秒。
  notificationPollIntervalMs: { min: 50000, max: 100000 },

  // 所有附件、头像及导入入口的单文件上限，与 YAML 的 cos.max_upload_bytes 一致。
  fileMaxUploadBytes: 5242880,
  // 附件预览解析时限，单位为毫秒。
  filePreviewTimeoutMs: 60000,
  // 附件预览资源目录，生产构建自动写入内容哈希。
  fileViewerAssetsPath: 'extra-assets/file-viewer',
  // 审计 JSON 差异计算的单次时限，单位为毫秒。
  auditDiffTimeoutMs: 2000,
  // 附件允许的 MIME 类型，支持 image/* 通配符；与 cos.allowed_content_types 一致。
  // XLSX、JSON 导入保留各自格式；头像另外仅接受 JPEG、PNG、WebP。
  fileAllowedContentTypes: [
    'application/pdf',
    'image/*',
    'application/msword',
    'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
    'application/vnd.ms-excel',
    'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    'application/vnd.ms-powerpoint',
    'application/vnd.openxmlformats-officedocument.presentationml.presentation',
  ],
  // 上传提示的分组名称及对应 MIME 类型，一组可包含多种格式，类型不能重复。
  // 允许类型中的每项（含通配符）都必须归组；仅展示命中白名单的组，按此处顺序排列。
  fileContentTypeGroups: {
    PDF: ['application/pdf'],
    图片: ['image/*'],
    Word: [
      'application/msword',
      'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
    ],
    Excel: [
      'application/vnd.ms-excel',
      'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    ],
    PPT: [
      'application/vnd.ms-powerpoint',
      'application/vnd.openxmlformats-officedocument.presentationml.presentation',
    ],
  },

  // 学生及毕业去向 XLSX 导入文件大小上限，单位为字节，与 YAML 的 student.max_file_bytes 一致。
  studentMaxFileBytes: 5242880,

  // 项目及参与评分 XLSX 导入文件大小上限，单位为字节，与 YAML 的 project.max_file_bytes 一致。
  projectMaxFileBytes: 5242880,
  // 每条项目参与记录可添加的评分子项数，与 YAML 的 project.max_details 一致。
  projectMaxDetails: 500,
  // 学生申报分数计算限制，与 YAML 的 project.score_* 保持一致。
  projectScoreMaxRules: 30,
  projectScoreRuleMaxDepth: 8,
  projectScoreRuleMaxNodes: 100,
  projectScoreRuleMaxValues: 100,
  projectScoreFormulaMaxLength: 4000,
  projectScorePreviewDelayMs: 300,
  // 允许配置的最多审批级数，与后端 YAML 保持一致。
  projectMaxApprovalSteps: 20,
  projectMaxApprovalTargets: 10,

  // 学生与党建月度工作共用以下限制；外部链接上限与 YAML 的 monthly_work.max_links 一致。
  monthlyWorkMaxLinks: 20,
  // 月度工作 XLSX 导入文件大小上限，单位为字节，与 YAML 的 monthly_work.max_file_bytes 一致。
  monthlyWorkMaxFileBytes: 5242880,

  // 党支部 XLSX 导入文件大小上限，单位为字节，与 YAML 的 party_branch.max_file_bytes 一致。
  partyBranchMaxFileBytes: 5242880,
  // 党建人员 XLSX 导入文件上限，与 YAML 的 cpc_person.max_file_bytes 一致。
  cpcPersonMaxFileBytes: 5242880,
  // 准入规则树的层数、节点数及每个条件候选值上限，与 YAML 的 party_branch.max_rule_* 一致。
  partyBranchRuleMaxDepth: 8,
  partyBranchRuleMaxNodes: 100,
  partyBranchRuleMaxValues: 100,

  // 时间线模板限制，与 YAML 的 cpc_timeline_template 对应参数保持一致。
  timelineMaxNodes: 100,
  timelineMaxFieldsPerNode: 20,
  timelineMaxRulesPerNode: 20,
  timelineMaxDefinitionBytes: 131072,
  timelineMaxOffset: 10000,

  // 前端默认每页条数，必须包含在下方选项中。
  tablePageSize: 5,
  // 分页下拉选项，条数不得重复，也不得超过 YAML 的 http.max_page_size。
  tablePageSizeOptions: [5, 10, 20, 50, 100],
};
