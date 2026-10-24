package dbmigrate

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"gorm.io/gorm"
	"oneimg/backend/config"
)

type Failure struct {
	Code  int
	Stage string
}

func (e *Failure) Error() string          { return e.Stage }
func fail(code int, message string) error { return &Failure{Code: code, Stage: message} }

type Options struct {
	Source     string
	DryRun     bool
	VerifyOnly bool
	Output     io.Writer
	// Fault injection is confined to package tests, never a CLI/environment option.
	hook   func(string, *gorm.DB) error
	commit func() error
}

func RunCLI(args []string) int {
	flags := flag.NewFlagSet("db-migrate", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	options := Options{Output: os.Stdout}
	flags.StringVar(&options.Source, "source", "", "consistent, upgraded SQLite backup")
	flags.BoolVar(&options.DryRun, "dry-run", false, "read-only preflight")
	flags.BoolVar(&options.VerifyOnly, "verify-only", false, "read-only full comparison")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || options.Source == "" || (options.DryRun && options.VerifyOnly) {
		fmt.Fprintln(os.Stderr, "usage: oneimg db-migrate --source /backup/oneimg.db [--dry-run | --verify-only]")
		return 2
	}
	cfg, err := config.LoadMigration()
	if err == nil {
		err = Run(cfg, options)
	}
	if err != nil {
		var failure *Failure
		code := 2
		if errors.As(err, &failure) {
			code = failure.Code
		}
		fmt.Fprintln(os.Stderr, "db-migrate:", err)
		if code == 4 {
			fmt.Fprintln(os.Stderr, "commit result unknown; run --verify-only before taking further action")
		}
		return code
	}
	return 0
}
