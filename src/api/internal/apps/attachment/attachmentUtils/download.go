package attachmentUtils

import (
	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// RespondDownload 仅在授权与读取审计提交后返回临时地址，不读取对象内容。
func RespondDownload(c echo.Context, data attachmentModels.URLData) error {
	// 含签名的响应禁止缓存，且不向目标文件发送来源地址。
	c.Response().Header().Set("Cache-Control", "no-store")
	c.Response().Header().Set("Referrer-Policy", "no-referrer")
	return response.Success(c, data)
}
