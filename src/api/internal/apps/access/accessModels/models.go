package accessModels

import (
	"time"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/workspace/workspaceModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/pagination"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// Permission 直接关联用户、操作与工作空间，不设置成员、角色或隐含权限。
type Permission struct {
	UserID      uint64    `gorm:"primaryKey"`
	Action      string    `gorm:"primaryKey"`
	WorkspaceID string    `gorm:"primaryKey"`
	CreatedAt   time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt   time.Time `gorm:"->;autoUpdateTime:false"`
}

// TableName 固定权限表名。
func (Permission) TableName() string { return "access_permissions" }

// Principal 由事务内认证与工作空间锁内授权产生，禁止从客户端绑定。
type Principal struct {
	User      userModels.User
	Workspace workspaceModels.Workspace
}

// Option 分离稳定英文标识与中文显示名称。
type Option struct {
	// Value 是稳定英文代码。
	Value string `json:"value" example:"workspace"`
	// Label 是中文显示名称。
	Label string `json:"label" example:"工作空间"`
}

// Endpoint 将操作与实际接口绑定，路径使用集中注册的路由模板。
type Endpoint struct {
	// Method 是大写 HTTP 方法。
	Method string `json:"method" example:"GET"`
	// Path 是接口路径模板，参数以冒号开头。
	Path string `json:"path" example:"/v1/workspaces/:workspace_id"`
}

// Action 是代码维护的权限目录项，外部请求不能新增或修改。
type Action struct {
	// Value 是稳定操作标识，不以中文名称参与权限判断。
	Value string `json:"value" example:"access_workspace"`
	// Label 是操作中文名称。
	Label string `json:"label" example:"进入工作空间"`
	// Description 说明操作范围及授权含义。
	Description string `json:"description" example:"进入已启用且获得显式授权的工作空间"`
	// Group 是操作所属的功能分组。
	Group Option `json:"group"`
	// Scope 区分 public、session、system 与 workspace，仅 workspace 可授予用户。
	Scope string `json:"scope" example:"workspace"`
	// Endpoints 是该操作对应的接口，尚无接口的保留操作为空数组。
	Endpoints []Endpoint `json:"endpoints"`
}

// CatalogData 返回只读操作目录。
type CatalogData struct {
	// Items 是已登记操作，只有 workspace 范围可勾选授权。
	Items []Action `json:"items"`
}

// UserInfo 是权限管理的固定账号目录，不关联账号敏感字段。
type UserInfo struct {
	// ID 是全局账号编号。
	ID uint64 `json:"id" example:"2"`
	// Username 是登录账号。
	Username string `json:"username" example:"zhangsan"`
	// DisplayName 是显示名称。
	DisplayName string `json:"display_name" example:"张三"`
	// IsEnabled 表示账号是否启用。
	IsEnabled bool `json:"is_enabled" example:"true"`
	// PermissionCount 是当前工作空间内有效的显式操作数量。
	PermissionCount int64 `json:"permission_count" example:"1"`
	// CreatedAt 是账号创建时间。
	CreatedAt time.Time `json:"created_at" example:"2026-11-03T00:00:00Z"`
	// UpdatedAt 是账号最近修改时间。
	UpdatedAt time.Time `json:"updated_at" example:"2026-11-03T00:00:00Z"`
}

// UserQuery 仅接收固定账号目录的查询条件。
type UserQuery struct {
	pagination.Query
	Keyword          string
	PermissionStatus string
}

// UserListData 是指定工作空间内的用户权限分页目录。
type UserListData struct {
	// Items 是符合授权状态的本页账号与当前工作空间的有效授权数量。
	Items []UserInfo `json:"items"`
	// Total 是符合条件的账号数量。
	Total int64 `json:"total" example:"1"`
	// Page 是当前页码。
	Page int `json:"page" example:"1"`
	// PageSize 是每页数量。
	PageSize int `json:"page_size" example:"20"`
}

// ActionsData 返回该用户在当前工作空间的显式授权。
type ActionsData struct {
	// Actions 是登记过的 workspace 操作标识，空数组表示无权限。
	Actions []string `json:"actions" example:"access_workspace"`
	// Revision 是工作空间及其授权的共同版本。
	Revision uint64 `json:"revision" example:"1"`
}

// UpdateActionsRequest 完整替换用户授权；空数组显式撤销全部权限。
type UpdateActionsRequest struct {
	// Actions 只能包含目录中可授予用户的标识，禁止重复或未知操作。
	Actions []string `json:"actions" validate:"required" example:"access_workspace"`
	// Revision 必须与最近读取的工作空间授权版本一致，防止旧页面重新授予已撤销权限。
	Revision uint64 `json:"revision" validate:"required" example:"1"`
}

// MyWorkspace 是当前账号可发现的工作空间基本信息。
type MyWorkspace struct {
	// ID 是工作空间编号。
	ID string `json:"id" example:"w7m2k9p4s6n8c3x5"`
	// Name 是工作空间显示名称。
	Name string `json:"name" example:"教学事务"`
	// Description 是工作空间用途说明。
	Description string `json:"description" example:"处理日常教学事务"`
}

// MyWorkspacesData 返回当前账号可访问的全部工作空间。
type MyWorkspacesData struct {
	// Items 是获准进入的启用工作空间，空结果为数组。
	Items []MyWorkspace `json:"items"`
}

// MyWorkspacesResponse 复用统一响应字段。
type MyWorkspacesResponse struct {
	response.Envelope[MyWorkspacesData]
}

// ContextData 是进入工作空间时实时返回的上下文，不持久化于会话。
type ContextData struct {
	// CanReviewProjectApplications 表示账号有填报查看权限且被指派为组织审批人，不替代显式授权。
	CanReviewProjectApplications bool `json:"can_review_project_applications" example:"true"`

	// Workspace 是当前工作空间的基本信息。
	Workspace workspaceModels.Info `json:"workspace"`
	// Actions 是当前有效的显式操作权限，不代表其他工作空间权限。
	Actions []string `json:"actions" example:"access_workspace"`
}

// CatalogResponse 复用统一响应字段。
type CatalogResponse struct{ response.Envelope[CatalogData] }

// UserListResponse 复用统一响应字段。
type UserListResponse struct {
	response.Envelope[UserListData]
}

// ActionsResponse 复用统一响应字段。
type ActionsResponse struct{ response.Envelope[ActionsData] }

// ContextResponse 复用统一响应字段。
type ContextResponse struct{ response.Envelope[ContextData] }

// EmptyData 表示删除成功后没有业务数据。
type EmptyData struct{}

// EmptyResponse 复用统一响应字段，成功时 data 为 null。
type EmptyResponse struct{ response.Envelope[*EmptyData] }
