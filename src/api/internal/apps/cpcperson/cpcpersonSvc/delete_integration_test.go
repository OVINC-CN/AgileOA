package cpcpersonSvc_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpctimeline/cpctimelineModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/student/studentModels"
)

func assertOwnedDetailCascades(
	t *testing.T, server *echo.Echo, tx *gorm.DB, actor fixtureActor,
	workspaceID string, studentID, branchID uint64,
) {
	t.Helper()
	grantPersonActions(t, tx, actor, workspaceID, accessConstants.CPCTimelineDelete)
	assertCount := func(model any, condition string, want int64, args ...any) {
		t.Helper()
		var count int64
		if err := tx.Model(model).Where("workspace_id = ?", workspaceID).
			Where(condition, args...).Count(&count).Error; err != nil || count != want {
			t.Fatalf("级联删除或受保护引用的数据数量异常：期望 %d，实际 %d，错误 %v", want, count, err)
		}
	}
	makeTemplate := func(name string) cpctimelineModels.Template {
		t.Helper()
		record := cpctimelineModels.Template{
			WorkspaceID: workspaceID, Name: name, Revision: 1,
		}
		if err := tx.Create(&record).Error; err != nil {
			t.Fatal(err)
		}
		node := cpctimelineModels.TemplateNode{
			WorkspaceID: workspaceID, TimelineTemplateID: record.ID, NodeID: "node_owned",
			Name: "虚构自有节点", Position: 1, TimeFieldID: "occurred_at",
			FieldDefinitions: `[{"id":"occurred_at","name":"发生时间","type":"date","required":true}]`,
			ValidationRules:  "[]",
		}
		if err := tx.Create(&node).Error; err != nil {
			t.Fatal(err)
		}
		return record
	}
	source, target := makeTemplate("虚构级联来源"), makeTemplate("虚构级联目标")
	transfer := cpctimelineModels.TemplateTransfer{
		WorkspaceID: workspaceID, SourceTemplateID: source.ID, TargetTemplateID: target.ID,
		NodeMappings: `[{"source_node_id":"node_owned","target_node_id":"node_owned","fields":[]}]`,
		Position:     1,
	}
	if err := tx.Create(&transfer).Error; err != nil {
		t.Fatal(err)
	}
	templatePath := "/v1/workspaces/" + workspaceID + "/cpc-timeline-templates/"
	deleteTemplate := func(record cpctimelineModels.Template, status int) {
		t.Helper()
		personRequest[any](t, server, actor, "DELETE",
			fmt.Sprintf("%s%d?revision=1", templatePath, record.ID), nil, status)
	}

	// 目标模板继续受其他来源的转移配置保护，节点不会因失败的删除而丢失。
	deleteTemplate(target, 409)
	assertCount(&cpctimelineModels.TemplateNode{}, "timeline_template_id = ?", 1, target.ID)
	status := "normal"
	person := cpcpersonModels.Person{
		WorkspaceID: workspaceID, StudentID: studentID, PartyBranchID: &branchID,
		Stage: "applicant", Status: "在编", TimelineTemplateID: &source.ID,
		TimelineStatus: &status, Revision: 1,
	}
	if err := tx.Create(&person).Error; err != nil {
		t.Fatal(err)
	}
	node := cpcpersonModels.TimelineNode{
		WorkspaceID: workspaceID, PersonID: person.ID, TimelineTemplateID: source.ID,
		NodeID: "node_owned", FieldValues: `{"occurred_at":"2026-11-01"}`,
		Status: "completed", Revision: 1,
	}
	if err := tx.Create(&node).Error; err != nil {
		t.Fatal(err)
	}
	personPath := fmt.Sprintf("/v1/workspaces/%s/cpc-people/%d?revision=1", workspaceID, person.ID)
	file := attachmentModels.PersonTimelineAttachment{
		PersonID: person.ID, NodeID: node.NodeID,
		Record: attachmentModels.Record{
			WorkspaceID: workspaceID, ObjectHash: strings.Repeat("f", 64),
			ObjectKey: "fictional/cascade-person-file", Filename: "虚构保护附件.txt",
			State: "pending", Revision: 1,
		},
	}
	if err := tx.Create(&file).Error; err != nil {
		t.Fatal(err)
	}

	// 人员节点可随人员删除，节点附件仍阻止整个删除，逐节点成功审计也必须回滚。
	personRequest[any](t, server, actor, "DELETE", personPath, nil, 409)
	assertCount(&cpcpersonModels.TimelineNode{}, "person_id = ?", 1, person.ID)
	if err := tx.Where("workspace_id = ? AND id = ?", workspaceID, file.ID).
		Delete(&attachmentModels.PersonTimelineAttachment{}).Error; err != nil {
		t.Fatal(err)
	}
	personRequest[any](t, server, actor, "DELETE", personPath, nil, 200)
	assertCount(&cpcpersonModels.TimelineNode{}, "person_id = ?", 0, person.ID)
	assertCount(&studentModels.Student{}, "id = ?", 1, studentID)
	assertCount(&cpctimelineModels.Template{}, "id = ?", 1, source.ID)
	var audits int64
	if err := tx.Model(&auditModels.Record{}).Where(
		"workspace_id = ? AND instance_id = ? AND resource_type_id = ? AND action_id = ? AND result_code = ?",
		workspaceID, fmt.Sprint(person.ID), "cpc_person_timeline_node",
		accessConstants.CPCPersonDelete, 0,
	).
		Count(&audits).Error; err != nil || audits != 1 {
		t.Fatal("人员级联删除没有保留独立的节点审计，或失败事务留下成功审计")
	}

	// 模板的后代附件同样阻止级联，失败时来源转移配置必须保留。
	templateFile := attachmentModels.TimelineNodeAttachment{
		TimelineTemplateID: source.ID, NodeID: node.NodeID, Record: file.Record,
	}
	templateFile.ID = 0
	if err := tx.Create(&templateFile).Error; err != nil {
		t.Fatal(err)
	}
	deleteTemplate(source, 409)
	assertCount(&cpctimelineModels.TemplateNode{}, "timeline_template_id = ?", 1, source.ID)
	assertCount(&cpctimelineModels.TemplateTransfer{}, "source_template_id = ?", 1, source.ID)
	if err := tx.Where("workspace_id = ? AND id = ?", workspaceID, templateFile.ID).
		Delete(&attachmentModels.TimelineNodeAttachment{}).Error; err != nil {
		t.Fatal(err)
	}
	deleteTemplate(source, 200)
	assertCount(&cpctimelineModels.TemplateNode{}, "timeline_template_id = ?", 0, source.ID)
	assertCount(&cpctimelineModels.TemplateTransfer{}, "source_template_id = ?", 0, source.ID)
	assertCount(&cpctimelineModels.TemplateNode{}, "timeline_template_id = ?", 1, target.ID)
	deleteTemplate(target, 200)
	assertCount(&cpctimelineModels.TemplateNode{}, "timeline_template_id = ?", 0, target.ID)
}
