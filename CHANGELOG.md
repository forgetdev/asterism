# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2026-06-13

### Added

- Man page `docs/asterism.1` covering all subcommands, flags, files, and examples
- `.goreleaser.yaml` for automated binary builds (Linux amd64/arm64, macOS arm64)
- GitHub Actions release workflow: tag push triggers goreleaser and attaches binaries
- Stable API promise: CLI flags and subcommand names will not change without a major version bump

## [0.9.0] - 2026-06-13

### Added

- `asterism scan` subcommand: identify calls matching suspicious patterns without reading every timeline
- `--long-hold <dur>` scan pattern: flag calls where bridge duration exceeds threshold
- `--many-transfers <n>` scan pattern: flag calls with more than n transfer events
- `--codec-failure` scan pattern: flag RTP/codec setup failures (requires `--full-log`)
- No-answer rate always computed and shown in scan summary line
- `--format csv` output for scan results (pipe to spreadsheet)
- `internal/scan` package: pattern-matching engine returning `[]Match` and `Summary`

## [0.8.0] - 2026-06-12

### Added

- HTML report: in-browser search/filter (caller, callee, result, linkedid) using vanilla JS
- HTML report: clickable call index at the top with jump-to anchors
- HTML report: Gantt-style SVG timeline per call showing bridge/hold/ringing periods
- HTML report: print-friendly CSS (`@media print`)

### Fixed

- Reserved SVG space for Gantt legend (was overlapping last bar row)

## [0.7.1] - 2026-06-11

### Fixed

- Corrected version string from 0.7.0 to 0.7.1 following queue items completing

## [0.7.0] - 2026-06-11

### Added

- Queue call detection: queue name, wait time, talk time, and agent shown in call header
- Queue abandoned call detection: `abandoned exit=TIMEOUT` shown in call header
- Queue aggregate statistics: queue call count, abandon rate, average wait time in `--stats`
- Registration failure detection: failed REGISTER attempts shown after call output
- `internal/fulllog` package: parse Asterisk full log for queue and registration events

## [0.6.0] - 2026-06-10

### Added

- Accept multiple positional CEL file arguments: `asterism analyze *.csv`
- `--cel-dir <path>` flag: read all `*.csv` files from a directory
- Duplicate event deduplication across overlapping rotated log files
- Aggregate `--stats` across all input files combined
- Progress indicator (file count) when processing multiple files

## [0.5.0] - 2026-06-09

### Added

- `--cel-columns <col1,col2,...>` flag: override the default CEL column order for non-standard `cel_custom.conf` layouts
- Column list validation at startup (unknown column names produce a clear error)
- Unit tests for `internal/filter` (all predicates)
- Unit tests for `internal/render/csv` (column count, transfer field, multi-leg billsec)
- Unit tests for `internal/correlate` (grouping, CDR attachment, multi-leg transfer)
- Unit tests for `internal/cdr` (strict parse, lenient mode, column count errors)

### Changed

- CEL parse errors now include row number and field count
- CDR parse errors now include row number

## [0.4.0] - 2026-06-08

### Added

- `ATTENDEDTRANSFER` and `BLINDTRANSFER` CEL event type recognition and timeline display
- Caller/callee identification corrected after transfer events
- Transfer target shown in call header
- `--from` / `--to` flags: filter by date range (timestamp or date)
- `--min-duration` / `--max-duration` flags: filter by call duration
- `--hangup-cause <name|code>` flag: filter by Q.850 hangup cause
- CSV call summary output: `--format csv` (one row per call: linkedid, start, duration, result, caller, callee, hangup cause)
- GitHub Actions CI workflow: `go test ./...` and `go vet` on every PR
- `golangci-lint` in CI
- Build status badge in README
- `docs/asterisk-setup.md`: configuration guide for CEL, CDR, and full log

## [0.3.0] - 2026-06-07

### Added

- HTML report output: `--format html`
- Timeline visualization: Gantt-style HTML rendering
- `--output <path>` flag: write output to a file instead of stdout
- SIP ladder diagram: `--ladder` flag for text output; always shown in HTML
- SIP response codes and timing in ladder diagrams

## [0.2.0] - 2026-06-06

### Added

- Parse SIP dialogs from PJSIP verbose log output
- Parse INVITE transactions, provisional responses (100/180/183), and final responses (200/4xx/5xx/6xx)
- Parse BYE/CANCEL flows
- Correlate SIP dialogs with CEL channels
- Codec negotiation detection: codecs from INVITE SDP shown in call header
- RTP setup failure detection: `native_rtp` WARNING lines flagged in call output
- 4xx/5xx error response detection
- `internal/sip` package

## [0.1.0] - 2026-06-05

### Added

- Parse Asterisk full log file
- Correlate CEL events with full log lines by channel name
- Show related dialplan log lines alongside CEL event timeline
- `--full-log <path>` flag
- `internal/fulllog` package (initial version)

## [0.0.5] - 2026-06-04

### Added

- `--stats` flag: aggregate statistics mode (total calls, answered, failed, busy, average duration)
- `internal/stats` package

## [0.0.4] - 2026-06-03

### Added

- `--format json` flag: JSON output
- Colorized terminal output with automatic TTY detection
- `--no-color` flag: disable ANSI color codes
- `--linkedid <id>` flag: show only the call with this linkedid
- `--channel <name>` flag: filter by channel name (substring)
- `--extension <ext>` flag: filter by extension
- `--event-type <types>` flag: show only specified event types (comma-separated)
- `--skip-bad-lines` flag: tolerant parse mode that skips malformed rows and reports a count
- `internal/filter` package

## [0.0.3] - 2026-06-02

### Added

- Parse CDR `Master.csv` (requires `loguniqueid=yes`, 18-column layout)
- Correlate CDR rows with CEL calls by uniqueid/linkedid
- Call summary header: disposition, total duration, billsec, hangup cause, dialstatus, caller, callee
- `--cdr <path>` flag
- Suppressed `LINKEDID_END` events from output (redundant noise)

## [0.0.2] - 2026-06-01

### Added

- Q.850 hangup cause code translation to human-readable strings
- Sub-millisecond offset precision: µs/ms granularity when gaps would round to 0s
- Strip outer parentheses from `appdata` field when already parenthesized
- Decode `eventextra` JSON blob into structured display fields

## [0.0.1] - 2026-05-31

### Added

- Initial release
- Parse Asterisk CEL CSV into typed events
- Correlate events by `linkedid`
- Render call timeline to stdout as text

[Unreleased]: https://github.com/forgetdev/asterism/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/forgetdev/asterism/compare/v0.9.0...v1.0.0
[0.9.0]: https://github.com/forgetdev/asterism/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/forgetdev/asterism/compare/v0.7.1...v0.8.0
[0.7.1]: https://github.com/forgetdev/asterism/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/forgetdev/asterism/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/forgetdev/asterism/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/forgetdev/asterism/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/forgetdev/asterism/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/forgetdev/asterism/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/forgetdev/asterism/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/forgetdev/asterism/compare/v0.0.5...v0.1.0
[0.0.5]: https://github.com/forgetdev/asterism/compare/v0.0.4...v0.0.5
[0.0.4]: https://github.com/forgetdev/asterism/compare/v0.0.3...v0.0.4
[0.0.3]: https://github.com/forgetdev/asterism/compare/v0.0.2...v0.0.3
[0.0.2]: https://github.com/forgetdev/asterism/compare/v0.0.1...v0.0.2
[0.0.1]: https://github.com/forgetdev/asterism/releases/tag/v0.0.1
