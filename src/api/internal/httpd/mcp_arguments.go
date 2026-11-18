package httpd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

func mcpQueryURL(tool mcpQueryTool, raw json.RawMessage) (*url.URL, error) {
	// 只做协议类型及固定字段目录校验，业务约束仍交由原路由处理。
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	if !json.Valid(raw) {
		return nil, errors.New("查询参数必须是 JSON 对象")
	}
	var values map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&values); err != nil || values == nil {
		return nil, errors.New("查询参数必须是 JSON 对象")
	}
	if len(values) > len(tool.Parameters) {
		return nil, errors.New("查询参数包含未登记字段")
	}
	known := make(map[string]bool, len(tool.Parameters))
	path := tool.Path
	query := make(url.Values)
	for _, parameter := range tool.Parameters {
		known[parameter.Name] = true
		value, exists := values[parameter.Name]
		if !exists {
			if parameter.Required || parameter.In == "path" {
				return nil, fmt.Errorf("缺少必要查询参数：%s", parameter.Name)
			}
			continue
		}
		items, err := mcpParameterValues(parameter, value)
		if err != nil {
			return nil, fmt.Errorf("查询参数类型无效：%s", parameter.Name)
		}
		if parameter.In == "path" {
			if len(items) != 1 || !mcpPathValue(items[0]) {
				return nil, fmt.Errorf("查询路径参数无效：%s", parameter.Name)
			}
			path = strings.ReplaceAll(path, "{"+parameter.Name+"}", items[0])
			continue
		}
		for _, item := range items {
			query.Add(parameter.Name, item)
		}
	}
	for name := range values {
		if !known[name] {
			return nil, errors.New("查询参数包含未登记字段")
		}
	}
	return &url.URL{Path: path, RawQuery: query.Encode()}, nil
}

func mcpParameterValues(parameter mcpParameter, value any) ([]string, error) {
	if parameter.Type != "array" {
		item, err := mcpScalarValue(parameter.Type, value)
		return []string{item}, err
	}
	if parameter.CollectionFormat != "multi" {
		return nil, errors.New("不支持该数组查询格式")
	}
	array, ok := value.([]any)
	if !ok {
		return nil, errors.New("查询参数必须是数组")
	}
	itemType, _ := parameter.Items["type"].(string)
	items := make([]string, 0, len(array))
	for _, item := range array {
		text, err := mcpScalarValue(itemType, item)
		if err != nil {
			return nil, err
		}
		items = append(items, text)
	}
	return items, nil
}

func mcpScalarValue(kind string, value any) (string, error) {
	switch typed := value.(type) {
	case string:
		if kind == "string" {
			return typed, nil
		}
	case bool:
		if kind == "boolean" {
			return strconv.FormatBool(typed), nil
		}
	case json.Number:
		if kind == "number" {
			return typed.String(), nil
		}
		if kind == "integer" && !strings.ContainsAny(typed.String(), ".eE") {
			return typed.String(), nil
		}
	}
	return "", errors.New("查询参数类型无效")
}

func mcpPathValue(value string) bool {
	// 禁止路径分隔、转义及导航片段，防止参数改变已登记的目标路由。
	return value != "" && value != "." && value != ".." &&
		!strings.ContainsAny(value, "/\\?#%{}") && !strings.ContainsFunc(value, unicode.IsControl)
}
