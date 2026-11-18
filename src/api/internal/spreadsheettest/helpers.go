package spreadsheettest

import (
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/spreadsheet"
)

// Fixture 将紧凑的测试表格转为真实 XLSX，不用于业务文件解析。
func Fixture(t testing.TB, table string) string {
	t.Helper()
	reader := csv.NewReader(strings.NewReader(table))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	content, err := spreadsheet.Encode(rows)
	if err != nil {
		t.Fatal(err)
	}
	return content
}

// Rows 读取测试导出数据，补齐 XLSX 省略的末尾空单元格。
func Rows(content string) ([][]string, error) {
	return spreadsheet.Read(content, spreadsheet.Limits{
		MaxBytes: 1 << 24, MaxRows: 100000, MaxColumns: excelize.MaxColumns,
	})
}

// Text 读取全部工作表，供字段隔离、说明及范围断言使用。
func Text(t testing.TB, content string) string {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = book.Close() }()
	lines := []string{}
	for _, sheet := range book.GetSheetList() {
		rows, err := book.GetRows(sheet)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			lines = append(lines, strings.Join(row, ","))
		}
	}
	return strings.Join(lines, "\n")
}
