# Headless Screenshot Save Fix

**Date:** 2026-04-04
**Scope:** `minzc/cmd/mzx/main_headless.go` — output path normalization

---

## Problem

`saveScreenPNG()` silently failed when the parent directory didn't exist:
```go
f, err := os.Create(path)
if err != nil {
    return  // silent — no error logged, no file created
}
```

This made `--screenshot /some/new/dir/out.png` appear to succeed (no error
message) while producing no file. Meanwhile `--dump-keyframes` and
`--dump-frames` worked because they called `os.MkdirAll` at startup.

Additionally, `--save-snapshot` in the RZX replay path silently dropped
`SaveSNA` errors.

## Changes

All in `minzc/cmd/mzx/main_headless.go`:

| Location | Before | After |
|----------|--------|-------|
| `saveScreenPNG()` | Silent return on `os.Create` error | `os.MkdirAll` parent dir + log error to stderr |
| `saveScreenPNG()` | `png.Encode` error ignored | Error logged to stderr |
| RZX `--save-snapshot` | `formats.SaveSNA` error silently dropped | `os.MkdirAll` + `log.Fatalf` on error (matches main path) |
| Main `--save-snapshot` | No parent dir creation | `os.MkdirAll` parent dir added |

Added `"path/filepath"` import.

## Output flag behavior after fix

| Flag | Creates parent dirs | Error handling |
|------|-------------------|----------------|
| `--dump-frames DIR` | yes (existing) | silent WriteFile |
| `--dump-keyframes DIR` | yes (existing) | silent WriteFile |
| `--dump-scr DIR` | yes (existing) | silent WriteFile |
| `--screenshot PATH` | **yes (fixed)** | **logged to stderr (fixed)** |
| `--save-snapshot PATH` | **yes (fixed)** | log.Fatalf (existing in main, **fixed in RZX**) |

## Tests

```
$ go build -tags mzx_headless ./cmd/mzx/
(clean)
```

No behavioral tests added — this is a CLI tool with no test harness. The fix
is mechanical (mkdir + error propagation).

## Commit

*(single file changed)*
- `minzc/cmd/mzx/main_headless.go` — 4 hunks, +12/-3 lines
