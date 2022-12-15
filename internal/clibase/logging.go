package clibase

import (
	"fmt"
	"io"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/writer"
	"github.com/spf13/pflag"
)

const (
	logDefaultLevel   = "info"
	logFlagFormatName = "log-format"
	logFlagLevelName  = "log-level"
	logTextFormatName = "text"
	logJSONFormatName = "json"
)

var (
	logFormats = map[string]log.Formatter{
		logJSONFormatName: &log.JSONFormatter{},
		logTextFormatName: &log.TextFormatter{},
	}
	logDefaultFormat = logTextFormatName

	// ErrorLogInitFailure is the error logged when the initial log configuration setup fails
	ErrorLogInitFailure = fmt.Errorf("failure during logging init")
	// ErrorLogLevelParse is the error logged when the specified log level cannot be parsed
	ErrorLogLevelParse = fmt.Errorf("unable to parse specified log level")
	// ErrorLogUnknownFormat is the error logged when an unrecognized log format is specified
	ErrorLogUnknownFormat = fmt.Errorf("unknown log format specified")
)

func init() {
	// Set the initial logger configuration (used for any messages logged before flags can change the config)
	if err := configureLogging(getLogSettings()); err != nil {
		log.Error(ErrorLogInitFailure.Error())
	}

	log.SetOutput(io.Discard) // Send all logs to nowhere by default
	log.AddHook(&writer.Hook{ // Send logs with level higher than warning to stderr
		Writer: os.Stderr,
		LogLevels: []log.Level{
			log.PanicLevel,
			log.FatalLevel,
			log.ErrorLevel,
			log.WarnLevel,
		},
	})
	log.AddHook(&writer.Hook{ // Send info, debug, and trace logs to stdout
		Writer: os.Stdout,
		LogLevels: []log.Level{
			log.InfoLevel,
			log.DebugLevel,
			log.TraceLevel,
		},
	})
}

func addLogFlags(flags *pflag.FlagSet) {
	logFlags := &pflag.FlagSet{}

	formats := make([]string, 0, len(logFormats))
	for k := range logFormats {
		formats = append(formats, k)
	}
	logFlags.String(logFlagFormatName, logDefaultFormat, fmt.Sprintf("The log format (valid values are: %s)", strings.Join(formats, ", ")))
	logFlags.String(logFlagLevelName, logDefaultLevel, "The log level (trace, debug, info, warn, err, fatal)")

	flags.AddFlagSet(logFlags)
}

func getLogSettings() (logFormat, logLevel string) {
	level, isDefined := os.LookupEnv("LOG_LEVEL")
	if !isDefined {
		level = logDefaultLevel
	}
	format, isDefined := os.LookupEnv("LOG_FORMAT")
	if !isDefined {
		format = logDefaultFormat
	}
	return format, level
}

func configureLogging(logFormat, logLevel string) error {
	log.WithFields(log.Fields{
		"current.log.level":    log.GetLevel(),
		"submitted.log.format": logFormat,
		"submitted.log.level":  logLevel,
	}).Trace("configureLogging START")

	formatter, ok := logFormats[logFormat]
	if !ok {
		log.WithFields(log.Fields{
			"submitted.log.format": logFormat,
		}).Error(ErrorLogUnknownFormat.Error())
		return ErrorLogUnknownFormat
	}
	log.SetFormatter(formatter)

	logLevelParsed, err := log.ParseLevel(logLevel)
	if err != nil {
		log.WithFields(log.Fields{
			"error":               err,
			"submitted.log.level": logLevel,
		}).Error(ErrorLogLevelParse.Error())
		return err
	}
	log.SetLevel(logLevelParsed)

	log.WithFields(log.Fields{
		"current.log.level":    log.GetLevel(),
		"submitted.log.format": logFormat,
		"submitted.log.level":  logLevel,
	}).Trace("configureLogging END")
	return nil
}
