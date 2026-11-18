package cpcpersonSvc

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/OVINC-CN/AgileOA/src/api/internal/spreadsheettest"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonModels"
)

func TestPersonPermissionsAndScopeAreIndependent(t *testing.T) {
	own, other := uint64(1), uint64(2)
	for _, test := range []struct {
		name    string
		access  personAccess
		action  string
		branch  *uint64
		allowed bool
	}{
		{"书记没有人员权限", personAccess{branchIDs: []uint64{own}}, accessConstants.CPCPersonList, &own, false},
		{"只有人员权限但不是书记", personAccess{actions: []string{accessConstants.CPCPersonList}}, accessConstants.CPCPersonList, &own, false},
		{"本支部书记", personAccess{actions: []string{accessConstants.CPCPersonList}, branchIDs: []uint64{own}}, accessConstants.CPCPersonList, &own, true},
		{"其他支部", personAccess{actions: []string{accessConstants.CPCPersonList}, branchIDs: []uint64{own}}, accessConstants.CPCPersonList, &other, false},
		{"仅管理范围没有细粒度权限", personAccess{actions: []string{accessConstants.CPCPartyBranchList}}, accessConstants.CPCPersonList, &other, false},
		{"读取全空间", personAccess{actions: []string{accessConstants.CPCPersonList, accessConstants.CPCPartyBranchList}}, accessConstants.CPCPersonList, &other, true},
		{"查看支部不扩大编辑范围", personAccess{actions: []string{accessConstants.CPCPersonUpdate, accessConstants.CPCPartyBranchList}, branchIDs: []uint64{own}}, accessConstants.CPCPersonUpdate, &other, false},
		{"查看支部不扩大删除范围", personAccess{actions: []string{accessConstants.CPCPersonDelete, accessConstants.CPCPartyBranchList}, branchIDs: []uint64{own}}, accessConstants.CPCPersonDelete, &other, false},
		{"编辑全空间", personAccess{actions: []string{accessConstants.CPCPersonUpdate, accessConstants.CPCPartyBranchUpdate}}, accessConstants.CPCPersonUpdate, &other, true},
		{"删除全空间", personAccess{actions: []string{accessConstants.CPCPersonDelete, accessConstants.CPCPartyBranchDelete}}, accessConstants.CPCPersonDelete, &other, true},
		{"编辑支部不扩大读取范围", personAccess{actions: []string{accessConstants.CPCPersonList, accessConstants.CPCPartyBranchUpdate}}, accessConstants.CPCPersonList, &other, false},
		{"编辑权限不能审核", personAccess{actions: []string{accessConstants.CPCPersonUpdate, accessConstants.CPCPartyBranchUpdate}}, accessConstants.CPCPersonTimelineTransferReview, &own, false},
		{"审核权限不要求编辑权限", personAccess{actions: []string{accessConstants.CPCPersonTimelineTransferReview}, branchIDs: []uint64{own}}, accessConstants.CPCPersonTimelineTransferReview, &own, true},
		{"审核不能跨支部", personAccess{actions: []string{accessConstants.CPCPersonTimelineTransferReview}, branchIDs: []uint64{own}}, accessConstants.CPCPersonTimelineTransferReview, &other, false},
		{"审核全空间", personAccess{actions: []string{accessConstants.CPCPersonTimelineTransferReview, accessConstants.CPCPartyBranchUpdate}}, accessConstants.CPCPersonTimelineTransferReview, &other, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.access.permits(test.action, test.branch) != test.allowed {
				t.Fatal("人员权限被书记身份或不对应的管理范围放宽")
			}
		})
	}
}

func TestAdmissionConditionsAreParameterized(t *testing.T) {
	connection, err := gorm.Open(mysql.New(mysql.Config{
		DSN: "fixture:fixture@tcp(127.0.0.1:1)/fixture", SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	raw := `{"type":"group","logic":"or","children":[{"type":"condition","field":"major","operator":"in","values":["x' OR 1=1 --"]},{"type":"condition","field":"advisor","operator":"in","values":["虚构导师"]},{"type":"condition","field":"education_level","operator":"in","values":["硕士"]},{"type":"condition","field":"student_no","operator":"in","values":["000001","S20260001"]}]}`
	expression, err := admissionExpression(&raw)
	if err != nil {
		t.Fatal(err)
	}
	result := connection.Table("students AS s").Where("s.workspace_id = ?", "w7m2k9p4s6n8c3x5").
		Where(expression).Find(&[]struct{ ID uint64 }{})
	if result.Error != nil || strings.Contains(result.Statement.SQL.String(), "OR 1=1") ||
		!strings.Contains(result.Statement.SQL.String(), "education_level") ||
		!strings.Contains(result.Statement.SQL.String(), "`s`.`student_no` IN (?,?)") ||
		strings.Contains(result.Statement.SQL.String(), "000001") ||
		len(result.Statement.Vars) != 6 {
		t.Fatal("准入规则值进入 SQL 文本或丢失租户条件")
	}
	if result.Statement.Vars[4] != "000001" || result.Statement.Vars[5] != "S20260001" {
		t.Fatal("学号未以完整文本参数匹配，或丢失前导零")
	}
	for _, value := range []string{
		`{"type":"condition","field":"identity_number","operator":"in","values":["x"]}`,
		`{"type":"group","logic":"or","children":[]}`,
		`{"type":"condition","field":"major","operator":"raw","values":["x"]}`,
		`{"type":"condition","field":"major","operator":"in","values":["x"],"extra":"x"}`,
		`null`,
	} {
		if _, err := admissionExpression(&value); err == nil {
			t.Fatal("未登记字段、表达式或无效规则被执行")
		}
	}
}

func TestXLSXDoesNotOverrideDerivedOrStudentFields(t *testing.T) {
	rows, err := parseWorkbook(
		spreadsheettest.Fixture(
			t,
			"学号,姓名,发展阶段,所属党支部,党内职务,时间线模板\n000001,只读姓名,预备党员,只读支部,自定义职务,虚构时间线\n",
		),
	)
	if err != nil || len(rows) != 1 {
		t.Fatalf("固定导出列不能重新导入：%v", err)
	}
	if _, err := clean(
		cpcpersonModels.Input{
			StudentID:     1,
			Stage:         "probationary_member",
			PartyPosition: rows[0]["党内职务"],
		},
	); err != nil {
		t.Fatal(err)
	}
	if _, err := parseWorkbook(
		spreadsheettest.Fixture(t, "学号,发展阶段,workspace_id\n1,applicant,other\n"),
	); err == nil {
		t.Fatal("任意数据库字段进入导入目录")
	}
	for _, value := range []string{"=1+1", "+SUM(1,1)", "@SUM(1)", "-1+1"} {
		file, err := xlsxFile(
			"cpc-people.xlsx",
			[][]string{importHeader, {"0001", "applicant", "在编", value, "虚构时间线"}},
		)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := parseWorkbook(file.Content)
		if err != nil || len(parsed) != 1 || parsed[0]["党内职务"] != value {
			t.Fatal("XLSX 往返改变了职务文本")
		}
	}
}

func TestPersonStatusDefaultsAndWorkbookDefaults(t *testing.T) {
	// 新旧模板都保留独立的人员状态，省略或留空时使用默认值。
	for _, test := range []struct {
		content string
		status  string
	}{
		{"学号,发展阶段,党内职务,时间线模板\n000001,applicant,,虚构时间线\n", cpcpersonConstants.DefaultStatus},
		{"学号,发展阶段,状态,时间线模板\n000001,applicant,,虚构时间线\n", cpcpersonConstants.DefaultStatus},
		{"学号,发展阶段,状态,学生状态,时间线模板\n000001,applicant,转出,在籍,虚构时间线\n", "转出"},
	} {
		rows, err := parseWorkbook(spreadsheettest.Fixture(t, test.content))
		if err != nil {
			t.Fatal(err)
		}
		input, err := clean(cpcpersonModels.Input{
			StudentID: 1, Stage: rows[0]["发展阶段"], Status: rows[0]["状态"],
		})
		if err != nil || input.Status != test.status {
			t.Fatalf("人员状态未使用默认值或被学生状态覆盖：%v", err)
		}
	}

	// API 和 XLSX 拒绝超长及控制字符，不接受无效状态进入动态选项。
	for _, status := range []string{strings.Repeat("中", 9), "转\t出", string([]byte{0xff})} {
		if _, err := clean(cpcpersonModels.Input{
			StudentID: 1, Stage: "applicant", Status: status,
		}); err == nil {
			t.Fatal("无效人员状态通过接口校验")
		}
		file, err := xlsxFile("cpc-people.xlsx", [][]string{
			importHeader, {"000001", "applicant", status, "", "虚构时间线"},
		})
		if !utf8.ValidString(status) {
			if err == nil {
				t.Fatal("无效文本编码进入 XLSX")
			}
			continue
		}

		if err != nil {
			t.Fatal(err)
		}
		if _, err := parseWorkbook(file.Content); err == nil {
			t.Fatal("无效人员状态通过导入校验")
		}
	}
}
