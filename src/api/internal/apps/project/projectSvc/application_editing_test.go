package projectSvc

import (
	"errors"
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

func TestReadOnlyApprovalPermissions(t *testing.T) {
	id := uint64(7)
	for _, kind := range []string{
		projectConstants.ApproverApplicant, projectConstants.ApproverClassPosition,
		projectConstants.ApproverStudent, projectConstants.ApproverUser,
	} {
		t.Run(kind, func(t *testing.T) {
			actor := applicationActor{
				StudentID: id, ClassName: "一班", Positions: []string{"班长"},
			}
			mode := projectConstants.ApplicationReviews
			if kind == projectConstants.ApproverUser {
				actor = applicationActor{
					UserID: id, Actions: []string{accessConstants.ProjectApplicationList},
				}
				mode = projectConstants.ApplicationManage
			}
			info := projectModels.ApplicationSummary{
				StudentID: id, ClassName: "一班", Status: projectConstants.ApplicationPending,
			}
			node := projectModels.ApplicationNode{
				Position: 1, Kind: kind, ClassPosition: "班长", StudentID: &id, UserID: &id,
			}
			applicationPermissions(&info, actor, mode, node)
			if info.CanEdit || !info.CanApprove || !info.CanReject {
				t.Fatal("关闭本级编辑只应禁止内容及附件编辑，不能移除通过或驳回资格")
			}
		})
	}
}

func TestReadOnlyApprovalRejectsContentChanges(t *testing.T) {
	rank, score := int32(2), 8.0
	before := projectModels.ApplicationInfo{
		ApplicationValues: projectModels.ApplicationValues{
			Statement: "已保存说明", Rank: &rank, Score: &score,
		},
		Details: []projectModels.ApplicationDetailInfo{{
			ApplicationDetailInput: projectModels.ApplicationDetailInput{
				Key: "0123456789abcdef0123456789abcdef",
				ScoreItem: projectModels.ScoreItem{
					Name: "志愿服务", Description: "已保存子项说明", Score: &score,
				},
			},
		}},
	}
	for _, operation := range []string{
		projectConstants.ApplicationApprove, projectConstants.ApplicationReject,
	} {
		for _, item := range []struct {
			name   string
			change func(*projectModels.ApplicationRequest)
		}{
			{"申报说明", func(r *projectModels.ApplicationRequest) { r.Statement = "篡改说明" }},
			{"排名", func(r *projectModels.ApplicationRequest) { r.Rank = nil }},
			{"总分", func(r *projectModels.ApplicationRequest) { r.RawScore = []byte("0") }},
			{"移除子项", func(r *projectModels.ApplicationRequest) { r.Details = nil }},
			{"新增子项", func(r *projectModels.ApplicationRequest) {
				r.Details = append(r.Details, projectModels.ApplicationDetailInput{})
			}},
			{"子项标识", func(r *projectModels.ApplicationRequest) { r.Details[0].Key = "其他标识" }},
			{"子项名称", func(r *projectModels.ApplicationRequest) { r.Details[0].Name = "其他子项" }},
			{"子项说明", func(r *projectModels.ApplicationRequest) { r.Details[0].Description = "新说明" }},
			{"子项分数", func(r *projectModels.ApplicationRequest) { r.Details[0].Score = nil }},
		} {
			t.Run(operation+"/"+item.name, func(t *testing.T) {
				request := projectModels.ApplicationRequest{
					ApplicationContent: applicationInput(before),
				}
				item.change(&request)
				var business *response.BusinessError
				if err := validateApplicationEditing(
					before,
					request,
					operation,
				); !errors.As(err, &business) ||
					business.Status != http.StatusForbidden {
					t.Fatalf("只读审批夹带内容修改时应返回 403：%v", err)
				}
			})
		}
		request := projectModels.ApplicationRequest{
			ApplicationContent: applicationInput(before), Comment: "仅提交审批意见",
		}
		if err := validateApplicationEditing(before, request, operation); err != nil {
			t.Fatalf("未修改内容的只读审批应可以继续流转：%v", err)
		}
		before.ScoreCalculationEnabled = true
		request.RawRank, request.RawScore = []byte(`"忽略"`), []byte(`"忽略"`)
		if err := validateApplicationEditing(before, request, operation); err != nil {
			t.Fatalf("强制计算应继续忽略客户端总分和排名：%v", err)
		}
		before.ScoreCalculationEnabled = false
	}
	if err := validateApplicationEditing(before, projectModels.ApplicationRequest{
		ApplicationContent: applicationInput(before),
	}, projectConstants.ApplicationUpdate); err == nil {
		t.Fatal("只读审批级别即使没有修改内容，也不能调用保存入口")
	}
}

func TestApplicationEditingUpdatePreservesWorkflowAndSignatures(t *testing.T) {
	for _, staleRevision := range []bool{false, true} {
		t.Run(map[bool]string{false: "更新", true: "并发冲突"}[staleRevision], func(t *testing.T) {
			database, mock := autoSubmitDatabase(t)
			application := projectModels.Application{
				ID: 5, WorkspaceID: "w7m2k9p4s6n8c3x5", ProjectID: 3, Revision: 4,
			}
			mock.ExpectBegin()
			// 不允许删除节点、签署、历史或修改审批进度，只修改指定级别开关及并发版本。
			mock.ExpectExec("UPDATE `project_application_nodes` SET `allow_edit`=\\? WHERE").
				WithArgs(false, application.WorkspaceID, application.ID, 2, false).
				WillReturnResult(sqlmock.NewResult(0, 2))
			rows := int64(1)
			if staleRevision {
				rows = 0
			}
			mock.ExpectExec("UPDATE `project_applications` SET `revision`=\\? WHERE").
				WithArgs(5, application.WorkspaceID, 3, application.ID, 4).
				WillReturnResult(sqlmock.NewResult(0, rows))
			if staleRevision {
				mock.ExpectRollback()
			} else {
				mock.ExpectCommit()
			}
			err := database.Transaction(func(tx *gorm.DB) error {
				_, err := updateApplicationEditing(tx, application, map[int]bool{2: false})
				return err
			})
			if (err != nil) != staleRevision {
				t.Fatalf("开关更新应保留审批流程，并发冲突应回滚：%v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
