package studentSvc

import (
	"encoding/csv"
	"maps"
	"strings"
	"testing"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/spreadsheettest"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/student/studentConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/student/studentModels"
)

func TestStudentRequiredFields(t *testing.T) {
	values := map[string]string{
		"student_no": "000001", "name": "虚构学生", "grade": "2026级", "status": "在籍",
		"education_level": "硕士", "major": "虚构专业", "class_name": "虚构班级",
		"admission_type": "非定向", "gender": "女", "ethnicity": "汉族",
		"accommodation_type": "on_campus", "identity_number": "00000020000102001X",
	}
	for key := range values {
		field, ok := studentConstants.Lookup(key)
		if !ok || !field.Required {
			t.Fatalf("字段 %s 缺少必填声明", key)
		}
		for _, empty := range []string{"", " \t "} {
			row := maps.Clone(values)
			row[key] = empty
			if _, err := parseRow(row); err == nil || !strings.Contains(err.Error(), field.Label) {
				t.Fatalf("字段 %s 的空值没有被拒绝：%v", key, err)
			}
		}
		// 缺少必填列时在表头校验阶段报告具体列名。
		header, record := []string{}, []string{}
		for column, value := range values {
			if column != key {
				header, record = append(header, column), append(record, value)
			}
		}
		file, err := xlsxFile("students.xlsx", [][]string{header, record})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := parseWorkbook(file.Content); err == nil ||
			!strings.Contains(err.Error(), field.Label) {
			t.Fatalf("缺少 %s 列时未被拒绝：%v", key, err)
		}
	}
}

// studentImportFixture 补齐既有测试未关注的必填列，保留用例显式提供的空值和非法值。
func studentImportFixture(t *testing.T, table string) string {
	t.Helper()
	reader := csv.NewReader(strings.NewReader(table))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	defaults := [][2]string{
		{"年级", "2026级"}, {"状态", "在籍"}, {"学历", "硕士"}, {"专业", "虚构专业"},
		{"班级", "虚构班级"}, {"录取类型", "非定向"}, {"性别", "女"}, {"民族", "汉族"},
		{"住宿类型", "on_campus"}, {"身份证号码", "00000020000102001X"},
	}
	seen := map[string]bool{}
	for _, label := range rows[0] {
		seen[headerKey(label)] = true
	}
	for _, item := range defaults {
		key := headerKey(item[0])
		if seen[key] || key == "accommodation_type" && seen["campus"] {
			continue
		}
		rows[0] = append(rows[0], item[0])
		for index := range rows[1:] {
			rows[index+1] = append(rows[index+1], item[1])
		}
	}
	file, err := xlsxFile("students.xlsx", rows)
	if err != nil {
		t.Fatal(err)
	}
	return file.Content
}

func TestXLSXUsesFixedFieldsAndTextEnums(t *testing.T) {
	// 原始示例列名在边界归一化；只有明确列出的十四个字段进入敏感信息。
	rows, err := parseWorkbook(
		studentImportFixture(t, "\ufeff学号,姓名,性别,班级职务,是否有宗教信仰,是否独生子女,住宿园区,身份证号码,出生日期\r\n"+
			"00000001,虚构学生,女,班长,否,是,校外住宿,FICTIONAL000000001,2000-02-31\r\n"),
	)
	if err != nil || len(rows) != 1 {
		t.Fatalf("固定模板解析失败：%v", err)
	}
	row := rows[0]
	if row.Input.StudentNo != "00000001" ||
		row.Input.BirthDate != "" ||
		row.Input.ArrivalMonth != "" ||
		row.Input.PreviousSchoolOrEmployer != "" ||
		row.Input.PreviousGraduationMonth != "" ||
		row.Input.Gender != "女" ||
		row.Input.ClassPosition != "班长" ||
		row.Input.AccommodationType != "off_campus" ||
		row.Sensitive["has_religion"] != "no" ||
		row.Sensitive["only_child"] != "yes" {
		t.Fatal("示例列、前导零或内置枚举未正确转换")
	}
	for _, content := range []string{
		"学号,姓名,workspace_id\n0001,虚构学生,other\n",
		"学号,姓名,性别,gender\n0001,虚构学生,女,女\n",
		"学号,姓名\n0001,虚构学生\n0001,另一虚构学生\n",
		"学号,姓名,是否有宗教信仰\n0001,虚构学生,unexpected\n",
		"学号,姓名,状态\n0001,虚构学生,超过八个字符的状态值\n",
		"学号,姓名\n" + strings.Repeat("1", 33) + ",虚构学生\n",
	} {
		if _, err := parseWorkbook(studentImportFixture(t, content)); err == nil {
			t.Fatal("未知列、重复列或无效字段被导入")
		}
	}
}

func TestXLSXAccommodationType(t *testing.T) {
	for _, test := range []struct {
		input, value, stored string
	}{
		{"校内", "on_campus", studentConstants.AccommodationOnCampus},
		{"off_campus", "off_campus", studentConstants.AccommodationOffCampus},
		{"出国联培", "joint_abroad", studentConstants.AccommodationJointAbroad},
		{"joint_abroad", "joint_abroad", studentConstants.AccommodationJointAbroad},
	} {
		rows, err := parseWorkbook(
			studentImportFixture(t, "学号,姓名,住宿园区,住宿类型\n0001,虚构学生,虚构园区,"+test.input+"\n"),
		)
		if err != nil || len(rows) != 1 {
			t.Fatalf("住宿类型 %q 导入失败：%v", test.input, err)
		}
		record, err := recordFromInput(rows[0].Input)
		if err != nil || record.AccommodationType != test.stored || record.Campus != "虚构园区" ||
			studentInfo(record).AccommodationType != test.value {
			t.Fatalf("住宿类型 %q 未正确转换或覆盖了住宿园区：%v", test.input, err)
		}
	}
	for _, value := range []string{"", "unknown", "未知", "未填写／未知"} {
		_, err := parseWorkbook(studentImportFixture(t,
			"学号,姓名,住宿类型\n0001,虚构学生,"+value+"\n"))
		if err == nil || !strings.Contains(err.Error(), "住宿类型") {
			t.Fatalf("未填写的住宿类型 %q 未被拒绝：%v", value, err)
		}
	}
}

func TestSensitiveFieldsCannotSelectBasicOrArbitraryColumns(t *testing.T) {
	for _, values := range [][]studentModels.FieldValue{
		{{Field: "phone", Value: "虚构电话"}},
		{{Field: "gender", Value: "女"}},
		{{Field: "workspace_id", Value: "other"}},
		{{Field: "identity_number", Value: "1"}, {Field: "identity_number", Value: "2"}},
		{{Field: "identity_number", Value: strings.Repeat("1", 19)}},
	} {
		if _, err := validateSensitive(values); err == nil {
			t.Fatal("敏感修改越过固定字段目录")
		}
	}
	for _, key := range []string{"only_child", "single_parent", "orphan", "student_loan", "registered_poverty", "has_religion", "accommodation_type"} {
		field, ok := studentConstants.Lookup(key)
		if !ok || field.Type != "enum" {
			t.Fatal("固定枚举缺少类型声明")
		}
		for _, option := range field.Options {
			code, ok := studentConstants.EnumCode(key, option.Value)
			if !ok || studentConstants.EnumValue(key, code) != option.Value {
				t.Fatal("文本枚举和英文协议不一致")
			}
		}
	}
}

func TestAcademicFieldsAndDormitoryHeaders(t *testing.T) {
	for _, header := range []string{"宿舍楼", "楼号"} {
		rows, err := parseWorkbook(
			studentImportFixture(t, "学号,姓名,年级,状态,学历,"+header+
				"\n00000009,虚构学生,2026级,自定义状态,博士,1号楼\n"),
		)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 || rows[0].Input.Grade != "2026级" ||
			rows[0].Input.Status != "自定义状态" ||
			rows[0].Input.EducationLevel != "博士" ||
			rows[0].Input.Building != "1号楼" {
			t.Fatal("年级、状态、学历或宿舍楼未进入基础字段")
		}
	}
}

func TestPreviousEducationXLSXRoundTrip(t *testing.T) {
	// 新字段保持年月精度，其他可选字段允许留空。
	rows, err := parseWorkbook(
		studentImportFixture(t, "学号,姓名,毕（肄）业学校及单位,来校时间,毕（肄）业时间\n"+
			"000010,虚构学生甲,虚构大学,2026-09,2026-06\n"+
			"000011,虚构学生乙,,,\n"),
	)
	if err != nil || len(rows) != 2 {
		t.Fatalf("既往教育信息导入失败：%v", err)
	}
	if rows[0].Input.PreviousSchoolOrEmployer != "虚构大学" ||
		rows[0].Input.ArrivalMonth != "2026-09" ||
		rows[0].Input.PreviousGraduationMonth != "2026-06" || rows[0].Sensitive["identity_number"] != "00000020000102001X" {
		t.Fatal("既往教育信息未作为基础字段保留原始年月")
	}
	students := make([]studentModels.Student, 0, len(rows))
	for _, row := range rows {
		record, err := recordFromInput(row.Input)
		if err != nil {
			t.Fatal(err)
		}
		students = append(students, studentModels.Student{
			BasicRecord: record, IdentityNumber: row.Sensitive["identity_number"],
		})
	}

	// 含身份证号码的导出可重新导入，不改变学校单位、年月或可选空值。
	file, err := xlsxFile(
		"students.xlsx",
		studentExportRecords(students, transferFields(true), true),
	)
	if err != nil {
		t.Fatal(err)
	}
	imported, err := parseWorkbook(file.Content)
	if err != nil || len(imported) != len(rows) {
		t.Fatalf("既往教育信息导出后无法重新导入：%v", err)
	}
	for index, row := range rows {
		if imported[index].Input != row.Input {
			t.Fatal("导入导出过程中既往教育信息或空值发生变化")
		}
	}
}

func TestStudentMonthValidation(t *testing.T) {
	for _, key := range []string{"arrival_month", "previous_graduation_month"} {
		field, _ := studentConstants.Lookup(key)
		for _, value := range []string{"", "1900-01", "2026-09", "9999-12"} {
			if err := validateValue(field, value); err != nil {
				t.Fatalf("有效年月 %q 被拒绝：%v", value, err)
			}
		}
		for _, value := range []string{
			"0000-01", "1899-12", "10000-01", "2026-00", "2026-13",
			"2026-9", "2026-09-01", "2026.09", "2026/09",
		} {
			_, err := parseWorkbook(
				studentImportFixture(t, "学号,姓名,"+key+"\n000012,虚构学生,"+value+"\n"),
			)
			if err == nil || !strings.Contains(err.Error(), field.Label) {
				t.Fatalf("无效年月 %q 未在导入时报告对应字段错误：%v", value, err)
			}
		}
	}
}

func TestSensitiveReplacementRequiresEveryField(t *testing.T) {
	values := []studentModels.FieldValue{}
	for _, field := range studentConstants.Fields {
		if field.Group == studentConstants.Sensitive {
			value := ""
			if field.Key == "identity_number" {
				value = "00000020000102001X"
			}
			values = append(values, studentModels.FieldValue{Field: field.Key, Value: value})
		}
	}
	if _, err := validateSensitiveReplacement(values); err != nil {
		t.Fatal("完整字段允许清空身份证号码以外的可选内容")
	}
	if _, err := validateSensitiveReplacement(values[:len(values)-1]); err == nil {
		t.Fatal("敏感编辑接受了缺失字段的部分更新")
	}
	for index := range values {
		if values[index].Field == "identity_number" {
			values[index].Value = " \t "
		}
	}
	if _, err := validateSensitiveReplacement(values); err == nil {
		t.Fatal("敏感编辑接受了清空身份证号码")
	}
}

func TestXLSXTemplateInstructionsDoNotConsumeStudentRows(t *testing.T) {
	file, err := xlsxFile("students.xlsx", [][]string{
		{"学号", "姓名", "是否有宗教信仰"},
		{"# 0001", "虚构学生", "是"},
		{"0002", "虚构学生乙", "否"},
	}, "说明不能作为人员记录导入")
	if err != nil {
		t.Fatal(err)
	}
	records, err := spreadsheettest.Rows(file.Content)
	if err != nil {
		t.Fatal(err)
	}
	var table strings.Builder
	writer := csv.NewWriter(&table)
	if err := writer.WriteAll(records); err != nil {
		t.Fatal(err)
	}
	rows, err := parseWorkbook(studentImportFixture(t, table.String()))
	if err != nil || len(rows) != 2 || rows[0].Input.StudentNo != "# 0001" ||
		rows[0].Sensitive["has_religion"] != "yes" {
		t.Fatal("填写说明或以井号开头的数据行解析错误")
	}
}

func TestReligionColumnsAreSensitiveTextWithNoStartYearFilter(t *testing.T) {
	rows, err := parseWorkbook(
		studentImportFixture(
			t,
			"学号,姓名,宗教信仰类别,宗教信仰标识,宗教信仰起始年份,家庭成员宗教信仰情况\n000009,虚构学生,天主教,疑似,2020年,虚构家庭成员情况\n",
		),
	)
	if err != nil || len(rows) != 1 || rows[0].Sensitive["religion_start_year"] != "2020年" ||
		rows[0].Sensitive["religion_category"] != "天主教" ||
		rows[0].Sensitive["family_religion"] != "虚构家庭成员情况" {
		t.Fatal("新增宗教信仰列没有按敏感文本导入")
	}
	field, ok := studentConstants.Lookup("religion_start_year")
	if !ok || field.Group != studentConstants.Sensitive || field.Enumerable {
		t.Fatal("宗教信仰起始年份不应作为筛选维度")
	}
}
