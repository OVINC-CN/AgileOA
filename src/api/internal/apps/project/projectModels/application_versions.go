package projectModels

import (
	"time"

	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// ApplicationVersion 是从普通审计快照提取的历史申报内容，不提供恢复或下载入口。
type ApplicationVersion struct {
	ApplicationValues
	// Revision 是当时的申报并发版本，可能因无审计的后台变更而不连续。
	Revision uint64 `json:"revision" example:"1"`
	// ActorName 是该版本写入审计的修改人身份快照，无法确认时为空。
	ActorName string `json:"actor_name" example:"张三(zhangsan)"`
	// UpdatedAt 是快照中的数据库更新时间。
	UpdatedAt time.Time `json:"updated_at"`
	// Details 保留当时的子项顺序，以稳定标识关联当前内容。
	Details []ApplicationVersionDetail `json:"details"`
}

type ApplicationVersionDetail struct {
	ScoreItem
	// ID 是当时的子项编号，兼容缺少稳定标识的旧快照。
	ID uint64 `json:"id" example:"1"`
	// DetailCode 是审计中的稳定子项标识。
	DetailCode string `json:"detail_code" example:"0123456789abcdef0123456789abcdef"`
	// Attachments 只返回历史文件元数据，不能据此下载已删除文件。
	Attachments []ApplicationVersionAttachment `json:"attachments"`
}

type ApplicationVersionAttachment struct {
	// Filename 是当时的原始文件名。
	Filename string `json:"filename" example:"申报材料.pdf"`
}

type ApplicationVersions struct {
	// Items 仅包含最近历史版、截止前最后一版和截止时刻及之后首版，去重后按版本倒序排列，最多三版；未设截止时间时仅最近历史版。
	Items []ApplicationVersion `json:"items"`
}

type ApplicationVersionsResponse struct {
	response.Envelope[ApplicationVersions]
}
