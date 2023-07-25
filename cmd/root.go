package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Env and params from: https://carolynvanslyck.com/blog/2020/08/sting-of-the-viper/
// Code https://github.com/carolynvs/stingoftheviper/blob/main/main.go#L11

const (
	// The environment variable prefix of all environment variables bound to our command line flags.
	// For example, --number is bound to AVAHI_CNAME_NUMBER.
	envPrefix = "AVAHI_CNAME"
)

var (
	logger  *zap.Logger
	verbose bool
)

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enables debug logging")

	cobra.OnInitialize(onInitialize)
}

func onInitialize() {
	zapOptions := []zap.Option{
		zap.AddStacktrace(zapcore.FatalLevel),
		zap.AddCallerSkip(1),
	}
	if !verbose {
		zapOptions = append(zapOptions,
			zap.IncreaseLevel(zap.LevelEnablerFunc(func(l zapcore.Level) bool { return l != zapcore.DebugLevel })),
		)
	}

	var err error
	logger, err = zap.NewDevelopment(zapOptions...)
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(-1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "go-avahi-cname",
	Short: "Register a mDNS/DNS-SD alias name for your computer using the Avahi daemon",
	// TODO: Write long
	Long: ``,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// You can bind cobra and viper in a few locations, but PersistencePreRunE on the root command works well
		return initializeConfig(cmd)
	},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(123)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logger.Fatal("We bowled a googly", zap.Error(err))
		os.Exit(-1)
	}
}

func initializeConfig(cmd *cobra.Command) error {
	v := viper.New()

	// When we bind flags to environment variables expect that the
	// environment variables are prefixed, e.g. a flag like --number
	// binds to an environment variable STING_NUMBER. This helps
	// avoid conflicts.
	v.SetEnvPrefix(envPrefix)

	// Environment variables can't have dashes in them, so bind them to their equivalent
	// keys with underscores, e.g. --favorite-color to STING_FAVORITE_COLOR
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	// Bind to environment variables
	// Works great for simple config names, but needs help for names
	// like --favorite-color which we fix in the bindFlags function
	v.AutomaticEnv()

	// Bind the current command's flags to viper
	// bindFlags(cmd, v)

	return nil
}

// Bind each cobra flag to its associated viper configuration (config file and environment variable)
// func bindFlags(cmd *cobra.Command, v *viper.Viper) {
// 	cmd.Flags().VisitAll(func(f *pflag.Flag) {
// 		// Determine the naming convention of the flags when represented in the config file
// 		configName := f.Name
// 		// If using camelCase in the config file, replace hyphens with a camelCased string.
// 		// Since viper does case-insensitive comparisons, we don't need to bother fixing the case, and only need to remove the hyphens.
// 		if replaceHyphenWithCamelCase {
// 			configName = strings.ReplaceAll(f.Name, "-", "")
// 		}

// 		// Apply the viper config value to the flag when the flag is not set and viper has a value
// 		if !f.Changed && v.IsSet(configName) {
// 			val := v.Get(configName)
// 			cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
// 		}
// 	})
// }
