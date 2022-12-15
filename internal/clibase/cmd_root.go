package clibase

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	// ErrorFlagCannotRetrieve is the error logged when attempting to retrieve the value of a flag fails
	ErrorFlagCannotRetrieve = fmt.Errorf("cannot retrieve flag value")
)

// AddRootFlags takes a pointer to an existing pflag.FlagSet and adds the default root/top level flags to it
func AddRootFlags(flags *pflag.FlagSet) {
	rootFlags := &pflag.FlagSet{}

	addLogFlags(rootFlags)
	flags.AddFlagSet(rootFlags)
}

// NewRootCmd returns a new Cobra root command
func NewRootCmd(cmdName, cmdDescription string) *cobra.Command {
	return &cobra.Command{
		Use:   cmdName,
		Short: cmdDescription,
	}
}

// New returns a new Cobra root command with the default configuration and subcommands
func New(cmdName, cmdDescription string) *cobra.Command {
	cmd := NewRootCmd(cmdName, cmdDescription)
	return NewFromRoot(cmd)
}

// NewFromRoot takes an existing Cobra root command and adds in the default flags, subcommands, and Init/Run entries
func NewFromRoot(rootCmd *cobra.Command) *cobra.Command {
	var persistentPreRunE func(*cobra.Command, []string) error
	if rootCmd.PersistentPreRunE != nil {
		oldPersistPreRunE := rootCmd.PersistentPreRunE
		rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
			if err := oldPersistPreRunE(cmd, args); err != nil {
				// Logging shit
				return err
			}
			return rootPersistentPreRunE(cmd, args)
		}
	} else {
		persistentPreRunE = rootPersistentPreRunE
	}

	rootCmd.PersistentPreRunE = persistentPreRunE
	AddRootFlags(rootCmd.PersistentFlags())
	addVersionCmd(rootCmd)
	return rootCmd
}

func rootPersistentPreRunE(cmd *cobra.Command, args []string) error {
	flags := cmd.Flags()
	logFormat, err := flags.GetString(logFlagFormatName)
	if err != nil {
		log.WithFields(log.Fields{
			"flag.name": logFlagFormatName,
			"error":     err,
		}).Error(ErrorFlagCannotRetrieve.Error())
		return err
	}
	logLevel, err := flags.GetString(logFlagLevelName)
	if err != nil {
		log.WithFields(log.Fields{
			"flag.name": logFlagLevelName,
			"error":     err,
		}).Error(ErrorFlagCannotRetrieve.Error())
		return err
	}

	checkCobraFlags(flags)

	return configureLogging(logFormat, logLevel)
}

func checkCobraFlags(flags *pflag.FlagSet) {
	// Warn if CLI flags don't follow style conventions
	flags.VisitAll(func(flag *pflag.Flag) {
		l := log.WithField("flag.name", flag.Name)
		l.Tracef("checking flag for style")

		if strings.Index(flag.Name, "_") > 0 {
			// We don't use --foo_bar, we use --foo-bar.
			l.WithField("violation", "flag names must use hyphen not underscore").Warnf("invalid flag name")
		}
	})
}
