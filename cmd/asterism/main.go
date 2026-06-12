// Command asterism reconstructs Asterisk calls from CEL log files.
//
// Usage:
//
//	asterism analyze [flags] <cel-csv-file>
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
	"github.com/forgetdev/asterism/internal/filter"
	"github.com/forgetdev/asterism/internal/model"
	"github.com/forgetdev/asterism/internal/render"
)

const usage = `asterism - reconstruct Asterisk calls from CEL log files

Usage:
  asterism analyze [flags] <cel-csv-file>

Flags:
  --cdr <path>          CDR Master.csv to merge disposition/duration/billsec
  --format text|json    output format (default: text)
  --no-color            disable ANSI colors in text output
  --linkedid <id>       show only the call with this linkedid
  --channel <name>      show only calls containing this channel (substring)
  --extension <ext>     show only calls with this extension in any event
  --event-type <types>  show only events of these types, comma-separated
                        e.g. HANGUP or APP_START,APP_END
  --skip-bad-lines      skip malformed CSV rows instead of aborting

Examples:
  asterism analyze cel.csv
  asterism analyze --cdr Master.csv cel.csv
  asterism analyze --format json cel.csv
  asterism analyze --linkedid 1779999013.2 cel.csv
  asterism analyze --event-type HANGUP cel.csv
  asterism analyze --skip-bad-lines --cdr Master.csv cel.csv
`

func main() {
	if len(os.Args) < 2 || os.Args[1] != "analyze" {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	fs := flag.NewFlagSet("analyze", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }

	cdrPath      := fs.String("cdr", "", "")
	format       := fs.String("format", "text", "")
	noColor      := fs.Bool("no-color", false, "")
	linkedID     := fs.String("linkedid", "", "")
	channel      := fs.String("channel", "", "")
	extension    := fs.String("extension", "", "")
	eventTypeStr := fs.String("event-type", "", "")
	skipBad      := fs.Bool("skip-bad-lines", false, "")

	if err := fs.Parse(os.Args[2:]); err != nil {
		os.Exit(2)
	}
	if fs.NArg() != 1 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	celPath := fs.Arg(0)

	if *format != "text" && *format != "json" {
		fmt.Fprintf(os.Stderr, "asterism: --format must be text or json\n")
		os.Exit(2)
	}

	if err := run(runConfig{
		celPath:      celPath,
		cdrPath:      *cdrPath,
		format:       *format,
		noColor:      *noColor,
		linkedID:     *linkedID,
		channel:      *channel,
		extension:    *extension,
		eventTypeStr: *eventTypeStr,
		skipBadLines: *skipBad,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "asterism: %v\n", err)
		os.Exit(1)
	}
}

type runConfig struct {
	celPath      string
	cdrPath      string
	format       string
	noColor      bool
	linkedID     string
	channel      string
	extension    string
	eventTypeStr string
	skipBadLines bool
}

func run(cfg runConfig) error {
	// Parse CEL.
	var events []model.Event
	if cfg.skipBadLines {
		var skipped int
		var err error
		events, skipped, err = cel.ParseFileLenient(cfg.celPath)
		if err != nil {
			return err
		}
		if skipped > 0 {
			fmt.Fprintf(os.Stderr, "asterism: skipped %d malformed CEL row(s)\n", skipped)
		}
	} else {
		var err error
		events, err = cel.ParseFile(cfg.celPath)
		if err != nil {
			return err
		}
	}
	if len(events) == 0 {
		return fmt.Errorf("no events found in input")
	}

	calls := correlate.ByLinkedID(events)

	// Parse and attach CDR if provided.
	if cfg.cdrPath != "" {
		if cfg.skipBadLines {
			records, skipped, err := cdr.ParseFileLenient(cfg.cdrPath)
			if err != nil {
				return err
			}
			if skipped > 0 {
				fmt.Fprintf(os.Stderr, "asterism: skipped %d malformed CDR row(s)\n", skipped)
			}
			calls = correlate.AttachCDR(calls, records)
		} else {
			records, err := cdr.ParseFile(cfg.cdrPath)
			if err != nil {
				return err
			}
			calls = correlate.AttachCDR(calls, records)
		}
	}

	// Apply call-level filters.
	filterOpts := filter.Options{
		LinkedID:  cfg.linkedID,
		Channel:   cfg.channel,
		Extension: cfg.extension,
	}
	calls = filter.Calls(calls, filterOpts)

	// Apply event-type filter.
	if cfg.eventTypeStr != "" {
		types := filter.ParseEventTypes(cfg.eventTypeStr)
		calls = filter.Events(calls, types)
	}

	// Render.
	switch cfg.format {
	case "json":
		if err := render.JSON(os.Stdout, calls); err != nil {
			return fmt.Errorf("rendering: %w", err)
		}
	default:
		color := !cfg.noColor && isTerminal(os.Stdout)
		if err := render.Text(os.Stdout, calls, render.TextOptions{Color: color}); err != nil {
			return fmt.Errorf("rendering: %w", err)
		}
	}
	return nil
}

// isTerminal reports whether f is a character device (i.e. a TTY).
// Uses only the standard library to keep the dependency count at zero.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
