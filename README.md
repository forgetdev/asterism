# asterism ⁂

[![CI](https://github.com/forgetdev/asterism/actions/workflows/ci.yml/badge.svg)](https://github.com/forgetdev/asterism/actions/workflows/ci.yml)

Reconstruct Asterisk calls from CEL log files. Built for engineers who debug
telephony in production and are tired of reading raw `full` logs at 3 a.m.

> *In astronomy, an asterism is a pattern of stars that belong together —
> not a constellation, but a recognizable shape made of points of light.
> This tool finds the asterism in your Asterisk logs: the pattern of
> events that, together, tell the story of a single call.*

## What it does

`asterism` reads Asterisk's CEL (Channel Event Logging) CSV output and groups
events by `linkedid` to produce a navigable timeline of what happened during
each call — which channels were created, which dialplan applications ran,
when bridges formed, and why the call ended.

It is designed for post-mortem analysis ("the call at 14:32 dropped — what
happened?") rather than live monitoring.

## Status

This is a personal project in active development. It is not production-ready
and should not be relied on for compliance, billing, or any decision-making
process. The output format will change between versions.

Current capabilities (v0.9.0):

- [x] Parse CEL and CDR CSV into typed events
- [x] Correlate events by linkedid; attach CDR disposition/billsec
- [x] Text, JSON, HTML, and CSV output
- [x] SIP ladder diagram (`--ladder`, always shown in HTML reports)
- [x] Blind and attended transfer awareness
- [x] Full log correlation — dialplan execution alongside CEL timeline
- [x] SIP signaling analysis from PJSIP verbose log
- [x] Hangup cause translation (Q.850)
- [x] Codec negotiation and RTP setup failure diagnostics
- [x] Aggregate statistics (`--stats`)
- [x] Rich filters: linkedid, channel, extension, date range, duration, hangup cause
- [x] Configurable CEL column layout (`--cel-columns`) for non-standard `cel_custom.conf`
- [x] Multi-file and batch mode: multiple positional args or `--cel-dir <directory>`
- [x] Duplicate event detection across overlapping rotated log files
- [x] Queue call detection: name, wait time, talk time, and agent from CEL + full log
- [x] Queue abandoned call detection: `abandoned  exit=TIMEOUT` shown in call header
- [x] Queue aggregate stats: queue call count, abandon rate, avg wait time in `--stats`
- [x] Registration failure detection: failed REGISTER attempts shown after call output
- [x] HTML report: in-browser search/filter (vanilla JS, no external deps, self-contained)
- [x] HTML report: clickable call index (jump-to anchors for each call block)
- [x] HTML report: Gantt-style SVG timeline per call (bridge/hold/ringing periods)
- [x] HTML report: print-friendly CSS (`@media print`)
- [x] `asterism scan` subcommand: identify suspicious calls without reading every timeline
- [x] Scan pattern: `--long-hold <dur>` — flag calls where bridge duration exceeds threshold
- [x] Scan pattern: `--many-transfers <n>` — flag calls with more than n transfer events
- [x] Scan pattern: `--codec-failure` — flag RTP/codec setup failures (requires `--full-log`)
- [x] Scan summary: no-answer rate always shown (fraction of unanswered calls)
- [x] Scan output: `--format csv` for piping results into spreadsheets

## Requirements

- Go 1.22 or newer
- Asterisk 18, 20, or 21 with CEL configured to write CSV
  (see `docs/asterisk-setup.md` for the required `cel_custom.conf` layout)

## Installation

```
go install github.com/forgetdev/asterism/cmd/asterism@latest
```

Or clone and build locally:

```
git clone https://github.com/forgetdev/asterism
cd asterism
go build -o asterism ./cmd/asterism
```

## Usage

```
asterism analyze /var/log/asterisk/cel-custom/Master.csv
```

Output is a textual timeline. Each call is a block grouped by its linkedid,
with events indented and prefixed by offset from call start.

To scan a large file for suspicious calls without reading every timeline:

```
asterism scan --long-hold 1h --many-transfers 2 cel.csv
asterism scan --format csv --long-hold 30m cel.csv > suspicious.csv
```

The scan subcommand prints a line per matching linkedid and a summary including
the no-answer rate across all scanned calls.

## How it works

`asterism` is a four-stage pipeline:

1. **Parsing** reads CEL CSV rows into typed `model.Event` values
2. **Correlation** groups events by `LinkedID` and sorts by timestamp
3. **Rendering** produces a text view of each call

The internals are documented in [`docs/architecture.md`](docs/architecture.md).

## What `asterism` is not

- Not a replacement for `sngrep`, Wireshark, or HEP capture — those operate
  on the wire, `asterism` operates on logs
- Not a CDR billing system
- Not a real-time monitoring dashboard
- Not affiliated with Sangoma, Digium, or the Asterisk project

## Why "asterism"?

The Unicode character **⁂** is called an asterism — three asterisks arranged
in a triangle, used in typography to mark a break in text. The Asterisk PBX
takes its name from the asterisk symbol (`*`), and "asterism" extends that
lineage: where `*` is a single star, `⁂` is a pattern of stars. That mirrors
what this tool does — taking individual events and revealing the pattern
they form together.

## License

MIT. See [LICENSE](LICENSE).
