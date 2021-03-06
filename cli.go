package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	logger "github.com/textileio/go-log/v2"
)

var (
	log = logger.Logger("cli")
)

// Flag describes a configuration flag.
type Flag struct {
	Name        string
	DefValue    interface{}
	Description string
}

// ConfigureCLI configures a Viper environment with flags and envs.
func ConfigureCLI(v *viper.Viper, envPrefix string, flags []Flag, flagSet *pflag.FlagSet) {
	v.SetEnvPrefix(envPrefix)
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	for _, flag := range flags {
		switch defval := flag.DefValue.(type) {
		case string:
			flagSet.String(flag.Name, defval, flag.Description)
			v.SetDefault(flag.Name, defval)
		case []string:
			flagSet.StringSlice(flag.Name, defval, flag.Description+"; repeatable")
			v.SetDefault(flag.Name, defval)
		case bool:
			flagSet.Bool(flag.Name, defval, flag.Description)
			v.SetDefault(flag.Name, defval)
		case int:
			flagSet.Int(flag.Name, defval, flag.Description)
			v.SetDefault(flag.Name, defval)
		case int64:
			flagSet.Int64(flag.Name, defval, flag.Description)
			v.SetDefault(flag.Name, defval)
		case uint64:
			flagSet.Uint64(flag.Name, defval, flag.Description)
			v.SetDefault(flag.Name, defval)
		case time.Duration:
			flagSet.Duration(flag.Name, defval, flag.Description)
			v.SetDefault(flag.Name, defval)
		default:
			log.Fatalf("unknown flag type: %T", flag)
		}
		if err := v.BindPFlag(flag.Name, flagSet.Lookup(flag.Name)); err != nil {
			log.Fatalf("binding flag %s: %s", flag.Name, err)
		}
	}
}

// ConfigureLogging configures the default logger with the right setup depending flag/envs.
// If logLevels is not nil, only logLevels values will be configured to Info/Debug depending
// on viper flags. if logLevels is nil, all sub-logs will be configured.
func ConfigureLogging(v *viper.Viper, logLevels []string) error {
	var format logger.LogFormat
	if v.GetBool("log-json") {
		format = logger.JSONOutput
	} else if v.GetBool("log-plaintext") {
		format = logger.PlaintextOutput
	} else {
		format = logger.ColorizedOutput
	}
	logger.SetupLogging(logger.Config{
		Format: format,
		Level:  logger.LevelError,
		Stderr: false,
		Stdout: true,
	})

	logLevel := logger.LevelInfo
	if v.GetBool("log-debug") {
		logLevel = logger.LevelDebug
	}

	if len(logLevels) == 0 {
		logger.SetAllLoggers(logLevel)
		return nil
	}

	mapLevel := make(map[string]logger.LogLevel, len(logLevels))
	for i := range logLevels {
		mapLevel[logLevels[i]] = logLevel
	}

	if err := logger.SetLogLevels(mapLevel); err != nil {
		return fmt.Errorf("set log levels: %s", err)
	}
	return nil
}

// ParseStringSlice returns a single slice of values that may have been set by either repeating
// a flag or using comma separation in a single flag.
// This is used to enable repeated flags as well as env vars that can't be repeated.
// In either case, Viper understands how to write the config entry as a list.
func ParseStringSlice(v *viper.Viper, key string) []string {
	vals := make([]string, 0)
	for _, val := range v.GetStringSlice(key) {
		parts := strings.Split(val, ",")
		for _, p := range parts {
			if p != "" {
				vals = append(vals, p)
			}
		}
	}
	return vals
}

// ExpandEnvVars expands env vars present in the config.
func ExpandEnvVars(v *viper.Viper, settings map[string]interface{}) {
	for name, val := range settings {
		if str, ok := val.(string); ok {
			v.Set(name, os.ExpandEnv(str))
		}
	}
}

// CheckErr ends in a fatal log if err is not nil.
func CheckErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// CheckErrf ends in a fatal log if err is not nil.
func CheckErrf(format string, err error) {
	if err != nil {
		log.Fatalf(format, err)
	}
}

// MarshalConfig marshals a *viper.Viper config to JSON. pretty controls if the
// result is indented or not. It replaces the masked fields with three
// asterisks, if they are present.
func MarshalConfig(v *viper.Viper, pretty bool, maskedFields ...string) ([]byte, error) {
	all := v.AllSettings()
	for _, f := range maskedFields {
		if _, exists := all[f]; exists {
			all[f] = "***"
		}
	}
	if pretty {
		return json.MarshalIndent(all, "", "  ")
	}
	return json.Marshal(all)
}

// HandleInterrupt attempts to cleanup while allowing the user to force stop the process.
func HandleInterrupt(cleanup func()) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	<-quit
	log.Info("Gracefully stopping... (press Ctrl+C again to force)")
	cleanup()
}
