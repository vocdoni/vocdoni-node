package commands

import (
	"os"

	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
)

const MaxListIterations = 256

var rootCmd = &cobra.Command{
	Use:   "dvotecli",
	Short: "dvote command line interface.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		au = aurora.NewAurora(opt.colorize)
	},
	SilenceUsage: true,
}

var (
	au  aurora.Aurora
	opt options
)

func init() {
	rootCmd.PersistentFlags().BoolVarP(
		&opt.colorize, "color", "c", true,
		"colorize output")
	rootCmd.PersistentFlags().StringVarP(
		&opt.host, "host", "", "ws://127.0.0.1:9090/dvote",
		"host to connect to")
	rootCmd.PersistentFlags().StringVarP(
		&opt.privKey, "key", "", "",
		"private key for signature (leave blank for auto-generate)")
}

// Execute ...
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
