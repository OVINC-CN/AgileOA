package spreadsheet

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"

	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

const (
	// ContentType 是 XLSX 文件的标准媒体类型。
	ContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	dataSheet   = "数据"
	notesSheet  = "填写说明"
	textFormat  = 49
)

// Limits 限制导入文件、解压内容、业务行数和列数。
type Limits struct {
	MaxBytes   int
	MaxRows    int
	MaxColumns int
}

// Encode 将所有单元格写为文本，以 Base64 在 JSON 响应中传输 XLSX。
func Encode(records [][]string, notes ...string) (string, error) {
	// 初始化文本样式，整列应用以便空白模板中的后续输入继续按文本处理。
	book := excelize.NewFile()
	defer func() { _ = book.Close() }()
	if err := book.SetSheetName("Sheet1", dataSheet); err != nil {
		return "", err
	}
	style, err := book.NewStyle(&excelize.Style{NumFmt: textFormat})
	if err != nil {
		return "", err
	}
	if err := writeSheet(book, dataSheet, records, style); err != nil {
		return "", err
	}
	if len(notes) > 0 {
		if _, err := book.NewSheet(notesSheet); err != nil {
			return "", err
		}
		rows := make([][]string, len(notes))
		for index, note := range notes {
			rows[index] = []string{strings.TrimPrefix(note, "# ")}
		}
		if err := writeSheet(book, notesSheet, rows, style); err != nil {
			return "", err
		}
	}

	// 只返回真实工作簿字节，文本原样保留且不作为公式。
	buffer, err := book.WriteToBuffer()
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buffer.Bytes()), nil
}

func writeSheet(book *excelize.File, sheet string, records [][]string, style int) error {
	// 校验工作表容量，禁止组件静默截断过长文本。
	columns := 0
	for rowIndex, row := range records {
		columns = max(columns, len(row))
		for columnIndex, value := range row {
			if !utf8.ValidString(value) {
				return invalid("单元格文本编码无效")
			}
			if utf8.RuneCountInString(value) > excelize.TotalCellChars {
				return invalid("单元格文本超过 XLSX 长度限制，请缩小导出内容")
			}
			cell, err := excelize.CoordinatesToCellName(columnIndex+1, rowIndex+1)
			if err != nil {
				return invalid("导出行数或列数超过 XLSX 限制")
			}
			if err := book.SetCellStr(sheet, cell, value); err != nil {
				return err
			}
		}
	}
	if columns == 0 {
		return nil
	}
	lastColumn, err := excelize.ColumnNumberToName(columns)
	if err != nil {
		return err
	}
	if err := book.SetColStyle(sheet, "A:"+lastColumn, style); err != nil {
		return err
	}
	lastCell, err := excelize.CoordinatesToCellName(columns, len(records))
	if err != nil {
		return err
	}
	return book.SetCellStyle(sheet, "A1", lastCell, style)
}

// Read 读取 Base64 XLSX 的首张工作表，不接受其他文件格式。
func Read(content string, limits Limits) ([][]string, error) {
	// 导入同时遵守业务及全局单文件上限，导出大小不受上传配置影响。
	limits.MaxBytes = int(min(int64(limits.MaxBytes), config.Config.COS.MaxUploadBytes))
	// 大小先于解析校验，避免无界解码或解压。
	if len(content) > base64.StdEncoding.EncodedLen(limits.MaxBytes) {
		return nil, invalid("导入文件超过大小限制")
	}
	data, err := base64.StdEncoding.DecodeString(content)
	if err != nil || len(data) > limits.MaxBytes || !bytes.HasPrefix(data, []byte("PK\x03\x04")) {
		return nil, invalid("请选择有效的 XLSX 文件")
	}
	return readWorkbook(data, limits)
}

func readWorkbook(data []byte, limits Limits) ([][]string, error) {
	// 解压大小沿用文件上限，防止压缩文件绕过现有导入限制。
	book, err := excelize.OpenReader(bytes.NewReader(data), excelize.Options{
		UnzipSizeLimit:    int64(limits.MaxBytes),
		UnzipXMLSizeLimit: int64(limits.MaxBytes),
		ShortDatePattern:  "yyyy-mm-dd",
		LongDatePattern:   "yyyy-mm-dd",
	})
	if err != nil {
		return nil, invalid("XLSX 文件无效或解压内容超过大小限制")
	}
	defer func() { _ = book.Close() }()
	sheets := book.GetSheetList()
	if len(sheets) == 0 {
		return nil, invalid("XLSX 文件缺少数据工作表")
	}
	reader, err := book.Rows(sheets[0])
	if err != nil {
		return nil, invalid("XLSX 数据工作表无效")
	}
	defer func() { _ = reader.Close() }()
	rows := [][]string{}
	for reader.Next() {
		row, err := reader.Columns()
		if err != nil {
			return nil, invalid("XLSX 单元格无效")
		}
		if len(rows) == 0 {
			// 统一清理表头两端的空白、BOM 和零宽空格，正文与固定字段名保持原样。
			for index, label := range row {
				row[index] = strings.TrimFunc(label, func(character rune) bool {
					return unicode.IsSpace(character) || character == '\ufeff' ||
						character == '\u200b'
				})
			}
		}
		// Excel 可能保留尾部仅含格式的空白列，不将这些列视为业务字段。
		width := 0
		if len(rows) > 0 {
			width = len(rows[0])
		}
		for len(row) > width && strings.TrimSpace(row[len(row)-1]) == "" {
			row = row[:len(row)-1]
		}
		if err := checkRow(rows, row, limits); err != nil {
			return nil, err
		}
		if len(rows) > 0 && len(row) < len(rows[0]) {
			// XLSX 省略尾部空单元格，补齐后继续由业务层校验固定字段。
			row = append(row, make([]string, len(rows[0])-len(row))...)
		}
		rows = append(rows, row)
	}
	if reader.Error() != nil {
		return nil, invalid("XLSX 数据工作表无效")
	}
	return rows, nil
}

func checkRow(rows [][]string, row []string, limits Limits) error {
	if len(rows) > limits.MaxRows {
		return invalid("导入行数超过限制")
	}
	if len(row) > limits.MaxColumns || (len(rows) > 0 && len(row) > len(rows[0])) {
		return invalid("文件列数与模板不一致")
	}
	return nil
}

func invalid(message string) error {
	return &response.BusinessError{Status: http.StatusBadRequest, Message: message}
}
