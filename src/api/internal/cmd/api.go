package cmd

import (
	"github.com/spf13/cobra"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/httpd"
)

var apiCmd = &cobra.Command{
	Use: "api", Short: "启动 HTTP API 服务", Args: cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		// 配置和数据库已由各自包的 init 完成初始化。
		if err := userSvc.Bootstrap(command.Context()); err != nil {
			return err
		}
		return httpd.Serve(command.Context())
	},
}
