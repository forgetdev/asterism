// Command asterism reconstructs Asterisk calls from CEL log files.
//
// Usage:
//
//	asterism analyze [--cdr <Master.csv>] <cel-csv-file>
//
// CEL is the required input. An optional CDR Master.csv enriches each call's
// header with disposition/duration/billsec. Full log integration arrives in
// later versions.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/forgetdev/asterism/internal/cdr"
	"github.com/forgetdev/asterism/internal/cel"
	"github.com/forgetdev/asterism/internal/correlate"
	"github.com/forgetdev/asterism/internal/render"
)

const usage = `asterism - reconstruct Asterisk calls from CEL log files

Usage:
  asterism analyze [--cdr <Master.csv>] <cel-csv-file>

Flags:
  --cdr <path>   CDR Master.csv to merge disposition/duration/billsec into
                 each call's header

Example:
  asterism analyze /var/log/asterisk/cel-custom/Master.csv
  asterism analyze --cdr /var/log/asterisk/cdr-csv/Master.csv cel.csv
`

func main() {
	if len(os.Args) < 2 || os.Args[1] != "analyze" {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	fs := flag.NewFlagSet("analyze", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	cdrPath := fs.String("cdr", "", "CDR Master.csv to merge into call headers")
	if err := fs.Parse(os.Args[2:]); err != nil {
		os.Exit(2)
	}

	if fs.NArg() != 1 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	celPath := fs.Arg(0)

	if err := run(celPath, *cdrPath); err != nil {
		fmt.Fprintf(os.Stderr, "asterism: %v\n", err)
		os.Exit(1)
	}
}

// run wires the pipeline: parse CEL, optionally parse and attach CDR, correlate,
// and render. Split out from main so the exit-code handling stays in one place.
func run(celPath, cdrPath string) error {
	events, err := cel.ParseFile(celPath)
	if err != nil {
		return err
	}
	if len(events) == 0 {
		return fmt.Errorf("no events found in input")
	}

	calls := correlate.ByLinkedID(events)

	if cdrPath != "" {
		records, err := cdr.ParseFile(cdrPath)
		if err != nil {
			return err
		}
		calls = correlate.AttachCDR(calls, records)
	}

	if err := render.Text(os.Stdout, calls); err != nil {
		return fmt.Errorf("rendering: %w", err)
	}
	return nil
}
