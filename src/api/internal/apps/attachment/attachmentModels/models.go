package attachmentModels

import (
	"time"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apis/tencentcos"

	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// Record 是各业务附件表共有的字段，签名和 URL 不入库。
type Record struct {
	ID          uint64 `gorm:"primaryKey"`
	WorkspaceID string
	ObjectHash  string
	ObjectKey   string `gorm:"column:object_key"`
	Filename    string
	Size        int64
	ContentType string
	UploadActor string
	State       string
	Revision    uint64
	CreatedAt   time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt   time.Time `gorm:"->;autoUpdateTime:false"`
}

// ProjectAttachment 保存项目所属文件及其 COS 对象键。
type ProjectAttachment struct {
	Record
	ProjectID uint64
}

// TableName 固定项目附件表。
func (ProjectAttachment) TableName() string { return "project_attachments" }

// ApplicationDetailAttachment 仅属于申报子项，获奖记录不引用或复制这些文件。
type ApplicationDetailAttachment struct {
	Record
	ApplicationID uint64
	DetailID      uint64
}

func (ApplicationDetailAttachment) TableName() string {
	return "project_application_detail_attachments"
}

// MonthlyWorkAttachment 保存月度工作所属文件及其 COS 对象键。
type MonthlyWorkAttachment struct {
	Record
	MonthlyWorkID uint64
}

// TableName 固定月度工作附件表。
func (MonthlyWorkAttachment) TableName() string { return "monthly_work_attachments" }

// TimelineNodeAttachment 保存时间线节点所属文件及其私有对象键。
type TimelineNodeAttachment struct {
	Record
	TimelineTemplateID uint64
	NodeID             string
}

// TableName 固定时间线节点附件表。
func (TimelineNodeAttachment) TableName() string { return "cpc_timeline_node_attachments" }

// PersonTimelineAttachment 保存人员独立时间线节点的私有附件。
type PersonTimelineAttachment struct {
	Record
	PersonID uint64
	NodeID   string
}

func (PersonTimelineAttachment) TableName() string { return "cpc_person_timeline_attachments" }

// OwnedRecord 是附件查询结果，OwnerID 对应所属业务记录的编号。
type OwnedRecord struct {
	Record
	OwnerID uint64
	NodeID  string
}

// Info 仅公开业务元数据，不公开存储地址和对象名。
type Info struct {
	// ID 是附件编号。
	ID uint64 `json:"id" example:"1"`
	// ApplicationID 是申报附件所属申报编号，其他类别不返回。
	ApplicationID uint64 `json:"application_id,omitempty" example:"1"`
	// DetailID 是申报附件所属评分子项编号，其他类别不返回。
	DetailID uint64 `json:"detail_id,omitempty" example:"1"`
	// ProjectID 是项目附件所属项目编号，其他类别不返回此字段。
	ProjectID uint64 `json:"project_id,omitempty" example:"1"`
	// MonthlyWorkID 是月度工作附件所属记录编号，其他类别不返回此字段。
	MonthlyWorkID uint64 `json:"monthly_work_id,omitempty" example:"1"`
	// TimelineTemplateID 是时间线节点附件所属模板编号，其他类别不返回此字段。
	TimelineTemplateID uint64 `json:"timeline_template_id,omitempty" example:"1"`
	// PersonID 是人员时间线附件所属人员编号，其他类别不返回。
	PersonID uint64 `json:"person_id,omitempty" example:"1"`
	// NodeID 是附件所属模板内的稳定节点标识，其他类别不返回此字段。
	NodeID string `json:"node_id,omitempty" example:"node_application"`
	// Category 是不可修改的业务类别。
	Category string `json:"category" example:"project"`
	// Filename 是中文原始文件名。
	Filename string `json:"filename" example:"项目申报说明.pdf"`
	// Size 是文件字节数。
	Size int64 `json:"size" example:"1024"`
	// State 是存储流程状态，pending 待完成、ready 可下载、deleting 待删除。
	State string `json:"state" example:"ready"`
	// Revision 是并发版本。
	Revision uint64 `json:"revision" example:"1"`
	// CreatedAt 是创建时间。
	CreatedAt time.Time `json:"created_at" example:"2026-01-01T00:00:00Z"`
	// UpdatedAt 是最近修改时间。
	UpdatedAt time.Time `json:"updated_at" example:"2026-01-01T00:00:00Z"`
}

type ListData struct {
	// Items 是附件记录。
	Items []Info `json:"items"`
	// Total 是记录总数。
	Total int64 `json:"total" example:"1"`
	// Page 是当前页码。
	Page int `json:"page" example:"1"`
	// PageSize 是每页条数。
	PageSize int `json:"page_size" example:"10"`
}

type URLData struct {
	// Filename 是原始文件名，供浏览器保存文件。
	Filename string `json:"filename" example:"申报材料.pdf"`
	// URL 是按需签发的短期 CDN URL。
	URL string `json:"url" example:"https://cdn.example.com/resource?eo-sign=..."`
}

type InfoResponse struct{ response.Envelope[Info] }
type ListResponse struct{ response.Envelope[ListData] }
type URLResponse struct{ response.Envelope[URLData] }

// UploadRequest 只接收本次文件的元数据，正文由客户端直传 COS。
type UploadRequest struct {
	// Filename 是保留的原始文件名，不能包含路径或策略通配符。
	Filename string `json:"filename" example:"材料.pdf"`
	// Size 是文件准确字节数，须符合 YAML 的单文件限制。
	Size int64 `json:"size" example:"1024"`
	// ContentType 是上传请求的声明媒体类型，由 STS 精确约束。
	ContentType string `json:"content_type" example:"application/pdf"`
}

// CompleteUploadRequest 只引用服务端已登记的上传记录，不接受对象路径。
type CompleteUploadRequest struct {
	// UploadID 是申请返回的上传编号。
	UploadID uint64 `json:"upload_id" example:"1"`
	// Revision 是申请返回的并发版本。
	Revision uint64 `json:"revision" example:"1"`
}

// UploadData 仅供申请上传的即时响应，不应进入查询缓存。
type UploadData struct {
	// UploadID 是待完成附件编号。
	UploadID uint64 `json:"upload_id" example:"1"`
	// Revision 是确认上传时使用的并发版本。
	Revision uint64 `json:"revision" example:"1"`
	// Upload 是仅限本对象的 COS 临时上传凭据。
	Upload tencentcos.UploadParameters `json:"upload"`
}

type UploadResponse struct{ response.Envelope[UploadData] }
