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

## v0.2.0 — SIP signaling analysis

Parsing free-form SIP from the log is the hardest parsing in the project.
Requires verbose logging and `pjsip set logger on` to be active when the call
was made.

- [ ] Parse SIP dialogs from logs
- [ ] Parse INVITE transactions
- [ ] Parse provisional responses (100/180/183)
- [ ] Parse final responses (200/4xx/5xx/6xx)
- [ ] Parse BYE/CANCEL flows
- [ ] Correlate SIP dialogs with CEL channels

### Diagnostics
- [ ] Detect codec negotiation issues
- [ ] Detect RTP setup failures
- [ ] Detect native_rtp warnings
- [ ] Detect one-way audio indicators
- [ ] Detect registration-related failures

---

## v0.3.0 — reporting and visualization

- [ ] HTML report output
- [ ] Timeline visualization
- [ ] Export individual calls
- [ ] Export filtered datasets

### SIP ladder diagram
- [ ] Generate SIP ladder diagram
- [ ] Show call flow visually
- [ ] Include SIP response codes
- [ ] Include timing information

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
