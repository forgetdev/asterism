# Roadmap

Versioning is deliberately incremental: small, frequent releases over large,
rare ones. Each version should be shippable on its own.

A note on identity: asterism is a **post-mortem call analysis tool**. It reads
logs and reconstructs what happened. Anything that turns it into a long-running
real-time daemon (live tail, metrics export, streaming) is a possible future
direction but represents a change in the project's nature — see "Future /
maybe" at the bottom. Do not let those items pull the core scope toward being
a monitoring product.

---

## v0.0.1 — done

- [x] Parse CEL CSV into typed events
- [x] Correlate events by linkedid
- [x] Text report to stdout with call timeline

## v0.0.2 — done

### CEL quality improvements
- [x] Translate Q.850 hangupcause codes to human-readable strings
- [x] Use µs/ms granularity when sub-millisecond gaps would round to 0s
- [x] Strip outer parens from appdata when already parenthesized
- [x] Decode the eventextra JSON blob into structured fields

---

## Infra — do before/alongside v0.0.3

These are not a release; they are foundation. The earlier they land, the more
regressions they catch as the codebase grows.

- [ ] GitHub Actions: run `go test ./...` and `go vet` on every PR
- [ ] Add `golangci-lint` to CI
- [ ] Build status badge in README
- [ ] `docs/asterisk-setup.md`: how to configure `cel_custom.conf` to produce
      the 13-column layout asterism expects (README already references this
      file but it does not exist yet)

---

## v0.0.3 — CDR and call summary — done

CDR provides what CEL cannot: disposition, duration, billsec. Parse it first,
then build the summary on top.

### CDR parsing
- [x] Parse CDR Master.csv
- [x] Correlate CDR rows with CEL calls by uniqueid/linkedid

### Call summary header
- [x] Show call disposition (ANSWERED, BUSY, FAILED, NO ANSWER)
- [x] Show total duration
- [x] Show talk time (billsec) when available
- [x] Show final hangup cause
- [x] Show final dialstatus
- [x] Detect caller and callee (simple heuristic from CEL: originator is first
      CHAN_START, callee is the Dial target — known to break on transfers,
      revisit later)

### Quick win
- [x] Hide LINKEDID_END from output (redundant noise; trivial render change)

Example summary header:

```text
Call Result: BUSY
Caller: 1002
Callee: 1001
Duration: 3.94s
Hangup Cause: USER_BUSY (17)
Dialstatus: BUSY
```

---

## v0.0.4 — usability and filtering — done

### Output options
- [x] JSON output: `--format json`
- [x] Colorized terminal output — MUST detect TTY (isatty) and support
      `--no-color`, or piping to a file fills it with ANSI codes
- [x] Filter by linkedid: `asterism analyze --linkedid 1779999013.2 cel.csv`
- [x] Filter by channel name
- [x] Filter by extension
- [x] Show only HANGUP events
- [x] Show only APP_START/APP_END events

### Robustness
- [x] `--skip-bad-lines`: tolerant mode that skips malformed CSV rows and
      reports a count at the end, instead of failing the whole parse.
      Production CEL data always has some dirt.

---

## v0.0.5 — statistics — done

Statistics is a different mode of operation: aggregate over many calls rather
than reconstruct one. Kept in its own version so it doesn't bloat v0.0.3.

- [x] Statistics mode (`--stats`)
- [x] Total calls
- [x] Answered calls
- [x] Failed calls
- [x] Busy calls
- [x] Average duration

---

## v0.1.0 — Asterisk full log correlation — done

The full log adds dialplan execution detail and SIP signaling that CEL omits.
Correlation is by channel name (the linkedid does not appear literally in the
full log — it shows as `[C-xxxxxxxx]`, so channel name is the join key).

- [x] Parse full Asterisk log
- [x] Extract channel lifecycle from logs
- [x] Correlate CEL events with log timestamps
- [x] Correlate CEL events with channel names
- [x] Show related log lines alongside CEL timeline

### Call outcome inference
- [ ] Automatically classify call outcome
- [ ] Detect BUSY scenarios
- [ ] Detect NO ANSWER scenarios
- [ ] Detect CONGESTION scenarios
- [ ] Detect NORMAL_CLEARING scenarios
- [ ] Detect CANCELLED scenarios

Note: call outcome inference from full log alone (no CEL/CDR) is deferred.
When CEL+CDR are present, outcome is already provided by CDR disposition.
Full-log-only outcome detection will be added in a future patch.

Example:

```text
Result: NO ANSWER
Reason: Dial timeout reached (30s)
```

---

## v0.2.0 — SIP signaling analysis — done

Parsing free-form SIP from the log is the hardest parsing in the project.
Requires verbose logging and `pjsip set logger on` to be active when the call
was made.

- [x] Parse SIP dialogs from logs
- [x] Parse INVITE transactions
- [x] Parse provisional responses (100/180/183)
- [x] Parse final responses (200/4xx/5xx/6xx)
- [x] Parse BYE/CANCEL flows
- [x] Correlate SIP dialogs with CEL channels

### Diagnostics
- [x] Detect codec negotiation (codecs from INVITE SDP shown in call header)
- [x] Detect RTP setup failures (native_rtp WARNING log lines)
- [x] Detect 4xx/5xx error responses
- [ ] Detect one-way audio indicators (deferred — requires RTP stats)
- [ ] Detect registration-related failures (deferred — requires REGISTER dialog)

---

## v0.3.0 — reporting and visualization — done

- [x] HTML report output (`--format html`)
- [x] Timeline visualization (timeline rendered inside HTML report)
- [x] Export individual calls (`--output <path>` + `--linkedid`)
- [x] Export filtered datasets (`--output <path>` + any filter flag)

### SIP ladder diagram
- [x] Generate SIP ladder diagram (`--ladder` flag for text; always shown in HTML)
- [x] Show call flow visually
- [x] Include SIP response codes
- [x] Include timing information

Example:

```text
   1002                    1001
    |                       |
    |------ INVITE -------->|
    |<----- 486 Busy -------|
    |                       |
    +-------- HANGUP -------+
```

---

## v0.4.0 — transfers, diagnostics, and quality-of-life

### Transfer awareness
The biggest correctness gap in the current analysis. CEL emits
`ATTENDEDTRANSFER` and `BLINDTRANSFER` events that asterism currently ignores,
causing transferred calls to appear fragmented and caller/callee detection to
be wrong after the transfer point.

- [x] Recognise `ATTENDEDTRANSFER` and `BLINDTRANSFER` CEL event types
- [x] Show transfer event in the timeline with from/to detail
- [x] Correctly identify caller and callee after a transfer
- [x] Show transfer target in call header

### Deferred diagnostics (carried from v0.1.0 / v0.2.0)
- [ ] Call outcome inference from full log alone (no CEL/CDR required)
- [ ] Detect one-way audio indicators (RTP stats in the full log)
- [ ] Detect registration-related failures (REGISTER SIP dialog parsing)

### Filter improvements
- [x] Filter by date range: `--from` / `--to` (timestamp or relative offset)
- [x] Filter by call duration: `--min-duration` / `--max-duration`
- [x] Filter by hangup cause: `--hangup-cause NORMAL_CLEARING`

### Export
- [x] CSV summary: one row per call with key fields (linkedid, start, duration,
      result, caller, callee, hangup cause) — good for spreadsheet analysis

### CEL schema flexibility
- [ ] Configurable CEL column mapping via a simple config flag or file, so
      users with non-standard `cel_custom.conf` layouts are not locked out
      (moved here from "Future / maybe" — blocking real users)

### Infrastructure (deferred from v0.0.3 infra block)
- [x] GitHub Actions: run `go test ./...` and `go vet` on every PR
- [x] Add `golangci-lint` to CI
- [ ] Build status badge in README
- [x] `docs/asterisk-setup.md`: how to configure CEL, CDR, and full log

---

## Future / maybe — NOT scheduled

These represent a change in the project's nature: from a batch analysis tool
into a long-running real-time daemon. That is a different kind of software
(persistent state, reconnection, backpressure, no memory leaks over days) and
arguably a separate project (`asterism-watch`?). Listed here so the ideas are
not lost, but deliberately unnumbered. Do not treat as a natural continuation
of v0.3.0.

### Live analysis
- [ ] Live tail mode (`--watch`): follow CEL file in real time
- [ ] Real-time call summaries

### Observability (would contradict the "not a monitoring dashboard" stance)
- [ ] Prometheus metrics export
- [ ] Structured event stream output
- [ ] WebSocket event feed

### Aspirational (north star — may never happen)
- [ ] Real-time anomaly detection
- [ ] Real-time SIP diagnostics

### Other future considerations
- [ ] Configurable CEL schema (today 13 columns are hardcoded; `cel_custom.conf`
      is user-customizable, so others may have different layouts)
- [ ] Better `--help` / man page when flag count grows
