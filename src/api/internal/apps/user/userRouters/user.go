package userRouters

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// LoginCaptcha 签发登录所需的一次性验证码鉴权密文。
// @Summary 获取登录验证码参数
// @Description 每次尝试登录前获取新密文，默认按客户端 IP 每秒最多一次且连续一分钟最多五次。未启用验证码时 data 为 null。
// @Tags 用户
// @Produce json
// @Param X-CSRF-Protection header string true "跨站请求保护标识，固定为 1" default(1)
// @Success 200 {object} userModels.LoginCaptchaResponse "登录验证码初始化参数"
// @Router /v1/login-captcha [post]
func LoginCaptcha(c echo.Context) error {
	return response.Success(c, userSvc.IssueLoginCaptcha(c.Request().Context()))
}

// Login 校验登录参数并设置仅供 HTTP 使用的会话 Cookie。
// @Summary 用户名密码登录
// @Description 启用验证码时，需提交验证通过后的票据及随机串。仅启用且允许密码登录的账号可登录。
// @Description 全局密码登录关闭时统一返回 401，不创建会话；账号原有密码设置、邮件登录和已有会话保持不变。
// @Description 会话通过 HttpOnly Cookie 返回，待改密会话完成改密后才能访问业务接口。
// @Tags 用户
// @Accept json
// @Produce json
// @Param X-CSRF-Protection header string true "跨站请求保护标识，固定为 1" default(1)
// @Param body body userModels.LoginRequest true "登录参数"
// @Success 200 {object} userModels.InfoResponse "登录成功并设置会话 Cookie"
// @Failure 401 {object} userModels.EmptyResponse "用户名或密码错误"
// @Router /v1/login [post]
func Login(c echo.Context) error {
	// 只接受固定的 JSON 请求模型，拒绝未知字段与多个文档。
	var request userModels.LoginRequest
	if err := decode(c, &request); err != nil {
		return userSvc.Reject(
			c.Request().Context(),
			accessConstants.UserLogin,
			http.StatusBadRequest,
		)
	}
	user, token, err := userSvc.Login(c.Request().Context(), request, c.RealIP())
	if err != nil {
		return err
	}

	// 审计与会话提交后才写 Cookie，不向 JSON、日志或浏览器存储暴露会话。
	setCookie(c, token, int(config.Config.User.SessionTTL/time.Second))
	c.Response().Header().Set("Cache-Control", "no-store")
	return response.Success(c, user)
}

// Info 返回当前登录账号的固定字段目录。
// @Summary 当前用户信息
// @Description 实时检查账号与会话状态，返回基本信息和身份类型；学生身份同时返回绑定空间，且仅在启用空间内保持在籍及登记学号、邮箱未变更时有效。成功和失败均不写入审计。
// @Tags 用户
// @Produce json
// @Success 200 {object} userModels.InfoResponse "当前登录用户"
// @Router /v1/user-info [get]
func Info(c echo.Context) error {
	// 当前身份由认证中间件提供，此接口不写入审计。
	user, err := userSvc.Info(c.Request().Context())
	if err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return response.Success(c, user)
}

// Logout 撤销服务端会话并清除 Cookie。
// @Summary 退出登录
// @Description 使用 Cookie 仅撤销当前会话，学生只删除当前绑定空间的学生会话。
// @Description API Token 和其他设备会话保持有效，审计失败时全部回滚。
// @Tags 用户
// @Produce json
// @Param X-CSRF-Protection header string true "跨站请求保护标识，固定为 1" default(1)
// @Success 200 {object} userModels.EmptyResponse "当前会话已撤销，data 为 null"
// @Router /v1/logout [post]
func Logout(c echo.Context) error {
	// 服务端成功撤销且审计持久化后再清理客户端 Cookie。
	if err := userSvc.Logout(c.Request().Context()); err != nil {
		return err
	}
	setCookie(c, "", -1)
	c.Response().Header().Set("Cache-Control", "no-store")
	return response.Success[*userModels.EmptyData](c, nil)
}

// ChangePassword 校验改密参数，在事务提交后更新会话 Cookie。
// @Summary 修改临时密码
// @Description 仅待改密会话可用。验证当前密码后保存新密码、清除待改密状态、作废旧会话并设置新 Cookie。
// @Description 已有 API Token 保持有效。
// @Tags 用户
// @Accept json
// @Produce json
// @Param X-CSRF-Protection header string true "跨站请求保护标识，固定为 1" default(1)
// @Param body body userModels.ChangePasswordRequest true "当前密码与新密码摘要"
// @Success 200 {object} userModels.InfoResponse "已修改密码并设置新会话 Cookie"
// @Router /v1/change-password [post]
func ChangePassword(c echo.Context) error {
	var request userModels.ChangePasswordRequest
	if err := decode(c, &request); err != nil {
		return userSvc.Reject(
			c.Request().Context(),
			accessConstants.UserChangePassword,
			http.StatusBadRequest,
		)
	}
	user, token, err := userSvc.ChangePassword(c.Request().Context(), request)
	if err != nil {
		return err
	}
	setCookie(c, token, int(config.Config.User.SessionTTL/time.Second))
	c.Response().Header().Set("Cache-Control", "no-store")
	return response.Success(c, user)
}

// Create 校验管理员添加普通用户的参数。
// @Summary 管理员添加用户
// @Description 仅系统管理员可添加普通账号。默认允许密码登录并要求首次登录更换初始密码；关闭密码登录时不接收口令摘要，也不要求修改密码，可通过邮件登录。工作空间权限需单独授予。
// @Tags 用户
// @Accept json
// @Produce json
// @Param X-CSRF-Protection header string true "跨站请求保护标识，固定为 1" default(1)
// @Param body body userModels.CreateRequest true "新用户参数"
// @Success 200 {object} userModels.InfoResponse "已创建的普通账号"
// @Router /v1/system/users [post]
func Create(c echo.Context) error {
	// 校验固定字段，后续授权与业务写入交给服务层。
	var request userModels.CreateRequest
	if err := decode(c, &request); err != nil {
		return userSvc.Reject(
			c.Request().Context(),
			accessConstants.UserCreate,
			http.StatusBadRequest,
		)
	}
	user, err := userSvc.Create(c.Request().Context(), request)
	if err != nil {
		return err
	}
	return response.Success(c, user)
}

func decode(c echo.Context, value any) error {
	// 表单与 XML 不能绕过 JSON 和自定义请求头带来的跨站保护。
	contentType, _, err := mime.ParseMediaType(c.Request().Header.Get("Content-Type"))
	if err != nil || contentType != echo.MIMEApplicationJSON {
		return echo.ErrBadRequest
	}
	decoder := json.NewDecoder(c.Request().Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return echo.ErrBadRequest
	}
	return c.Validate(value)
}

func setCookie(c echo.Context, token string, maxAge int) {
	// Cookie 域从 YAML 读取，Path 固定为 /；本地 HTTP 可关闭 Secure，HTTPS 部署必须开启。
	expires := time.Now().Add(config.Config.User.SessionTTL)
	if maxAge < 0 {
		expires = time.Unix(1, 0)
	}
	// #nosec G124 -- Secure 从 YAML 读取，支持本地 HTTP；生产 HTTPS 必须启用。
	c.SetCookie(&http.Cookie{
		Name:     config.Config.User.CookieName,
		Domain:   config.Config.User.CookieDomain,
		Path:     "/",
		Value:    token,
		Expires:  expires,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   config.Config.User.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}
