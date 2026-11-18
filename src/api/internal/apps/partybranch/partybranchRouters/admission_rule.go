package partybranchRouters

import (
	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/partybranch/partybranchModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/partybranch/partybranchSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// ReadAdmissionRule 复用党支部查看权限读取准入规则。
// @Summary 查看党支部准入规则
// @Description 复用党支部查看权限，仅查询当前空间；返回完整规则树及固定字段、逻辑、运算符目录，读取审计提交后返回。
// @Tags 党支部
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "党支部编号"
// @Success 200 {object} partybranchModels.AdmissionRuleResponse "准入规则树及当前版本"
// @Router /v1/workspaces/{workspace_id}/cpc-party-branches/{id}/admission-rule [get]
func ReadAdmissionRule(c echo.Context) error {
	// 校验空间及党支部编号，读取沿用基础列表授权。
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	data, err := partybranchSvc.ReadAdmissionRule(
		c.Request().Context(), workspaceID, id,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// SaveAdmissionRule 按当前党支部版本替换或清空准入规则树。
// @Summary 编辑党支部准入规则
// @Description 复用党支部编辑权限。条件组 type=group，logic 支持 and、or，children 可多层嵌套。
// @Description 条件字段支持 advisor（导师姓名）、major（专业）、education_level（学生学历）、student_no（学号），operator 固定 in，values 为非空候选值集合，按完整文本匹配；学号最多 32 字符并保留前导零。
// @Description rule=null 清空，未配置不表示全部准入；版本与党支部基础信息共用，写入和审计同事务，不自动授权、添加或重排人员。
// @Tags 党支部
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "党支部编号"
// @Param body body partybranchModels.AdmissionRuleRequest true "完整规则树及当前 revision"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} partybranchModels.AdmissionRuleResponse "保存后的规则树及新版本"
// @Router /v1/workspaces/{workspace_id}/cpc-party-branches/{id}/admission-rule [put]
func SaveAdmissionRule(c echo.Context) error {
	// 校验路径、版本和固定模型，拒绝未知请求字段。
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	var request partybranchModels.AdmissionRuleRequest
	if accessUtils.Decode(c, &request) != nil {
		return echo.ErrBadRequest
	}
	data, err := partybranchSvc.SaveAdmissionRule(
		c.Request().Context(), workspaceID, id, request,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}
