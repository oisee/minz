// ez80-c-probe is an experimental differential-test driver, not the production
// C target. Only pointer-free, global-free byte-return kernels are admitted.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/ez80"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: ez80-c-probe SOURCE.c OUTPUT_PREFIX")
		os.Exit(1)
	}
	if err := run(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		var unsupported *unsupportedError
		if errors.As(err, &unsupported) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

type unsupportedError struct{ error }

func unsupported(message string) error { return &unsupportedError{errors.New(message)} }

func run(source, prefix string) error {
	src, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	h, err := c89.CompileWithOpts(string(src), source, c89.CompileOpts{NoLibc: true})
	if err != nil {
		return err
	}
	// Reject surviving pointer-bearing IR before selecting native addressing.
	// Source int semantics remain 16-bit; the differential corpus must avoid
	// assumptions that differ across C data models (including folded sizeof).
	// Genuine 24-bit C pointers require a frontend ABI implementation.
	raw := hir.LowerModule(h)
	if len(raw.Globals) != 0 || raw.Strings.Len() != 0 {
		return unsupported("unsupported probe: global/string storage")
	}
	for _, f := range raw.Funcs {
		for _, p := range f.Contract.Params {
			if mir2.IsPointer(p.Ty) {
				return unsupported("unsupported probe: C pointer parameter")
			}
		}
		for _, r := range f.Contract.Returns {
			if mir2.IsPointer(r.Ty) {
				return unsupported("unsupported probe: C pointer return")
			}
		}
		for _, b := range f.Blocks {
			for _, p := range b.Params {
				if mir2.IsPointer(p.Ty) {
					return unsupported("unsupported probe: C pointer block parameter")
				}
			}
			for _, i := range b.Insts {
				if mir2.IsPointer(i.Ty) || mir2.IsPointer(i.SrcTy) {
					return unsupported("unsupported probe: C pointer expression")
				}
			}
		}
	}
	value := func(m *mir2.Module) (int64, error) {
		out, err := mir2.NewVM(m).Call("main", nil)
		if err != nil {
			return 0, err
		}
		if len(out) != 1 {
			return 0, fmt.Errorf("probe requires one return")
		}
		return out[0].I, nil
	}
	rawResult, err := value(raw)
	if err != nil {
		return err
	}
	h.PointerBits = 24
	h.Target = hir.TargetAgon
	m := hir.LowerModule(h)
	for _, f := range m.Funcs {
		mir2.FoldConstants(f)
		mir2.PropagateCopies(f)
		mir2.DeadStoreElim(f)
	}
	optResult, err := value(m)
	if err != nil {
		return err
	}
	if rawResult != optResult {
		return fmt.Errorf("raw/optimized VM mismatch: %d != %d", rawResult, optResult)
	}
	if len(m.Funcs) != 1 {
		return unsupported("unsupported probe: requires only main")
	}
	f := m.FuncByName("main")
	if f == nil {
		return fmt.Errorf("probe lacks main")
	}
	ar := mir2.PBQPAllocate(f, mir2.ComputeLiveness(f), ez80.EZ80CostTable{})
	asm, err := ez80.MIR2CodegenChecked(m, ar)
	if err != nil {
		// Allocation/emission failures in admitted operations are regressions,
		// not unsupported cases that the differential harness may skip.
		if strings.Contains(err.Error(), "incompatible allocation") || strings.Contains(err.Error(), "emitter reached") {
			return err
		}
		return &unsupportedError{err}
	}
	a := z80asm.NewAssembler()
	a.SetCPUMode(z80asm.CPUModeEZ80ADL)
	bin, err := a.AssembleString(asm)
	if err != nil {
		return err
	}
	if len(bin.Errors) != 0 {
		return fmt.Errorf("assembler errors: %v", bin.Errors)
	}
	receipt := struct {
		Symbols     map[string]int `json:"symbols"`
		VMResult    int64          `json:"vm_result"`
		RawVMResult int64          `json:"raw_vm_result"`
		BinaryBytes int            `json:"binary_bytes"`
		CIntBits    int            `json:"c_int_bits"`
		AddressBits int            `json:"native_address_bits"`
	}{bin.Symbols, optResult, rawResult, len(bin.Binary), 16, 24}
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	for _, file := range []struct {
		path string
		data []byte
	}{{prefix + ".asm", []byte(asm)}, {prefix + ".bin", bin.Binary}, {prefix + ".json", append(data, '\n')}} {
		if err := os.WriteFile(file.path, file.data, 0600); err != nil {
			return err
		}
	}
	return nil
}
