package pagination

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// Query 保存经过边界校验的分页参数。
type Query struct {
	Page     int
	PageSize int
}

// Parse 读取分页请求，默认值与单页上限来自 YAML。
func Parse(c echo.Context) (Query, error) {
	query := Query{Page: 1, PageSize: config.Config.HTTP.DefaultPageSize}
	var err error
	if value := c.QueryParam("page"); value != "" {
		query.Page, err = strconv.Atoi(value)
	}
	if err == nil && c.QueryParam("page_size") != "" {
		query.PageSize, err = strconv.Atoi(c.QueryParam("page_size"))
	}
	// 固定最大页码保证在所有支持的整数位宽上计算偏移时不会溢出。
	const maxPage = 1000000
	if err != nil || query.Page < 1 || query.Page > maxPage || query.PageSize < 1 ||
		query.PageSize > config.Config.HTTP.MaxPageSize {
		return Query{}, &response.BusinessError{Status: http.StatusBadRequest, Message: "分页参数无效"}
	}
	return query, nil
}
