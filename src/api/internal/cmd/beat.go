package cmd

import (
	"github.com/spf13/cobra"

	"github.com/OVINC-CN/AgileOA/src/api/internal/crond/beat"
)

var beatCmd = &cobra.Command{
	Use: "beat", Short: "启动周期任务调度", Args: cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		// 直接使用已初始化的周期调度资源。
		return beat.Serve(command.Context())
	},
}
