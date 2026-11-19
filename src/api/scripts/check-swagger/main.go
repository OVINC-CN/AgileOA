package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
)

type schema struct {
	Description string            `json:"description"`
	Properties  map[string]schema `json:"properties"`
}

type operation struct {
	Responses map[string]json.RawMessage `json:"responses"`
}

type document struct {
	Paths       map[string]map[string]operation `json:"paths"`
	Definitions map[string]schema               `json:"definitions"`
}

func main() {
	// 校验失败时供本地检查与 CI 共同阻止提交。
	if err := check(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Swagger 路由和全部响应字段说明校验通过")
}

func check() error {
	// 加载生成文档，逐一检查字段描述。
	content, err := os.ReadFile("internal/httpd/docs/swagger.json")
	if err != nil {
		return err
	}
	var doc document
	if err := json.Unmarshal(content, &doc); err != nil {
		return err
	}
	for modelName, model := range doc.Definitions {
		for name, property := range model.Properties {
			if strings.TrimSpace(property.Description) == "" {
				return fmt.Errorf("文档字段缺少说明：%s.%s", modelName, name)
			}
		}
	}

	// 从集中路由文件解析全部业务路由，不将文档资源当作业务接口。
	file, err := parser.ParseFile(token.NewFileSet(), "internal/httpd/routers.go", nil, 0)
	if err != nil {
		return err
	}
	var routeError error
	routeCount := 0
	groups := map[string]string{}
	ast.Inspect(file, func(node ast.Node) bool {
		// 解析集中注册的 Echo Group 前缀，支持嵌套分组。
		if assignment, ok := node.(*ast.AssignStmt); ok && len(assignment.Lhs) == 1 &&
			len(assignment.Rhs) == 1 {
			name, named := assignment.Lhs[0].(*ast.Ident)
			group, called := assignment.Rhs[0].(*ast.CallExpr)
			if named && called && len(group.Args) > 0 {
				selector, selected := group.Fun.(*ast.SelectorExpr)
				literal, literalPath := group.Args[0].(*ast.BasicLit)
				if selected && selector.Sel.Name == "Group" && literalPath {
					parent, _ := selector.X.(*ast.Ident)
					prefix, err := strconv.Unquote(literal.Value)
					if err == nil && parent != nil {
						groups[name.Name] = groups[parent.Name] + prefix
					}
				}
			}
		}
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		method := strings.ToLower(selector.Sel.Name)
		switch method {
		case "get", "post", "put", "patch", "delete", "head", "options":
		default:
			return true
		}
		literal, ok := call.Args[0].(*ast.BasicLit)
		if !ok {
			routeError = fmt.Errorf("路由路径必须能被文档检查解析")
			return false
		}
		path, err := strconv.Unquote(literal.Value)
		if err != nil {
			routeError = err
			return false
		}
		if path == "/v1/swagger/*" {
			return true
		}
		if receiver, ok := selector.X.(*ast.Ident); ok {
			path = groups[receiver.Name] + path
		}
		segments := strings.Split(path, "/")
		for index, segment := range segments {
			if strings.HasPrefix(segment, ":") {
				segments[index] = "{" + strings.TrimPrefix(segment, ":") + "}"
			}
		}
		path = strings.Join(segments, "/")
		routeCount++
		op, found := doc.Paths[path][method]
		expectedStatuses := []string{"200"}
		// 附件下载仅返回 JSON 地址，不提供重定向模式。
		if method == "get" && strings.HasSuffix(path, "/download") {
			expectedStatuses = []string{"200", "400"}
		}
		// 申报与审批接口显式声明身份、节点、版本及服务失败契约。
		if strings.Contains(path, "/project-applications") ||
			strings.Contains(path, "/project-reviews") {
			expectedStatuses = []string{"200", "400", "401", "403", "404", "409", "503"}
		}
		if path == "/v1/workspaces/{workspace_id}/project-applications/export" {
			expectedStatuses = []string{"200", "400", "401", "403", "404", "503"}
		}
		if (strings.Contains(path, "/project-applications/") || strings.Contains(path, "/project-reviews/")) &&
			strings.HasSuffix(path, "/download") {
			expectedStatuses = []string{"200", "400", "401", "403", "404", "409", "503"}
		}
		if path == "/v1/workspaces/{workspace_id}/projects/approval-options" ||
			path == "/v1/workspaces/{workspace_id}/student-service/projects" {
			expectedStatuses = []string{"200", "400"}
		}
		if method == "post" && strings.HasSuffix(path, "/score-preview") {
			expectedStatuses = []string{"200", "400", "401", "403", "404", "409", "503"}
		}
		// 项目配置及评分显式声明配置校验、版本冲突与引用保护状态码。
		if (method == "post" &&
			(path == "/v1/workspaces/{workspace_id}/projects" ||
				path == "/v1/workspaces/{workspace_id}/project-participations")) ||
			(method == "put" &&
				(path == "/v1/workspaces/{workspace_id}/projects/{id}" ||
					path == "/v1/workspaces/{workspace_id}/project-participations/{id}")) {
			expectedStatuses = []string{"200", "400", "409"}
		}
		// 配置截止时间时同步入队，项目保存及导入须声明队列不可用状态。
		if (method == "post" && path == "/v1/workspaces/{workspace_id}/projects") ||
			(method == "put" && path == "/v1/workspaces/{workspace_id}/projects/{id}") {
			expectedStatuses = []string{"200", "400", "409", "503"}
		}
		if method == "post" && path == "/v1/workspaces/{workspace_id}/projects/import" {
			expectedStatuses = []string{"200", "400", "503"}
		}
		if method == "get" &&
			path == "/v1/workspaces/{workspace_id}/project-participations/detail-names" {
			expectedStatuses = []string{"200", "400", "404"}
		}
		if method == "post" && path == "/v1/login" {
			expectedStatuses = []string{"200", "401"}
		}
		if path == "/v1/login-email-verify" {
			expectedStatuses = []string{"200", "400", "401", "503"}
		}
		if path == "/v1/login-email" || path == "/v1/student-login-email" {
			expectedStatuses = []string{"200", "400", "401", "403", "429", "503"}
		}
		if path == "/v1/callback/email" {
			expectedStatuses = []string{"200", "400", "401", "503"}
		}
		if path == "/v1/workspaces/{workspace_id}/student-service" {
			expectedStatuses = []string{"200", "400", "401", "403", "503"}
		}
		if (method == "post" && path == "/v1/system/workspaces") ||
			(method == "put" && path == "/v1/system/workspaces/{workspace_id}") {
			expectedStatuses = []string{"200", "409"}
		}
		if path == "/v1/mcp" {
			expectedStatuses = []string{
				"200", "202", "400", "401", "403", "404", "405", "413", "415", "500", "503", "504",
			}
		}
		if path == "/v1/system/users/{id}/api-token" {
			expectedStatuses = []string{"200", "400", "401", "403", "404", "503"}
			if method == "put" {
				expectedStatuses = append(expectedStatuses, "409")
			}
		}
		if method == "post" && path == "/v1/notifications/read" {
			expectedStatuses = []string{"200", "400", "404"}
		}
		if method == "get" && (path == "/v1/notifications" || path == "/v1/home/calendar") {
			expectedStatuses = []string{"200", "400"}
		}
		// 学生通知独立校验绑定身份、空间参数及审计可用性。
		if strings.HasPrefix(path, "/v1/workspaces/{workspace_id}/student-service/notifications") {
			expectedStatuses = []string{"200", "400", "401", "403", "503"}
			if method == "post" {
				expectedStatuses = append(expectedStatuses, "404")
			}
		}
		if path == "/v1/workspaces/{workspace_id}/cpc-people" && method == "post" {
			expectedStatuses = []string{"200", "409"}
		}
		if path == "/v1/workspaces/{workspace_id}/cpc-people/{id}" {
			if method == "get" {
				expectedStatuses = []string{"200", "404"}
			} else {
				expectedStatuses = []string{"200", "409"}
			}
		}
		if method == "get" &&
			(path == "/v1/workspaces/{workspace_id}/cpc-people/timeline-view/{template_id}" ||
				path == "/v1/workspaces/{workspace_id}/cpc-people/timeline-view/{template_id}/nodes/{node_id}" ||
				path == "/v1/workspaces/{workspace_id}/cpc-people/timeline-view/{template_id}/nodes/{node_id}/statistics") {
			expectedStatuses = []string{"200", "404"}
		}
		if method == "post" &&
			path == "/v1/workspaces/{workspace_id}/cpc-people/{id}/timeline/bind" {
			expectedStatuses = []string{"200", "409"}
		}
		if (method == "get" &&
			path == "/v1/workspaces/{workspace_id}/cpc-people/{id}/timeline/transfer-options") ||
			(method == "post" &&
				path == "/v1/workspaces/{workspace_id}/cpc-people/{id}/timeline/transfer-preview") {
			expectedStatuses = []string{"200", "400", "409"}
		}
		if method == "get" &&
			path == "/v1/workspaces/{workspace_id}/cpc-people/{id}/timeline/transfer-review" {
			expectedStatuses = []string{"200", "400"}
		}
		if method == "post" &&
			(path == "/v1/workspaces/{workspace_id}/cpc-people/{id}/timeline/transfer" ||
				path == "/v1/workspaces/{workspace_id}/cpc-people/{id}/timeline/transfer-review" ||
				path == "/v1/workspaces/{workspace_id}/cpc-people/{id}/timeline/transfer-reject" ||
				path == "/v1/workspaces/{workspace_id}/cpc-people/{id}/timeline/transfer-cancel") {
			expectedStatuses = []string{"200", "400", "409"}
		}
		if method == "put" &&
			path == "/v1/workspaces/{workspace_id}/cpc-people/{id}/timeline/nodes/{node_id}" {
			expectedStatuses = []string{"200", "400", "409"}
		}
		if method == "post" &&
			path == "/v1/workspaces/{workspace_id}/cpc-people/{id}/timeline/nodes/{node_id}/reset" {
			expectedStatuses = []string{"200", "400", "409", "503"}
		}
		if method == "post" &&
			path == "/v1/workspaces/{workspace_id}/cpc-people/{id}/timeline/nodes/{node_id}/skip" {
			expectedStatuses = []string{"200", "400", "409"}
		}
		if method == "post" &&
			path == "/v1/workspaces/{workspace_id}/cpc-timeline-templates/{id}/clone" {
			expectedStatuses = []string{"200", "400", "403", "404", "409", "503"}
		}
		if !found || len(op.Responses) != len(expectedStatuses) {
			routeError = fmt.Errorf("路由响应文档与约定状态码不一致：%s", path)
		}
		for _, status := range expectedStatuses {
			if op.Responses[status] == nil {
				routeError = fmt.Errorf("路由响应文档缺少约定状态码：%s %s", path, status)
			}
		}
		return true
	})
	if routeError != nil {
		return routeError
	}
	documentedOperations := 0
	for _, methods := range doc.Paths {
		documentedOperations += len(methods)
	}
	if routeCount == 0 || routeCount != documentedOperations {
		return fmt.Errorf("文档与实际业务路由数量不一致")
	}
	return nil
}
