# Asterisk setup for asterism

This document describes how to configure Asterisk so that it produces the log
files asterism can read. Without this configuration CEL and CDR will not be
written and asterism will have no input.

---

## CEL (Channel Event Logging)

CEL is the primary input for asterism. It must be explicitly enabled — it is
**off by default** in a fresh Asterisk installation.

### Why CEL might not be generating

In order of likelihood:

1. **`enable=yes` missing from `cel.conf`** — the most common reason.
   Edit `/etc/asterisk/cel.conf`:
   ```ini
   [general]
   enable=yes
   ```

2. **`cel_custom.conf` not configured** — the CSV backend needs a `[mappings]`
   section. Without it the module loads but writes nothing.

3. **`cel_custom.so` not loaded** — check that the module appears in
   `/etc/asterisk/modules.conf` (or is not explicitly noloaded):
   ```
   asterisk -rx "module show like cel_custom"
   ```
   It should show `Running`.

4. **Wrong output path** — by default `cel_custom.conf` writes to
   `/var/log/asterisk/cel-custom/Master.csv`. Verify the directory exists and
   is writable by the `asterisk` user:
   ```bash
   ls -la /var/log/asterisk/cel-custom/
   ```

5. **Module not reloaded after config change** — after editing any of the above
   files, reload the module:
   ```
   asterisk -rx "module reload cel_custom"
   ```

### `/etc/asterisk/cel.conf`

```ini
[general]
enable=yes
; Log all apps. Set to specific apps to reduce noise.
apps=all
; Log all events asterism uses.
events=CHAN_START,CHAN_END,ANSWER,HANGUP,APP_START,APP_END,BRIDGE_ENTER,BRIDGE_EXIT,BLINDTRANSFER,ATTENDEDTRANSFER,LINKEDID_END
```

### `/etc/asterisk/cel_custom.conf`

asterism expects exactly **13 columns** in this order:

```ini
[mappings]
Master.csv => ${eventtype},${eventtime},${calleridnum},${calleridname},${channel},${exten},${context},${uniqueid},${linkedid},${bridgepeer},${appname},${appdata},${eventextra}
```

Changing this column layout will break asterism's strict parser. If your
installation uses a different layout, use `--skip-bad-lines` as a workaround
until configurable schema support is added.

### Verify CEL is working

Make a test call, then check:

```bash
tail -f /var/log/asterisk/cel-custom/Master.csv
```

You should see a line per channel event. If the file is empty or missing after
a call, work through the checklist above.

---

## CDR (Call Detail Records)

CDR is optional but enables asterism to show call disposition (ANSWERED/BUSY/
etc.), billable seconds, and source/destination numbers.

### `/etc/asterisk/cdr.conf`

```ini
[general]
enable=yes
; Required for asterism to join CDR rows to CEL calls.
loguniqueid=yes
; Optional but used by asterism.
loguserfield=yes
; Use GMT timestamps so CDR times are consistent regardless of server timezone.
usegmtime=yes
```

### `/etc/asterisk/cdr_csv.conf`

```ini
[general]
usegmtime=yes
loguniqueid=yes
loguserfield=yes
```

The CDR CSV is written to `/var/log/asterisk/cdr-csv/Master.csv` by default.

### Column count

With `loguniqueid=yes` and `loguserfield=yes` the CSV has **18 columns**.
Without them it has 16, and asterism's strict CDR parser will reject every row.
Use `--skip-bad-lines` if your CDR has a different column count.

---

## Full log (optional)

The full log enables asterism to show dialplan execution and SIP signaling
alongside the CEL timeline.

### Enable verbose logging

```
asterisk -rx "core set verbose 3"
asterisk -rx "core set debug 0"
```

Or set permanently in `/etc/asterisk/logger.conf`:

```ini
[logfiles]
full => notice,warning,error,verbose
```

### Enable PJSIP SIP message logging

For the SIP ladder diagram and SIP diagnostics to work, PJSIP message logging
must be active when the call is made:

```
asterisk -rx "pjsip set logger on"
```

Or enable it permanently via the PJSIP logger module configuration. Note that
SIP message logging is verbose — enable it only for debugging sessions.

Pass the full log path to asterism with `--full-log`:

```bash
asterism analyze --full-log /var/log/asterisk/full --cdr Master.csv cel.csv
```
