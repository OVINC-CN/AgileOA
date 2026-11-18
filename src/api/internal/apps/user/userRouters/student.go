package userRouters

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// SendStudentLoginEmail 校验空间名称、学号、邮箱及安全验证后发送学生数字验证码。
// @Summary 发送学生登录邮件
// @Description 仅启用空间的名称、学号和登记邮箱逐字匹配，且学生状态为在籍时发送；不匹配统一提示用户不存在。邮件仅包含 6 位数字验证码，返回随机校验标识及剩余有效秒数、重发等待秒数；重发成功后旧验证码失效。
// @Tags 用户
// @Accept json
// @Produce json
// @Param X-CSRF-Protection header string true "跨站请求保护标识，固定为 1" default(1)
// @Param body body userModels.SendStudentLoginEmailRequest true "空间名称、学号、邮箱和验证码"
// @Success 200 {object} userModels.EmailLoginChallengeResponse "验证码已发送，返回本次校验信息"
// @Failure 400 {object} userModels.EmptyResponse "参数或验证码无效"
// @Failure 401 {object} userModels.EmptyResponse "用户不存在"
// @Failure 403 {object} userModels.EmptyResponse "邮箱已被禁止接收邮件"
// @Failure 429 {object} userModels.EmptyResponse "请求过于频繁"
// @Failure 503 {object} userModels.EmptyResponse "学生邮件登录暂时不可用"
// @Router /v1/student-login-email [post]
func SendStudentLoginEmail(c echo.Context) error {
	// 固定模型不接收目标空间编号或客户端身份。
	var request userModels.SendStudentLoginEmailRequest
	if err := decode(c, &request); err != nil {
		return userSvc.Reject(
			c.Request().Context(), accessConstants.StudentLoginEmailSend, http.StatusBadRequest,
		)
	}
	result, err := userSvc.SendStudentLoginEmail(
		c.Request().Context(),
		request,
		c.RealIP(),
	)
	if err != nil {
		return err
	}

	// 原始登录码仅通过邮件传递。
	c.Response().Header().Set("Cache-Control", "no-store")
	return response.Success(c, result)
}
