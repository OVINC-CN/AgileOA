package userRouters

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// SendLoginEmail 校验用户名、邮箱和安全验证后发送数字验证码。
// @Summary 发送登录邮件
// @Description 仅启用账号的用户名和已登记邮箱完全匹配时发送。安全验证规则与密码登录一致；邮件只包含 6 位数字验证码。返回独立随机校验标识及剩余有效秒数、重发等待秒数；重发成功后旧验证码失效。
// @Tags 用户
// @Accept json
// @Produce json
// @Param X-CSRF-Protection header string true "跨站请求保护标识，固定为 1" default(1)
// @Param body body userModels.SendLoginEmailRequest true "用户名、邮箱和验证码参数"
// @Success 200 {object} userModels.EmailLoginChallengeResponse "验证码已发送，返回本次校验信息"
// @Failure 400 {object} userModels.EmptyResponse "参数或验证码无效"
// @Failure 401 {object} userModels.EmptyResponse "用户名或邮箱错误"
// @Failure 403 {object} userModels.EmptyResponse "邮箱已被禁止接收邮件"
// @Failure 429 {object} userModels.EmptyResponse "请求过于频繁"
// @Failure 503 {object} userModels.EmptyResponse "账号或邮件登录暂时不可用"
// @Router /v1/login-email [post]
func SendLoginEmail(c echo.Context) error {
	// 固定请求模型拒绝未知字段，验证码在查询账号前验证。
	var request userModels.SendLoginEmailRequest
	if err := decode(c, &request); err != nil {
		return userSvc.Reject(
			c.Request().Context(), accessConstants.UserLoginEmailSend, http.StatusBadRequest,
		)
	}
	result, err := userSvc.SendLoginEmail(c.Request().Context(), request, c.RealIP())
	if err != nil {
		return err
	}

	// 不向响应返回邮箱详情、登录码或内部邮件服务状态。
	c.Response().Header().Set("Cache-Control", "no-store")
	return response.Success(c, result)
}

// LoginByEmail 消费邮件登录码，在事务提交后设置会话 Cookie。
// @Summary 邮件验证码登录
// @Description 学生验证码同时复核绑定的启用空间、学号、登记邮箱和在籍状态，直接建立独立学生会话，Cookie 值以 aws_ 开头；响应 identity_type=student 且提供绑定空间编号及名称，不授予管理账号权限。
// @Description 校验发送接口返回的 challenge_id 和邮件中的 6 位数字验证码，按每次签发原子累计错误次数，达到 YAML 的 max_attempts 后失效，不因错误重试延长有效期。
// @Description 复核有效期、账号启用状态、登记邮箱和会话版本。首次邮件登录或邮箱变更后，未提交用户名时返回 username_required=true、user=null，不消费验证码或设置 Cookie；用户名逐字匹配后才消费验证码并建立会话。已核验邮箱可直接登录；消费后不恢复验证码，待改密账号仍须先修改密码。
// @Tags 用户
// @Accept json
// @Produce json
// @Param X-CSRF-Protection header string true "跨站请求保护标识，固定为 1" default(1)
// @Param body body userModels.EmailLoginRequest true "本次校验标识、6 位数字验证码及待核验的用户名"
// @Success 200 {object} userModels.EmailLoginResponse "待核验用户名，或登录成功并设置会话 Cookie"
// @Failure 400 {object} userModels.EmptyResponse "参数无效"
// @Failure 401 {object} userModels.EmptyResponse "验证码错误、已过期、已失效或用户名不匹配"
// @Failure 429 {object} userModels.EmptyResponse "请求过于频繁"
// @Failure 503 {object} userModels.EmptyResponse "账号或邮件登录暂时不可用"
// @Router /v1/login-email-verify [post]
func LoginByEmail(c echo.Context) error {
	// 登录码只从请求体读取，避免进入 API URL、日志或追踪属性。
	var request userModels.EmailLoginRequest
	if err := decode(c, &request); err != nil {
		return userSvc.Reject(
			c.Request().Context(),
			accessConstants.UserLogin,
			http.StatusBadRequest,
		)
	}
	result, token, err := userSvc.LoginByEmail(c.Request().Context(), request)
	if err != nil {
		return err
	}

	// 会话提交后设置根路径 Cookie，待核验用户名时不设置 Cookie。
	if token != "" {
		setCookie(c, token, int(config.Config.User.SessionTTL/time.Second))
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return response.Success(c, result)
}
