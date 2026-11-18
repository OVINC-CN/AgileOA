package userRouters

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// APIToken 校验账号编号并返回可再次查看的凭据。
// @Summary 查看用户 API Token
// @Description 仅系统管理员可查看。未配置时 token 为空；读取审计提交后返回完整凭据，响应禁止缓存。
// @Tags 用户管理
// @Produce json
// @Param id path int true "账号编号"
// @Success 200 {object} userModels.APITokenResponse "API Token 及生成、重置时间"
// @Failure 400 {object} userModels.EmptyResponse "账号编号无效"
// @Failure 401 {object} userModels.EmptyResponse "请先登录"
// @Failure 403 {object} userModels.EmptyResponse "仅系统管理员可访问"
// @Failure 404 {object} userModels.EmptyResponse "用户不存在"
// @Failure 503 {object} userModels.EmptyResponse "账号或暂时无法完成操作，请稍后重试"
// @Router /v1/system/users/{id}/api-token [get]
func APIToken(c echo.Context) error {
	id, err := apiTokenUserID(c, accessConstants.UserAPITokenRead)
	if err != nil {
		return err
	}
	data, err := userSvc.APIToken(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// ResetAPIToken 校验账号编号并生成或重置凭据。
// @Summary 配置用户 API Token
// @Description 仅系统管理员可为启用且已完成改密的账号生成或重置 Token。每个账号最多一个，旧值立即失效。
// @Description 仅撤销全部会话、停用账号、主动撤销或重置 Token 时撤销已有 Token，退出登录及密码变更不影响它。
// @Description Token 仅用于 /v1/mcp，通过 Authorization: Bearer 传递，沿用账号状态和业务权限；其他接口使用 Cookie。
// @Tags 用户管理
// @Produce json
// @Param id path int true "账号编号"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} userModels.APITokenResponse "新 API Token，支持后续再次查看"
// @Failure 400 {object} userModels.EmptyResponse "账号编号无效"
// @Failure 401 {object} userModels.EmptyResponse "请先登录"
// @Failure 403 {object} userModels.EmptyResponse "系统权限或跨站保护校验失败"
// @Failure 404 {object} userModels.EmptyResponse "用户不存在"
// @Failure 409 {object} userModels.EmptyResponse "账号停用或尚未完成密码修改"
// @Failure 503 {object} userModels.EmptyResponse "账号或暂时无法完成操作，请稍后重试"
// @Router /v1/system/users/{id}/api-token [put]
func ResetAPIToken(c echo.Context) error {
	id, err := apiTokenUserID(c, accessConstants.UserAPITokenWrite)
	if err != nil {
		return err
	}
	data, err := userSvc.ResetAPIToken(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// DeleteAPIToken 校验账号编号并撤销凭据。
// @Summary 撤销用户 API Token
// @Description 仅系统管理员可撤销。删除凭据与审计同事务，未配置时成功返回空数据。
// @Tags 用户管理
// @Produce json
// @Param id path int true "账号编号"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} userModels.EmptyResponse "API Token 已撤销，data 为 null"
// @Failure 400 {object} userModels.EmptyResponse "账号编号无效"
// @Failure 401 {object} userModels.EmptyResponse "请先登录"
// @Failure 403 {object} userModels.EmptyResponse "系统权限或跨站保护校验失败"
// @Failure 404 {object} userModels.EmptyResponse "用户不存在"
// @Failure 503 {object} userModels.EmptyResponse "账号或暂时无法完成操作，请稍后重试"
// @Router /v1/system/users/{id}/api-token [delete]
func DeleteAPIToken(c echo.Context) error {
	id, err := apiTokenUserID(c, accessConstants.UserAPITokenDelete)
	if err != nil {
		return err
	}
	if err := userSvc.DeleteAPIToken(c.Request().Context(), id); err != nil {
		return err
	}
	return response.Success[*userModels.EmptyData](c, nil)
}

func apiTokenUserID(c echo.Context, action string) (uint64, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return 0, userSvc.Reject(c.Request().Context(), action, http.StatusBadRequest)
	}
	return id, nil
}
