# asterism architecture

## Pipeline

```
CEL CSV file
   ↓
cel.Parse       → []model.Event   (parsing)
   ↓
correlate.ByLinkedID → []model.Call  (grouping + sorting)
   ↓
render.Text     → stdout         (output)
```

Each stage is pure with respect to the next: parsing does no I/O on output,
correlation does no I/O at all, rendering writes to an `io.Writer` (which the
CLI binds to `os.Stdout`, but tests can bind to a `bytes.Buffer`).

This separation matters because we will add formats (HTML, JSON) and sources
(CDR, full log) without touching the correlation logic.

## Domain model

A **Call** is a logical telephony interaction, identified by its `LinkedID`.

A **Channel** (Asterisk term) is one leg of a call, identified by its
`UniqueID`. All channels of the same Call share a `LinkedID`. The originating
channel's `UniqueID` equals the `LinkedID`.

An **Event** is one row in CEL CSV. Channels emit events during their
lifetime: CHAN_START at creation, ANSWER when picked up, BRIDGE_ENTER/EXIT
when joining/leaving a media bridge, HANGUP at termination.

A typical two-party answered call produces ~15 events: two CHAN_STARTs,
two ANSWERs, two BRIDGE_ENTERs, an APP_START/END for the Dial app,
two BRIDGE_EXITs, two HANGUPs, two CHAN_ENDs, and one LINKEDID_END.

## Why CEL first, not full log

The full log is text-oriented and parsing it reliably requires regex over
free-form messages whose exact format varies between Asterisk versions and
verbose levels. CEL, by contrast, is structured CSV with a stable schema
defined by `cel_custom.conf`.

Starting with CEL gives us a working call reconstruction with one parser.
Full log integration arrives in a later version as enrichment — it adds
SIP signaling detail and dialplan execution traces that CEL omits, but is
not required for the basic timeline.

## What's intentionally absent in v0.0.1

- **Hangup cause translation.** CEL emits `hangupcause: 16` etc. We display
  the raw JSON blob without decoding Q.850 codes. Later versions will
  translate to human-readable strings.
- **Bridge merging.** When a call has multiple bridges (e.g., transfers),
  we treat each bridge join/leave as an independent event. We do not yet
  reason about which channels were bridged together at a given moment.
- **Diagnostic inference.** "This call probably failed because of codec
  mismatch" requires combining hangupcause, bridge_technology, and timing.
  That logic lives in a future `diagnose` package.

## Coding conventions

- Packages under `internal/` are not importable by other modules. This is
  deliberate: the API is unstable. Once we have an exported package suitable
  for library use, it moves to `pkg/`.
- No third-party dependencies in v0.0.1. Standard library only. This keeps
  the project trivially auditable and reproducible.
- Each package has a doc comment at the top of one file explaining its role.
- Errors include context via `fmt.Errorf("%w")` wrapping.
