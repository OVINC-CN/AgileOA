package accessUtils

import (
	"encoding/json"
	"io"
	"mime"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/workspace/workspaceUtils"
)

// Decode 只接受固定 JSON 模型，拒绝未知字段、多个文档及非 JSON 请求体。
func Decode(c echo.Context, value any) error {
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

// PathID 读取正整数路径编号，工作空间身份不从请求体或会话隐式推导。
func PathID(c echo.Context, name string) (uint64, error) {
	id, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil || id == 0 {
		return 0, echo.ErrBadRequest
	}
	return id, nil
}

// WorkspaceID 只接受 16 位英文小写字母和数字混合编号。
func WorkspaceID(c echo.Context) (string, error) {
	value := c.Param("workspace_id")
	if !workspaceUtils.ValidID(value) {
		return "", echo.ErrBadRequest
	}
	return value, nil
}
