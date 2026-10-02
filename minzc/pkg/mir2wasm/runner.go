// runner.go — Run WASM module via wazero and evaluate assert results.
//
// Compiles MIR2 → WAT → WASM binary (via wazero's built-in compiler),
// then calls exported functions and checks return values.
package mir2wasm

import (
	"context"
	"fmt"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/tetratelabs/wazero"
)

// RunAsserts compiles the module to WASM and evaluates all assertions.
// Only runs asserts with Via=="" or Via=="wasm" (unless force=true).
func RunAsserts(hm *hir.Module, m *mir2.Module, force bool) error {
	// Compile MIR2 → WASM binary directly
	wasmBin, err := CompileBinary(m)
	if err != nil {
		return fmt.Errorf("wasm compile: %w", err)
	}

	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	mod, err := rt.Instantiate(ctx, wasmBin)
	if err != nil {
		return fmt.Errorf("wasm instantiate: %w", err)
	}
	defer mod.Close(ctx)

	check := func(a hir.Assert, sandbox bool) error {
		fn := mod.ExportedFunction(a.FuncName)
		if fn == nil {
			return fmt.Errorf("line %d: assert %q [wasm]: function %q not exported", a.Line, a.Source, a.FuncName)
		}
		args := make([]uint64, len(a.Args))
		for i, v := range a.Args {
			args[i] = uint64(v)
		}
		results, err := fn.Call(ctx, args...)
		if err != nil {
			return fmt.Errorf("line %d: assert %q [wasm]: call error: %w", a.Line, a.Source, err)
		}
		if len(results) == 0 {
			if sandbox {
				return nil
			}
			return fmt.Errorf("line %d: assert %q [wasm]: no return value", a.Line, a.Source)
		}
		got := int64(results[0]) & 0xFF
		if got != a.Expected {
			return fmt.Errorf("line %d: assert %q [wasm]: got %d, want %d", a.Line, a.Source, got, a.Expected)
		}
		return nil
	}
	for _, a := range hm.Asserts {
		if !force && a.Via != "" && a.Via != "wasm" {
			continue
		}
		if err := hm.RecordAssert(check(a, false)); err != nil {
			return err
		}
	}
	for _, sb := range hm.Sandboxes {
		for _, a := range sb.Asserts {
			if mod.ExportedFunction(a.FuncName) == nil {
				continue // Preserve sandbox behavior; skipped assertions earn no receipt.
			}
			if !force && a.Via != "" && a.Via != "wasm" {
				continue
			}
			if err := hm.RecordAssert(check(a, true)); err != nil {
				return fmt.Errorf("sandbox %q: %w", sb.Name, err)
			}
		}
	}

	return nil
}

// watToWasm is no longer needed — we compile directly to WASM binary
// via CompileBinary(). Kept as documentation.
func watToWasm(wat string) ([]byte, error) {
	return nil, fmt.Errorf("use CompileBinary() for direct MIR2→WASM binary encoding")
}
