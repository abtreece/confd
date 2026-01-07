# Logging

confd logs everything to stdout. You can control the types of messages that get printed by using the `--log-level` flag and corresponding configuration file settings. See the [Configuration Guide](configuration-guide.md) for more details.

## Log Levels

Use the `--log-level` flag to set the verbosity. Valid levels are: `panic`, `fatal`, `error`, `warn`, `info`, and `debug`.

## Log Formats

confd supports two log formats, controlled by the `--log-format` flag:

### Text Format (default)

The default text format includes timestamp, hostname, process name, PID, level, and message:

```
2013-11-03T19:04:53-08:00 confd[21356]: INFO SRV domain set to confd.io
2013-11-03T19:04:53-08:00 confd[21356]: INFO Starting confd
2013-11-03T19:04:53-08:00 confd[21356]: INFO etcd nodes set to http://etcd0.confd.io:4001, http://etcd1.confd.io:4001
2013-11-03T19:04:54-08:00 confd[21356]: INFO Target config /tmp/myconf2.conf out of sync
2013-11-03T19:04:54-08:00 confd[21356]: INFO Target config /tmp/myconf2.conf has been updated
```

### JSON Format

Use `--log-format=json` for structured JSON output, which is easier to parse with log indexing solutions (ELK, Splunk, etc.):

```bash
confd env --log-format=json --onetime
```

```json
{"level":"info","msg":"Backend set to env","time":"2024-01-07T10:30:00Z"}
{"level":"info","msg":"Starting confd","time":"2024-01-07T10:30:00Z"}
{"level":"info","msg":"Target config /tmp/myconf.conf has been updated","time":"2024-01-07T10:30:01Z"}
```

Or set in `confd.toml`:

```toml
log-format = "json"
```
