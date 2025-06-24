package cmd

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	activator "github.com/yourusername/activator/pkg"
)

var logLevel string
var kubeconfig string

var rootCmd = &cobra.Command{
	Use:   "activator",
	Short: "Kubernetes preview environment activator",
	Run: func(cmd *cobra.Command, args []string) {
		level, err := zerolog.ParseLevel(logLevel)
		if err != nil {
			log.Fatal().Err(err).Msg("Invalid log level")
			os.Exit(1)
		}
		configureLogger(level)
		activator.Start(kubeconfig)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Set log level: trace, debug, info, warn, error")
	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file (if empty, use in-cluster config)")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal().Err(err).Msg("Command failed")
		os.Exit(1)
	}
}

func configureLogger(level zerolog.Level) {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	zerolog.SetGlobalLevel(level)
	if level == zerolog.TraceLevel {
		zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
			return fmt.Sprintf("%s:%d", file, line)
		}
		zerolog.CallerFieldName = "caller"
		log.Logger = log.Output(zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: "2006-01-02 15:04:05.000",
			PartsOrder: []string{
				zerolog.TimestampFieldName,
				zerolog.LevelFieldName,
				zerolog.CallerFieldName,
				zerolog.MessageFieldName,
			},
		}).With().Caller().Logger()
	} else if level == zerolog.DebugLevel {
		log.Logger = log.Output(zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: "2006-01-02 15:04:05.000",
			PartsOrder: []string{
				zerolog.TimestampFieldName,
				zerolog.LevelFieldName,
				zerolog.MessageFieldName,
			},
		})
	} else {
		log.Logger = log.Output(os.Stderr)
	}
}
