package cmd

import (
	"github.com/spf13/cobra"
	"github.com/wenzapen/crawler/cmd/worker"
	"github.com/wenzapen/crawler/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "print version",
	Long:  "print version",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		version.Printer()
	},
}

func Execute() {
	var rootCmd = &cobra.Command{Use: "crawler"}
	rootCmd.AddCommand(master.MasterCmd, worker.WorkerCmd, versionCmd)
	rootCmd.Execute()

}
