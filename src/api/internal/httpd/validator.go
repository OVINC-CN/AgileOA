package httpd

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func (v requestValidator) Validate(value any) error {
	// 将校验错误交给统一处理函数，避免返回第三方库的英文错误。
	if err := v.validate.Struct(value); err != nil {
		return echo.NewHTTPError(http.StatusUnprocessableEntity).SetInternal(err)
	}
	return nil
}
