package emailConstants

// Event 描述腾讯云固定事件代码及中文名称，编号沿用回调协议。
type Event struct {
	Value string
	Label string
	ID    int
}

// Events 是可配置异常事件的固定目录，不允许任意外部事件进入黑名单规则。
var Events = map[string]Event{
	"delivered":   {Value: "delivered", Label: "递送成功", ID: 1},
	"dropped":     {Value: "dropped", Label: "邮件丢弃", ID: 2},
	"bounce":      {Value: "bounce", Label: "收件方退信", ID: 3},
	"open":        {Value: "open", Label: "打开邮件", ID: 4},
	"click":       {Value: "click", Label: "点击链接", ID: 5},
	"spamreport":  {Value: "spamreport", Label: "举报邮件", ID: 6},
	"unsubscribe": {Value: "unsubscribe", Label: "退订邮件", ID: 7},
	"deferred":    {Value: "deferred", Label: "延迟递送", ID: 8},
}
