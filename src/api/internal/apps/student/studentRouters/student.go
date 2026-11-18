package studentRouters

import (
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/student/studentConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/student/studentModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/student/studentSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/pagination"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/queryfilter"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// List 学生基本信息明细表。
// @Summary 学生基本信息明细表
// @Description 按学生编号倒序分页，返回固定基础字段，不包含敏感信息；按敏感字段筛选时需要处理敏感信息权限。
// @Tags 学生信息
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param keyword query string false "学号或姓名关键词，最多 100 个字符"
// @Param position_status query []string false "班内任职状态：assigned、unassigned 或留空；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param grade query []string false "年级；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param status query []string false "用户手动维护的学生状态，最多 8 个字符；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param education_level query []string false "学历；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param major query []string false "专业；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param class_name query []string false "班级；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param class_position query []string false "班内任职；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param admission_type query []string false "录取类型；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param advisor query []string false "导师姓名；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param gender query []string false "性别；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param ethnicity query []string false "民族；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param political_status query []string false "政治面貌；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param accommodation_type query []string false "住宿类型：unknown 未知、on_campus 校内、off_campus 校外、joint_abroad 出国联培，留空表示全部；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param campus query []string false "住宿园区；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param building query []string false "宿舍楼；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param room query []string false "宿舍号；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param only_child query []string false "是否独生子女；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param single_parent query []string false "是否单亲家庭；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param orphan query []string false "是否孤儿；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param student_loan query []string false "是否助学贷款；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param registered_poverty query []string false "是否建档立卡生；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param has_religion query []string false "是否有宗教信仰；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param religion_category query []string false "宗教信仰类别；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param religion_identification query []string false "宗教信仰标识；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param page query int false "页码，从 1 开始" default(1)
// @Param page_size query int false "每页数量，默认与上限来自 YAML"
// @Success 200 {object} studentModels.ListResponse "审计提交后返回"
// @Router /v1/workspaces/{workspace_id}/students [get]
func List(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	query, err := parseQuery(c)
	if err != nil {
		return err
	}
	data, err := studentSvc.List(
		c.Request().Context(),
		workspaceID,
		query,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Statistics 学生数量统计。
// @Summary 学生数量统计
// @Description 与明细共用筛选，返回学生、班级、宗教信仰和校外住宿四项数量；按敏感字段筛选时需要处理敏感信息权限。
// @Tags 学生信息
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param keyword query string false "学号或姓名关键词，最多 100 个字符"
// @Param position_status query []string false "班内任职状态：assigned、unassigned 或留空；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param grade query []string false "年级；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param status query []string false "用户手动维护的学生状态，最多 8 个字符；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param education_level query []string false "学历；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param major query []string false "专业；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param class_name query []string false "班级；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param class_position query []string false "班内任职；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param admission_type query []string false "录取类型；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param advisor query []string false "导师姓名；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param gender query []string false "性别；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param ethnicity query []string false "民族；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param political_status query []string false "政治面貌；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param accommodation_type query []string false "住宿类型：unknown 未知、on_campus 校内、off_campus 校外、joint_abroad 出国联培，留空表示全部；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param campus query []string false "住宿园区；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param building query []string false "宿舍楼；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param room query []string false "宿舍号；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param only_child query []string false "是否独生子女；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param single_parent query []string false "是否单亲家庭；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param orphan query []string false "是否孤儿；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param student_loan query []string false "是否助学贷款；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param registered_poverty query []string false "是否建档立卡生；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param has_religion query []string false "是否有宗教信仰；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param religion_category query []string false "宗教信仰类别；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param religion_identification query []string false "宗教信仰标识；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Success 200 {object} studentModels.StatisticsResponse "审计提交后返回"
// @Router /v1/workspaces/{workspace_id}/students/statistics [get]
func Statistics(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	query, err := parseQuery(c)
	if err != nil {
		return err
	}
	data, err := studentSvc.Statistics(
		c.Request().Context(),
		workspaceID,
		query,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Fields 学生字段目录。
// @Summary 学生字段目录
// @Description 返回固定字段、基本或敏感类别、类型、长度、中文说明、示例和内置英文选项。
// @Tags 学生信息
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} studentModels.FieldsResponse "审计提交后返回"
// @Router /v1/workspaces/{workspace_id}/students/fields [get]
func Fields(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := studentSvc.Fields(c.Request().Context(), workspaceID)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Options 学生筛选选项。
// @Summary 学生筛选选项
// @Description 仅从当前工作空间基础字段 DISTINCT 查询，复用查看列表权限。
// @Tags 学生信息
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param field query string true "固定目录中可枚举的基础字段代码"
// @Success 200 {object} studentModels.OptionsResponse "审计提交后返回"
// @Router /v1/workspaces/{workspace_id}/students/options [get]
func Options(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := studentSvc.Options(
		c.Request().Context(),
		workspaceID,
		c.QueryParam("field"),
		false,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// SensitiveOptions 查询敏感字段枚举。
// @Summary 学生敏感字段枚举
// @Description 从当前工作空间明细 DISTINCT 查询，复用处理敏感信息权限。
// @Tags 学生信息
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param field query string true "固定目录中可枚举的敏感字段代码"
// @Success 200 {object} studentModels.OptionsResponse "敏感字段枚举"
// @Router /v1/workspaces/{workspace_id}/students/sensitive-options [get]
func SensitiveOptions(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := studentSvc.Options(
		c.Request().Context(),
		workspaceID,
		c.QueryParam("field"),
		true,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Create 添加学生基本信息。
// @Summary 添加学生基本信息
// @Description 必须填写字段目录中的必填字段及身份证号码，同时需要处理敏感信息权限。出生日期仅由身份证号码生成，业务和审计同事务。
// @Tags 学生信息
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Accept json
// @Param body body studentModels.CreateRequest true "固定字段请求"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} studentModels.MutationResponse "审计提交后返回"
// @Router /v1/workspaces/{workspace_id}/students [post]
func Create(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	var request studentModels.CreateRequest
	if accessUtils.Decode(c, &request) != nil {
		return echo.ErrBadRequest
	}
	data, err := studentSvc.Create(
		c.Request().Context(),
		workspaceID,
		request,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Detail 查看学生基本信息详情。
// @Summary 查看学生基本信息详情
// @Description 返回基础信息与短时头像链接，住宿类型为校外时附带校外住宿明细。
// @Tags 学生信息
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "学生记录编号"
// @Success 200 {object} studentModels.DetailResponse "审计提交后返回"
// @Router /v1/workspaces/{workspace_id}/students/{id} [get]
func Detail(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	data, err := studentSvc.Detail(
		c.Request().Context(),
		workspaceID,
		id,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Update 修改学生基本信息。
// @Summary 修改学生基本信息
// @Description 使用版本防止覆盖其他人的修改，可同时保存校外住宿明细及额外授权的完整敏感信息。
// @Tags 学生信息
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "学生记录编号"
// @Accept json
// @Param body body studentModels.UpdateRequest true "固定字段请求"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} studentModels.MutationResponse "审计提交后返回"
// @Router /v1/workspaces/{workspace_id}/students/{id} [put]
func Update(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	var request studentModels.UpdateRequest
	if accessUtils.Decode(c, &request) != nil {
		return echo.ErrBadRequest
	}
	data, err := studentSvc.Update(
		c.Request().Context(),
		workspaceID,
		id,
		request,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Delete 删除学生。
// @Summary 删除学生
// @Description 硬删除学生及自有校外住宿明细，同事务保留审计与加密敏感快照；学生会话和其他业务引用阻止删除。
// @Tags 学生信息
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "学生记录编号"
// @Param revision query int true "当前学生版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} accessModels.EmptyResponse "审计提交后返回"
// @Router /v1/workspaces/{workspace_id}/students/{id} [delete]
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
	if err := studentSvc.Delete(
		c.Request().Context(),
		workspaceID,
		id,
		revision,
	); err != nil {
		return err
	}
	return response.Success[*accessModels.EmptyData](c, nil)
}

// ReadSensitive 查看学生敏感信息。
// @Summary 查看学生敏感信息
// @Description 统一的处理敏感信息权限，固定返回身份证号、家庭详细住址和八项本人家庭专项信息。
// @Tags 学生信息
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "学生记录编号"
// @Success 200 {object} studentModels.SensitiveResponse "审计提交后返回"
// @Router /v1/workspaces/{workspace_id}/students/{id}/sensitive [get]
func ReadSensitive(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	data, err := studentSvc.Sensitive(
		c.Request().Context(),
		workspaceID,
		id,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// UpdateSensitive 修改学生敏感信息。
// @Summary 修改学生敏感信息
// @Description 统一使用处理敏感信息权限；必须提交全部十四个敏感字段，全量更新，空值清除字段。
// @Tags 学生信息
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "学生记录编号"
// @Accept json
// @Param body body studentModels.SensitiveRequest true "固定字段请求"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} studentModels.MutationResponse "审计提交后返回"
// @Router /v1/workspaces/{workspace_id}/students/{id}/sensitive [put]
func UpdateSensitive(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	var request studentModels.SensitiveRequest
	if accessUtils.Decode(c, &request) != nil {
		return echo.ErrBadRequest
	}
	data, err := studentSvc.UpdateSensitive(
		c.Request().Context(),
		workspaceID,
		id,
		request,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Template 学生导入模板。
// @Summary 学生导入模板
// @Description 生成所有单元格为文本的 XLSX 空模板，始终包含必填身份证号码列，可包含全部敏感字段。填写说明单独存放，日期使用 yyyy-mm 或 yyyy-mm-dd。
// @Tags 学生信息
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param include_sensitive query bool false "是否包含全部敏感字段，默认 false"
// @Success 200 {object} studentModels.FileResponse "审计提交后返回"
// @Router /v1/workspaces/{workspace_id}/students/template [get]
func Template(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	includeSensitive, err := parseIncludeSensitive(c)
	if err != nil {
		return err
	}
	data, err := studentSvc.Template(
		c.Request().Context(),
		workspaceID,
		includeSensitive,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Import 导入学生。
// @Summary 导入学生
// @Description 仅支持 Base64 XLSX，必须提供全部必填字段及身份证号码，同时需要处理敏感信息权限；出生日期写入值被忽略。仅新增，任一行失败整批回滚。
// @Tags 学生信息
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Accept json
// @Param body body studentModels.ImportRequest true "固定字段请求"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} studentModels.ImportResponse "审计提交后返回"
// @Router /v1/workspaces/{workspace_id}/students/import [post]
func Import(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	var request studentModels.ImportRequest
	if accessUtils.Decode(c, &request) != nil {
		return echo.ErrBadRequest
	}
	data, err := studentSvc.Import(
		c.Request().Context(),
		workspaceID,
		request,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Export 导出学生。
// @Summary 导出学生
// @Description 导出文本格式 XLSX，日期使用 yyyy-mm 或 yyyy-mm-dd，默认导出筛选结果的基本信息。
// @Description 按敏感字段筛选或导出敏感字段均需要处理敏感信息权限，XJ 明细与校外住宿明细始终检查此权限。
// @Tags 学生信息
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param include_sensitive query bool false "常规导出是否包含全部敏感字段，默认 false"
// @Param format query string false "留空为常规导出，religion 为 XJ 明细表，off_campus 为校外住宿明细表，报表仅包含筛选范围内符合类型的学生" Enums(religion,off_campus)
// @Param keyword query string false "学号或姓名关键词，最多 100 个字符"
// @Param position_status query []string false "班内任职状态：assigned、unassigned 或留空；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param grade query []string false "年级；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param status query []string false "用户手动维护的学生状态，最多 8 个字符；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param education_level query []string false "学历；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param major query []string false "专业；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param class_name query []string false "班级；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param class_position query []string false "班内任职；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param admission_type query []string false "录取类型；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param advisor query []string false "导师姓名；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param gender query []string false "性别；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param ethnicity query []string false "民族；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param political_status query []string false "政治面貌；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param accommodation_type query []string false "住宿类型：unknown 未知、on_campus 校内、off_campus 校外、joint_abroad 出国联培，留空表示全部；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param campus query []string false "住宿园区；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param building query []string false "宿舍楼；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param room query []string false "宿舍号；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param only_child query []string false "是否独生子女；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param single_parent query []string false "是否单亲家庭；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param orphan query []string false "是否孤儿；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param student_loan query []string false "是否助学贷款；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param registered_poverty query []string false "是否建档立卡生；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param has_religion query []string false "是否有宗教信仰；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param religion_category query []string false "宗教信仰类别；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param religion_identification query []string false "宗教信仰标识；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Success 200 {object} studentModels.FileResponse "审计提交后返回"
// @Router /v1/workspaces/{workspace_id}/students/export [get]
func Export(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	query, err := parseQuery(c)
	if err != nil {
		return err
	}
	includeSensitive, err := parseIncludeSensitive(c)
	if err != nil {
		return err
	}
	data, err := studentSvc.Export(
		c.Request().Context(),
		workspaceID,
		query,
		includeSensitive,
		c.QueryParam("format"),
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

func parseIncludeSensitive(c echo.Context) (bool, error) {
	if c.QueryParam("include_sensitive") == "" {
		return false, nil
	}
	value, err := strconv.ParseBool(c.QueryParam("include_sensitive"))
	if err != nil {
		return false, echo.ErrBadRequest
	}
	return value, nil
}

func parseQuery(c echo.Context) (studentModels.ListQuery, error) {
	page, err := pagination.Parse(c)
	if err != nil {
		return studentModels.ListQuery{}, err
	}
	query := studentModels.ListQuery{
		Query:   page,
		Keyword: strings.TrimSpace(c.QueryParam("keyword")),
		Filters: map[string][]string{},
	}
	for _, field := range studentConstants.Fields {
		if !field.Enumerable {
			continue
		}
		if values := queryfilter.Values(c.QueryParams()[field.Key], false); len(values) > 0 {
			query.Filters[field.Key] = values
		}
	}
	if values := queryfilter.Values(c.QueryParams()["position_status"], false); len(values) > 0 {
		query.Filters["position_status"] = values
	}
	return query, nil
}

// Profile 返回当前学生在绑定空间内的学籍信息。
// @Summary 查看我的个人信息
// @Description 仅在籍学生会话可读取本人在启用空间中的学生编号及学籍字段：学号、姓名、年级、状态、学历、专业、班级、班内任职、录取类型和来校时间。学生编号仅从可信登录身份提取，不接受客户端指定，无需额外配置 action 授权。审计提交后返回。
// @Tags 学生服务
// @Produce json
// @Param workspace_id path string true "学生会话绑定的工作空间编号"
// @Success 200 {object} studentModels.ProfileResponse "当前学生的学籍信息"
// @Failure 400 {object} accessModels.EmptyResponse "工作空间编号无效"
// @Failure 401 {object} accessModels.EmptyResponse "学生会话无效"
// @Failure 403 {object} accessModels.EmptyResponse "不能访问该学生服务"
// @Failure 503 {object} accessModels.EmptyResponse "学生服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/student-service [get]
func Profile(c echo.Context) error {
	// 只接收所属空间，学生编号与身份始终由认证中间件提供。
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	result, err := studentSvc.Profile(c.Request().Context(), workspaceID)
	if err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return response.Success(c, result)
}
