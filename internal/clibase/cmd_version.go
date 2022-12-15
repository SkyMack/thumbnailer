package clibase

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/spf13/pflag"

	"github.com/spf13/cobra"
)

func version(name string, flags *pflag.FlagSet) error {
	depPrefix, err := flags.GetString("dep-prefix")
	if err != nil {
		return err
	}

	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return nil
	}
	fmt.Printf("%s (%s %s)\n", name, buildInfo.Main.Path, buildInfo.Main.Version)

	fmt.Printf("\n")
	fmt.Printf("  Compiled with: %s\n", runtime.Compiler)
	fmt.Printf("         GOARCH: %s\n", runtime.GOARCH)
	fmt.Printf("           GOOS: %s\n", runtime.GOOS)
	fmt.Printf("     Go Version: %s\n", runtime.Version())
	fmt.Printf("\n")

	for _, pkg := range buildInfo.Deps {
		if !strings.HasPrefix(pkg.Path, depPrefix) {
			continue
		}
		output := fmt.Sprintf("%s %s", pkg.Path, pkg.Version)
		if pkg.Replace != nil {
			var struckthrough string
			for _, r := range output {
				struckthrough += "\u0336" + string(r)
			}
			output = fmt.Sprintf("%s\u0336  => %s", struckthrough, pkg.Replace.Path)
		}
		fmt.Printf("  %s\n", output)
	}
	return nil
}

func addVersionCmd(rootCmd *cobra.Command) {
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "output the binary version",
		RunE: func(cmd *cobra.Command, args []string) error {
			return version(rootCmd.Name(), cmd.Flags())
		},
	}
	versionFlags := versionCmd.Flags()
	versionFlags.String("dep-prefix", "github.com/SkyMack", "only introspect packages under this prefix")

	rootCmd.AddCommand(versionCmd)
}
