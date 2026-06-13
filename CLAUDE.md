# asterism

Post-mortem Asterisk call analysis tool. Reads CEL and CDR CSV logs, reconstructs call timelines, and renders them to stdout. The project's identity is a **batch analysis tool** — not a daemon, not a monitoring product.

## Build & run

```bash
go build -o asterism ./cmd/asterism
./asterism analyze [--cdr <Master.csv>] <cel-csv-file>
```

## Test

```bash
go test ./...
go vet ./...
```

## Package layout

| Package | Role |
|---|---|
| `cmd/asterism` | CLI entrypoint; wires flags → parse → correlate → render |
| `internal/cel` | Parse CEL `Master.csv` into `[]model.Event` |
| `internal/cdr` | Parse CDR `Master.csv` into `[]model.CDR` |
| `internal/correlate` | Group events into `[]model.Call` by LinkedID; attach CDRs |
| `internal/model` | Domain types: `Event`, `CDR`, `Call`, `ExtraData`, `QueueInfo` |
| `internal/q850` | Q.850 cause code → human-readable string |
| `internal/sip` | Parse PJSIP SIP messages from full log; diagnostics |
| `internal/fulllog` | Parse Asterisk full log; correlate with CEL; queue/regfail detection |
| `internal/stats` | Aggregate statistics over a set of calls |
| `internal/render` | Text, JSON, HTML, CSV renderers for call timelines and stats |
| `internal/filter` | Predicate filters (linkedid, channel, extension, date, duration…) |

## Key design decisions

- CEL parser is strict: wrong column count = error (silent data corruption is worse than a loud failure).
- CDR parser is equally strict (requires `loguniqueid=yes` → 18 columns).
- Extra field decoded at render time, not at parse time (schema varies by event type).
- `LINKEDID_END` events are suppressed in output (redundant noise).
- `.gitignore` anchors `/asterism` (root binary only) so `cmd/asterism/` source is not excluded.

## Test fixtures

- `testdata/fixture-02-ramal-answered/` — complete answered call (CEL + CDR)
- `testdata/fixture-06-reg-failure/` — answered queue call + queue-timeout/abandoned call + registration failure (CEL + CDR + full log)
- `testdata/cdr-csv/Master.csv` — larger CDR sample for parser testing

## Roadmap

See `docs/TODO.md`. Current version: **v0.8.1**.

## Claude Code

Run `/compact` when context usage reaches 80%.
