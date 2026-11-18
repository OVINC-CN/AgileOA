package cpctimelineSvc

import (
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpctimeline/cpctimelineModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

func TestClonePreservesDefinitionAndOnlyAppendsFixedSuffix(t *testing.T) {
	input := fixtureInput()
	input.Description = "虚构模板说明"
	input.Definition.Nodes[0].GroupName = "申请阶段"
	input.Definition.Nodes[0].IsBaseline = true
	input.Definition.Nodes[0].Fields[0].StudentField = "birth_date"
	input.Transfers = []cpctimelineModels.Transfer{{
		TargetTemplateID: 11,
		NodeMappings: []cpctimelineModels.TransferNodeMapping{{
			SourceNodeID: "node_first", TargetNodeID: "node_target",
			Fields: []cpctimelineModels.TransferFieldMapping{{
				SourceFieldID: "note", TargetFieldID: "note_target",
			}},
		}},
	}}
	source := info(cpctimelineModels.Template{
		ID: 7, Name: input.Name, Description: input.Description, Revision: 5,
	}, input.Definition)
	source.Transfers = input.Transfers
	cloned, err := cloneInput(source)
	if err != nil {
		t.Fatal(err)
	}
	if cloned.Name != "虚构时间线 (副本)" || cloned.Description != input.Description ||
		!reflect.DeepEqual(cloned.Definition, input.Definition) ||
		!reflect.DeepEqual(cloned.Transfers, input.Transfers) {
		t.Fatal("克隆未完整保留模板定义及转移配置，或未使用固定名称")
	}
	source.Name = cloned.Name
	again, err := cloneInput(source)
	if err != nil || again.Name != "虚构时间线 (副本) (副本)" {
		t.Fatalf("克隆已有副本时没有在完整原名后追加固定后缀：%s，%v", again.Name, err)
	}
}

func TestCloneRejectsLongNamesWithoutTruncation(t *testing.T) {
	for _, length := range []int{95, 96, 100} {
		source := info(cpctimelineModels.Template{Name: strings.Repeat("名", length)},
			fixtureInput().Definition)
		cloned, err := cloneInput(source)
		if length == 95 {
			if err != nil || cloned.Name != source.Name+" (副本)" {
				t.Fatalf("100 字符的完整副本名称被拒绝或截断：%v", err)
			}
			continue
		}
		var business *response.BusinessError
		if !errors.As(err, &business) || business.Status != http.StatusBadRequest ||
			cloned.Name != source.Name+" (副本)" {
			t.Fatal("超长副本名称没有拒绝，或修改了原名称")
		}
	}
}

func TestCloneRejectsUnfinishedAttachmentsBeforeCopying(t *testing.T) {
	for _, state := range []string{attachmentConstants.Pending, attachmentConstants.Deleting} {
		t.Run(state, func(t *testing.T) {
			database, mock := nodeTestDB(t)
			mock.ExpectQuery("SELECT .* FROM `cpc_timeline_node_attachments` WHERE "+
				"workspace_id = \\? AND timeline_template_id = \\? ORDER BY id ASC").
				WithArgs(fixtureWorkspaceID, uint64(7)).
				WillReturnRows(sqlmock.NewRows([]string{
					"id", "workspace_id", "timeline_template_id", "node_id", "state",
				}).AddRow(11, fixtureWorkspaceID, 7, "node_first", state))
			copies, err := planCloneAttachments(database, fixtureWorkspaceID, 7, 8)
			var business *response.BusinessError
			if !errors.As(err, &business) || business.Status != http.StatusConflict ||
				len(copies) != 0 {
				t.Fatal("未完成或待删除的附件未阻止整个克隆")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCloneRejectsAttachmentsOverCurrentNodeLimit(t *testing.T) {
	original := config.Config.CPCTimeline.MaxAttachments
	t.Cleanup(func() { config.Config.CPCTimeline.MaxAttachments = original })
	config.Config.CPCTimeline.MaxAttachments = 1
	database, mock := nodeTestDB(t)
	mock.ExpectQuery("SELECT .* FROM `cpc_timeline_node_attachments` WHERE "+
		"workspace_id = \\? AND timeline_template_id = \\? ORDER BY id ASC").
		WithArgs(fixtureWorkspaceID, uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "node_id", "state"}).
			AddRow(11, "node_first", attachmentConstants.Ready).
			AddRow(12, "node_first", attachmentConstants.Ready))
	copies, err := planCloneAttachments(database, fixtureWorkspaceID, 7, 8)
	var business *response.BusinessError
	if !errors.As(err, &business) || business.Status != http.StatusConflict || len(copies) != 0 {
		t.Fatal("附件超限时没有在复制任何对象前拒绝克隆")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
