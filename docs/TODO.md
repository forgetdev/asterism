# Roadmap & Known Issues

## v0.0.2 (next)
- [ ] Translate Q.850 hangupcause codes to human-readable strings
- [ ] Use µs/ms granularity when sub-millisecond gaps would round to 0s
- [ ] Strip outer parens from appdata when already parenthesized
- [ ] Parse CDR Master.csv and merge into Call summary

## v0.0.3
- [ ] Filter by linkedid: `asterism analyze --linkedid 1779999013.2 cel.csv`
- [ ] JSON output: `--format json` for machine consumption
- [ ] Hide LINKEDID_END from output (it's redundant noise)

## v0.1.0
- [ ] Parse full log for SIP signaling and dialplan trace
- [ ] Correlate full log with CEL by channel name
- [ ] Diagnostic inference (codec hints, native_rtp warnings)

## v0.2.0
- [ ] HTML output with SIP ladder diagram
- [ ] Live tail mode (--watch)
