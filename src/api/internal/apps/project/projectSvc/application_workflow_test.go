package projectSvc

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectModels"
)

func TestNewApplicationDeadline(t *testing.T) {
	cutoff := time.Date(2026, 12, 31, 15, 59, 0, 0, time.UTC)
	for _, item := range []struct {
		name     string
		open     bool
		deadline *time.Time
		now      time.Time
		allowed  bool
	}{
		{"不限时间", true, nil, cutoff.Add(time.Hour), true},
		{"截止前", true, &cutoff, cutoff.Add(-time.Microsecond), true},
		{"截止时刻", true, &cutoff, cutoff, false},
		{"截止后", true, &cutoff, cutoff.Add(time.Microsecond), false},
		{"关闭新建", false, nil, cutoff, false},
	} {
		t.Run(item.name, func(t *testing.T) {
			project := projectModels.ProjectInfo{ProjectInput: projectModels.ProjectInput{
				AllowSelfApply: item.open, SelfApplyDeadline: item.deadline,
			}}
			if err := requireNewApplication(project, item.now); (err == nil) != item.allowed {
				t.Fatalf("新建申报截止时间校验结果不符合预期：%v", err)
			}
		})
	}
}

func TestApplicationRejectionAndResubmission(t *testing.T) {
	// 任意一级驳回都退回申报人，重新提交后从第一级完整流转。
	for rejectedStep := 1; rejectedStep <= 3; rejectedStep++ {
		step, status, err := nextApplicationState(
			rejectedStep,
			3,
			projectConstants.ApplicationPending,
			projectConstants.ApplicationReject,
		)
		if err != nil || step != 0 || status != projectConstants.ApplicationRejected {
			t.Fatalf("第 %d 级驳回没有退回申报人：%d %s %v", rejectedStep, step, status, err)
		}
		step, status, err = nextApplicationState(
			step,
			3,
			status,
			projectConstants.ApplicationUpdate,
		)
		if err != nil || step != 0 || status != projectConstants.ApplicationRejected {
			t.Fatal("申报人保存修改不能推进审批")
		}
		step, status, err = nextApplicationState(
			step,
			3,
			status,
			projectConstants.ApplicationSubmit,
		)
		if err != nil || step != 1 || status != projectConstants.ApplicationPending {
			t.Fatal("重新提交必须从第一级重新审批")
		}
		for current := 1; current <= 3; current++ {
			step, status, err = nextApplicationState(
				step,
				3,
				status,
				projectConstants.ApplicationApprove,
			)
			if err != nil {
				t.Fatal(err)
			}
			if current < 3 && (step != current+1 || status != projectConstants.ApplicationPending) {
				t.Fatal("非末级通过应进入下一级，不能提前结束审批")
			}
		}
		if status != projectConstants.ApplicationApproved {
			t.Fatal("末级通过应结束审批")
		}
		if _, _, err := nextApplicationState(
			step, 3, status, projectConstants.ApplicationUpdate,
		); err == nil {
			t.Fatal("已通过申报不能再被修改")
		}
	}
}

func TestFirstApplicationRejectionReturnsToApplicant(t *testing.T) {
	step, status, err := nextApplicationState(1, 2, projectConstants.ApplicationPending,
		projectConstants.ApplicationReject)
	if err != nil || step != 0 || status != projectConstants.ApplicationRejected {
		t.Fatal("首级驳回必须退回申报人")
	}
	if _, _, err := nextApplicationState(
		step,
		2,
		status,
		projectConstants.ApplicationReject,
	); err == nil {
		t.Fatal("申报人节点不能继续驳回")
	}
	step, status, err = nextApplicationState(step, 2, status, projectConstants.ApplicationSubmit)
	if err != nil || step != 1 || status != projectConstants.ApplicationPending {
		t.Fatal("申报人重填必须回到首级")
	}
}

func TestApproverAssignmentBoundaries(t *testing.T) {
	studentID, userID := uint64(7), uint64(7)
	cases := []struct {
		name  string
		actor applicationActor
		node  projectModels.ApplicationNode
		want  bool
	}{
		{"本人申报", applicationActor{StudentID: 7}, projectModels.ApplicationNode{
			Kind: projectConstants.ApproverApplicant}, true},
		{"他人申报", applicationActor{StudentID: 8}, projectModels.ApplicationNode{
			Kind: projectConstants.ApproverApplicant}, false},
		{"本班独立职务", applicationActor{StudentID: 8, ClassName: "一班", Positions: []string{"班长"}},
			projectModels.ApplicationNode{Kind: projectConstants.ApproverClassPosition,
				ClassPosition: "班长"}, true},
		{"不能跨班", applicationActor{StudentID: 8, ClassName: "二班", Positions: []string{"班长"}},
			projectModels.ApplicationNode{Kind: projectConstants.ApproverClassPosition,
				ClassPosition: "班长"}, false},
		{"副班长不匹配班长", applicationActor{StudentID: 8, ClassName: "一班", Positions: []string{"副班长"}},
			projectModels.ApplicationNode{Kind: projectConstants.ApproverClassPosition,
				ClassPosition: "班长"}, false},
		{"组织编号不冒用学生", applicationActor{UserID: 7}, projectModels.ApplicationNode{
			Kind: projectConstants.ApproverStudent, StudentID: &studentID}, false},
		{"学生编号不冒用组织", applicationActor{StudentID: 7}, projectModels.ApplicationNode{
			Kind: projectConstants.ApproverUser, UserID: &userID}, false},
		{"指定学生", applicationActor{StudentID: 7}, projectModels.ApplicationNode{
			Kind: projectConstants.ApproverStudent, StudentID: &studentID}, true},
		{"指定组织", applicationActor{UserID: 7}, projectModels.ApplicationNode{
			Kind: projectConstants.ApproverUser, UserID: &userID}, true},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			if got := actorMatches(item.actor, 7, "一班", item.node); got != item.want {
				t.Fatalf("处理人匹配结果为 %t，期望 %t", got, item.want)
			}
		})
	}
}

func TestAssignedOrganizationApproverNeedsNoAdditionalGrant(t *testing.T) {
	// 组织账号先取得查看填报管理权限，再按节点处理，不需要学生服务或额外审批权限。
	userID := uint64(7)
	info := projectModels.ApplicationSummary{
		StudentID:      9,
		Status:         projectConstants.ApplicationPending,
		AllowSelfApply: true,
	}
	node := projectModels.ApplicationNode{
		Kind:      projectConstants.ApproverUser,
		UserID:    &userID,
		Position:  2,
		AllowEdit: true,
	}
	applicationPermissions(
		&info,
		applicationActor{UserID: userID},
		projectConstants.ApplicationManage,
		node,
	)
	if info.CanEdit || info.CanApprove || info.CanReject {
		t.Fatal("缺少查看填报管理权限时，节点指派不能授予任何操作资格")
	}
	applicationPermissions(
		&info,
		applicationActor{UserID: userID, Actions: []string{accessConstants.ProjectApplicationList}},
		projectConstants.ApplicationManage,
		node,
	)
	if !info.CanEdit || !info.CanApprove || !info.CanReject {
		t.Fatal("指定组织审批人应可以处理当前节点")
	}
	info.CanEdit, info.CanApprove, info.CanReject = false, false, false
	applicationPermissions(
		&info,
		applicationActor{UserID: 8, Actions: []string{accessConstants.ProjectApplicationList}},
		projectConstants.ApplicationManage,
		node,
	)
	if info.CanEdit || info.CanApprove || info.CanReject {
		t.Fatal("管理入口不能使非节点用户获得审批权")
	}
}

func TestClosedProjectPreservesExistingApplications(t *testing.T) {
	node := projectModels.ApplicationNode{Kind: projectConstants.ApproverApplicant}
	for _, status := range []string{
		projectConstants.ApplicationDraft,
		projectConstants.ApplicationRejected,
	} {
		info := projectModels.ApplicationSummary{StudentID: 7, Status: status}
		applicationPermissions(
			&info,
			applicationActor{StudentID: 7},
			projectConstants.ApplicationMine,
			node,
		)
		if !info.CanEdit || !info.CanApprove || info.CanReject {
			t.Fatalf("关闭申报不应阻止已有草稿或驳回单提交：%s", status)
		}
	}
	info := projectModels.ApplicationSummary{
		StudentID: 7,
		Status:    projectConstants.ApplicationPending,
	}
	applicationPermissions(
		&info,
		applicationActor{StudentID: 7},
		projectConstants.ApplicationMine,
		node,
	)
	if info.CanEdit || info.CanApprove {
		t.Fatal("申报人不能修改审批中的申报单")
	}
	userID := uint64(7)
	for _, status := range []string{
		projectConstants.ApplicationPending,
		projectConstants.ApplicationRejected,
	} {
		info := projectModels.ApplicationSummary{StudentID: 9, Status: status}
		applicationPermissions(
			&info,
			applicationActor{
				UserID:  userID,
				Actions: []string{accessConstants.ProjectApplicationList},
			},
			projectConstants.ApplicationManage,
			projectModels.ApplicationNode{
				Kind: projectConstants.ApproverUser, UserID: &userID, Position: 1, AllowEdit: true,
			},
		)
		if !info.CanEdit || !info.CanApprove || !info.CanReject {
			t.Fatalf("关闭申报不应阻止当前节点审批或驳回后的重新提交：%s", status)
		}
	}
}

func TestApplicationAuditUsesBusinessDetailCode(t *testing.T) {
	// 上传前保存草稿也必须经过凭据字段保护，业务标识不应误用 key 保留名。
	info := projectModels.ApplicationInfo{Details: []projectModels.ApplicationDetailInfo{
		{
			ApplicationDetailInput: projectModels.ApplicationDetailInput{
				Key:       "0123456789abcdef0123456789abcdef",
				ScoreItem: projectModels.ScoreItem{Name: "志愿服务"},
			},
		},
	}}
	content, err := json.Marshal(applicationSnapshot(info))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(content, []byte(`"detail_code"`)) ||
		bytes.Contains(content, []byte(`"key"`)) ||
		bytes.Contains(content, []byte(`"detail_key"`)) {
		t.Fatal("申报审计仍使用了凭据保留字段")
	}
}

func TestParallelApprovalModesAndIdentityNamespaces(t *testing.T) {
	reviewers := []projectModels.ApplicationReviewer{
		{IdentityType: "student", ID: 7},
		{IdentityType: "user", ID: 7},
	}
	partial := map[string]bool{identityKey("student", 7): true}
	if !allReviewersApproved(projectConstants.SignAny, true, reviewers, partial) {
		t.Fatal("或签应由任一处理人通过")
	}
	if allReviewersApproved(projectConstants.SignAll, true, reviewers, partial) {
		t.Fatal("学生编号不能冒用同号组织用户完成并签")
	}
	partial[identityKey("user", 7)] = true
	if !allReviewersApproved(projectConstants.SignAll, true, reviewers, partial) {
		t.Fatal("全部审批人通过后并签才能完成")
	}
	if allReviewersApproved(projectConstants.SignAll, false, reviewers, partial) {
		t.Fatal("并签要求的班委岗位缺人时不能自动完成")
	}
	if allReviewersApproved(projectConstants.SignAny, true, nil, partial) {
		t.Fatal("没有处理人的节点不能自动通过")
	}
}

func TestMultiTargetApprovalGrouping(t *testing.T) {
	studentID, userID := uint64(7), uint64(9)
	steps := groupApprovalSteps([]projectModels.ApprovalStep{
		{
			Position:      1,
			Kind:          projectConstants.ApproverClassPosition,
			SignMode:      projectConstants.SignAll,
			ClassPosition: "班长",
			ReviewerLabel: "班长",
		},
		{
			Position:      1,
			TargetOrder:   1,
			Kind:          projectConstants.ApproverClassPosition,
			SignMode:      projectConstants.SignAll,
			ClassPosition: "团支书",
			ReviewerLabel: "团支书",
		},
		{
			Position:      2,
			Kind:          projectConstants.ApproverStudent,
			SignMode:      projectConstants.SignAny,
			StudentID:     &studentID,
			ReviewerLabel: "学生 · 用户(甲)(0007)",
		},
		{
			Position:      3,
			Kind:          projectConstants.ApproverUser,
			SignMode:      projectConstants.SignAny,
			UserID:        &userID,
			ReviewerLabel: "系统管理员(admin)",
		},
	})
	if len(steps) != 3 || len(steps[0].ClassPositions) != 2 || steps[0].ReviewerLabel != "班长、团支书" ||
		steps[0].SignMode != projectConstants.SignAll {
		t.Fatal("同级多选对象未正确分组")
	}
	if len(steps[1].StudentIDs) != 1 || len(steps[2].UserIDs) != 1 {
		t.Fatal("学生与组织用户没有独立归组")
	}
	if steps[1].ReviewerLabel != "学生 · 用户(甲)(0007)" ||
		steps[1].ReviewerOptions[0].Label != steps[1].ReviewerLabel ||
		steps[2].ReviewerLabel != "系统管理员(admin)" ||
		steps[2].ReviewerOptions[0].Label != steps[2].ReviewerLabel {
		t.Fatal("审批人快照的名称和账号格式被改变")
	}
	copySteps := append([]projectModels.ApprovalStepInput{}, steps...)
	copySteps[0].ReviewerLabel = "其他显示快照"
	if !sameApprovalSteps(steps, copySteps) {
		t.Fatal("显示名称不应阻止关闭申报")
	}
}
