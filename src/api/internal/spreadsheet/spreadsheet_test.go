package spreadsheet

import (
	"bytes"
	"encoding/base64"
	"reflect"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestEncodePreservesTextCellsAndTemplateColumns(t *testing.T) {
	records := [][]string{
		{"学号", "证件号码", "月份", "日期", "计数", "是否", "公式文本", "空值"},
		{"000001", "123456789012345678", "2026-09", "2026-11-06", "0", "false", "=1+1", ""},
		{"'原始文本", "+SUM(1,1)", "-1", "@名称", "多行\n文本", "含,分隔符", "\"引号\"", ""},
	}
	content, err := Encode(records, "# 月份使用 yyyy-mm，日期使用 yyyy-mm-dd。")
	if err != nil {
		t.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString(content)
	if err != nil || !bytes.HasPrefix(data, []byte("PK")) {
		t.Fatal("导出内容不是真实 XLSX")
	}
	book, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = book.Close() })
	if !reflect.DeepEqual(book.GetSheetList(), []string{dataSheet, notesSheet}) {
		t.Fatal("数据与填写说明没有分别保存")
	}
	for rowIndex, row := range records {
		for columnIndex, value := range row {
			cell, err := excelize.CoordinatesToCellName(columnIndex+1, rowIndex+1)
			if err != nil {
				t.Fatal(err)
			}
			actual, err := book.GetCellValue(dataSheet, cell)
			if err != nil || actual != value {
				t.Fatalf("单元格 %s 文本变化：%q，%v", cell, actual, err)
			}
			kind, err := book.GetCellType(dataSheet, cell)
			if err != nil || (kind != excelize.CellTypeSharedString &&
				kind != excelize.CellTypeInlineString) {
				t.Fatalf("单元格 %s 没有按文本存储：%v，%v", cell, kind, err)
			}
			formula, err := book.GetCellFormula(dataSheet, cell)
			if err != nil || formula != "" {
				t.Fatalf("单元格 %s 被当作公式", cell)
			}
			styleID, err := book.GetCellStyle(dataSheet, cell)
			if err != nil {
				t.Fatal(err)
			}
			style, err := book.GetStyle(styleID)
			if err != nil || style.NumFmt != textFormat {
				t.Fatalf("单元格 %s 没有文本格式", cell)
			}
		}
	}
	columnStyle, err := book.GetColStyle(dataSheet, "A")
	if err != nil || columnStyle == 0 {
		t.Fatal("模板后续填写的单元格未继承文本格式")
	}
	rows, err := Read(content, Limits{MaxBytes: 1 << 20, MaxRows: 2, MaxColumns: 8})
	if err != nil || !reflect.DeepEqual(rows, records) {
		t.Fatalf("XLSX 导入未保留文本、空值或前导零：%v", err)
	}
}

func TestReadRejectsInvalidOrOversizedWorkbooks(t *testing.T) {
	content, err := Encode([][]string{{"学号", "姓名"}, {"0001", "虚构学生"}, {"0002", "虚构学生"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name    string
		content string
		limits  Limits
	}{
		{"无效文件", "AAAA", Limits{1 << 20, 2, 2}},
		{"无效 Base64", "???", Limits{1 << 20, 2, 2}},
		{"拒绝 CSV", "学号,姓名\r\n0001,虚构学生\r\n", Limits{1 << 20, 2, 2}},
		{"拒绝 Base64 CSV", base64.StdEncoding.EncodeToString([]byte("学号,姓名")), Limits{1 << 20, 2, 2}},
		{"文件过大", content, Limits{1, 2, 2}},
		{"解压过大", content, Limits{10000, 2, 2}},
		{"行数过多", content, Limits{1 << 20, 1, 2}},
		{"列数过多", content, Limits{1 << 20, 2, 1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Read(test.content, test.limits); err == nil {
				t.Fatal("无效或超限文件通过导入校验")
			}
		})
	}
	if _, err := Encode([][]string{{strings.Repeat("中", excelize.TotalCellChars+1)}}); err == nil {
		t.Fatal("超长文本被静默截断")
	}
}

func TestReadNormalizesWindowsHeaderMarkersOnly(t *testing.T) {
	content, err := Encode([][]string{
		{"\ufeff学号\u00a0", "\u200b 姓名\u3000", "日期\r\n", " ", ""},
		{"00001", " 原始姓名 ", "2026-11-06", "", ""},
	})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := Read(content, Limits{MaxBytes: 1 << 20, MaxRows: 1, MaxColumns: 3})
	if err != nil || !reflect.DeepEqual(rows, [][]string{
		{"学号", "姓名", "日期"}, {"00001", " 原始姓名 ", "2026-11-06"},
	}) {
		t.Fatalf("表头标记没有清理或误改了正文：%v", err)
	}
}
