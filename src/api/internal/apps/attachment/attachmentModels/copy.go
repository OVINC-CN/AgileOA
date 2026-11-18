package attachmentModels

// TimelineNodeCopy 保存一次节点附件复制的来源及独立目标，不向客户端返回对象键。
type TimelineNodeCopy struct {
	Source TimelineNodeAttachment
	Target TimelineNodeAttachment
}
