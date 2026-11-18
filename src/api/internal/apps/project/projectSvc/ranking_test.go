package projectSvc

import (
	"math"
	"slices"
	"testing"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectModels"
)

func TestApplicationRankingDimensionCombinations(t *testing.T) {
	// 八名学生覆盖全部维度组合，按分数从高到低构造独立分组的预期排名。
	rows := make([]applicationRankingRow, 0, 8)
	for _, grade := range []string{"2025", "2026"} {
		for _, major := range []string{"甲专业", "乙专业"} {
			for _, className := range []string{"01班", "02班"} {
				score := 100 - float64(len(rows))
				rows = append(rows, applicationRankingRow{
					ID: uint64(len(rows) + 1), Revision: 7, Score: &score,
					Grade: grade, Major: major, ClassName: className,
				})
			}
		}
	}
	for _, test := range []struct {
		name       string
		dimensions []string
		want       []int32
	}{
		{"全部", nil, []int32{1, 2, 3, 4, 5, 6, 7, 8}},
		{"年级", []string{"grade"}, []int32{1, 2, 3, 4, 1, 2, 3, 4}},
		{"专业", []string{"major"}, []int32{1, 2, 1, 2, 3, 4, 3, 4}},
		{"班级", []string{"class_name"}, []int32{1, 1, 2, 2, 3, 3, 4, 4}},
		{"年级专业", []string{"grade", "major"}, []int32{1, 2, 1, 2, 1, 2, 1, 2}},
		{"年级班级", []string{"grade", "class_name"}, []int32{1, 1, 2, 2, 1, 1, 2, 2}},
		{"专业班级", []string{"major", "class_name"}, []int32{1, 1, 1, 1, 2, 2, 2, 2}},
		{"全部维度", []string{"grade", "major", "class_name"}, []int32{1, 1, 1, 1, 1, 1, 1, 1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			changes, err := rankApplicationRows(rows, test.dimensions)
			if err != nil || len(changes) != len(rows) {
				t.Fatalf("排名分组失败：%v，变更 %d 条", err, len(changes))
			}
			for index, change := range changes {
				if change.ID != rows[index].ID || change.Revision != 7 ||
					change.After == nil || *change.After != test.want[index] {
					t.Fatalf("申报 %d 排名错误：%v，期望 %d", change.ID,
						rankingLogValue(change.After), test.want[index])
				}
			}
		})
	}
}

func TestApplicationRankingTiesNullAndIdempotence(t *testing.T) {
	high, zero, negative := 90.0, 0.0, -10.0
	previous := int32(5)
	rows := []applicationRankingRow{
		{ID: 1, Score: &high},
		{ID: 2, Rank: &previous},
		{ID: 3, Score: &high},
		{ID: 4, Score: &zero},
		{ID: 5, Score: &negative},
		{ID: 6},
	}
	changes, err := rankApplicationRows(rows, nil)
	if err != nil || len(changes) != 6 {
		t.Fatalf("并列、零分、负分或空分排名失败：%v，变更 %d 条", err, len(changes))
	}
	want := []int32{1, 3, 1, 3, 6, 3}
	for index, change := range changes {
		if change.ID != rows[index].ID || change.After == nil || *change.After != want[index] {
			t.Fatalf("申报 %d 排名错误：%v", change.ID, rankingLogValue(change.After))
		}
		rows[index].Rank = change.After
	}
	if changes, err := rankApplicationRows(rows, nil); err != nil || len(changes) != 0 {
		t.Fatalf("重复任务不应改写未变化记录：%v，变更 %d 条", err, len(changes))
	}
	for _, invalid := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		rows[5].Score = &invalid
		if changes, err := rankApplicationRows(rows, nil); err == nil || len(changes) != 0 {
			t.Fatal("非有限总分必须整批失败，不得部分改写排名")
		}
	}
}

func TestRankingDimensionsValidation(t *testing.T) {
	for _, test := range []struct {
		enabled    bool
		dimensions []string
	}{
		{false, []string{"grade"}},
		{true, []string{"education_level"}},
		{true, []string{"major", "major"}},
		{true, []string{""}},
	} {
		if err := cleanRankingDimensions(&projectModels.ScoreCalculation{
			Enabled: test.enabled, RankingDimensions: test.dimensions,
		}); err == nil {
			t.Fatalf("应拒绝未启用、未知或重复的排名维度：%v", test.dimensions)
		}
	}
	value := projectModels.ScoreCalculation{
		Enabled: true, RankingDimensions: []string{"class_name", "major", "grade"},
	}
	if err := cleanRankingDimensions(&value); err != nil ||
		!slices.Equal(value.RankingDimensions, []string{"grade", "major", "class_name"}) {
		t.Fatalf("排名维度未按固定目录保存：%v", err)
	}
}
