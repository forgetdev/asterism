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
| `internal/model` | Domain types: `Event`, `CDR`, `Call`, `ExtraData` |
| `internal/q850` | Q.850 cause code → human-readable string |
| `internal/render` | Text renderer for call timelines |

## Key design decisions

- CEL parser is strict: wrong column count = error (silent data corruption is worse than a loud failure).
- CDR parser is equally strict (requires `loguniqueid=yes` → 18 columns).
- Extra field decoded at render time, not at parse time (schema varies by event type).
- `LINKEDID_END` events are suppressed in output (redundant noise).
- `.gitignore` anchors `/asterism` (root binary only) so `cmd/asterism/` source is not excluded.

## Test fixtures

- `testdata/fixture-02-ramal-answered/` — a complete answered call (CEL + CDR)
- `testdata/cdr-csv/Master.csv` — larger CDR sample for parser testing

## Roadmap

See `docs/TODO.md`. Current version: **v0.0.3**. Next: v0.0.4 (output options, filtering, robustness).
