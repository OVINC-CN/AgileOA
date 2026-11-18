package attachmentConstants

// 业务类别由代码固定，不接受外部任意类别或敏感学生档案。
const (
	CategoryProject                  = "project"
	CategoryProjectApplicationDetail = "project_application_detail"
	CategoryMonthlyWork              = "monthly_work"
	CategoryCPCMonthlyWork           = "cpc_monthly_work"
	CategoryCPCTimelineNode          = "cpc_timeline_node"
	CategoryCPCPersonTimelineNode    = "cpc_person_timeline_node"
	DirectoryRandomBytes             = 5
	Pending                          = "pending"
	Ready                            = "ready"
	Deleting                         = "deleting"
	FilenameLength                   = 255
	ObjectKeyMaxBytes                = 1024
)
