# confd Architecture

This document describes the internal architecture of confd to help contributors and users understand how the system works.

## Table of Contents

- [High-Level Architecture](#high-level-architecture)
- [Package Structure](#package-structure)
- [Data Flow](#data-flow)
- [Processing Modes](#processing-modes)
- [Sequence Diagrams](#sequence-diagrams)
- [Extension Points](#extension-points)

## High-Level Architecture

confd follows a modular architecture with clear separation between configuration parsing, backend communication, template processing, and file management.

### ASCII Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                  confd                                       │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────────────────┐  │
│  │   CLI/Kong   │───▶│    Config    │───▶│         Processor            │  │
│  │   Parsing    │    │   Loading    │    │  (Interval/Watch/Batch)      │  │
│  └──────────────┘    └──────────────┘    └──────────────────────────────┘  │
│         │                   │                          │                    │
│         ▼                   ▼                          ▼                    │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────────────────┐  │
│  │    Flags     │    │  conf.d/     │    │     Template Resources       │  │
│  │   Env Vars   │    │  *.toml      │    │   (parse, render, sync)      │  │
│  │  confd.toml  │    │              │    └──────────────────────────────┘  │
│  └──────────────┘    └──────────────┘              │         │             │
│                                                    │         │             │
│                                                    ▼         ▼             │
│                      ┌──────────────┐    ┌────────────┐ ┌────────────┐     │
│                      │   Backend    │◀───│  GetValues │ │  Output    │     │
│                      │   Client     │    │  WatchPfx  │ │  Writer    │     │
│                      └──────────────┘    └────────────┘ └────────────┘     │
│                             │                                │             │
│                             ▼                                ▼             │
│                      ┌──────────────────────────┐     ┌────────────┐       │
│                      │        Backends          │     │ check_cmd  │       │
│                      ├──────────────────────────┤     │ reload_cmd │       │
│                      │ etcd    │ consul │ vault │     └────────────┘       │
│                      │ redis   │ zk     │ ssm   │                          │
│                      │ dynamodb│ env    │ file  │                          │
│                      │ acm     │ imds   │ sm    │                          │
│                      └──────────────────────────┘                          │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Mermaid Diagram

```mermaid
graph TB
    subgraph "Entry Point"
        CLI[CLI/Kong Parser]
        CFG[Config Loader]
    end

    subgraph "Processing Layer"
        PROC[Processor Interface]
        INT[IntervalProcessor]
        WATCH[WatchProcessor]
        BATCH[BatchWatchProcessor]
    end

    subgraph "Template Layer"
        TR[TemplateResource]
        TC[TemplateCache]
        TF[TemplateFuncs]
        MKV[memkv Store]
    end

    subgraph "Backend Layer"
        BC[Backend Client]
        SC[StoreClient Interface]
    end

    subgraph "Backends"
        ETCD[etcd]
        CONSUL[Consul]
        VAULT[Vault]
        REDIS[Redis]
        ZK[Zookeeper]
        SSM[AWS SSM]
        DDB[DynamoDB]
        ENV[Environment]
        FILE[File]
        ACM[AWS ACM]
        IMDS[AWS IMDS]
        SM[Secrets Manager]
    end

    subgraph "Output"
        FS[File Stager]
        CMD[Command Executor]
        DEST[Destination Files]
    end

    CLI --> CFG
    CFG --> PROC
    PROC --> INT
    PROC --> WATCH
    PROC --> BATCH

    INT --> TR
    WATCH --> TR
    BATCH --> TR

    TR --> TC
    TR --> TF
    TR --> MKV
    TR --> BC
    TR --> FS

    BC --> SC
    SC --> ETCD
    SC --> CONSUL
    SC --> VAULT
    SC --> REDIS
    SC --> ZK
    SC --> SSM
    SC --> DDB
    SC --> ENV
    SC --> FILE
    SC --> ACM
    SC --> IMDS
    SC --> SM

    FS --> DEST
    FS --> CMD
```

## Package Structure

```
cmd/confd/
├── main.go          # Entry point
├── cli.go           # CLI argument definitions (Kong)
└── config.go        # Configuration file loading

pkg/
├── backends/        # Backend abstraction layer
│   ├── client.go    # StoreClient interface & Factory
│   ├── config.go    # Backend configuration
│   ├── types/       # Shared types (HealthResult)
│   ├── acm/         # AWS ACM backend
│   ├── consul/      # Consul backend
│   ├── dynamodb/    # DynamoDB backend
│   ├── env/         # Environment variables backend
│   ├── etcd/        # etcd backend
│   ├── file/        # File (YAML/JSON) backend
│   ├── imds/        # AWS EC2 IMDS backend
│   ├── redis/       # Redis backend
│   ├── secretsmanager/  # AWS Secrets Manager
│   ├── ssm/         # AWS SSM Parameter Store
│   ├── vault/       # HashiCorp Vault backend
│   └── zookeeper/   # Zookeeper backend
│
├── template/        # Template processing
│   ├── processor.go         # Processor interface & implementations
│   ├── resource.go          # TemplateResource core logic
│   ├── template_funcs.go    # Custom template functions
│   ├── template_cache.go    # Compiled template caching
│   ├── template_renderer.go # Template rendering
│   ├── backend_fetcher.go   # Backend data fetching
│   ├── file_stager.go       # File staging and syncing
│   ├── command_executor.go  # check_cmd/reload_cmd execution
│   ├── client_cache.go      # Backend client caching
│   ├── include.go           # Template includes
│   ├── validate.go          # Configuration validation
│   ├── preflight.go         # Pre-flight checks
│   └── errors.go            # Error types and aggregation
│
├── memkv/           # In-memory key-value store
│   ├── store.go     # Store implementation
│   └── kvpair.go    # KVPair type
│
├── metrics/         # Prometheus metrics
│   ├── metrics.go   # Metric definitions
│   ├── backend.go   # Backend instrumentation wrapper
│   └── health.go    # Health check handlers
│
├── service/         # Service management
│   ├── shutdown.go  # Graceful shutdown coordination
│   ├── reload.go    # SIGHUP reload handling
│   └── systemd.go   # systemd integration
│
├── log/             # Structured logging (slog)
│   └── log.go       # Log wrapper
│
└── util/            # Utilities
    ├── util.go      # File operations
    ├── diff.go      # Diff generation
    └── format.go    # Output formatting
```

### Package Dependencies

```mermaid
graph TD
    CMD[cmd/confd] --> BACKENDS[pkg/backends]
    CMD --> TEMPLATE[pkg/template]
    CMD --> METRICS[pkg/metrics]
    CMD --> SERVICE[pkg/service]
    CMD --> LOG[pkg/log]

    TEMPLATE --> BACKENDS
    TEMPLATE --> MEMKV[pkg/memkv]
    TEMPLATE --> METRICS
    TEMPLATE --> LOG
    TEMPLATE --> UTIL[pkg/util]

    BACKENDS --> TYPES[pkg/backends/types]
    BACKENDS --> LOG

    METRICS --> BACKENDS
    METRICS --> LOG

    SERVICE --> BACKENDS
    SERVICE --> LOG
```

## Data Flow

### Configuration Loading

Configuration is loaded from multiple sources with the following precedence (highest to lowest):

1. Command-line flags
2. Environment variables (`CONFD_*` prefix)
3. Configuration file (`/etc/confd/confd.toml`)
4. Default values

```mermaid
flowchart LR
    DEFAULTS[Defaults] --> TOML[confd.toml]
    TOML --> ENV[Environment]
    ENV --> FLAGS[CLI Flags]
    FLAGS --> FINAL[Final Config]
```

### Template Processing Pipeline

Each template resource goes through a well-defined processing pipeline:

```mermaid
flowchart TB
    subgraph "1. Load"
        TOML[conf.d/*.toml] --> PARSE[Parse TOML]
        PARSE --> TR[TemplateResource]
    end

    subgraph "2. Fetch"
        TR --> KEYS[Get Keys]
        KEYS --> BACKEND[Backend GetValues]
        BACKEND --> STORE[memkv Store]
    end

    subgraph "3. Render"
        STORE --> TMPL[Load Template]
        TMPL --> CACHE[Template Cache]
        CACHE --> FUNCS[Apply Functions]
        FUNCS --> RENDER[Render Output]
    end

    subgraph "4. Stage"
        RENDER --> STAGE[Create Stage File]
        STAGE --> VALIDATE[Format Validation]
        VALIDATE --> COMPARE[Compare with Dest]
    end

    subgraph "5. Sync"
        COMPARE -->|Changed| CHECK[Run check_cmd]
        CHECK -->|Success| SYNC[Atomic Rename]
        SYNC --> RELOAD[Run reload_cmd]
        COMPARE -->|Unchanged| SKIP[Skip Update]
    end
```

### Key Resolution

Keys specified in template resources are resolved hierarchically:

```
Global Prefix (/production) + Resource Prefix (/myapp) + Key (/database/host)
                                    ↓
                    /production/myapp/database/host
```

## Processing Modes

confd supports three processing modes, each suited for different use cases:

### 1. One-time Mode (`--onetime`)

Processes all templates once and exits. Useful for initialization scripts.

```
Start → Load Templates → Process All → Exit
```

### 2. Interval Mode (default)

Polls the backend at a fixed interval (default: 600 seconds).

```mermaid
flowchart TB
    START[Start] --> LOAD[Load Templates]
    LOAD --> PROCESS[Process All]
    PROCESS --> WAIT[Wait Interval]
    WAIT --> RELOAD{SIGHUP?}
    RELOAD -->|Yes| LOAD
    RELOAD -->|No| STOP{Stop Signal?}
    STOP -->|No| LOAD
    STOP -->|Yes| SHUTDOWN[Graceful Shutdown]
```

### 3. Watch Mode (`--watch`)

Watches the backend for changes and processes templates immediately when keys change.

```mermaid
flowchart TB
    START[Start] --> LOAD[Load Templates]
    LOAD --> SPAWN[Spawn Watchers]
    SPAWN --> WATCH[Watch Each Prefix]
    WATCH --> CHANGE{Key Changed?}
    CHANGE -->|Yes| DEBOUNCE[Debounce]
    DEBOUNCE --> PROCESS[Process Template]
    PROCESS --> WATCH
    CHANGE -->|No| STOP{Stop Signal?}
    STOP -->|No| WATCH
    STOP -->|Yes| SHUTDOWN[Graceful Shutdown]
```

#### Batch Watch Mode (`--watch --batch-interval`)

Collects changes across all templates and processes them together.

```mermaid
flowchart TB
    START[Start] --> LOAD[Load Templates]
    LOAD --> SPAWN[Spawn Watchers]
    SPAWN --> WATCH[Watch All Prefixes]
    WATCH --> CHANGE{Key Changed?}
    CHANGE -->|Yes| QUEUE[Queue Template]
    QUEUE --> TIMER{Timer Running?}
    TIMER -->|No| START_TIMER[Start Batch Timer]
    START_TIMER --> WATCH
    TIMER -->|Yes| WATCH

    BATCH[Batch Timer Fires] --> PROCESS[Process All Queued]
    PROCESS --> WATCH
```

## Sequence Diagrams

### Startup Sequence

```mermaid
sequenceDiagram
    participant Main as main()
    participant Kong as Kong Parser
    participant Config as Config Loader
    participant Backend as Backend Factory
    participant Metrics as Metrics Server
    participant Processor as Processor
    participant Systemd as Systemd Notifier

    Main->>Kong: Parse CLI args
    Kong->>Config: Load config file
    Config->>Config: Process env vars
    Config->>Backend: Create StoreClient
    Backend->>Backend: Initialize connection

    alt Metrics Enabled
        Main->>Metrics: Start HTTP server
        Metrics->>Backend: Wrap with instrumentation
    end

    Main->>Processor: Create processor
    Main->>Processor: Start processing

    alt Systemd Enabled
        Main->>Systemd: Notify READY
        Systemd->>Systemd: Start watchdog
    end

    Main->>Main: Wait for signals
```

### Template Render Cycle

```mermaid
sequenceDiagram
    participant P as Processor
    participant TR as TemplateResource
    participant BF as BackendFetcher
    participant BC as StoreClient
    participant MKV as memkv.Store
    participant TM as TemplateRenderer
    participant FS as FileStager
    participant CE as CommandExecutor

    P->>TR: process()
    TR->>BF: FetchData(keys)
    BF->>BC: GetValues(keys)
    BC-->>BF: map[string]string
    BF->>MKV: Store values

    TR->>TM: Render(store)
    TM->>TM: Load/cache template
    TM->>TM: Execute with funcMap
    TM-->>TR: rendered content

    TR->>FS: Stage(content)
    FS->>FS: Write to temp file
    FS->>FS: Validate format (optional)
    FS->>FS: Compare with dest

    alt Content Changed
        FS-->>TR: changed=true
        TR->>CE: RunCheckCmd()
        CE-->>TR: success
        TR->>FS: Sync()
        FS->>FS: Atomic rename
        FS->>FS: Set permissions
        TR->>CE: RunReloadCmd()
    else Content Unchanged
        FS-->>TR: changed=false
        TR->>TR: Skip update
    end
```

### Graceful Shutdown

```mermaid
sequenceDiagram
    participant Signal as Signal Handler
    participant Main as Main Loop
    participant Systemd as Systemd
    participant Proc as Processor
    participant Shutdown as ShutdownManager
    participant Metrics as Metrics Server
    participant Backend as StoreClient

    Signal->>Main: SIGTERM/SIGINT
    Main->>Systemd: Notify STOPPING
    Main->>Main: Cancel context
    Main->>Proc: Close stopChan

    Proc->>Proc: Stop watch loops
    Proc->>Proc: Process pending (batch mode)
    Proc-->>Main: Close doneChan

    Main->>Shutdown: Shutdown()

    Note over Shutdown: Phase 1
    Shutdown->>Shutdown: Wait for in-flight cmds

    Note over Shutdown: Phase 2
    Shutdown->>Metrics: Shutdown(ctx)
    Metrics-->>Shutdown: Done

    Note over Shutdown: Phase 3
    Shutdown->>Backend: Close()
    Backend-->>Shutdown: Done

    Shutdown-->>Main: Complete
    Main->>Main: Exit(0)
```

### SIGHUP Reload

```mermaid
sequenceDiagram
    participant Signal as Signal Handler
    participant Main as Main Loop
    participant Reload as ReloadManager
    participant Systemd as Systemd
    participant Proc as Processor

    Signal->>Main: SIGHUP
    Main->>Systemd: Notify RELOADING
    Main->>Reload: TriggerReload()
    Reload->>Proc: reloadChan <- signal

    Proc->>Proc: Stop current watches
    Proc->>Proc: Reload conf.d/*.toml
    Proc->>Proc: Restart watches

    Main->>Systemd: Notify READY
```

## Extension Points

### Adding a New Backend

1. Create a new package under `pkg/backends/`
2. Implement the `StoreClient` interface:

```go
type StoreClient interface {
    GetValues(ctx context.Context, keys []string) (map[string]string, error)
    WatchPrefix(ctx context.Context, prefix string, keys []string,
                waitIndex uint64, stopChan chan bool) (uint64, error)
    HealthCheck(ctx context.Context) error
    Close() error
}
```

3. Add the backend to the factory in `pkg/backends/client.go`
4. Add CLI command in `cmd/confd/cli.go`

### Adding Template Functions

Add functions to `pkg/template/template_funcs.go`:

```go
func init() {
    funcMap["myFunc"] = func(args ...any) string {
        // Implementation
    }
}
```

### Custom Health Checks

Implement the optional `DetailedHealthChecker` interface for extended diagnostics:

```go
type DetailedHealthChecker interface {
    HealthCheckDetailed(ctx context.Context) (*HealthResult, error)
}
```

## Key Design Decisions

### Why Factory Pattern for Backends?

The `backends.New()` factory allows runtime backend selection based on configuration, making it easy to switch backends without code changes.

### Why Separate Processor Implementations?

Different processing strategies (interval vs watch vs batch) have fundamentally different control flows. Separate implementations keep each mode simple and maintainable.

### Why memkv Store?

The in-memory key-value store provides a consistent interface for templates regardless of backend. It also enables features like `getv`, `gets`, and `lsdir` template functions.

### Why Atomic File Updates?

Writing to a temporary file and using atomic rename ensures that:
- Destination files are never partially written
- Applications reading config see complete files
- File permissions are set correctly before the file is visible

### Why Debouncing in Watch Mode?

Backend changes often come in bursts (e.g., multiple keys updated together). Debouncing prevents unnecessary template re-renders and reload command executions.
