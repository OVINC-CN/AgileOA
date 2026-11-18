package projectSvc

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
	"github.com/OVINC-CN/AgileOA/src/api/internal/storage"
)

func TestApplicationDeletionStorageRequirement(t *testing.T) {
	// 没有附件时不依赖对象存储，未完成上传及待删除附件也不能跳过对象清理。
	if err := requireApplicationDeletionFiles(nil); err != nil {
		t.Fatalf("无附件的填报删除不应依赖对象存储：%v", err)
	}
	for _, state := range []string{
		attachmentConstants.Pending, attachmentConstants.Ready, attachmentConstants.Deleting,
	} {
		err := requireApplicationDeletionFiles([]attachmentModels.ApplicationDetailAttachment{{
			Record: attachmentModels.Record{ID: 7, State: state, Revision: 1},
		}})
		if storage.Enabled() {
			if err != nil {
				t.Fatalf("已配置存储时应允许清理 %s 附件：%v", state, err)
			}
			continue
		}
		var business *response.BusinessError
		if !errors.As(err, &business) || business.Status != http.StatusServiceUnavailable {
			t.Fatalf("存储不可用时不能遗留 %s 附件对象：%v", state, err)
		}
	}
}

func TestApplicationDeletionDetectsFileChangesBetweenTransactions(t *testing.T) {
	for _, item := range []struct {
		name     string
		files    []attachmentModels.ApplicationDetailAttachment
		prepared map[uint64]uint64
		want     bool
	}{
		{"删除意图一致", []attachmentModels.ApplicationDetailAttachment{{
			Record: attachmentModels.Record{ID: 7, State: "deleting", Revision: 2},
		}}, map[uint64]uint64{7: 2}, true},
		{"中途新增附件", []attachmentModels.ApplicationDetailAttachment{{
			Record: attachmentModels.Record{ID: 7, State: "deleting", Revision: 2},
		}, {Record: attachmentModels.Record{ID: 8, State: "ready", Revision: 1}}},
			map[uint64]uint64{7: 2}, false},
		{"中途移除附件", nil, map[uint64]uint64{7: 2}, false},
		{"替换为同版本附件", []attachmentModels.ApplicationDetailAttachment{{
			Record: attachmentModels.Record{ID: 8, State: "deleting", Revision: 2},
		}}, map[uint64]uint64{7: 2}, false},
		{"附件版本变化", []attachmentModels.ApplicationDetailAttachment{{
			Record: attachmentModels.Record{ID: 7, State: "deleting", Revision: 3},
		}}, map[uint64]uint64{7: 2}, false},
		{"附件状态变化", []attachmentModels.ApplicationDetailAttachment{{
			Record: attachmentModels.Record{ID: 7, State: "ready", Revision: 2},
		}}, map[uint64]uint64{7: 2}, false},
	} {
		t.Run(item.name, func(t *testing.T) {
			if applicationDeletionFilesMatch(item.files, item.prepared) != item.want {
				t.Fatal("附件清理未完整复核删除意图，可能误删并发修改的内容")
			}
		})
	}
}

func TestApplicationDeletionSnapshotKeepsRootAndOriginalNodeReferences(t *testing.T) {
	// 即使处理人不再可用，删除审计仍保存全部流程原始引用及历史身份快照。
	tx, mock := autoSubmitDatabase(t)
	const workspaceID = "w7m2k9p4s6n8c3x5"
	mock.ExpectQuery("SELECT .* FROM `project_application_nodes` WHERE workspace_id = \\? AND application_id = \\?").
		WithArgs(workspaceID, 7).
		WillReturnRows(sqlmock.NewRows([]string{
			"position", "target_order", "kind", "sign_mode", "user_id", "reviewer_label",
		}).AddRow(0, 0, "applicant", "any", nil, "申报人").
			AddRow(1, 0, "user", "all", 9, "虚构审批人(fixture.reviewer)"))
	info := projectModels.ApplicationInfo{
		ApplicationSummary: projectModels.ApplicationSummary{ID: 7, StudentID: 8, Revision: 3},
		Details: []projectModels.ApplicationDetailInfo{{
			ID: 10,
			ApplicationDetailInput: projectModels.ApplicationDetailInput{
				Key: "0123456789abcdef0123456789abcdef",
				ScoreItem: projectModels.ScoreItem{
					Name: "志愿服务", Description: "虚构说明",
				},
			},
			Attachments: []attachmentModels.Info{{ID: 11, Filename: "虚构附件.pdf", State: "ready"}},
		}},
		History: []projectModels.ApplicationHistory{{
			ID: 12, ActorType: "user", ActorID: 9, ActorName: "历史处理人", Operation: "approve",
		}},
		ApprovalRecords: []projectModels.ApplicationApproval{
			{Position: 1, ActorType: "user", ActorID: 9},
		},
	}
	snapshot, err := applicationDeletionSnapshot(tx, workspaceID, info)
	if err != nil {
		t.Fatal(err)
	}
	nodes := snapshot["approval_nodes"].([]auditModels.Snapshot)
	if len(nodes) != 2 || nodes[0]["kind"] != "applicant" ||
		nodes[1]["reviewer_label"] != "虚构审批人(fixture.reviewer)" {
		t.Fatal("删除审计遗漏申报人节点或历史处理人引用")
	}
	content, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"志愿服务", "虚构说明", "虚构附件.pdf", "历史处理人", "approvals"} {
		if !bytes.Contains(content, []byte(value)) {
			t.Fatalf("删除审计缺少关联快照：%s", value)
		}
	}
	if bytes.Contains(content, []byte(`"key"`)) || bytes.Contains(content, []byte("object_key")) {
		t.Fatal("删除审计使用凭据保留字段或暴露私有对象键")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
