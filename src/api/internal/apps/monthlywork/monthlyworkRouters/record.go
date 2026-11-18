package monthlyworkRouters

import (
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/monthlywork/monthlyworkConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/monthlywork/monthlyworkModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/monthlywork/monthlyworkSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/queryfilter"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// List 查询月度工作时间轴记录。
// @Summary 查询月度工作
// @Description 年份与月份均支持多选，省略时不限制对应维度，一次返回筛选范围内的全部记录，不分页；按日期及编号倒序，附件元数据单独检查读取权限，审计提交后返回。
// @Tags 月度工作
// @Description 学生工作与党建工作通过各自路径访问，分别授权并隔离数据；请求体不接受类别字段。
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param year query []int false "年份，1900 至 9999；省略时不限制年份；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param month query []int false "月份，1 至 12；省略时不限制月份；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param keyword query string false "工作项目、负责人、参与人或详细说明关键词，最多 100 字符"
// @Param scope query []string false "工作范围：mine 为当前用户负责或参与的工作，all 为全部工作，省略时兼容查询全部工作；支持多选，同一参数可重复传递" Enums(mine,all) collectionFormat(multi)
// @Success 200 {object} monthlyworkModels.ListResponse "工作记录"
// @Router /v1/workspaces/{workspace_id}/monthly-work [get]
// @Router /v1/workspaces/{workspace_id}/cpc-monthly-work [get]
func List(c echo.Context) error {
	// 提取租户及固定筛选参数。
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	query := monthlyworkModels.Query{
		Keyword: strings.TrimSpace(c.QueryParam("keyword")),
		Scopes:  queryfilter.Values(c.QueryParams()["scope"], false),
	}
	query.Years, err = queryfilter.Integers(c.QueryParams()["year"])
	if err != nil {
		return echo.ErrBadRequest
	}
	query.Months, err = queryfilter.Integers(c.QueryParams()["month"])
	if err != nil {
		return echo.ErrBadRequest
	}
	data, err := monthlyworkSvc.List(
		c.Request().Context(),
		workspaceID,
		workCategory(c),
		query,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Create 新增月度工作记录。
// @Summary 新增月度工作
// @Description 保存成功后向负责人及参与人发送站内工作分配通知，跳过当前操作人，通知与工作及审计同事务写入。
// @Description 日期、工作项目和负责人必填，参与人可多选，说明及多条外部链接可选；负责人及参与人须为当前空间可用账号，不推导权限；附件在创建记录后单独上传。
// @Tags 月度工作
// @Description 学生工作与党建工作通过各自路径访问，分别授权并隔离数据；请求体不接受类别字段。
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param body body monthlyworkModels.Request true "工作记录，新建时 revision 为零"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} monthlyworkModels.InfoResponse "创建后的工作记录"
// @Router /v1/workspaces/{workspace_id}/monthly-work [post]
// @Router /v1/workspaces/{workspace_id}/cpc-monthly-work [post]
func Create(c echo.Context) error {
	return save(c, false)
}

// Update 修改月度工作、参与人及外部链接集合。
// @Summary 修改月度工作
// @Description 仅向新负责人及新增参与人发送站内通知，跳过当前操作人，保留原指派或普通编辑不重复通知。
// @Description 在工作空间锁内复核独立修改权限及并发版本；participant_ids 省略或传 null 时保留，空数组清空；原参与人停用或撤权后可保留，新增参与人按当前可用账号校验；附件通过独立接口管理。
// @Tags 月度工作
// @Description 学生工作与党建工作通过各自路径访问，分别授权并隔离数据；请求体不接受类别字段。
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "工作记录编号"
// @Param body body monthlyworkModels.Request true "完整字段及当前 revision"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} monthlyworkModels.InfoResponse "修改后的工作记录"
// @Router /v1/workspaces/{workspace_id}/monthly-work/{id} [put]
// @Router /v1/workspaces/{workspace_id}/cpc-monthly-work/{id} [put]
func Update(c echo.Context) error {
	return save(c, true)
}

func save(c echo.Context, update bool) error {
	// 校验路径与固定请求体，业务处理交由服务层。
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	var id uint64
	if update {
		id, err = accessUtils.PathID(c, "id")
		if err != nil {
			return err
		}
	}
	var request monthlyworkModels.Request
	if accessUtils.Decode(c, &request) != nil {
		return echo.ErrBadRequest
	}
	data, err := monthlyworkSvc.Save(
		c.Request().Context(),
		workspaceID,
		workCategory(c),
		id,
		request,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Delete 显式硬删除无附件的月度工作。
// @Summary 删除月度工作
// @Description 检查当前版本，硬删除工作并级联删除参与人关联和附件记录，历史审计保留。
// @Tags 月度工作
// @Description 学生工作与党建工作通过各自路径访问，分别授权并隔离数据；请求体不接受类别字段。
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "工作记录编号"
// @Param revision query int true "当前并发版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} accessModels.EmptyResponse "删除成功"
// @Router /v1/workspaces/{workspace_id}/monthly-work/{id} [delete]
// @Router /v1/workspaces/{workspace_id}/cpc-monthly-work/{id} [delete]
func Delete(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	revision, err := strconv.ParseUint(c.QueryParam("revision"), 10, 64)
	if err != nil || revision == 0 {
		return echo.ErrBadRequest
	}
	if err := monthlyworkSvc.Delete(
		c.Request().Context(),
		workspaceID,
		workCategory(c),
		id,
		revision,
	); err != nil {
		return err
	}
	return response.Success[*accessModels.EmptyData](c, nil)
}

// Fields 返回月度工作的固定字段目录。
// @Summary 月度工作字段目录
// @Description 复用月度工作列表权限，提供中文说明及示例，不接受任意数据库列名。
// @Tags 月度工作
// @Description 学生工作与党建工作通过各自路径访问，分别授权并隔离数据；请求体不接受类别字段。
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} monthlyworkModels.FieldsResponse "固定字段目录"
// @Router /v1/workspaces/{workspace_id}/monthly-work/fields [get]
// @Router /v1/workspaces/{workspace_id}/cpc-monthly-work/fields [get]
func Fields(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := monthlyworkSvc.Fields(
		c.Request().Context(),
		workspaceID,
		workCategory(c),
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Template 下载月度工作导入模板。
// @Summary 月度工作导入模板
// @Description 提供固定中文 XLSX 表头；参与人账号可选，多人用英文逗号分隔；外部链接使用 JSON 数组；附件在记录创建后上传。
// @Tags 月度工作
// @Description 学生工作与党建工作通过各自路径访问，分别授权并隔离数据；请求体不接受类别字段。
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} monthlyworkModels.FileResponse "导入模板"
// @Router /v1/workspaces/{workspace_id}/monthly-work/template [get]
// @Router /v1/workspaces/{workspace_id}/cpc-monthly-work/template [get]
func Template(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := monthlyworkSvc.Template(
		c.Request().Context(),
		workspaceID,
		workCategory(c),
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Import 整批新增月度工作。
// @Summary 导入月度工作
// @Description 每条新增记录向负责人及参与人发送站内通知，跳过当前操作人，整批回滚时不发送通知。
// @Description 只接受固定中文 XLSX 表头及字段，兼容省略参与人账号列的旧模板；参与人用英文逗号分隔并校验当前空间可用账号；任一行校验、写入或审计失败均整批回滚。
// @Tags 月度工作
// @Description 学生工作与党建工作通过各自路径访问，分别授权并隔离数据；请求体不接受类别字段。
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param body body monthlyworkModels.ImportRequest true "Base64 XLSX 文本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} monthlyworkModels.ImportResponse "导入结果"
// @Router /v1/workspaces/{workspace_id}/monthly-work/import [post]
// @Router /v1/workspaces/{workspace_id}/cpc-monthly-work/import [post]
func Import(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	var request monthlyworkModels.ImportRequest
	if accessUtils.Decode(c, &request) != nil {
		return echo.ErrBadRequest
	}
	data, err := monthlyworkSvc.Import(
		c.Request().Context(),
		workspaceID,
		workCategory(c),
		request,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Owners 枚举当前工作空间的负责人及参与人用户。
// @Summary 月度工作负责人及参与人选项
// @Description 仅返回当前空间具有进入权限的启用账号编号、显示名称及账号，复用月度工作列表权限；不推导或授予任何权限。
// @Tags 月度工作
// @Description 学生工作与党建工作通过各自路径访问，分别授权并隔离数据；请求体不接受类别字段。
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} monthlyworkModels.OwnersResponse "负责人及参与人用户选项"
// @Router /v1/workspaces/{workspace_id}/monthly-work/owners [get]
// @Router /v1/workspaces/{workspace_id}/cpc-monthly-work/owners [get]
func Owners(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := monthlyworkSvc.Owners(
		c.Request().Context(),
		workspaceID,
		workCategory(c),
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// FilterOptions 查询当前空间完整的年份筛选项。
// @Summary 月度工作年份筛选枚举
// @Description 复用月度工作列表权限，仅查询当前空间已有记录的完整年份，供自定义选择器本地搜索。
// @Tags 月度工作
// @Description 学生工作与党建工作通过各自路径访问，分别授权并隔离数据；请求体不接受类别字段。
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} monthlyworkModels.FilterOptionsResponse "已有年份"
// @Router /v1/workspaces/{workspace_id}/monthly-work/filter-options [get]
// @Router /v1/workspaces/{workspace_id}/cpc-monthly-work/filter-options [get]
func FilterOptions(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := monthlyworkSvc.FilterOptions(
		c.Request().Context(),
		workspaceID,
		workCategory(c),
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

func workCategory(c echo.Context) string {
	// 使用已注册的路由模板确定类别，不读取客户端提交的类别。
	if strings.Contains(c.Path(), "/cpc-monthly-work") {
		return monthlyworkConstants.CategoryCPC
	}
	return monthlyworkConstants.CategoryStudent
}
