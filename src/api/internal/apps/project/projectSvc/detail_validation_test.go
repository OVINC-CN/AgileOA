package projectSvc

import (
	"errors"
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

func TestCleanParticipationRejectsRepeatedDetailNames(t *testing.T) {
	for _, names := range [][]string{{"志愿服务", "志愿服务"}, {"志愿服务", " 志愿服务 "}} {
		input := projectModels.ParticipationInput{
			ParticipationFields: projectModels.ParticipationFields{ProjectID: 1, StudentID: 2},
			Details:             []projectModels.ScoreItem{{Name: names[0]}, {Name: names[1]}},
		}
		if _, err := cleanParticipation(input); err == nil {
			t.Fatal("重复子项或首尾空格绕过了共用校验")
		}
	}
}

func TestDetailNamesUseDatabaseEquivalence(t *testing.T) {
	connection, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	database, err := gorm.Open(mysql.New(mysql.Config{
		Conn: connection, SkipInitializeWithVersion: true,
	}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT .*project_detail_options").
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("Merit"))
	mock.ExpectQuery("SELECT .*project_detail_options").
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("Merit"))
	items := []projectModels.ScoreItem{{Name: "Merit"}, {Name: "merit"}}
	err = validateDetailNames(database.WithContext(t.Context()), "fixture", 1, items)
	var failure *response.BusinessError
	if !errors.As(err, &failure) || failure.Message != "同一条记录中的子项名称不能重复" {
		t.Fatalf("数据库等价名称没有被识别为重复：%v", err)
	}
	if items[1].Name != "merit" {
		t.Fatal("校验不应改写已有内容的名称")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplicationDetailsReleaseNamesBeforeReuse(t *testing.T) {
	existing := map[string]projectModels.ApplicationDetail{
		"a": {DetailKey: "a", Name: "甲"},
		"b": {DetailKey: "b", Name: "乙"},
	}
	items := []projectModels.ApplicationDetailInput{
		{Key: "a", ScoreItem: projectModels.ScoreItem{Name: "乙"}},
		{Key: "b", ScoreItem: projectModels.ScoreItem{Name: "丙"}},
	}
	ordered, err := orderApplicationDetails(existing, items)
	if err != nil || len(ordered) != 2 || ordered[0].Key != "b" || ordered[1].Key != "a" {
		t.Fatalf("名称释放顺序不正确：%v %v", ordered, err)
	}
	if !reflect.DeepEqual(items[0], ordered[1]) {
		t.Fatal("排序改变了子项内容")
	}
	items[1].Name = "甲"
	if _, err := orderApplicationDetails(existing, items); err == nil {
		t.Fatal("名称互换不能写入一半后留下冲突")
	}
}

func TestDetailUniqueConstraintErrors(t *testing.T) {
	for _, name := range []string{"uk_participation_detail_name", "uk_application_detail_name"} {
		err := detailWriteError(
			&mysqlDriver.MySQLError{Number: 1062, Message: "Duplicate entry for " + name},
		)
		var failure *response.BusinessError
		if !errors.As(err, &failure) || failure.Message != "同一条记录中的子项名称不能重复" {
			t.Fatalf("数据库唯一约束没有返回子项提示：%v", err)
		}
	}
}
