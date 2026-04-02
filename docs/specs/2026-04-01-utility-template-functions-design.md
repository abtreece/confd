# Utility Template Functions

**Date**: 2026-04-01
**Issue**: #272
**Status**: Approved

## Summary

Add 28 utility template functions to confd, covering common config templating needs: defaults/conditionals, JSON manipulation, formatting, regex, string transforms, collections, and hashing. All implemented with Go standard library only — no external dependencies.

## Motivation

confd's template functions cover core config management well (key/value access, DNS lookups, environment variables) but lack utility functions that are standard in tools like Helm. Users frequently need defaults, conditionals, JSON serialization, regex, and collection manipulation. Rather than importing the Sprig library (semi-maintained, heavy transitive dependency tree, includes security-sensitive functions inappropriate for config management), we implement the most valuable subset natively.

## Design

### Integration Point

New file `pkg/template/utility_funcs.go` exports a `utilityFuncMap()` function returning `map[string]interface{}` (matching the existing `newFuncMap()` return type). This is merged into the main FuncMap in `newFuncMap()` before confd's existing functions are assigned:

```go
func newFuncMap() map[string]interface{} {
    m := utilityFuncMap()     // utility functions as base layer
    // confd's existing functions overlay on top — last write wins
    m["base"] = path.Base
    m["split"] = strings.Split
    // ... rest of existing confd functions
    return m
}
```

confd's existing functions always win if names overlap. The ordering (utility first, confd second) makes this explicit and auditable.

### No Changes To

- Store functions (memkv)
- Include function
- Template renderer / caching
- Template resource assembly flow

### Function List

#### Defaults & Conditionals (4)

| Function | Signature | Description |
|----------|-----------|-------------|
| `default` | `default(defaultVal, val interface{}) interface{}` | Returns `val` if non-empty, otherwise `defaultVal` |
| `ternary` | `ternary(trueVal, falseVal interface{}, condition bool) interface{}` | Returns `trueVal` if condition is true; condition is last arg for pipeline use: `{{ .val | ternary "yes" "no" }}` |
| `coalesce` | `coalesce(vals ...interface{}) interface{}` | Returns first non-empty value |
| `empty` | `empty(val interface{}) bool` | Returns true if val is zero/nil/empty |

"Empty" means: nil, false, 0, "", empty slice, empty map. Uses `reflect.Value.IsZero()` for struct types. For types where emptiness is ambiguous (channels, funcs), returns false.

#### JSON (3)

| Function | Signature | Description |
|----------|-----------|-------------|
| `toJson` | `toJson(val interface{}) (string, error)` | Marshal to compact JSON |
| `fromJson` | `fromJson(s string) (interface{}, error)` | Unmarshal JSON string |
| `toPrettyJson` | `toPrettyJson(val interface{}) (string, error)` | Marshal to indented JSON (2-space) |

These complement confd's existing `json` (unmarshal to map) and `jsonArray` (unmarshal to array) with the reverse direction and pretty-printing.

#### Formatting (4)

| Function | Signature | Description |
|----------|-----------|-------------|
| `indent` | `indent(spaces int, s string) string` | Indent every line by N spaces |
| `nindent` | `nindent(spaces int, s string) string` | Newline + indent every line by N spaces |
| `quote` | `quote(s string) string` | Wrap in double quotes |
| `squote` | `squote(s string) string` | Wrap in single quotes |

#### Regex (3)

| Function | Signature | Description |
|----------|-----------|-------------|
| `regexMatch` | `regexMatch(pattern, s string) (bool, error)` | True if string matches pattern |
| `regexFind` | `regexFind(pattern, s string) (string, error)` | First match, empty string if none |
| `regexReplaceAll` | `regexReplaceAll(pattern, s, repl string) (string, error)` | Replace all matches |

All regex functions return errors for invalid patterns rather than panicking.

#### Strings (6)

| Function | Signature | Description |
|----------|-----------|-------------|
| `trimPrefix` | `strings.TrimPrefix` | Remove prefix from string (matches `trimSuffix` pattern) |
| `repeat` | `repeat(count int, s string) string` | Repeat string N times |
| `nospace` | `nospace(s string) string` | Remove all whitespace |
| `snakecase` | `snakecase(s string) string` | Convert to snake_case |
| `camelcase` | `camelcase(s string) string` | Convert to camelCase |
| `kebabcase` | `kebabcase(s string) string` | Convert to kebab-case |

Case conversion handles: camelCase, PascalCase, snake_case, kebab-case, and space-separated input.

#### Collections (7)

| Function | Signature | Description |
|----------|-----------|-------------|
| `dict` | `dict(pairs ...interface{}) (map[string]interface{}, error)` | Create map from key/value pairs (alias for existing `map` function; included because `dict` is the standard name users expect from Helm/Sprig) |
| `list` | `list(vals ...interface{}) []interface{}` | Create list from arguments |
| `hasKey` | `hasKey(m map[string]interface{}, key string) bool` | Check if map contains key |
| `keys` | `keys(m map[string]interface{}) []string` | Return sorted map keys |
| `values` | `values(m map[string]interface{}) []interface{}` | Return map values (sorted by key) |
| `append` | `appendList(list []interface{}, val interface{}) []interface{}` | Append to list |
| `pluck` | `pluck(key string, maps ...map[string]interface{}) []interface{}` | Extract key from list of maps |

Note: The Go implementation function is named `appendList` to avoid shadowing Go's builtin `append` in the source file. It is registered in the FuncMap as `"append"`.

`dict` returns an error if given an odd number of arguments. `keys` returns sorted keys for deterministic template output.

#### Hashing (1)

| Function | Signature | Description |
|----------|-----------|-------------|
| `sha256sum` | `sha256sum(s string) string` | Hex-encoded SHA-256 hash |

### Validation Support

No changes to `validate.go` required. `newValidationFuncMap()` calls `newFuncMap()` internally, so the utility functions are inherited automatically. Store-specific stubs are layered on top by `newValidationFuncMap()` as before.

### Error Handling

- Regex functions return `(result, error)` — invalid patterns produce template errors, not panics
- `dict` returns error on odd argument count
- `fromJson` returns error on malformed JSON
- `toJson`/`toPrettyJson` return error on unmarshalable values
- Collection functions that receive wrong types return zero values (not panics)

## Files Changed

| File | Change |
|------|--------|
| `pkg/template/utility_funcs.go` | **New** — 28 function implementations |
| `pkg/template/utility_funcs_test.go` | **New** — table-driven tests for all functions |
| `pkg/template/template_funcs.go` | Modify `newFuncMap()` to merge utility functions as base layer |
| `docs/templates.md` | Document new functions with examples |
| `CHANGELOG` | Add entry for new template functions |

## Testing

- Table-driven unit tests in `utility_funcs_test.go` covering:
  - Happy path for each function
  - Edge cases: nil, zero values, empty strings, empty collections
  - Error cases: invalid regex, odd dict args, malformed JSON
  - Type handling: `empty`/`default`/`coalesce` with all Go types
- Existing template tests continue to pass unchanged (no behavior changes)
- One integration test template exercising new + existing functions together

## Not In Scope

- Sprig compatibility mode or `--sprig` flag
- Sprig as a dependency
- Security-sensitive functions (bcrypt, genPrivateKey, randAlpha)
- Semver functions (would require external dep)
- Date manipulation beyond what Go's `time` package provides (confd already has `datetime`)
