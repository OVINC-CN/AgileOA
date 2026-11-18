package projectSvc

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectModels"
)

func TestApplicationVersionProjection(t *testing.T) {
	content := json.RawMessage(`{
		"id":5,"revision":2,"updated_at":"2026-11-16T00:00:00Z",
		"statement":"历史说明","rank":null,"score":0,
		"student_no":"私密身份","approvals":[{"actor_id":9}],
		"details":[{"id":7,"detail_code":"0123456789abcdef0123456789abcdef",
			"name":"志愿服务","description":"原始内容","score":0,
			"attachments":[{"id":1,"filename":"材料.pdf","size":123,"state":"ready",
				"object_key":"private/object","url":"private-url"}]}]
	}`)
	current := projectModels.Application{ID: 5, Revision: 3}
	version, err := applicationVersion(content, current)
	if err != nil || version == nil || version.Score == nil || *version.Score != 0 {
		t.Fatalf("历史零分或空排名还原失败：%v", err)
	}
	encoded, err := json.Marshal(version)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{
		"私密身份", "approvals", "actor_id", "object_key", "private", "state", "size",
	} {
		if strings.Contains(string(encoded), value) {
			t.Fatalf("正文投影泄露额外字段：%s", value)
		}
	}
	if !strings.Contains(string(encoded), "材料.pdf") ||
		version.Details[0].DetailCode != "0123456789abcdef0123456789abcdef" {
		t.Fatal("历史正文遗漏附件名或稳定子项标识")
	}
	for _, item := range []struct{ id, revision uint64 }{{6, 2}, {5, 2}, {5, 1}} {
		value, err := applicationVersion(content, projectModels.Application{
			ID: item.id, Revision: item.revision,
		})
		if err != nil || value != nil {
			t.Fatalf("其他对象、当前或未来版本不应返回：%v", err)
		}
	}
	workflow := json.RawMessage(`{"id":5,"revision":1,"approval_nodes":[],"approvals":[]}`)
	if version, err := applicationVersion(workflow, current); err != nil || version != nil {
		t.Fatalf("流程专用快照不能被当作空白正文版本：%v", err)
	}
}

func TestApplicationVersionsReadBoundarySnapshotsAndDeduplicate(t *testing.T) {
	database, mock := autoSubmitDatabase(t)
	const workspaceID = "w7m2k9p4s6n8c3x5"
	deadline := time.Date(2026, time.November, 16, 0, 0, 0, 0, time.UTC)
	actions := applicationVersionActions()
	args := make([]driver.Value, 0, 4+len(actions))
	args = append(args, workspaceID, "project_application", "5", 0)
	for _, action := range actions {
		args = append(args, action)
	}
	snapshot := func(revision int) string {
		return fmt.Sprintf(
			`{"id":5,"revision":%d,"statement":"","rank":null,"score":null,"details":null,"updated_at":%q}`,
			revision,
			deadline.Add(time.Duration(revision-2)*time.Microsecond).Format(time.RFC3339Nano),
		)
	}
	pattern := "SELECT .* FROM `audit_events` WHERE .*workspace_id = \\? " +
		"AND resource_type_id = \\? AND instance_id = \\? AND result_code = \\?.*" +
		"action_id IN .*ORDER BY id DESC$"
	mock.ExpectQuery(pattern).
		WithArgs(args...).
		WillReturnRows(sqlmock.NewRows(
			[]string{"id", "instance_data", "instance_origin_data", "snapshot_user_info"},
		).AddRow(3, snapshot(5), snapshot(4), `{"display_name":"下次修改人","username":"next"}`).
			AddRow(2, snapshot(4), snapshot(3), `{"display_name":"虚构修改人","username":"editor"}`).
			AddRow(1, snapshot(2), snapshot(1), `{"display_name":"虚构学生","username":"0007"}`))
	result, err := applicationVersions(database, workspaceID,
		projectModels.Application{ID: 5, Revision: 5}, &deadline)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 3 || result.Items[0].Revision != 4 || result.Items[1].Revision != 2 ||
		result.Items[2].Revision != 1 ||
		result.Items[0].Details == nil {
		t.Fatalf("关键历史版本、前快照补全、去重或排序错误：%+v", result)
	}
	if result.Items[0].ActorName != "虚构修改人(editor)" ||
		result.Items[1].ActorName != "虚构学生(0007)" || result.Items[2].ActorName != "" {
		t.Fatalf("修改人未使用本版本写入后的身份快照，或把前快照误归给后续操作人：%+v", result.Items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplicationVersionSelection(t *testing.T) {
	start := time.Date(2026, time.November, 16, 0, 0, 0, 0, time.UTC)
	versions := []projectModels.ApplicationVersion{
		{Revision: 5, UpdatedAt: start.Add(5 * time.Hour)},
		{Revision: 4, UpdatedAt: start.Add(4 * time.Hour)},
		{Revision: 3, UpdatedAt: start.Add(3 * time.Hour)},
		{Revision: 2, UpdatedAt: start.Add(2 * time.Hour)},
		{Revision: 1, UpdatedAt: start.Add(time.Hour)},
	}
	between := start.Add(3*time.Hour + 30*time.Minute)
	exact := start.Add(3 * time.Hour)
	before := start.Add(10 * time.Hour)
	after := start
	for _, scenario := range []struct {
		name     string
		deadline *time.Time
		want     []uint64
	}{
		{name: "三个不同关键版本", deadline: &between, want: []uint64{5, 4, 3}},
		{name: "截止时刻归截止后", deadline: &exact, want: []uint64{5, 3, 2}},
		{name: "全部在截止前去重", deadline: &before, want: []uint64{5}},
		{name: "全部在截止后取首版", deadline: &after, want: []uint64{5, 1}},
		{name: "无截止时间仅上一版", want: []uint64{5}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			items := selectApplicationVersions(versions, scenario.deadline)
			got := make([]uint64, len(items))
			for index, item := range items {
				got[index] = item.Revision
			}
			if !slices.Equal(got, scenario.want) {
				t.Fatalf("关键版本选择不符合边界或去重规则：%v", got)
			}
		})
	}
	if items := selectApplicationVersions(
		[]projectModels.ApplicationVersion{},
		&between,
	); items == nil ||
		len(items) != 0 {
		t.Fatal("无历史版本时必须保持空数组")
	}
}
