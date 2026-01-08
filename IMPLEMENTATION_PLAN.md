# Implementation Plan: Issues #342 and #344

## Overview

This document outlines the implementation plan for:
- **Issue #342**: Configurable graceful shutdown with drain period
- **Issue #344**: Troubleshooting guide documentation

---

## Issue #342: Graceful Shutdown Implementation

### Current State Analysis

#### Critical Bug in Existing Code
Location: `cmd/confd/cli.go:427-439`

**Problem**: Signal handler closes `doneChan` instead of signaling `stopChan`:
```go
case s := <-signalChan:
    log.Info("Captured %v. Exiting...", s)
    close(doneChan)  // ❌ Wrong channel
```

**Impact**:
- Processors wait on `stopChan`, not `doneChan`
- Closing `doneChan` doesn't actually stop processors gracefully
- Creates potential for double-close panic (processor also defers `close(doneChan)`)
- Operations may be interrupted mid-execution

#### Existing Shutdown Logic
- **IntervalProcessor** (`pkg/template/processor.go:37-66`): Basic stopChan handling
- **WatchProcessor** (`pkg/template/processor.go:68-147`): Stops debounce timers
- **BatchWatchProcessor** (`pkg/template/processor.go:150-260`): Has good cleanup logic (processes pending changes before exit)

### Implementation Steps

#### 1. Add Configuration Fields for Graceful Shutdown

**File**: `cmd/confd/config.go`

Add to `TOMLConfig` struct:
```go
type TOMLConfig struct {
    // ... existing fields ...

    // Graceful shutdown settings
    ShutdownTimeout  int    `toml:"shutdown_timeout"`   // seconds, default 15
    ShutdownCleanup  string `toml:"shutdown_cleanup"`   // path to cleanup script
}
```

**File**: `cmd/confd/cli.go`

Add to `CLI` struct:
```go
type CLI struct {
    // ... existing fields ...

    // Shutdown flags
    ShutdownTimeout  int    `name:"shutdown-timeout" help:"graceful shutdown timeout in seconds" default:"15"`
    ShutdownCleanup  string `name:"shutdown-cleanup" help:"cleanup script to run on shutdown"`
}
```

Update `loadConfigFile()` in `config.go` to apply TOML settings.

#### 2. Create Shutdown Manager Package

**New File**: `pkg/shutdown/manager.go`

```go
package shutdown

import (
    "context"
    "os"
    "os/exec"
    "os/signal"
    "sync"
    "syscall"
    "time"

    "github.com/abtreece/confd/pkg/log"
)

type Manager struct {
    timeout       time.Duration
    cleanupScript string
    stopChan      chan struct{}
    doneChan      chan struct{}
    errChan       chan error
    mu            sync.Mutex
    started       bool
    shutdownFuncs []func() error
}

type Config struct {
    Timeout       time.Duration
    CleanupScript string
    StopChan      chan struct{}
    DoneChan      chan struct{}
    ErrChan       chan error
}

func New(cfg Config) *Manager
func (m *Manager) RegisterCleanup(fn func() error)
func (m *Manager) Start() error
func (m *Manager) Shutdown(ctx context.Context) error
func (m *Manager) executeCleanup() error
```

**Features**:
- Centralized signal handling for SIGTERM, SIGINT, SIGQUIT
- Timeout management with force-kill on timeout
- Cleanup hook execution
- Status reporting with detailed logging
- Support for registering cleanup functions

#### 3. Implement Signal Handler with Different Behaviors

**In**: `pkg/shutdown/manager.go`

```go
func (m *Manager) Start() error {
    signalChan := make(chan os.Signal, 1)

    // SIGTERM and SIGINT: graceful shutdown
    signal.Notify(signalChan, syscall.SIGTERM, syscall.SIGINT)

    // SIGQUIT: immediate shutdown
    quitChan := make(chan os.Signal, 1)
    signal.Notify(quitChan, syscall.SIGQUIT)

    go func() {
        select {
        case s := <-signalChan:
            log.Info("Received signal %v, initiating graceful shutdown (timeout: %v)", s, m.timeout)
            ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
            defer cancel()

            if err := m.Shutdown(ctx); err != nil {
                log.Error("Shutdown error: %v", err)
                m.errChan <- err
            }

        case s := <-quitChan:
            log.Warn("Received signal %v, forcing immediate shutdown", s)
            close(m.stopChan)
            close(m.doneChan)
        }
    }()

    return nil
}
```

#### 4. Update Main Loop in cli.go

**File**: `cmd/confd/cli.go`

Modify `run()` function:

```go
func run(cli *CLI, backendCfg backends.Config) error {
    // ... existing setup ...

    stopChan := make(chan struct{})
    doneChan := make(chan struct{})
    errChan := make(chan error, 10)

    // Create shutdown manager
    shutdownMgr := shutdown.New(shutdown.Config{
        Timeout:       time.Duration(cli.ShutdownTimeout) * time.Second,
        CleanupScript: cli.ShutdownCleanup,
        StopChan:      stopChan,
        DoneChan:      doneChan,
        ErrChan:       errChan,
    })

    // Register processor cleanup if needed
    if batchProc, ok := processor.(*template.BatchWatchProcessor); ok {
        shutdownMgr.RegisterCleanup(func() error {
            return batchProc.DrainPending()
        })
    }

    // Start shutdown manager
    if err := shutdownMgr.Start(); err != nil {
        return err
    }

    // Start processor
    go processor.Process()

    // Wait for completion or error
    select {
    case err := <-errChan:
        log.Error("%s", err.Error())
        return err
    case <-doneChan:
        log.Info("Shutdown complete")
        return nil
    }
}
```

#### 5. Enhance Processor Shutdown Handling

**File**: `pkg/template/processor.go`

**IntervalProcessor**:
- Add timeout context to prevent hanging on backend calls
- Log shutdown progress

**WatchProcessor**:
- Ensure all monitor goroutines properly cleanup
- Add pending operation tracking

**BatchWatchProcessor**:
- Already has good logic, extract to public method:

```go
func (p *batchWatchProcessor) DrainPending() error {
    p.mu.Lock()
    pending := p.pending
    p.pending = make(map[string]*TemplateResource)
    p.mu.Unlock()

    if len(pending) > 0 {
        log.Info("Draining %d pending template changes before shutdown", len(pending))
        for _, t := range pending {
            if err := t.process(); err != nil {
                return err
            }
        }
    }
    return nil
}
```

#### 6. Add Shutdown Logging and Status Reporting

**In**: `pkg/shutdown/manager.go`

```go
func (m *Manager) Shutdown(ctx context.Context) error {
    log.Info("=== Graceful Shutdown Initiated ===")
    log.Info("Step 1/4: Stopping new events")

    close(m.stopChan)

    log.Info("Step 2/4: Waiting for in-flight operations (timeout: %v)", m.timeout)

    // Wait for processor to finish or timeout
    select {
    case <-m.doneChan:
        log.Info("All operations completed successfully")
    case <-ctx.Done():
        log.Warn("Shutdown timeout exceeded, forcing termination")
        return fmt.Errorf("shutdown timeout exceeded")
    }

    log.Info("Step 3/4: Executing cleanup hooks")
    if err := m.executeCleanup(); err != nil {
        log.Error("Cleanup hook failed: %v", err)
        return err
    }

    log.Info("Step 4/4: Closing backend connections")
    // Backend cleanup happens via defer in processor

    log.Info("=== Graceful Shutdown Complete ===")
    return nil
}
```

#### 7. Add Context Timeout to Backend Operations

**Files**: Various `pkg/backends/*/client.go`

Add context timeout to `GetValues()` and `WatchPrefix()` operations to prevent hanging during shutdown:

```go
func (c *Client) GetValues(keys []string) (map[string]string, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Use ctx in backend calls
}
```

#### 8. Update Tests

**New File**: `pkg/shutdown/manager_test.go`

Test cases:
- Signal handling (SIGTERM, SIGINT, SIGQUIT)
- Graceful shutdown with timeout
- Cleanup hook execution
- Force termination on timeout
- Multiple cleanup functions

**Update**: `cmd/confd/cli_test.go`

Test cases:
- Shutdown manager integration
- Signal propagation to processors
- doneChan closing after processor completion

#### 9. Documentation Updates

**File**: `docs/configuration-guide.md`

Add section:
```markdown
## Graceful Shutdown Configuration

### shutdown_timeout (default: 15)
Maximum time in seconds to wait for graceful shutdown before forcing termination.

### shutdown_cleanup
Path to a script that will be executed during shutdown for cleanup tasks.

### Signal Handling
- **SIGTERM/SIGINT**: Graceful shutdown with timeout
- **SIGQUIT**: Immediate forced shutdown
```

**File**: `docs/command-line-flags.md`

Add:
- `--shutdown-timeout`
- `--shutdown-cleanup`

**File**: `README.md`

Add section about graceful shutdown and Kubernetes integration.

### Testing Strategy

1. **Unit Tests**: Test shutdown manager independently
2. **Integration Tests**: Test with actual backends and processors
3. **Manual Testing**: Test with Kubernetes pod termination
4. **Signal Testing**: Verify different signal behaviors

### Rollout Considerations

1. Default timeout of 15 seconds is conservative
2. Kubernetes users should set pod `terminationGracePeriodSeconds` > `shutdown_timeout`
3. Backward compatible (no breaking changes)
4. Fix critical bug (close stopChan instead of doneChan)

---

## Issue #344: Troubleshooting Guide

### Implementation Steps

#### 1. Create Troubleshooting Document Structure

**New File**: `docs/troubleshooting.md`

Structure:
```markdown
# Troubleshooting Guide

## Table of Contents
1. Connection Issues
2. Template Errors
3. Permission Issues
4. Watch Mode Issues
5. Common Misconfigurations
6. Debugging Techniques
7. Platform-Specific Issues

## 1. Connection Issues

### Backend: etcd
### Backend: Consul
### Backend: Vault
### Backend: Redis
### Backend: AWS SSM
### Backend: DynamoDB
### Backend: Zookeeper

## 2. Template Errors

### Syntax Errors
### Missing Keys
### Type Conversion Issues
### Template Function Errors

## 3. Permission Issues

### File Ownership
### Command Execution Permissions
### SELinux/AppArmor

## 4. Watch Mode Issues

### Updates Not Triggering
### Performance Problems
### Backend Limitations

## 5. Common Misconfigurations

### Configuration Syntax
### Path Issues
### Backend Configuration

## 6. Debugging Techniques

### Debug Logging
### Dry-Run Mode
### Validation Flags
### Inspection Commands

## 7. Platform-Specific Issues

### Kubernetes
### Docker
### systemd
```

#### 2. Gather Common Issues from Existing Codebase

Research areas:
- Error messages in code
- Test cases (especially error scenarios)
- Validation logic
- Common pitfalls in templates

**Search commands**:
```bash
# Find error messages
grep -r "Error\|Err\|error" pkg/ cmd/ | grep -v ".go:" | grep "fmt.Errorf\|errors.New"

# Find validation logic
grep -r "validate\|Validate" pkg/ cmd/

# Find test error cases
grep -r "t.Error\|t.Fatal" -A 3 *_test.go
```

#### 3. Content Development

For each section, provide:

1. **Problem Description**: Clear description of the issue
2. **Symptoms**: How the problem manifests
3. **Common Causes**: List of typical root causes
4. **Diagnosis**: How to identify the issue
5. **Solutions**: Step-by-step fixes
6. **Example**: Concrete example showing the issue and fix

#### 4. Example Section (Connection Issues - etcd)

```markdown
### Backend: etcd

#### Problem: "connection refused" error

**Symptoms:**
```
FATAL error updating config: connection refused
```

**Common Causes:**
1. etcd server not running
2. Wrong node address or port
3. Firewall blocking connection
4. TLS misconfiguration

**Diagnosis:**
```bash
# Test direct connection
curl http://127.0.0.1:2379/version

# Check if etcd is listening
netstat -tuln | grep 2379

# Test with confd preflight
confd --preflight -node http://127.0.0.1:2379 etcd
```

**Solutions:**

1. **Verify etcd is running:**
```bash
systemctl status etcd
# or
docker ps | grep etcd
```

2. **Check node configuration:**
```toml
[nodes]
nodes = ["http://127.0.0.1:2379"]  # Ensure scheme and port are correct
```

3. **Test connectivity:**
```bash
# For HTTP
etcdctl --endpoints=http://127.0.0.1:2379 endpoint health

# For HTTPS
etcdctl --endpoints=https://127.0.0.1:2379 \
  --cacert=/path/to/ca.crt \
  --cert=/path/to/client.crt \
  --key=/path/to/client.key \
  endpoint health
```

4. **Configure TLS if needed:**
```bash
confd --client-cert=/path/to/client.crt \
      --client-key=/path/to/client.key \
      --client-ca-keys=/path/to/ca.crt \
      etcd
```
```
```

#### 5. Include Real Examples from Codebase

Extract patterns from:
- Integration tests (`integration/` directory)
- Backend-specific README files (`pkg/backends/*/README.md`)
- Template examples in test files
- Error handling in resource.go

#### 6. Add Debugging Commands

Document useful debugging flags:
```markdown
## 6. Debugging Techniques

### Enable Debug Logging
```bash
confd --log-level=debug etcd
```

### Dry-Run Mode (Preview Changes)
```bash
confd --noop etcd
```

### Validate Configuration
```bash
# Check config file syntax
confd --check-config

# Validate specific resource
confd --check-config --resource=/etc/confd/conf.d/myapp.toml

# Validate template syntax
confd --validate

# Run preflight connectivity check
confd --preflight etcd
```

### Inspect Template Processing
```bash
# Keep staged files for inspection
confd --keep-stage-file etcd

# Show diff in noop mode
confd --noop --diff etcd
```
```

#### 7. Link to Related Documentation

Cross-reference:
- Configuration guide
- Template guide
- Backend-specific READMEs
- Command-line flags documentation

#### 8. Update Main Documentation Index

**File**: `README.md`

Add link to troubleshooting guide in documentation section:
```markdown
## Documentation

- [Quick Start Guide](docs/quick-start-guide.md)
- [Configuration Guide](docs/configuration-guide.md)
- [Template Resources](docs/template-resources.md)
- [Templates](docs/templates.md)
- **[Troubleshooting Guide](docs/troubleshooting.md)** ← NEW
- [Command-Line Flags](docs/command-line-flags.md)
```

### Content Sections Detail

#### 1. Connection Issues (~30% of document)
- Cover all 8 backends
- Include TLS/SSL troubleshooting
- Network connectivity diagnosis
- Authentication issues
- Timeout problems

#### 2. Template Errors (~20% of document)
- Syntax error examples
- Missing key handling (with `getenv`, `gets` fallbacks)
- Type conversion issues
- Custom function errors
- Common template mistakes

#### 3. Permission Issues (~10% of document)
- File permission errors
- check_cmd/reload_cmd execution issues
- SELinux/AppArmor denials
- User/group configuration

#### 4. Watch Mode Issues (~15% of document)
- Watch not triggering updates
- Backend limitations (DynamoDB, SSM don't support watch)
- High CPU usage in watch mode
- Debouncing configuration

#### 5. Common Misconfigurations (~10% of document)
- TOML syntax errors
- Path configuration mistakes
- Prefix confusion
- Backend vs global configuration

#### 6. Debugging Techniques (~10% of document)
- All debugging flags with examples
- Log interpretation
- Using validation flags
- Inspecting staged files

#### 7. Platform-Specific Issues (~5% of document)
- Kubernetes pod startup ordering
- Docker container networking
- systemd service configuration
- Environment variable handling

### Quality Checklist

- [ ] Every issue has clear problem/symptom/cause/solution
- [ ] All code examples are tested and working
- [ ] All backend types are covered
- [ ] Cross-references to other docs are accurate
- [ ] Table of contents matches content
- [ ] Examples use proper markdown formatting
- [ ] Command examples show actual output where helpful
- [ ] Links to external resources (etcd docs, Consul docs, etc.)

---

## Implementation Order

### Phase 1: Critical Bug Fix and Core Infrastructure
1. Create `pkg/shutdown/manager.go` package
2. Fix signal handling bug in `cli.go` (close stopChan, not doneChan)
3. Add configuration fields for shutdown timeout
4. Update main loop to use shutdown manager
5. Add basic tests

### Phase 2: Enhanced Shutdown Features
6. Implement different signal behaviors (SIGTERM/SIGINT/SIGQUIT)
7. Add cleanup hook execution
8. Enhance processor shutdown handling
9. Add timeout contexts to backend operations
10. Add comprehensive logging

### Phase 3: Testing and Documentation
11. Write unit tests for shutdown manager
12. Write integration tests for graceful shutdown
13. Update configuration guide
14. Update command-line flags documentation
15. Update README with Kubernetes guidance

### Phase 4: Troubleshooting Guide
16. Create troubleshooting.md structure
17. Research and document connection issues (all backends)
18. Document template errors with examples
19. Document permission issues
20. Document watch mode issues
21. Document debugging techniques
22. Document platform-specific issues
23. Add cross-references and table of contents
24. Update README to link to troubleshooting guide

---

## Testing Checklist

### Issue #342 Testing
- [ ] Unit tests for shutdown manager
- [ ] Signal handling tests (SIGTERM, SIGINT, SIGQUIT)
- [ ] Timeout behavior tests
- [ ] Cleanup hook execution tests
- [ ] Integration test with IntervalProcessor
- [ ] Integration test with WatchProcessor
- [ ] Integration test with BatchWatchProcessor
- [ ] Test with real backend (etcd or consul)
- [ ] Kubernetes pod termination test
- [ ] Verify no double-close panics

### Issue #344 Testing
- [ ] All command examples are runnable
- [ ] All links are valid
- [ ] All code blocks have proper syntax
- [ ] Markdown renders correctly
- [ ] Table of contents links work
- [ ] Cross-references are accurate
- [ ] Examples cover common scenarios
- [ ] Platform-specific advice is tested

---

## Files to Create

### Issue #342
- `pkg/shutdown/manager.go` - New shutdown manager
- `pkg/shutdown/manager_test.go` - Tests for shutdown manager

### Issue #344
- `docs/troubleshooting.md` - Complete troubleshooting guide

## Files to Modify

### Issue #342
- `cmd/confd/config.go` - Add shutdown config fields
- `cmd/confd/cli.go` - Fix signal handling, integrate shutdown manager
- `pkg/template/processor.go` - Enhance shutdown handling, add DrainPending()
- `pkg/template/processor_test.go` - Add shutdown tests
- `docs/configuration-guide.md` - Document shutdown options
- `docs/command-line-flags.md` - Document shutdown flags
- `README.md` - Add shutdown documentation section

### Issue #344
- `README.md` - Add link to troubleshooting guide

---

## Estimated Effort

### Issue #342: Graceful Shutdown
- **Core Implementation**: 6-8 hours
- **Testing**: 4-5 hours
- **Documentation**: 2-3 hours
- **Total**: 12-16 hours

### Issue #344: Troubleshooting Guide
- **Research & Structure**: 2-3 hours
- **Content Writing**: 6-8 hours
- **Examples & Testing**: 3-4 hours
- **Review & Polish**: 1-2 hours
- **Total**: 12-17 hours

### Combined Total: 24-33 hours

---

## Success Criteria

### Issue #342
- [x] Critical bug fixed (stopChan vs doneChan)
- [ ] Graceful shutdown with configurable timeout works
- [ ] Different signal behaviors implemented
- [ ] Cleanup hooks execute properly
- [ ] Detailed shutdown logging present
- [ ] All tests pass
- [ ] Documentation complete
- [ ] No breaking changes

### Issue #344
- [ ] Comprehensive troubleshooting guide created
- [ ] All 7 sections complete with examples
- [ ] All backends covered
- [ ] All debugging techniques documented
- [ ] Examples are tested and accurate
- [ ] Linked from main README
- [ ] Markdown formatting correct
