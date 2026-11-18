package cpcpersonSvc_test

import (
	"encoding/json"
	"fmt"
	"net/url"
	"testing"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpctimeline/cpctimelineModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/student/studentConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/student/studentModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/spreadsheettest"
)

func assertTimelineStudentFields(
	t *testing.T, server *echo.Echo, tx *gorm.DB, actor fixtureActor,
	workspaceID string, person cpcpersonModels.Info, data cpcpersonModels.TimelineData,
) {
	t.Helper()
	base := "/v1/workspaces/" + workspaceID + "/cpc-people"
	path := fmt.Sprintf("%s/%d/timeline", base, person.ID)
	templatePath := fmt.Sprintf(
		"/v1/workspaces/%s/cpc-timeline-templates/%d", workspaceID, *data.TemplateID,
	)
	grantPersonActions(t, tx, actor, workspaceID, accessConstants.StudentSensitiveHandle)
	setBirth := func(identity string) {
		var student studentModels.Student
		if err := tx.Select("revision").Where("workspace_id = ? AND id = ?",
			workspaceID, person.StudentID).Take(&student).Error; err != nil {
			t.Fatal(err)
		}
		values := []studentModels.FieldValue{}
		for _, field := range studentConstants.Fields {
			if field.Group == studentConstants.Sensitive {
				value := ""
				if field.Key == "identity_number" {
					value = identity
				}
				values = append(values, studentModels.FieldValue{Field: field.Key, Value: value})
			}
		}
		personRequest[studentModels.MutationData](t, server, actor, "PUT", fmt.Sprintf(
			"/v1/workspaces/%s/students/%d/sensitive", workspaceID, person.StudentID,
		), studentModels.SensitiveRequest{Revision: student.Revision, Values: values}, 200)
	}
	setBirth("00000020080229001X")

	// 已填写节点切换为引用，清除旧值并同步所有绑定人员，引用可作为发生时间。
	reference := data.Nodes[0].Node
	reference.Fields[0].StudentField = "adulthood_date"
	reference.Fields = append(reference.Fields, cpctimelineModels.Field{
		ID: "birth", Name: "出生日期", Type: "date", StudentField: "birth_date",
	})
	dependent := cpctimelineModels.Node{
		ID: "node_student_dependent", Name: "成年日期核对", TimeFieldID: "date",
		Fields: []cpctimelineModels.Field{
			{ID: "date", Name: "核对日期", Type: "date", Required: true},
		},
		Rules: []cpctimelineModels.Rule{{
			ID: "rule_student", ReferenceNodeID: reference.ID,
			MinOffset: &cpctimelineModels.Offset{}, MaxOffset: &cpctimelineModels.Offset{},
		}},
	}
	update := cpctimelineModels.Request{
		Input: cpctimelineModels.Input{
			Name: data.TemplateName,
			Definition: cpctimelineModels.Definition{
				Nodes: []cpctimelineModels.Node{reference, dependent},
			},
		},
		Revision: data.TemplateRevision,
	}
	personRequest[cpctimelineModels.Info](t, server, actor, "PUT", templatePath, update, 200)
	data = personRequest[cpcpersonModels.TimelineData](t, server, actor, "GET", path, nil, 200)
	if data.Nodes[0].Values[reference.TimeFieldID] != "2026-02-28" ||
		data.Nodes[0].Values["birth"] != "2008-02-29" {
		t.Fatal("模板切换引用后仍使用历史填写值，或成年日期未按月末截断")
	}
	var stored cpcpersonModels.TimelineNode
	if err := tx.Where("workspace_id = ? AND person_id = ? AND node_id = ?",
		workspaceID, person.ID, reference.ID).Take(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.FieldValues != "{}" {
		t.Fatal("切换引用后没有清理已填写数据，或重复保存了引用结果")
	}
	personRequest[any](t, server, actor, "PUT", path+"/nodes/"+reference.ID,
		cpcpersonModels.TimelineNodeRequest{
			Values:   map[string]any{reference.TimeFieldID: "1990-01-01"},
			Revision: data.Nodes[0].Revision, TemplateRevision: data.TemplateRevision,
		}, 400)
	data = personRequest[cpcpersonModels.TimelineData](
		t, server, actor, "PUT", path+"/nodes/"+dependent.ID,
		cpcpersonModels.TimelineNodeRequest{
			Values:   map[string]any{"date": "2026-02-28"},
			Revision: data.Nodes[1].Revision, TemplateRevision: data.TemplateRevision,
		}, 200,
	)
	if data.Status != "normal" || data.Nodes[1].Checks[0].MinDate != "2026-02-28" {
		t.Fatal("依赖节点未使用引用日期校验")
	}

	// 学生日期变化后，详情、异常索引、字段筛选、统计和完整导出即时一致。
	setBirth("00000020080301001X")
	data = personRequest[cpcpersonModels.TimelineData](t, server, actor, "GET", path, nil, 200)
	if data.Nodes[0].Values[reference.TimeFieldID] != "2026-03-01" ||
		data.Status != "abnormal" || data.Nodes[1].Checks[0].MinDate != "2026-03-01" {
		t.Fatal("学生资料修改未更新引用及依赖规则")
	}
	condition, err := json.Marshal(cpcpersonModels.TimelineFilter{
		NodeID: data.Nodes[0].NumericID, FieldID: reference.TimeFieldID,
		Operator: "eq", Value: "2026-03-01",
	})
	if err != nil {
		t.Fatal(err)
	}
	query := "timeline_status=abnormal&timeline_filter=" + url.QueryEscape(string(condition))
	listed := personRequest[cpcpersonModels.ListData](
		t, server, actor, "GET", base+"?"+query, nil, 200,
	)
	stats := personRequest[cpcpersonModels.StatisticsData](
		t, server, actor, "GET", base+"/statistics?"+query, nil, 200,
	)
	full := personRequest[cpcpersonModels.FileData](
		t, server, actor, "GET", base+"/export?format=full&"+query, nil, 200,
	)
	rows, err := spreadsheettest.Rows(full.Content)
	if err != nil || listed.Total != 1 || listed.Items[0].ID != person.ID || stats.Total != 1 ||
		len(rows) != 2 || rows[1][len(rows[1])-3] != "2026-03-01" ||
		rows[1][len(rows[1])-2] != "2008-03-01" {
		t.Fatalf("引用日期及异常筛选未同步应用于名单、统计和导出：%v", err)
	}

	// 修改引用来源重算依赖节点，重置只清理手动数据，不清空学生引用。
	update.Revision = data.TemplateRevision
	update.Definition.Nodes[0].Fields[0].StudentField = "birth_date"
	personRequest[cpctimelineModels.Info](t, server, actor, "PUT", templatePath, update, 200)
	data = personRequest[cpcpersonModels.TimelineData](t, server, actor, "GET", path, nil, 200)
	if data.Nodes[1].Checks[0].MinDate != "2008-03-01" {
		t.Fatal("模板修改引用来源未重算已有依赖节点")
	}
	data = personRequest[cpcpersonModels.TimelineData](
		t, server, actor, "POST", path+"/nodes/"+reference.ID+"/reset",
		cpcpersonModels.ResetTimelineNodeRequest{
			Revision: data.Nodes[0].Revision, TemplateRevision: data.TemplateRevision,
		}, 200,
	)
	if data.Nodes[0].Values[reference.TimeFieldID] != "2008-03-01" {
		t.Fatal("重置引用节点不应清除当前学生日期")
	}
	for _, identity := range []string{"无法解析", ""} {
		setBirth(identity)
		data = personRequest[cpcpersonModels.TimelineData](t, server, actor, "GET", path, nil, 200)
		if len(data.Nodes[0].Values) != 0 || len(data.Nodes[1].Checks) != 0 ||
			data.Status != "normal" || data.Nodes[1].Values["date"] != "2026-02-28" {
			t.Fatal("学生日期无效或缺失时应为空、清除依赖异常并保留其他手动值")
		}
		listed = personRequest[cpcpersonModels.ListData](
			t, server, actor, "GET", base+"?timeline_status=abnormal", nil, 200,
		)
		if listed.Total != 0 {
			t.Fatal("清空学生日期后异常索引未同步更新")
		}
	}
	personRequest[cpcpersonModels.TimelineData](
		t, server, actor, "PUT", path+"/nodes/"+reference.ID,
		cpcpersonModels.TimelineNodeRequest{
			Values: map[string]any{}, Revision: data.Nodes[0].Revision,
			TemplateRevision: data.TemplateRevision,
		}, 200,
	)
}
