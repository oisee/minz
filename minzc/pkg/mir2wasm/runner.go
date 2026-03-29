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

	// Run each assert
	for _, a := range hm.Asserts {
		if !force && a.Via != "" && a.Via != "wasm" {
			continue // skip non-wasm asserts unless forced
		}

		fn := mod.ExportedFunction(a.FuncName)
		if fn == nil {
			return fmt.Errorf("line %d: assert %q [wasm]: function %q not exported",
				a.Line, a.Source, a.FuncName)
		}

		// Prepare arguments
		args := make([]uint64, len(a.Args))
		for i, v := range a.Args {
			args[i] = uint64(v)
		}

		results, err := fn.Call(ctx, args...)
		if err != nil {
			return fmt.Errorf("line %d: assert %q [wasm]: call error: %w",
				a.Line, a.Source, err)
		}

		if len(results) == 0 {
			return fmt.Errorf("line %d: assert %q [wasm]: no return value",
				a.Line, a.Source)
		}

		got := int64(results[0])
		// Mask to expected width (u8 = 0xFF, u16 = 0xFFFF)
		got = got & 0xFF // default u8 mask
		// TODO: use function return type to determine mask width

		if got != a.Expected {
			return fmt.Errorf("line %d: assert %q [wasm]: got %d, want %d",
				a.Line, a.Source, got, a.Expected)
		}
	}

	// Sandbox asserts
	for _, sb := range hm.Sandboxes {
		for _, a := range sb.Asserts {
			if !force && a.Via != "" && a.Via != "wasm" {
				continue
			}
			fn := mod.ExportedFunction(a.FuncName)
			if fn == nil {
				continue // skip missing functions in sandbox
			}
			args := make([]uint64, len(a.Args))
			for i, v := range a.Args {
				args[i] = uint64(v)
			}
			results, err := fn.Call(ctx, args...)
			if err != nil {
				return fmt.Errorf("sandbox %q: assert %q [wasm]: %w", sb.Name, a.Source, err)
			}
			if len(results) > 0 {
				got := int64(results[0]) & 0xFF
				if got != a.Expected {
					return fmt.Errorf("sandbox %q: assert %q [wasm]: got %d, want %d",
						sb.Name, a.Source, got, a.Expected)
				}
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
