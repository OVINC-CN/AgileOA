package cmd

import (
	"github.com/spf13/cobra"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/crond/worker"
)

var workerCmd = &cobra.Command{
	Use: "worker", Short: "启动后台工作进程", Args: cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		// 直接使用已初始化的工作进程资源。
		return worker.Serve(command.Context())
	},
}
