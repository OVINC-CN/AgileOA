package studentdashboardSvc

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/studentdashboard/studentdashboardModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
)

func TestAccommodationNormalizesLocationsBeforeFilteringAndCounting(t *testing.T) {
	// SQL 必须先限定租户及在籍状态，再统一非校内位置，外层只使用固定字段筛选与分组。
	matcher := sqlmock.QueryMatcherFunc(func(_, actual string) error {
		for _, fragment := range []string{
			"CASE WHEN accommodation_type IN",
			"CASE WHEN accommodation_type = ? THEN TRIM(campus) ELSE '' END AS campus",
			"CASE WHEN accommodation_type = ? THEN TRIM(building) ELSE '' END AS building",
			"CASE WHEN accommodation_type = ? THEN TRIM(room) ELSE '' END AS room",
			"WHERE workspace_id = ? AND status = ?",
			") AS a WHERE a.accommodation_type IN (?)",
			"a.campus IN (?)", "a.building IN (?)", "a.room IN (?)",
			"GROUP BY accommodation_type, campus, building, room",
		} {
			if !strings.Contains(actual, fragment) {
				return fmt.Errorf("住宿查询缺少固定约束：%s", fragment)
			}
		}
		return nil
	})
	connection, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(matcher))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	database, err := gorm.Open(
		mysql.New(mysql.Config{Conn: connection, SkipInitializeWithVersion: true}),
		&gorm.Config{DisableAutomaticPing: true, SkipDefaultTransaction: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	maxOptions := config.Config.Student.MaxOptions
	config.Config.Student.MaxOptions = 10
	t.Cleanup(func() { config.Config.Student.MaxOptions = maxOptions })
	const workspaceID = "w7m2k9p4s6n8c3x5"
	mock.ExpectQuery("住宿统计").WithArgs(
		"in", "out", "abrd", "", "in", "in", "in", workspaceID, "在籍",
		"in", "虚构园区", "1号楼", "0101",
	).WillReturnRows(sqlmock.NewRows([]string{
		"accommodation_type", "campus", "building", "room", "count",
	}).AddRow("in", "虚构园区", "1号楼", "0101", 2))
	data, err := loadAccommodation(
		database.WithContext(t.Context()),
		workspaceID,
		studentdashboardModels.AccommodationQuery{
			AccommodationTypes: []string{"on_campus"},
			Campuses:           []string{"虚构园区"},
			Buildings:          []string{"1号楼"},
			Rooms:              []string{"0101"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Items) != 1 || data.Items[0].AccommodationType.Value != "on_campus" ||
		data.Items[0].AccommodationType.Label != "校内" || data.Total != 2 ||
		data.Items[0].Room != "0101" {
		t.Fatalf("住宿枚举、人数或宿舍号未保留：%+v", data)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAccommodationExportKeepsLocationsTextAndIncludesTotals(t *testing.T) {
	t.Parallel()
	data := studentdashboardModels.AccommodationData{
		Items: []studentdashboardModels.AccommodationSummary{
			{
				AccommodationType: studentdashboardModels.Option{Value: "on_campus", Label: "校内"},
				Campus:            "虚构园区", Building: "1号楼", Room: "0101", Count: 2,
			},
			{
				AccommodationType: studentdashboardModels.Option{Value: "off_campus", Label: "校外"},
				Count:             3,
			},
		},
		Total: 5,
	}
	want := [][]string{
		{"住宿类型", "住宿园区", "宿舍楼", "宿舍号", "人数"},
		{"校内", "虚构园区", "1号楼", "0101", "2"},
		{"校外", "", "", "", "3"},
		{"合计", "", "", "", "5"},
	}
	if got := accommodationRecords(data); !reflect.DeepEqual(got, want) {
		t.Fatalf("住宿导出未保留列顺序、前导零或合计：%v", got)
	}
	if err := validateAccommodation(studentdashboardModels.AccommodationQuery{
		AccommodationTypes: []string{"other"},
	}); err == nil {
		t.Fatal("住宿筛选接受了未知类型")
	}
}
