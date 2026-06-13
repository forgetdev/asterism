# asterism ⁂

[![CI](https://github.com/forgetdev/asterism/actions/workflows/ci.yml/badge.svg)](https://github.com/forgetdev/asterism/actions/workflows/ci.yml)

Post-mortem analysis tool for Asterisk calls - parses  CEL/CDR logs and reconstructs call timelines. Built for engineers who debug
telephony in production and are tired of reading raw `full` logs at 3 a.m.

> *In astronomy, an asterism is a pattern of stars that belong together —
> not a constellation, but a recognizable shape made of points of light.
> This tool finds the asterism in your Asterisk logs: the pattern of
> events that, together, tell the story of a single call.*

## Stability

From v1.0.0 onward, all CLI flags and subcommand names are stable. Breaking
changes to flags or subcommand interfaces will not be made without a major
version bump. Output format changes that add new fields are not considered
breaking.

## What it does

`asterism` reads Asterisk's CEL (Channel Event Logging) CSV output and groups
events by `linkedid` to produce a navigable timeline of what happened during
each call — which channels were created, which dialplan applications ran,
when bridges formed, and why the call ended.

It is designed for post-mortem analysis ("the call at 14:32 dropped — what
happened?") rather than live monitoring.

## Status

This is a personal project. It is not intended for compliance or billing use.
As of v1.0.0 the CLI interface is stable (see the Stability section above).

Current capabilities (v1.0.0):

**Analysis**
- [x] Parse CEL and CDR CSV into typed events
- [x] Correlate events by linkedid; attach CDR disposition/billsec
- [x] Hangup cause translation (Q.850)
- [x] Blind and attended transfer awareness
- [x] Full log correlation — dialplan execution alongside CEL timeline

**Output**
- [x] Text, JSON, HTML, and CSV output
- [x] SIP ladder diagram (`--ladder`, always shown in HTML reports)
- [x] Aggregate statistics (`--stats`)

**HTML report**
- [x] In-browser search/filter (vanilla JS, no external deps, self-contained)
- [x] Clickable call index with jump-to anchors
- [x] Gantt-style SVG timeline per call (bridge/hold/ringing periods)
- [x] Print-friendly CSS (`@media print`)

**Diagnostics**
- [x] SIP signaling analysis from PJSIP verbose log
- [x] Codec negotiation and RTP setup failure detection
- [x] Registration failure detection: failed REGISTER attempts shown after call output

**Queue**
- [x] Queue call detection: name, wait time, talk time, and agent from CEL + full log
- [x] Abandoned call detection: `exit=TIMEOUT` shown in call header
- [x] Queue aggregate stats: call count, abandon rate, avg wait time in `--stats`

**Filtering & batch**
- [x] Rich filters: linkedid, channel, extension, date range, duration, hangup cause
- [x] Multi-file and batch mode: multiple positional args or `--cel-dir <directory>`
- [x] Duplicate event detection across overlapping rotated log files
- [x] Configurable CEL column layout (`--cel-columns`) for non-standard `cel_custom.conf`

**Scan**
- [x] `asterism scan` subcommand: identify suspicious calls without reading every timeline
- [x] `--long-hold <dur>` — flag calls where bridge duration exceeds threshold
- [x] `--many-transfers <n>` — flag calls with more than n transfer events
- [x] `--codec-failure` — flag RTP/codec setup failures (requires `--full-log`)
- [x] No-answer rate always shown in scan summary
- [x] `--format csv` for piping scan results into spreadsheets

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
