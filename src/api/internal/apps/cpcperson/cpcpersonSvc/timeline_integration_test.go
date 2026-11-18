package cpcpersonSvc_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/OVINC-CN/AgileOA/src/api/internal/spreadsheettest"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apis/tencentcos"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpctimeline/cpctimelineModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/storage"
)

func assertPersonTimelines(
	t *testing.T, server *echo.Echo, tx *gorm.DB, first, manager fixtureActor,
	workspaceID, otherWorkspace string, personX, personY cpcpersonModels.Info,
	template cpctimelineModels.Template,
) {
	t.Helper()
	base := "/v1/workspaces/" + workspaceID + "/cpc-people"
	path := fmt.Sprintf("%s/%d/timeline", base, personY.ID)
	personRequest[any](
		t,
		server,
		first,
		"GET",
		fmt.Sprintf("%s/%d/timeline", base, personX.ID),
		nil,
		404,
	)
	personRequest[any](
		t,
		server,
		manager,
		"GET",
		fmt.Sprintf(
			"/v1/workspaces/%s/cpc-people/%d/timeline",
			otherWorkspace,
			personY.ID,
		),
		nil,
		404,
	)
	options := personRequest[[]cpcpersonModels.Option](
		t, server, first, "GET", base+"/timeline-templates/update", nil, 200,
	)
	if len(options) != 1 {
		t.Fatal("未复用人员权限提供模板选项")
	}
	personRequest[any](t, server, manager, "POST", base, map[string]any{
		"student_id": personX.StudentID, "stage": personX.Stage, "party_position": "",
	}, 400)
	personRequest[any](
		t,
		server,
		manager,
		"PUT",
		fmt.Sprintf("%s/%d", base, personX.ID),
		map[string]any{
			"student_id":           personX.StudentID,
			"stage":                personX.Stage,
			"party_position":       personX.PartyPosition,
			"revision":             personX.Revision,
			"timeline_template_id": template.ID,
		},
		400,
	)
	data := personRequest[cpcpersonModels.TimelineData](t, server, first, "GET", path, nil, 200)
	if len(data.Nodes) != 1 || data.Nodes[0].Status != "pending" || len(data.Nodes[0].Values) != 0 {
		t.Fatal("新建人员未获得独立的待处理节点")
	}
	if data.Nodes[0].IsBaseline {
		t.Fatal("未配置的节点不应默认为基准节点")
	}
	request := cpcpersonModels.TimelineNodeRequest{
		Values: map[string]any{}, Revision: 1, TemplateRevision: 1,
	}
	personRequest[any](t, server, first, "PUT", path+"/nodes/node_first", request, 400)
	request.Values["occurred_at"] = "2026-11-06"
	data = personRequest[cpcpersonModels.TimelineData](
		t,
		server,
		first,
		"PUT",
		path+"/nodes/node_first",
		request,
		200,
	)
	if data.Nodes[0].Status != "completed" {
		t.Fatal("填写必填日期后未标记完成")
	}
	personRequest[any](t, server, first, "PUT", path+"/nodes/node_first", request, 409)

	// 重置沿用编辑范围，拒绝过期版本及撤销后的编辑权限。
	resetPath := path + "/nodes/node_first/reset"
	reset := cpcpersonModels.ResetTimelineNodeRequest{
		Revision: data.Nodes[0].Revision, TemplateRevision: data.TemplateRevision,
	}
	personRequest[any](t, server, first, "POST",
		fmt.Sprintf("%s/%d/timeline/nodes/node_first/reset", base, personX.ID), reset, 404)
	personRequest[any](t, server, manager, "POST", fmt.Sprintf(
		"/v1/workspaces/%s/cpc-people/%d/timeline/nodes/node_first/reset",
		otherWorkspace, personY.ID,
	), reset, 404)
	staleReset := reset
	staleReset.Revision--
	personRequest[any](t, server, first, "POST", resetPath, staleReset, 409)
	staleReset = reset
	staleReset.TemplateRevision++
	personRequest[any](t, server, first, "POST", resetPath, staleReset, 409)
	personRequest[any](t, server, first, "POST", path+"/nodes/node_missing/reset", reset, 400)
	if err := tx.Where("workspace_id = ? AND user_id = ? AND action = ?",
		workspaceID, first.user.ID, accessConstants.CPCPersonUpdate).
		Delete(&accessModels.Permission{}).Error; err != nil {
		t.Fatal(err)
	}
	personRequest[any](t, server, first, "POST", resetPath, reset, 403)
	grantPersonActions(t, tx, first, workspaceID, accessConstants.CPCPersonUpdate)
	data = personRequest[cpcpersonModels.TimelineData](
		t, server, first, "POST", resetPath, reset, 200,
	)
	if data.Nodes[0].Status != "pending" || data.Nodes[0].Revision != reset.Revision+1 ||
		len(data.Nodes[0].Values) != 0 || len(data.Nodes[0].Attachments) != 0 {
		t.Fatal("重置未清空填写值和人员附件、恢复待处理或更新版本")
	}
	personRequest[any](t, server, first, "POST", resetPath, reset, 409)
	reset.Revision = data.Nodes[0].Revision
	data = personRequest[cpcpersonModels.TimelineData](
		t, server, first, "POST", resetPath, reset, 200,
	)
	if data.Nodes[0].Revision != reset.Revision {
		t.Fatal("重复重置已清空的待处理节点不应修改版本")
	}

	// 模板变更后退回待处理的旧填写值也必须清空，包含当前目录不再展示的字段。
	if err := tx.Model(&cpcpersonModels.TimelineNode{}).
		Where("workspace_id = ? AND person_id = ? AND node_id = ?",
			workspaceID, personY.ID, "node_first").
		Update("field_values", `{"occurred_at":"2026-11-06","legacy_count":0,"legacy_flag":false}`).
		Error; err != nil {
		t.Fatal(err)
	}
	data = personRequest[cpcpersonModels.TimelineData](
		t, server, first, "POST", resetPath, reset, 200,
	)
	var cleared cpcpersonModels.TimelineNode
	if err := tx.Where("workspace_id = ? AND person_id = ? AND node_id = ?",
		workspaceID, personY.ID, "node_first").Take(&cleared).Error; err != nil {
		t.Fatal(err)
	}
	if len(data.Nodes[0].Values) != 0 || cleared.FieldValues != "{}" ||
		data.Nodes[0].Revision != reset.Revision+1 {
		t.Fatal("待处理节点的旧填写值未完整清空或未更新版本")
	}
	data = assertTimelineResetAttachments(
		t, server, tx, first, workspaceID, personX.ID, data,
	)
	request.Revision = data.Nodes[0].Revision
	personRequest[cpcpersonModels.TimelineData](
		t, server, first, "PUT", path+"/nodes/node_first", request, 200,
	)

	// 模板节点增删影响所有绑定者，时间范围只标记异常而不阻止保存。
	grantPersonActions(t, tx, manager, workspaceID, accessConstants.CPCTimelineUpdate)
	firstNode := cpctimelineModels.Node{
		ID:          "node_first",
		Name:        "虚构节点",
		GroupName:   "申请阶段",
		IsBaseline:  true,
		TimeFieldID: "occurred_at",
		Fields: []cpctimelineModels.Field{
			{ID: "occurred_at", Name: "发生时间", Type: "date", Required: true},
		},
		Rules: []cpctimelineModels.Rule{},
	}
	nextNode := firstNode
	nextNode.ID, nextNode.Name = "node_next", "后续节点"
	nextNode.IsBaseline = false
	nextNode.Rules = []cpctimelineModels.Rule{
		{
			ID:              "rule_range",
			ReferenceNodeID: firstNode.ID,
			MinOffset: &cpctimelineModels.Offset{
				Days: 1,
			},
			MaxOffset: &cpctimelineModels.Offset{Days: 3},
		},
	}
	templatePath := fmt.Sprintf(
		"/v1/workspaces/%s/cpc-timeline-templates/%d",
		workspaceID,
		template.ID,
	)
	update := cpctimelineModels.Request{
		Input: cpctimelineModels.Input{
			Name: template.Name,
			Definition: cpctimelineModels.Definition{
				Nodes: []cpctimelineModels.Node{firstNode, nextNode},
			},
		},
		Revision: 1,
	}
	personRequest[cpctimelineModels.Info](t, server, manager, "PUT", templatePath, update, 200)
	data = personRequest[cpcpersonModels.TimelineData](t, server, first, "GET", path, nil, 200)
	if len(data.Nodes) != 2 || data.Nodes[1].Status != "pending" {
		t.Fatal("模板新增节点未同步到已绑定人员")
	}
	if !data.Nodes[0].IsBaseline || data.Nodes[1].IsBaseline {
		t.Fatal("人员时间线未按当前模板返回各节点的基准标记")
	}
	if data.Nodes[0].GroupName != "申请阶段" || data.Nodes[1].GroupName != "申请阶段" {
		t.Fatal("人员时间线未按当前模板返回各节点的分组")
	}
	request.Values["occurred_at"], request.Revision, request.TemplateRevision = "2026-11-12", 1, 2
	data = personRequest[cpcpersonModels.TimelineData](
		t,
		server,
		first,
		"PUT",
		path+"/nodes/node_next",
		request,
		200,
	)
	if data.Status != "abnormal" || len(data.Nodes[1].Issues) != 1 {
		t.Fatal("保存异常时间未生成状态及详细提示")
	}
	data = assertTimelineNodeSkipping(
		t, server, first, manager, workspaceID, otherWorkspace, personX.ID, data,
	)
	data = personRequest[cpcpersonModels.TimelineData](t, server, first, "POST",
		path+"/nodes/node_next/reset", cpcpersonModels.ResetTimelineNodeRequest{
			Revision: data.Nodes[1].Revision, TemplateRevision: data.TemplateRevision,
		}, 200)
	if data.Nodes[1].Status != "pending" || data.Status != "normal" ||
		len(data.Nodes[1].Values) != 0 || len(data.Nodes[1].Issues) != 0 ||
		len(data.Nodes[1].Checks) != 1 || data.Nodes[1].Checks[0].Result != "pending" {
		t.Fatal("重置未清空发生时间、恢复待处理或重算异常及规则状态")
	}
	progress := personRequest[cpcpersonModels.Info](
		t, server, first, "GET", fmt.Sprintf("%s/%d", base, personY.ID), nil, 200,
	)
	if progress.TimelinePending != 1 || progress.TimelineCompleted != 1 {
		t.Fatal("重置后人员完成进度未更新")
	}
	filtered := personRequest[cpcpersonModels.ListData](
		t,
		server,
		first,
		"GET",
		base+"?timeline_status=abnormal",
		nil,
		200,
	)
	if filtered.Total != 0 {
		t.Fatal("重置后异常筛选仍命中已清空日期的人员")
	}

	// 重置参考日期同样清除依赖节点的时间异常，保留其他节点的填写值。
	request.Revision = data.Nodes[1].Revision
	data = personRequest[cpcpersonModels.TimelineData](
		t, server, first, "PUT", path+"/nodes/node_next", request, 200,
	)
	data = personRequest[cpcpersonModels.TimelineData](t, server, first, "POST",
		resetPath, cpcpersonModels.ResetTimelineNodeRequest{
			Revision: data.Nodes[0].Revision, TemplateRevision: data.TemplateRevision,
		}, 200)
	if data.Status != "normal" || len(data.Nodes[0].Values) != 0 ||
		len(data.Nodes[1].Issues) != 0 || len(data.Nodes[1].Checks) != 0 ||
		data.Nodes[1].Values["occurred_at"] != "2026-11-12" ||
		data.Nodes[1].Status != "completed" {
		t.Fatal("重置参考节点未清除依赖异常，或错误修改了其他节点")
	}
	filtered = personRequest[cpcpersonModels.ListData](
		t, server, first, "GET", base+"?timeline_status=abnormal", nil, 200,
	)
	if filtered.Total != 0 {
		t.Fatal("清空参考日期后名单仍使用旧异常状态")
	}
	request.Values["occurred_at"], request.Revision = "2026-11-06", data.Nodes[0].Revision
	personRequest[cpcpersonModels.TimelineData](
		t, server, first, "PUT", path+"/nodes/node_first", request, 200,
	)
	other := personRequest[cpcpersonModels.TimelineData](t, server, manager, "GET",
		fmt.Sprintf("%s/%d/timeline", base, personX.ID), nil, 200)
	if other.Nodes[0].Status != "pending" || other.Status != "normal" {
		t.Fatal("同模板的不同人员共享了填写值或异常状态")
	}
	firstNode.IsBaseline = false
	update.Revision, update.Definition.Nodes = 2, []cpctimelineModels.Node{firstNode}
	personRequest[any](t, server, manager, "PUT", templatePath, update, 400)
	update.ConfirmNodeDeletion = true
	personRequest[cpctimelineModels.Info](t, server, manager, "PUT", templatePath, update, 200)
	data = personRequest[cpcpersonModels.TimelineData](t, server, first, "GET", path, nil, 200)
	if len(data.Nodes) != 1 || data.Status != "normal" || data.Nodes[0].Status != "completed" {
		t.Fatal("删除模板节点未同步清理人员节点或刷新异常状态，或错误删除了保留节点的值")
	}
	if data.Nodes[0].IsBaseline || data.Nodes[0].Values["occurred_at"] != "2026-11-06" {
		t.Fatal("取消基准标记未同步到人员时间线，或错误修改了节点填写值")
	}
	// 两种导出沿用相同人员范围，完整数据平铺时间线，基础数据保持原目录。
	full := personRequest[cpcpersonModels.FileData](
		t,
		server,
		first,
		"GET",
		base+"/export?format=full",
		nil,
		200,
	)
	rows, err := spreadsheettest.Rows(full.Content)
	expectedHeader := "1-发生时间"
	if err != nil || len(rows) != 2 || rows[0][len(rows[0])-1] != expectedHeader ||
		rows[1][len(rows[1])-1] != "2026-11-06" ||
		full.Filename != "cpc-people-full.xlsx" {
		t.Fatalf("完整导出未按节点标识和字段名平铺，或未保留人员范围：%v", err)
	}
	basic := personRequest[cpcpersonModels.FileData](
		t,
		server,
		first,
		"GET",
		base+"/export?format=basic",
		nil,
		200,
	)
	if strings.Contains(spreadsheettest.Text(t, basic.Content), expectedHeader) {
		t.Fatal("基础导出不应包含时间线字段")
	}
	assertTimelineFieldFilters(
		t,
		server,
		first,
		manager,
		base,
		otherWorkspace,
		data.Nodes[0].NumericID,
	)
	assertPersonTimelineView(
		t, server, tx, first, manager, workspaceID, otherWorkspace, personY, data,
	)
	personRequest[any](t, server, first, "GET", base+"/export?format=unknown", nil, 400)
	assertTimelineStudentFields(t, server, tx, manager, workspaceID, personY, data)
}

func assertTimelineResetAttachments(
	t *testing.T, server *echo.Echo, tx *gorm.DB, actor fixtureActor,
	workspaceID string, otherPersonID uint64, data cpcpersonModels.TimelineData,
) cpcpersonModels.TimelineData {
	t.Helper()
	// 同时覆盖待上传、可下载和待删除附件，其他人员及模板附件不属于重置范围。
	makeRecord := func(owner, state string) attachmentModels.Record {
		key := fmt.Sprintf("workspaces/%s/reset-fixture/%s/%s/虚构附件.txt", workspaceID, owner, state)
		digest := sha256.Sum256([]byte(key))
		return attachmentModels.Record{
			WorkspaceID: workspaceID, ObjectHash: hex.EncodeToString(digest[:]),
			ObjectKey: key, Filename: "虚构附件.txt", Size: 7, State: state, Revision: 1,
		}
	}
	files := make([]attachmentModels.PersonTimelineAttachment, 0, 4)
	for _, state := range []string{"pending", "ready", "deleting"} {
		files = append(files, attachmentModels.PersonTimelineAttachment{
			Record: makeRecord("target", state), PersonID: data.PersonID, NodeID: "node_first",
		})
	}
	files = append(files, attachmentModels.PersonTimelineAttachment{
		Record: makeRecord("other", "ready"), PersonID: otherPersonID, NodeID: "node_first",
	})
	if err := tx.Create(&files).Error; err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := tx.Where("workspace_id = ? AND id IN ?",
			workspaceID, []uint64{files[0].ID, files[1].ID, files[2].ID, files[3].ID}).
			Delete(&attachmentModels.PersonTimelineAttachment{}).Error; err != nil {
			t.Error(err)
		}
	}()
	templateFile := attachmentModels.TimelineNodeAttachment{
		Record: makeRecord("template", "ready"), TimelineTemplateID: *data.TemplateID,
		NodeID: "node_first",
	}
	if err := tx.Create(&templateFile).Error; err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := tx.Where("workspace_id = ? AND id = ?", workspaceID, templateFile.ID).
			Delete(&attachmentModels.TimelineNodeAttachment{}).Error; err != nil {
			t.Error(err)
		}
	}()
	if storage.Enabled() {
		for _, file := range files[:3] {
			if _, err := tencentcos.Client.Object.Put(
				t.Context(), file.ObjectKey, strings.NewReader("fixture"), nil,
			); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := storage.Delete(t.Context(), file.ObjectKey); err != nil {
					t.Error(err)
				}
			}()
		}
	}

	// 未配置存储时必须阻断重置，不能只清空字段或删除元数据。
	path := fmt.Sprintf("/v1/workspaces/%s/cpc-people/%d/timeline", workspaceID, data.PersonID)
	request := cpcpersonModels.ResetTimelineNodeRequest{
		Revision: data.Nodes[0].Revision, TemplateRevision: data.TemplateRevision,
	}
	status := 200
	if !storage.Enabled() {
		status = 503
	}
	personRequest[any](t, server, actor, "POST", path+"/nodes/node_first/reset", request, status)
	after := personRequest[cpcpersonModels.TimelineData](t, server, actor, "GET", path, nil, 200)
	if !storage.Enabled() {
		if after.Nodes[0].Revision != request.Revision || len(after.Nodes[0].Attachments) != 3 {
			t.Fatal("附件服务不可用时出现了部分节点重置")
		}
		return after
	}
	if after.Nodes[0].Revision != request.Revision+1 || len(after.Nodes[0].Attachments) != 0 ||
		len(after.Nodes[0].Values) != 0 || after.Nodes[0].Status != "pending" ||
		len(after.Nodes[0].TemplateAttachments) != 1 {
		t.Fatal("待处理节点重置未清空所有状态的人员附件，或错误清除了模板附件")
	}
	var otherFiles int64
	if err := tx.Model(&attachmentModels.PersonTimelineAttachment{}).
		Where("workspace_id = ? AND person_id = ? AND id = ?", workspaceID, otherPersonID, files[3].ID).
		Count(&otherFiles).Error; err != nil || otherFiles != 1 {
		t.Fatal("重置错误删除了其他人员附件")
	}
	return after
}
