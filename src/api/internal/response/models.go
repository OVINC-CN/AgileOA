package response

// Envelope 是所有业务接口统一使用的响应结构。
type Envelope[T any] struct {
	// Trace 是当前请求的 OpenTelemetry Trace ID，关闭上报时仍然存在。
	Trace string `json:"trace" example:"4bf92f3577b34da6a3ce929d0e0e4736"`
	// ErrorMsg 是简体中文错误说明；请求成功时为空字符串。
	ErrorMsg string `json:"error_msg" example:""`
	// Data 是接口业务数据，结构由具体接口定义。
	Data T `json:"data" extensions:"x-nullable"`
}

// ErrorData 表示错误响应中的空业务数据。
type ErrorData struct{}
