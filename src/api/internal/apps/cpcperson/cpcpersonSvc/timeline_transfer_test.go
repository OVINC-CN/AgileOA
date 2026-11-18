package cpcpersonSvc

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpctimeline/cpctimelineModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

func transferPreviewFixture() (
	cpcpersonModels.Info, cpcpersonModels.TimelineData, []cpctimelineModels.Node,
	cpctimelineModels.Transfer, []attachmentModels.PersonTimelineAttachment,
) {
	sourceID := uint64(1)
	person := cpcpersonModels.Info{ID: 3, Revision: 4, CanUpdate: true}
	source := cpcpersonModels.TimelineData{
		PersonID: person.ID, PersonRevision: person.Revision, TemplateID: &sourceID,
		TemplateName: "来源模板", TemplateRevision: 7,
		Nodes: []cpcpersonModels.TimelineNodeInfo{
			{
				Node: cpctimelineModels.Node{
					ID:   "node_a",
					Name: "来源甲",
					Fields: []cpctimelineModels.Field{
						{ID: "date", Name: "日期", Type: "date"},
						{ID: "count", Name: "数量", Type: "number"},
						{ID: "flag", Name: "确认", Type: "boolean"},
						{ID: "note", Name: "备注", Type: "text"},
					},
				},
				Values: map[string]any{
					"date":  "2026-11-06",
					"count": float64(0),
					"flag":  false,
					"note":  "原备注",
				},
				Status:   "completed",
				Revision: 10,
			},
			{
				Node: cpctimelineModels.Node{
					ID:   "node_b",
					Name: "来源乙",
					Fields: []cpctimelineModels.Field{
						{
							ID:   "date",
							Name: "日期",
							Type: "date",
						},
						{ID: "extra", Name: "补充", Type: "text"},
					},
				},
				Values:   map[string]any{"date": "2026-11-07", "extra": "合并内容"},
				Status:   "completed",
				Revision: 2,
			},
		},
	}
	nodes := []cpctimelineModels.Node{
		{ID: "node_x", Name: "合并节点", TimeFieldID: "date", Fields: []cpctimelineModels.Field{
			{
				ID:       "date",
				Type:     "date",
				Required: true,
			},
			{ID: "count", Type: "number", Required: true},
			{
				ID:       "flag",
				Type:     "boolean",
				Required: true,
			},
			{ID: "extra", Type: "text", Required: true},
		}},
		{ID: "node_y", Name: "分发节点", TimeFieldID: "date", Fields: []cpctimelineModels.Field{
			{ID: "date", Type: "date", Required: true},
		}},
		{ID: "node_z", Name: "学生引用", TimeFieldID: "birth", Fields: []cpctimelineModels.Field{
			{ID: "birth", Type: "date", Required: true, StudentField: "birth_date"},
		}},
	}
	mapping := cpctimelineModels.Transfer{
		TargetTemplateID: 2,
		NodeMappings: []cpctimelineModels.TransferNodeMapping{
			{
				SourceNodeID: "node_a",
				TargetNodeID: "node_x",
				Fields: []cpctimelineModels.TransferFieldMapping{
					{
						SourceFieldID: "date",
						TargetFieldID: "date",
					},
					{SourceFieldID: "count", TargetFieldID: "count"},
					{SourceFieldID: "flag", TargetFieldID: "flag"},
				},
			},
			{
				SourceNodeID: "node_a",
				TargetNodeID: "node_y",
				Fields: []cpctimelineModels.TransferFieldMapping{
					{SourceFieldID: "date", TargetFieldID: "date"},
				},
			},
			{
				SourceNodeID: "node_b",
				TargetNodeID: "node_x",
				Fields: []cpctimelineModels.TransferFieldMapping{
					{SourceFieldID: "extra", TargetFieldID: "extra"},
				},
			},
		},
	}
	files := []attachmentModels.PersonTimelineAttachment{
		{
			Record: attachmentModels.Record{
				ID:         1,
				ObjectHash: "shared",
				ObjectKey:  "private/shared",
				State:      "ready",
				Revision:   2,
			},
			PersonID: person.ID,
			NodeID:   "node_a",
		},
		{
			Record: attachmentModels.Record{
				ID:         2,
				ObjectHash: "shared",
				ObjectKey:  "private/shared",
				State:      "ready",
				Revision:   2,
			},
			PersonID: person.ID,
			NodeID:   "node_b",
		},
		{
			Record: attachmentModels.Record{
				ID:         3,
				ObjectHash: "other",
				ObjectKey:  "private/other",
				State:      "ready",
				Revision:   1,
			},
			PersonID: person.ID,
			NodeID:   "node_b",
		},
	}
	return person, source, nodes, mapping, files
}

func TestTimelineTransferDistributesAndMergesValuesAndFileReferences(t *testing.T) {
	person, source, nodes, mapping, files := transferPreviewFixture()
	plan, err := buildTimelineTransfer(
		person,
		source,
		cpctimelineModels.Template{
			ID:       2,
			Name:     "目标模板",
			Revision: 8,
		},
		nodes,
		mapping,
		files,
		"2000-02-29",
	)
	if err != nil {
		t.Fatal(err)
	}
	first, second, third := plan.Preview.Timeline.Nodes[0], plan.Preview.Timeline.Nodes[1], plan.Preview.Timeline.Nodes[2]
	if first.Values["count"] != float64(0) || first.Values["flag"] != false ||
		first.Values["extra"] != "合并内容" || second.Values["date"] != "2026-11-06" {
		t.Fatal("多对多字段映射丢失了零、否、合并值或分发值")
	}
	if first.Status != "completed" || second.Status != "completed" || third.Status != "pending" ||
		third.Values["birth"] != "2000-02-29" || first.Revision <= source.Nodes[0].Revision {
		t.Fatal("处理状态、实时引用或重建版本不符合预期")
	}
	if len(first.Attachments) != 2 || len(second.Attachments) != 1 || len(plan.Files) != 3 ||
		plan.Files[0].Source.ObjectKey != "private/shared" || len(plan.Preview.DiscardedFields) != 2 {
		t.Fatal("附件未复用对象键、合并去重或一对多分发，或未展示丢弃值")
	}
	if files[0].NodeID != "node_a" || files[0].Revision != 2 {
		t.Fatal("预览修改了来源附件")
	}
}

func TestTimelineTransferRequiresCompleteSourcesAndTargetValues(t *testing.T) {
	for _, item := range []struct{ left, right, expected string }{
		{"completed", "completed", "completed"},
		{"skipped", "skipped", "skipped"},
		{"skipped", "completed", "pending"},
		{"completed", "pending", "pending"},
	} {
		t.Run(item.left+"_"+item.right, func(t *testing.T) {
			person, source, nodes, mapping, _ := transferPreviewFixture()
			source.Nodes[0].Status, source.Nodes[1].Status = item.left, item.right
			plan, err := buildTimelineTransfer(person, source,
				cpctimelineModels.Template{ID: 2}, nodes, mapping, nil, "")
			if err != nil || plan.Preview.Timeline.Nodes[0].Status != item.expected {
				t.Fatalf("合并状态错误：%v", err)
			}
		})
	}
	person, source, nodes, mapping, _ := transferPreviewFixture()
	delete(source.Nodes[1].Values, "extra")
	plan, err := buildTimelineTransfer(person, source,
		cpctimelineModels.Template{ID: 2}, nodes, mapping, nil, "")
	if err != nil || plan.Preview.Timeline.Nodes[0].Status != "pending" {
		t.Fatal("目标必填字段缺失时未退回待处理")
	}
}

func TestTimelineTransferEvaluatesTargetRulesAndProtectsFiles(t *testing.T) {
	person, source, nodes, mapping, files := transferPreviewFixture()
	nodes[1].Rules = []cpctimelineModels.Rule{{
		ID: "rule_after", ReferenceNodeID: "node_x", MinOffset: &cpctimelineModels.Offset{Days: 1},
	}}
	plan, err := buildTimelineTransfer(person, source,
		cpctimelineModels.Template{ID: 2}, nodes, mapping, files, "")
	if err != nil || plan.Preview.Timeline.Status != "abnormal" ||
		len(plan.Preview.Timeline.Nodes[1].Issues) != 1 {
		t.Fatal("预览未按目标规则计算时间异常")
	}

	// 目标时间异常仅供预览，发起和审核仍须使用匹配的预览版本。
	plan.Preview.PreviewVersion = strings.Repeat("a", 64)
	request := cpcpersonModels.TimelineTransferRequest{
		TargetTemplateID: 2,
		PreviewVersion:   plan.Preview.PreviewVersion,
	}
	if err := validateTransferPreview(plan, request); err != nil {
		t.Fatalf("目标时间异常阻止了有效预览确认：%v", err)
	}
	for _, version := range []string{"", strings.Repeat("b", 64)} {
		request.PreviewVersion = version
		var failure *response.BusinessError
		if err := validateTransferPreview(plan, request); !errors.As(err, &failure) ||
			failure.Status != http.StatusConflict {
			t.Fatalf("目标时间异常时未拒绝缺失或过期的预览版本：%v", err)
		}
	}

	// 附件仍须全部可用，且映射后的数量不能超限。
	for _, state := range []string{"pending", "deleting"} {
		files[0].State = state
		if _, err := buildTimelineTransfer(person, source,
			cpctimelineModels.Template{ID: 2}, nodes, mapping, files, ""); err == nil {
			t.Fatal("未完成或待删除附件未阻止转移")
		}
	}
	files[0].State = "ready"
	mapping.NodeMappings = mapping.NodeMappings[:2]
	if _, err := buildTimelineTransfer(person, source,
		cpctimelineModels.Template{ID: 2}, nodes, mapping, files, ""); err == nil {
		t.Fatal("带附件的未映射来源节点未阻止转移")
	}
	_, _, _, mapping, _ = transferPreviewFixture()
	before := config.Config.CPCTimeline.MaxAttachments
	config.Config.CPCTimeline.MaxAttachments = 1
	t.Cleanup(func() { config.Config.CPCTimeline.MaxAttachments = before })
	if _, err := buildTimelineTransfer(person, source,
		cpctimelineModels.Template{ID: 2}, nodes, mapping, files, ""); err == nil {
		t.Fatal("合并附件超过每节点上限时未阻止转移")
	}
}
