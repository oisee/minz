package abap

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

//go:embed abap_parser.wasm
var abapParserWasm []byte

// wasmRuntime is a cached wazero runtime for reuse across multiple parses.
var (
	wasmOnce    sync.Once
	wasmRuntime wazero.Runtime
	wasmErr     error
)

func initWasmRuntime() {
	ctx := context.Background()
	wasmRuntime = wazero.NewRuntime(ctx)
	wasi_snapshot_preview1.MustInstantiate(ctx, wasmRuntime)
}

// ParseWasm parses ABAP source using the embedded Wasm parser (no Node.js needed).
// Falls back to Node.js bridge if Wasm fails.
func ParseWasm(src, name string) (*Program, error) {
	wasmOnce.Do(initWasmRuntime)
	if wasmRuntime == nil {
		return nil, fmt.Errorf("wasm runtime init failed")
	}

	ctx := context.Background()
	stdin := bytes.NewReader([]byte(src))
	var stdout, stderr bytes.Buffer

	config := wazero.NewModuleConfig().
		WithStdin(stdin).
		WithStdout(&stdout).
		WithStderr(&stderr).
		WithArgs("abap-parser").
		WithName("") // anonymous module (allows re-instantiation)

	mod, err := wasmRuntime.InstantiateWithConfig(ctx, abapParserWasm, config)
	if err != nil {
		return nil, fmt.Errorf("wasm exec: %w (stderr: %s)", err, stderr.String())
	}
	if mod != nil {
		mod.Close(ctx)
	}

	if stdout.Len() == 0 {
		return nil, fmt.Errorf("wasm: empty output (stderr: %s)", stderr.String())
	}

	var result ParseResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("wasm JSON: %w", err)
	}

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("abaplint: %s", result.Errors[0])
	}

	return buildProgram(name, &result)
}
