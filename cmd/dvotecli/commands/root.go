package commands

import (
	"fmt"
	"os"

	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:              "dvotecli",
	Short:            "dvote command line interface.",
	PersistentPreRun: setColor,
}

var au aurora.Aurora
var colorize bool
var host string
var privKey string

func init() {
	rootCmd.PersistentFlags().BoolVarP(&colorize, "color", "c", true, "colorize output")
	rootCmd.PersistentFlags().StringVarP(&host, "host", "", "ws://127.0.0.1:9090/dvote", "host to connect to")
	rootCmd.PersistentFlags().StringVarP(&privKey, "key", "", "", "private key for signature (leave blank for auto-generate)")
	au = aurora.NewAurora(true)
}

// Execute ...
func Execute() {

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func setColor(cmd *cobra.Command, args []string) {
	au = aurora.NewAurora(colorize)
}
