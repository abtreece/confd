# Logging

confd logs everything to stdout. You can control the types of messages that get printed by using the `--log-level` flag and corresponding configuration file settings. See the [Configuration Guide](configuration-guide.md) for more details.

**Note**: confd now uses Go's standard library `log/slog` for logging, providing better performance and structured logging capabilities.

## Log Levels

Use the `--log-level` flag to set the verbosity. Valid levels are: `panic`, `fatal`, `error`, `warn`, `info`, and `debug`.

## Log Formats

confd supports two log formats, controlled by the `--log-format` flag:

### Text Format (default)

The default text format includes timestamp, hostname, process name, PID, level, and message. When using structured logging, attributes are appended as key-value pairs:

```
2013-11-03T19:04:53-08:00 confd[21356]: INFO SRV domain set to confd.io
2013-11-03T19:04:53-08:00 confd[21356]: INFO Starting confd
2013-11-03T19:04:53-08:00 confd[21356]: INFO etcd nodes set to http://etcd0.confd.io:4001, http://etcd1.confd.io:4001
2013-11-03T19:04:54-08:00 confd[21356]: INFO Target config /tmp/myconf2.conf out of sync
2013-11-03T19:04:54-08:00 confd[21356]: INFO Target config /tmp/myconf2.conf has been updated
```

With structured logging attributes:
```
2024-01-14T10:30:00-08:00 hostname confd[21356]: INFO Backend configured backend=etcd node_count=3
```

### JSON Format

Use `--log-format=json` for structured JSON output, which is easier to parse with log indexing solutions (ELK, Splunk, etc.):

```bash
confd env --log-format=json --onetime
```

```json
{"time":"2024-01-14T10:30:00Z","level":"INFO","msg":"Backend set to env"}
{"time":"2024-01-14T10:30:00Z","level":"INFO","msg":"Starting confd"}
{"time":"2024-01-14T10:30:01Z","level":"INFO","msg":"Target config /tmp/myconf.conf has been updated"}
```

Or set in `confd.toml`:

```toml
log-format = "json"
```

## Structured Logging

confd supports structured logging using Go's `log/slog` package. This allows you to attach key-value pairs to log messages for better searchability and analysis:

```go
// Traditional printf-style (still supported)
log.Info("Processing template %s", templatePath)

// Structured logging with context
log.InfoContext(ctx, "Processing template", 
    "path", templatePath,
    "backend", backendName,
    "duration_ms", elapsed.Milliseconds())

// Creating a logger with persistent attributes
templateLogger := log.With("template", "app.conf", "component", "renderer")
templateLogger.Info("Template rendered successfully")
```

Structured logging benefits:
- **Better searchability**: Query logs by specific field values
- **Machine-readable**: Easy to parse and analyze programmatically
- **Performance**: Efficient key-value encoding with minimal allocations
- **Context propagation**: Automatic trace/span ID extraction from context
