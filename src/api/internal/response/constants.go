package response

// MessageNotReady 等错误文案统一使用简体中文。
const (
	MessageNotReady         = "服务暂未就绪"
	MessageBadRequest       = "请求参数无效"
	MessageUnauthorized     = "请先登录"
	MessageForbidden        = "没有访问权限"
	MessageNotFound         = "资源不存在"
	MessageMethodNotAllowed = "不支持的请求方法"
	MessageRequestTooLarge  = "请求内容过大"
	MessageRateLimited      = "请求过于频繁"
	MessageTimeout          = "请求超时"
	MessageInternalError    = "服务器内部错误"
)
