package homeModels

import "github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"

// HealthResponse 复用统一响应字段，业务数据固定为空。
type HealthResponse struct {
	response.Envelope[*EmptyData]
}

// EmptyData 表示不对外提供业务数据。
type EmptyData struct{}
