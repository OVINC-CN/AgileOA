package httpd

import (
	_ "embed"

	"github.com/swaggo/swag"
)

//go:embed docs/swagger.json
var swaggerDocumentJSON string

type swaggerDocument struct{}

func init() {
	// 注册内嵌文档，不生成包含软件包注释的 Go 文档文件。
	swag.Register(swag.Name, swaggerDocument{})
}

// ReadDoc 返回生成后的 Swagger JSON。
func (swaggerDocument) ReadDoc() string {
	// 文档生成和检查由 Makefile 统一管理。
	return swaggerDocumentJSON
}
