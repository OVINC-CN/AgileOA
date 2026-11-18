package emailRouters

import (
	"encoding/json"
	"io"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/email/emailModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/email/emailSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// Callback 校验腾讯云邮件事件并持久化，不读取账号凭据或请求 IP。
// @Summary 接收邮件事件回调
// @Description 接收腾讯云邮件事件，不依赖登录 Cookie、CSRF 标识或既有发信记录。标准字段独立落库并按内容去重，未知扩展字段忽略；点击地址仅保存站点和路径。email.abnormal_events 配置的事件直接永久拉黑收件邮箱，后续所有业务发信均拒绝该地址。不保存或校验 IP。
// @Tags 邮件
// @Accept json
// @Produce json
// @Param body body emailModels.CallbackRequest true "腾讯云邮件通知事件"
// @Success 200 {object} emailModels.EmptyResponse "回调已落库，重复回调直接成功，data 为 null"
// @Failure 400 {object} emailModels.EmptyResponse "回调参数无效"
// @Failure 503 {object} emailModels.EmptyResponse "回调暂时不可用，应重试"
// @Router /v1/callback/email [post]
func Callback(c echo.Context) error {
	// 云端可能新增自定义字段；只绑定固定目录，不保存原始 JSON 或额外字段。
	var request emailModels.CallbackRequest
	decoder := json.NewDecoder(c.Request().Body)
	if err := decoder.Decode(&request); err != nil {
		return echo.ErrBadRequest
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return echo.ErrBadRequest
	}
	if err := c.Validate(&request); err != nil {
		return echo.ErrBadRequest
	}
	if err := emailSvc.ReceiveCallback(c.Request().Context(), request); err != nil {
		return err
	}

	// 仅在回调及所需黑名单提交后返回成功，响应不包含内部处理细节。
	return response.Success[*emailModels.EmptyData](c, nil)
}
