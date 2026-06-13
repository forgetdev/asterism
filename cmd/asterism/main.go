// Command asterism reconstructs Asterisk calls from CEL log files.
//
// Usage:
//
//	asterism analyze [flags] <cel-csv-file> [<cel-csv-file>...]
//
// CEL is the required input. An optional CDR Master.csv enriches each call's
// header with disposition/duration/billsec. Full log integration arrives in
// later versions.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/forgetdev/asterism/internal/cdr"
	"github.com/forgetdev/asterism/internal/cel"
	"github.com/forgetdev/asterism/internal/correlate"
	"github.com/forgetdev/asterism/internal/filter"
	"github.com/forgetdev/asterism/internal/fulllog"
	"github.com/forgetdev/asterism/internal/model"
	"github.com/forgetdev/asterism/internal/render"
	"github.com/forgetdev/asterism/internal/sip"
	"github.com/forgetdev/asterism/internal/stats"
)

const usage = `asterism - reconstruct Asterisk calls from CEL log files

Usage:
  asterism analyze [flags] <cel-csv-file> [<cel-csv-file>...]
  asterism analyze [flags] --cel-dir <directory>

Flags:
  --cdr <path>              CDR Master.csv to merge disposition/duration/billsec
  --cel-dir <path>          directory of CEL CSV files to merge (reads all *.csv)
  --format text|json|html|csv  output format (default: text)
  --output <path>           write output to file instead of stdout
  --no-color                disable ANSI colors in text output
  --ladder                  add ASCII SIP ladder diagram to text output
  --linkedid <id>           show only the call with this linkedid
  --channel <name>          show only calls containing this channel (substring)
  --extension <ext>         show only calls with this extension in any event
  --event-type <types>      show only events of these types, comma-separated
                            e.g. HANGUP or APP_START,APP_END
  --from <timestamp>        show only calls starting at or after this time
  --to <timestamp>          show only calls starting at or before this time
                            timestamp format: "YYYY-MM-DD" or "YYYY-MM-DD HH:MM:SS"
  --min-duration <dur>      show only calls with duration >= this (e.g. 30s, 5m)
  --max-duration <dur>      show only calls with duration <= this
  --hangup-cause <cause>    show only calls with this hangup cause
                            accepts a name ("NORMAL_CLEARING") or code ("16")
  --cel-columns <cols>      comma-separated CEL column names when your cel_custom.conf
                            differs from the default 13-column layout
                            default: eventtype,eventtime,calleridnum,calleridname,
                                     channel,exten,context,uniqueid,linkedid,
                                     bridgepeer,appname,appdata,eventextra
  --full-log <path>         Asterisk full log to correlate with call timelines
  --skip-bad-lines          skip malformed CSV rows instead of aborting
  --stats                   print aggregate statistics instead of call timelines

Examples:
  asterism analyze cel.csv
  asterism analyze monday.csv tuesday.csv wednesday.csv
  asterism analyze --cel-dir /var/log/asterisk/cel-custom/
  asterism analyze --cdr Master.csv cel.csv
  asterism analyze --format json cel.csv
  asterism analyze --format html --output report.html --full-log full cel.csv
  asterism analyze --format csv --output summary.csv --cdr Master.csv cel.csv
  asterism analyze --ladder --full-log full cel.csv
  asterism analyze --linkedid 1779999013.2 cel.csv
  asterism analyze --from "2026-06-12 16:00:00" --to "2026-06-12 17:00:00" cel.csv
  asterism analyze --min-duration 60s --hangup-cause NORMAL_CLEARING cel.csv
  asterism analyze --event-type HANGUP cel.csv
  asterism analyze --skip-bad-lines --cdr Master.csv cel.csv
  asterism analyze --stats --cdr Master.csv cel.csv
`

func main() {
	if len(os.Args) < 2 || os.Args[1] != "analyze" {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	fs := flag.NewFlagSet("analyze", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }

	cdrPath       := fs.String("cdr", "", "")
	celDir        := fs.String("cel-dir", "", "")
	format        := fs.String("format", "text", "")
	outputPath    := fs.String("output", "", "")
	noColor       := fs.Bool("no-color", false, "")
	ladder        := fs.Bool("ladder", false, "")
	linkedID      := fs.String("linkedid", "", "")
	channel       := fs.String("channel", "", "")
	extension     := fs.String("extension", "", "")
	eventTypeStr  := fs.String("event-type", "", "")
	fromStr       := fs.String("from", "", "")
	toStr         := fs.String("to", "", "")
	minDurStr     := fs.String("min-duration", "", "")
	maxDurStr     := fs.String("max-duration", "", "")
	hangupCause   := fs.String("hangup-cause", "", "")
	celColumnsStr := fs.String("cel-columns", "", "")
	skipBad       := fs.Bool("skip-bad-lines", false, "")
	statsMode     := fs.Bool("stats", false, "")
	fullLogPath   := fs.String("full-log", "", "")

	if err := fs.Parse(os.Args[2:]); err != nil {
		os.Exit(2)
	}
	if fs.NArg() == 0 && *celDir == "" {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	if *format != "text" && *format != "json" && *format != "html" && *format != "csv" {
		fmt.Fprintf(os.Stderr, "asterism: --format must be text, json, html, or csv\n")
		os.Exit(2)
	}

	if err := run(runConfig{
		celPaths:      fs.Args(),
		celDir:        *celDir,
		cdrPath:       *cdrPath,
		fullLogPath:   *fullLogPath,
		format:        *format,
		outputPath:    *outputPath,
		noColor:       *noColor,
		showLadder:    *ladder,
		linkedID:      *linkedID,
		channel:       *channel,
		extension:     *extension,
		eventTypeStr:  *eventTypeStr,
		fromStr:       *fromStr,
		toStr:         *toStr,
		minDurStr:     *minDurStr,
		maxDurStr:     *maxDurStr,
		hangupCause:   *hangupCause,
		celColumnsStr: *celColumnsStr,
		skipBadLines:  *skipBad,
		statsMode:     *statsMode,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "asterism: %v\n", err)
		os.Exit(1)
	}
}

type runConfig struct {
	celPaths      []string
	celDir        string
	cdrPath       string
	format        string
	outputPath    string
	noColor       bool
	showLadder    bool
	linkedID      string
	channel       string
	extension     string
	eventTypeStr  string
	fromStr       string
	toStr         string
	minDurStr     string
	maxDurStr     string
	hangupCause   string
	celColumnsStr string
	fullLogPath   string
	skipBadLines  bool
	statsMode     bool
}

func run(cfg runConfig) error {
	// Collect CEL file paths.
	paths := append([]string(nil), cfg.celPaths...)
	if cfg.celDir != "" {
		dirPaths, err := celPathsFromDir(cfg.celDir)
		if err != nil {
			return err
		}
		paths = append(paths, dirPaths...)
	}
	if len(paths) == 0 {
		return fmt.Errorf("no CEL files specified")
	}

	// Parse CEL — merge events from all files.
	var celCols []string
	if cfg.celColumnsStr != "" {
		celCols = splitColumns(cfg.celColumnsStr)
	}

	multi := len(paths) > 1
	var allEvents []model.Event
	totalSkipped := 0
	for i, path := range paths {
		if multi {
			fmt.Fprintf(os.Stderr, "asterism: reading %s (%d/%d)\n",
				filepath.Base(path), i+1, len(paths))
		}
		if cfg.skipBadLines {
			evs, skipped, err := parseCELLenient(path, celCols)
			if err != nil {
				return err
			}
			totalSkipped += skipped
			allEvents = append(allEvents, evs...)
		} else {
			evs, err := parseCELStrict(path, celCols)
			if err != nil {
				return err
			}
			allEvents = append(allEvents, evs...)
		}
	}
	if totalSkipped > 0 {
		fmt.Fprintf(os.Stderr, "asterism: skipped %d malformed CEL row(s)\n", totalSkipped)
	}

	// Deduplicate events that appear in overlapping files.
	if multi {
		before := len(allEvents)
		allEvents = deduplicateEvents(allEvents)
		if dupes := before - len(allEvents); dupes > 0 {
			fmt.Fprintf(os.Stderr, "asterism: removed %d duplicate event(s)\n", dupes)
		}
	}

	if len(allEvents) == 0 {
		return fmt.Errorf("no events found in input")
	}

	calls := correlate.ByLinkedID(allEvents)

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

	// Queue info is derived from CEL events and is always available.
	calls = fulllog.AttachQueueInfo(calls)

	// Parse and attach full log if provided.
	var regFailures []model.RegistrationFailure
	if cfg.fullLogPath != "" {
		logLines, err := fulllog.ParseFile(cfg.fullLogPath, 0)
		if err != nil {
			return err
		}
		calls = fulllog.AttachLog(calls, logLines)
		// Re-run queue enrichment so agent names are populated from log lines.
		calls = fulllog.AttachQueueInfo(calls)

		sipMsgs, err := sip.ParseFile(cfg.fullLogPath, 0)
		if err != nil {
			return err
		}
		if len(sipMsgs) > 0 {
			calls = sip.AttachSIP(calls, sipMsgs)
		}

		regFailures, err = fulllog.ParseRegistrationFailures(cfg.fullLogPath)
		if err != nil {
			return err
		}
	}

	// Apply call-level filters.
	filterOpts := filter.Options{
		LinkedID:    cfg.linkedID,
		Channel:     cfg.channel,
		Extension:   cfg.extension,
		HangupCause: cfg.hangupCause,
	}
	if cfg.fromStr != "" {
		t, err := filter.ParseTime(cfg.fromStr)
		if err != nil {
			return fmt.Errorf("--from: %w", err)
		}
		filterOpts.From = t
	}
	if cfg.toStr != "" {
		t, err := filter.ParseTime(cfg.toStr)
		if err != nil {
			return fmt.Errorf("--to: %w", err)
		}
		filterOpts.To = t
	}
	if cfg.minDurStr != "" {
		d, err := time.ParseDuration(cfg.minDurStr)
		if err != nil {
			return fmt.Errorf("--min-duration: %w", err)
		}
		filterOpts.MinDuration = d
	}
	if cfg.maxDurStr != "" {
		d, err := time.ParseDuration(cfg.maxDurStr)
		if err != nil {
			return fmt.Errorf("--max-duration: %w", err)
		}
		filterOpts.MaxDuration = d
	}
	calls = filter.Calls(calls, filterOpts)

	// Apply event-type filter.
	if cfg.eventTypeStr != "" {
		types := filter.ParseEventTypes(cfg.eventTypeStr)
		calls = filter.Events(calls, types)
	}

	// Open output destination.
	out := os.Stdout
	if cfg.outputPath != "" {
		f, err := os.Create(cfg.outputPath)
		if err != nil {
			return fmt.Errorf("opening output file: %w", err)
		}
		defer f.Close()
		out = f
	}

	// Render.
	color := !cfg.noColor && cfg.outputPath == "" && isTerminal(os.Stdout)
	opts := render.TextOptions{Color: color, ShowLadder: cfg.showLadder}

	if cfg.statsMode {
		r := stats.Compute(calls)
		switch cfg.format {
		case "json":
			if err := render.JSONStats(out, r); err != nil {
				return fmt.Errorf("rendering: %w", err)
			}
		default:
			if err := render.TextStats(out, r, opts); err != nil {
				return fmt.Errorf("rendering: %w", err)
			}
		}
		return nil
	}

	switch cfg.format {
	case "json":
		if err := render.JSON(out, calls); err != nil {
			return fmt.Errorf("rendering: %w", err)
		}
	case "html":
		if err := render.HTML(out, calls); err != nil {
			return fmt.Errorf("rendering: %w", err)
		}
	case "csv":
		if err := render.CSV(out, calls); err != nil {
			return fmt.Errorf("rendering: %w", err)
		}
	default:
		if err := render.Text(out, calls, opts); err != nil {
			return fmt.Errorf("rendering: %w", err)
		}
		if len(regFailures) > 0 {
			if err := render.TextRegistrationFailures(out, regFailures, opts); err != nil {
				return fmt.Errorf("rendering: %w", err)
			}
		}
	}
	return nil
}

func parseCELStrict(path string, cols []string) ([]model.Event, error) {
	var evs []model.Event
	var err error
	if len(cols) > 0 {
		evs, err = cel.ParseFileWithColumns(path, cols)
	} else {
		evs, err = cel.ParseFile(path)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return evs, nil
}

func parseCELLenient(path string, cols []string) ([]model.Event, int, error) {
	if len(cols) > 0 {
		return cel.ParseFileLenientWithColumns(path, cols)
	}
	return cel.ParseFileLenient(path)
}

// celPathsFromDir returns all *.csv files in dir, sorted alphabetically.
// Sorting by name yields chronological order when files are date-named
// (e.g. cel-2026-06-10.csv, cel-2026-06-11.csv).
func celPathsFromDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading --cel-dir %s: %w", dir, err)
	}
	var paths []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), ".csv") {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no .csv files found in %s", dir)
	}
	sort.Strings(paths)
	return paths, nil
}

// deduplicateEvents removes events that are identical by (UniqueID, Type,
// Timestamp). This handles overlapping log rotations where the same event
// appears in two consecutive files.
func deduplicateEvents(events []model.Event) []model.Event {
	type key struct {
		uniqueID string
		typ      model.EventType
		ts       int64
	}
	seen := make(map[key]struct{}, len(events))
	out := make([]model.Event, 0, len(events))
	for _, e := range events {
		k := key{e.UniqueID, e.Type, e.Timestamp.UnixNano()}
		if _, dup := seen[k]; !dup {
			seen[k] = struct{}{}
			out = append(out, e)
		}
	}
	return out
}

func splitColumns(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
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
