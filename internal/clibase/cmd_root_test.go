package clibase

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRootCmd(t *testing.T) {
	name := "command-name"
	description := "command description here"

	t.Run("Has proper name and use values", func(t *testing.T) {
		cmd := New(name, description)
		assert.Equal(t, name, cmd.Name())
		assert.Equal(t, description, cmd.Short)
	})

	t.Run("Executes", func(t *testing.T) {
		cmd := New(name, description)
		args := []string{
			"version",
		}
		cmd.SetArgs(args)

		outBuf := bytes.NewBufferString("")
		cmd.SetOut(outBuf)
		err := cmd.Execute()
		assert.NoError(t, err)
		out, err := ioutil.ReadAll(outBuf)
		assert.NoError(t, err)
		fmt.Println(string(out))
	})

	t.Run("Valid LogLevel Flag", func(t *testing.T) {
		cmd := New(name, description)
		args := []string{
			"version",
			fmt.Sprintf("--%s", logFlagLevelName),
			"info",
		}
		cmd.SetArgs(args)

		outBuf := bytes.NewBufferString("")
		cmd.SetOut(outBuf)
		err := cmd.Execute()
		assert.NoError(t, err)
		out, err := ioutil.ReadAll(outBuf)
		assert.NoError(t, err)
		fmt.Println(string(out))
	})

	t.Run("Invalid LogLevel Flag", func(t *testing.T) {
		cmd := New(name, description)
		args := []string{
			"version",
			fmt.Sprintf("--%s", logFlagLevelName),
			"notarealloglevel",
		}
		cmd.SetArgs(args)

		outBuf := bytes.NewBufferString("")
		cmd.SetOut(outBuf)
		err := cmd.Execute()
		assert.Error(t, err)
	})

	t.Run("Valid LogFormat Flag", func(t *testing.T) {
		cmd := New(name, description)
		args := []string{
			"version",
			fmt.Sprintf("--%s", logFlagFormatName),
			logJSONFormatName,
		}
		cmd.SetArgs(args)

		outBuf := bytes.NewBufferString("")
		cmd.SetOut(outBuf)
		err := cmd.Execute()
		assert.NoError(t, err)
		out, err := ioutil.ReadAll(outBuf)
		assert.NoError(t, err)
		fmt.Println(string(out))
	})

	t.Run("Invalid LogFormat Flag", func(t *testing.T) {
		cmd := New(name, description)
		args := []string{
			"version",
			fmt.Sprintf("--%s", logFlagFormatName),
			"notarealformat",
		}
		cmd.SetArgs(args)

		outBuf := bytes.NewBufferString("")
		cmd.SetOut(outBuf)
		err := cmd.Execute()
		assert.Error(t, err)
	})
}
