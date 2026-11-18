package monthlyworkSvc

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/monthlywork/monthlyworkModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
)

func TestWorkRejectsInvalidDateOwnerAndExternalLinks(t *testing.T) {
	input := monthlyworkModels.Input{Date: "2024-02-29", WorkItem: "虚构工作", OwnerID: 1}
	if _, err := clean(input); err != nil {
		t.Fatal("有效闰日被拒绝")
	}
	for _, date := range []string{"2026-02-29", "2026-2-03", "1899-12-31", "2026-11-03T00:00:00Z"} {
		changed := input
		changed.Date = date
		if _, err := clean(changed); err == nil {
			t.Fatalf("无效日期被接受：%s", date)
		}
	}
	changed := input
	changed.OwnerID = 0
	if _, err := clean(changed); err == nil {
		t.Fatal("允许缺少负责人用户编号")
	}
	for _, address := range []string{"javascript:alert(1)", "data:text/html,hello", "//example.com", "https://user:pass@example.com", "https://example.com/含 空格", "https://example.com/a\nb"} {
		changed := input
		changed.Links = []monthlyworkModels.Link{{URL: address}}
		if _, err := clean(changed); err == nil {
			t.Fatalf("无效外部链接被接受：%q", address)
		}
	}
	changed = input
	changed.Links = make([]monthlyworkModels.Link, config.Config.MonthlyWork.MaxLinks+1)
	if _, err := clean(changed); err == nil {
		t.Fatal("允许超过限制的外部链接")
	}
}

func TestWorkParticipantIDsValidation(t *testing.T) {
	input := monthlyworkModels.Input{
		Date: "2026-11-03", WorkItem: "虚构工作", OwnerID: 1,
		ParticipantIDs: []uint64{3, 2, 3, 1},
	}
	cleaned, err := clean(input)
	if err != nil || !slices.Equal(cleaned.ParticipantIDs, []uint64{1, 2, 3}) {
		t.Fatal("参与人编号未去重排序，或错误拒绝负责人同时参与")
	}
	if !slices.Equal(input.ParticipantIDs, []uint64{3, 2, 3, 1}) {
		t.Fatal("参与人规范化修改了调用方的输入")
	}
	input.ParticipantIDs = nil
	cleaned, err = clean(input)
	if err != nil || cleaned.ParticipantIDs == nil || len(cleaned.ParticipantIDs) != 0 {
		t.Fatal("无参与人未规范化为空数组")
	}
	input.ParticipantIDs = []uint64{0}
	if _, err := clean(input); err == nil {
		t.Fatal("允许无效参与人编号")
	}
	input.ParticipantIDs = make([]uint64, config.Config.MonthlyWork.MaxOptions+1)
	if _, err := clean(input); err == nil {
		t.Fatal("允许超过限制的参与人数")
	}
}

func TestLinkAuditTracksChangesWithoutCopyingSignedURL(t *testing.T) {
	info := monthlyworkModels.Info{Input: monthlyworkModels.Input{
		Date:     "2026-11-03",
		WorkItem: "虚构工作",
		OwnerID:  1,
		Links: []monthlyworkModels.Link{
			{Name: "工作材料", URL: "https://example.com/work?signature=fictional-first"},
		},
	}}
	before, err := json.Marshal(snapshot(info))
	if err != nil {
		t.Fatal(err)
	}
	info.Links[0].URL = "https://example.com/work?signature=fictional-second"
	after, err := json.Marshal(snapshot(info))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) == string(after) || strings.Contains(string(before), "fictional-first") ||
		strings.Contains(
			string(after),
			"fictional-second",
		) || !strings.Contains(string(after), "url_hash") {
		t.Fatal("链接修改未改变审计摘要或审计复制了签名参数")
	}
}
