# asterism

Reconstruct and inspect Asterisk calls from log files. Built for engineers who debug telephony in production and are tired of reading raw `full` logs at 3 a.m.

> Status: early development. APIs and flags will change.

## What it does

`asterism` reads Asterisk's log files, CDR, and CEL data, then groups events by `linkedid` to produce a navigable timeline of what happened during a call — which channels were created, which dialplan applications ran, when bridges formed, which SIP responses came back, and why the call ended.

It is designed for post-mortem analysis ("the call at 14:32 dropped — what happened?") rather than live monitoring.

## Why

Asterisk produces excellent diagnostic information, but spread across several files in formats that don't talk to each other. Reconstructing a single call today means grepping `full` for the right `[C-xxxxxxxx]`, cross-referencing CDR rows by `uniqueid`, decoding CEL events, and matching SIP signaling by call-id — all by hand.

`asterism` does this mechanically and outputs a structured view of the call. Nothing more.

## Status

This is a personal project in active development. It is not production-ready and should not be relied on for compliance, billing, or any decision-making process. The output format will change between versions.

Current capabilities:

- [ ] Parse `full` log into structured events
- [ ] Parse CDR CSV
- [ ] Parse CEL CSV
- [ ] Correlate events by `linkedid`
- [ ] CLI report (text)
- [ ] HTML output with SIP-style ladder diagram
- [ ] Live tail mode
- [ ] PJSIP history integration

## Requirements

- Go 1.22 or newer (to build from source)
- Asterisk log files from version 18, 20, or 21 (older versions may work but are not tested)

## Installation

```
go install github.com/<your-user>/asterism/cmd/asterism@latest
```

Or download a release binary from the releases page (none yet).

## Usage

Analyze a single call by `linkedid`:

```
asterism analyze --log /var/log/asterisk/full \
                --cdr /var/log/asterisk/cdr-csv/Master.csv \
                --linkedid 1715275431.42
```

Output is a structured text timeline. Add `--format html` to generate a self-contained HTML file with a ladder diagram.

For more options:

```
asterism --help
```

## How it works

`asterism` is a four-stage pipeline:

1. **Ingestion** reads raw files from disk (no daemon, no live capture yet)
2. **Parsing** turns each source format into typed events
3. **Correlation** groups events by `linkedid` and orders them by timestamp
4. **Rendering** produces a text or HTML view

The internals are documented in [`docs/architecture.md`](docs/architecture.md).

## What `asterism` is not

- Not a replacement for `sngrep`, Wireshark, or HEP capture — those operate on the wire, `asterism` operates on logs
- Not a CDR billing system
- Not a real-time monitoring dashboard
- Not affiliated with Sangoma, Digium, or the Asterisk project

## Contributing

Issues and pull requests welcome once the project reaches `v0.1.0`. For now, the parser grammar and event taxonomy are in flux and contributions would be premature.

If you use Asterisk in production and would like to test against your logs, please open an issue describing your environment (Asterisk version, log volume, what you'd want to extract). Real-world samples drive the parser's evolution.

## License

MIT. See [LICENSE](LICENSE).

## Author

Built by an infrastructure engineer working with Asterisk in production. Contact via GitHub issues.
