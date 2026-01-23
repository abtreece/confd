# E2E Test Implementation Plan

## Overview

This document outlines additional E2E tests to be implemented for confd, building on the existing watch mode and operations tests.

## Current E2E Coverage

| Area | Tests | Location |
|------|-------|----------|
| Watch Mode | etcd, Consul, Redis, Zookeeper basic/multi-update/multi-key, debounce, batch | `test/e2e/watch/` |
| Reconnection | Backend restart, graceful degradation | `test/e2e/watch/reconnect_test.go` |
| Operations | Health endpoints, Prometheus metrics, signals | `test/e2e/operations/` |
| Features | Commands (check/reload), Permissions, Template Functions | `test/e2e/features/` |

## Proposed New E2E Tests

### Phase 1: High Impact Features (Priority)

#### 1.1 Commands Feature Tests

**Location:** `test/e2e/features/commands_test.go`

**Why:** check_cmd and reload_cmd are critical for production deployments but only tested with shell scripts.

**Test Cases:**
| Test | Description |
|------|-------------|
| `TestCommands_CheckCmd_BlocksOnFailure` | Verify check_cmd failure prevents file write |
| `TestCommands_CheckCmd_AllowsOnSuccess` | Verify check_cmd success allows file write |
| `TestCommands_ReloadCmd_ExecutesAfterWrite` | Verify reload_cmd runs after successful write |
| `TestCommands_CheckCmd_TemplatePath` | Verify `{{.src}}` substitution in check_cmd |
| `TestCommands_CommandTimeout` | Verify commands timeout properly |
| `TestCommands_MultipleCommands` | Verify chained commands execute in order |

**Implementation Approach:**
- Use env backend (no external services needed)
- Create test scripts that track execution order via temp files
- Use `ConfdBinary` helper from operations package

---

#### 1.2 File Permissions Tests

**Location:** `test/e2e/features/permissions_test.go`

**Why:** File permissions affect security posture; currently only shell-tested.

**Test Cases:**
| Test | Description |
|------|-------------|
| `TestPermissions_Mode0644` | Verify standard readable file mode |
| `TestPermissions_Mode0600` | Verify owner-only readable mode |
| `TestPermissions_Mode0755` | Verify executable script mode |
| `TestPermissions_PreservesExisting` | Verify existing file permissions behavior |
| `TestPermissions_DirectoryCreation` | Verify directory permissions for new paths |

**Implementation Approach:**
- Use env backend
- Check file mode with `os.Stat()` after confd writes
- Test on Unix-like systems (skip on Windows)

---

#### 1.3 Template Functions Tests

**Location:** `test/e2e/features/functions_test.go`

**Why:** Template functions are core to confd's value proposition; ensure they work end-to-end.

**Test Cases:**
| Test | Description |
|------|-------------|
| `TestFunctions_StringManipulation` | split, join, toUpper, toLower, replace, trim |
| `TestFunctions_MathOperations` | add, sub, mul, div, mod |
| `TestFunctions_Encoding` | base64Encode, base64Decode |
| `TestFunctions_JSON` | json, jsonArray parsing |
| `TestFunctions_NetworkLookup` | lookupIP (with mocked DNS) |
| `TestFunctions_Composition` | Nested function calls |

**Implementation Approach:**
- Use env backend with complex data values
- Verify rendered output matches expected transformations
- Some network functions may need mocking or skip in CI

---

#### 1.4 Zookeeper Watch Mode Tests

**Location:** `test/e2e/watch/zookeeper_test.go`

**Why:** Zookeeper is a supported watch backend but missing from Go E2E tests.

**Test Cases:**
| Test | Description |
|------|-------------|
| `TestZookeeperWatch_BasicChange` | Detect single key change |
| `TestZookeeperWatch_MultipleUpdates` | Handle sequential updates |
| `TestZookeeperWatch_MultipleKeys` | Handle templates with multiple keys |
| `TestZookeeperWatch_GracefulShutdown` | Clean shutdown on context cancellation |

**Implementation Approach:**
- Add ZookeeperContainer to `test/e2e/containers/`
- Follow patterns from etcd_test.go and consul_test.go
- Use testcontainers-go for Zookeeper
- Note: Reconnection test skipped due to port changes on container restart

---

### Phase 2: Feature Tests (Medium Priority)

#### 2.1 Template Include Tests

**Location:** `test/e2e/features/include_test.go`

**Test Cases:**
| Test | Description |
|------|-------------|
| `TestInclude_BasicInclude` | Include another template file |
| `TestInclude_SubdirectoryInclude` | Include from subdirectory (partials/) |
| `TestInclude_NestedInclude` | Include within included template |
| `TestInclude_CycleDetection` | Prevent infinite recursion |
| `TestInclude_MaxDepth` | Enforce maximum include depth |
| `TestInclude_MissingTemplate` | Handle missing include gracefully |

---

#### 2.2 Failure Mode Tests

**Location:** `test/e2e/features/failuremode_test.go`

**Test Cases:**
| Test | Description |
|------|-------------|
| `TestFailureMode_BestEffort_ContinuesOnError` | Process remaining templates after error |
| `TestFailureMode_FailFast_StopsOnError` | Stop immediately on first error |
| `TestFailureMode_ExitCodes` | Verify correct exit codes |
| `TestFailureMode_ErrorAggregation` | Collect and report all errors |

---

#### 2.3 Per-Resource Backend Tests

**Location:** `test/e2e/features/per_resource_backend_test.go`

**Test Cases:**
| Test | Description |
|------|-------------|
| `TestPerResourceBackend_OverrideGlobal` | Template-level backend override |
| `TestPerResourceBackend_MixedBackends` | Multiple backends in single run |
| `TestPerResourceBackend_Precedence` | Configuration precedence rules |

**Implementation Approach:**
- Requires multiple backend containers (env + file + etcd)
- More complex setup but valuable for production scenarios

---

### Phase 3: Advanced Scenarios (Lower Priority)

#### 3.1 Timeout and Retry Tests

**Location:** `test/e2e/resilience/timeout_test.go`

**Test Cases:**
- Backend operation timeouts
- Command execution timeouts
- Retry behavior with exponential backoff
- Partial success scenarios

---

#### 3.2 Concurrent Template Processing Tests

**Location:** `test/e2e/concurrency/concurrent_test.go`

**Test Cases:**
- Multiple templates updating simultaneously
- File lock handling
- Template cache consistency under load

---

#### 3.3 Configuration Reload Tests

**Location:** `test/e2e/operations/reload_test.go`

**Test Cases:**
- Add new templates via SIGHUP
- Remove templates via SIGHUP
- Update backend configuration via SIGHUP

---

## Directory Structure

```
test/e2e/
├── containers/           # Backend containers
│   ├── consul.go
│   ├── etcd.go
│   ├── redis.go
│   └── zookeeper.go
├── testenv/             # Test environment helpers
├── watch/               # Watch mode tests
│   ├── etcd_test.go
│   ├── consul_test.go
│   ├── redis_test.go
│   ├── zookeeper_test.go
│   └── ...
├── operations/          # Operations tests (existing)
│   ├── healthcheck_test.go
│   ├── metrics_test.go
│   ├── signals_test.go
│   └── reload_test.go      # NEW
├── features/            # NEW - Feature-specific tests
│   ├── doc.go
│   ├── commands_test.go
│   ├── permissions_test.go
│   ├── functions_test.go
│   ├── include_test.go
│   ├── failuremode_test.go
│   └── per_resource_backend_test.go
├── resilience/          # NEW - Resilience/fault tolerance tests
│   ├── doc.go
│   └── timeout_test.go
└── concurrency/         # NEW - Concurrency tests
    ├── doc.go
    └── concurrent_test.go
```

## Implementation Status

### ✅ Completed: Sprint 1 - Commands and Permissions
- Created `test/e2e/features/` package with doc.go
- Implemented `commands_test.go` (6 tests)
- Implemented `permissions_test.go` (5 tests)
- Updated CI workflow to remove migrated shell tests

### ✅ Completed: Sprint 2 - Functions and Zookeeper
- Implemented `functions_test.go` (6 tests: StringManipulation, MathOperations, Encoding, JSON, NetworkLookup, Composition)
- Added `ZookeeperContainer` to `test/e2e/containers/zookeeper.go`
- Implemented `zookeeper_test.go` (4 tests: BasicChange, MultipleUpdates, MultipleKeys, GracefulShutdown)
- Note: Shell functions tests retained as they cover additional functions (base, dir, parseBool, getenv, map, reverse, exists, gets/range)

### Planned Sprints

#### Sprint 3: Include and Failure Modes
1. Implement include_test.go (6 tests)
2. Implement failuremode_test.go (4 tests)

#### Sprint 4: Advanced Scenarios
1. Implement per_resource_backend_test.go (3 tests)
2. Implement timeout_test.go
3. Implement reload_test.go

## Shared Test Helpers

### Binary Helper (Reuse from operations)
```go
// Can reuse operations.ConfdBinary for feature tests
// May need to extend with additional helper methods
```

### Feature Test Environment
```go
// test/e2e/features/testenv.go
type FeatureTestEnv struct {
    *operations.TestEnv
    // Additional helpers for feature tests
}

func NewFeatureTestEnv(t *testing.T) *FeatureTestEnv
func (e *FeatureTestEnv) WriteScript(name, content string) string
func (e *FeatureTestEnv) GetFileMode(path string) os.FileMode
```

## Success Criteria

1. All new E2E tests pass locally and in CI
2. Test execution time remains reasonable (< 5 min for new tests)
3. Tests are parallel where possible
4. Clear failure messages for debugging
5. No flaky tests

## Migration from Shell Tests

After E2E tests are verified:
1. Remove corresponding shell tests from `test/integration/features/`
2. Update `integration-tests.yml` workflow
3. Update documentation

## Notes

- Use `t.Parallel()` where tests don't share state
- Use random ports to avoid conflicts
- Use `t.TempDir()` for automatic cleanup
- Follow existing patterns from operations tests
- Prefer poll-based waiting over sleep
