package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestIndirectCallEmissionScope(t *testing.T) {
	source, err := filepath.Abs("../../../examples/c89/func_ptr.c")
	if err != nil {
		t.Fatal(err)
	}
	oldOutput, oldBackend, oldMode, oldForce, oldEmit := outputFile, backend, assertMode, assertForce, emitFormat
	defer func() {
		outputFile, backend, assertMode, assertForce, emitFormat = oldOutput, oldBackend, oldMode, oldForce, oldEmit
	}()
	for _, tc := range []struct {
		name, mode, be, emit, out string
		native                    bool
	}{
		{"mir2", "mir2", "z80", "", "", false},
		{"all_mir2_annotations", "all", "z80", "", "", false},
		{"c", "all", "c", "", "output.c", false},
		{"mir2_dump", "all", "z80", "mir2", "output.mir2", false},
		{"hir_dump", "all", "z80", "hir", "output.hir", false},
		{"z80_assembly", "mir2", "z80", "", "output.a80", true},
		{"z80_binary", "none", "z80", "", "output.bin", true},
		{"default_z80_product", "none", "z80", "", "", true},
		{"z80_assert", "z80", "z80", "mir2", "output.mir2", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outputFile, backend, assertMode, assertForce, emitFormat = "", tc.be, tc.mode, "", tc.emit
			if tc.out != "" {
				outputFile = filepath.Join(t.TempDir(), tc.out)
			}
			src, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			input := filepath.Join(t.TempDir(), "func_ptr.c")
			if err := os.WriteFile(input, src, 0644); err != nil {
				t.Fatal(err)
			}
			if tc.name == "z80_assert" {
				src, err := os.ReadFile(source)
				if err != nil {
					t.Fatal(err)
				}
				input = filepath.Join(t.TempDir(), "func_ptr.c")
				if err := os.WriteFile(input, []byte(strings.ReplaceAll(string(src), "via mir2", "via z80")), 0644); err != nil {
					t.Fatal(err)
				}
			}
			err = compileViaHIR(input)
			if err != nil {
				t.Fatal(err)
			}
			if outputFile != "" {
				data, err := os.ReadFile(outputFile)
				if err != nil || len(data) == 0 {
					t.Fatalf("missing product: %v", err)
				}
				if tc.be == "c" && !strings.Contains(string(data), "#include") {
					t.Fatal("C backend did not emit C")
				}
			}
			if tc.native {
				src, err := os.ReadFile(input)
				if err != nil {
					t.Fatal(err)
				}
				hm, err := c89.Compile(string(src), input)
				if err != nil {
					t.Fatal(err)
				}
				steps, err := pipeline.CompileHIRSteps(hm, pipeline.DefaultOptions())
				if err != nil {
					t.Fatal(err)
				}
				asm := steps.Assembly
				if tc.out == "output.a80" || tc.name == "default_z80_product" {
					path := outputFile
					if path == "" {
						path = strings.TrimSuffix(input, filepath.Ext(input)) + ".a80"
					}
					data, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					asm = string(data)
					if asm != steps.Assembly {
						t.Fatal("CLI assembly differs from exact-source API")
					}
				}
				assembler := z80asm.NewAssembler()
				if parsed, e := z80asm.ParseTarget(target); e == nil {
					assembler.SetTarget(parsed)
				}
				assembled, err := assembler.AssembleString(asm)
				if err != nil || assembled == nil || len(assembled.Errors) != 0 {
					t.Fatalf("assemble: %v %+v", err, assembled)
				}
				if tc.out == "output.bin" {
					data, err := os.ReadFile(outputFile)
					wantBinary := assembled.Binary
					if parsed, e := z80asm.ParseTarget(target); e == nil {
						if cfg := z80asm.GetTargetConfig(parsed); cfg != nil && cfg.OutputFormat.Generator != nil {
							wantBinary, e = cfg.OutputFormat.Generator(assembled)
							if e != nil {
								t.Fatal(e)
							}
						}
					}
					if err != nil || !bytes.Equal(data, wantBinary) {
						t.Fatalf("CLI BIN differs from target-formatted assembled source: %v", err)
					}
				}
				indirectFixtureNative(t, &steps, assembled)
			}
		})
	}
}

// Skipping a product must still execute MIR2 assertions, including sandboxes.
// A Z80 sandbox assertion must force guarded emission despite the skip option.
func TestIndirectIntermediateAsserts(t *testing.T) {
	src, err := os.ReadFile("../../../examples/c89/func_ptr.c")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		bad, z80 bool
	}{
		{"mir2_sandbox", false, false},
		{"bad_mir2_sandbox", true, false},
		{"z80_sandbox", false, true},
		{"bad_z80_sandbox", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hm, err := c89.Compile(string(src), "func_ptr.c")
			if err != nil {
				t.Fatal(err)
			}
			hm.Sandboxes = []hir.Sandbox{{Name: "indirect", Asserts: hm.Asserts}}
			hm.Asserts = nil
			if tc.bad {
				hm.Sandboxes[0].Asserts[0].Expected++
			}
			if tc.z80 {
				hm.Sandboxes[0].Asserts[0].Via = "z80"
			}
			steps, err := pipeline.CompileHIRSteps(hm, pipeline.Options{ContractOpt: true, SkipZ80Emission: true})
			switch {
			case tc.z80 && !tc.bad:
				if err != nil {
					t.Fatal(err)
				}
				if steps.Assembly == "" {
					t.Fatal("Z80 sandbox must force emission despite skip")
				}
				assembled, e := z80asm.NewAssembler().AssembleString(steps.Assembly)
				if e != nil || assembled == nil || len(assembled.Errors) != 0 {
					t.Fatalf("assemble: %v %+v", e, assembled)
				}
				indirectFixtureNative(t, &steps, assembled)
			case tc.bad:
				if err == nil || !strings.Contains(err.Error(), "sandbox") {
					t.Fatalf("MIR2 assertion was skipped: %v", err)
				}
			default:
				if err != nil {
					t.Fatal(err)
				}
				if steps.Assembly != "" || steps.MIR2Module == nil {
					t.Fatal("wanted MIR2 without Z80 emission")
				}
			}
		})
	}
}

// Expected values derive directly from the unchanged C89 fixture arithmetic.
// Execute an actual CALL, require restored SP, and protect every non-stack,
// non-spill byte; the call target/trampoline must remain executable unchanged.
func indirectFixtureNative(t *testing.T, steps *pipeline.Steps, a *z80asm.Result) {
	t.Helper()
	if !strings.Contains(steps.Assembly, "CALL __call_ix") || !strings.Contains(steps.Assembly, "JP (IX)") {
		t.Fatal("native proof lost indirect trampoline")
	}
	typed := false
	for _, f := range steps.MIR2Module.Funcs {
		for _, b := range f.Blocks {
			for _, i := range b.Insts {
				if i.Op == mir2.OpCallIndirect && len(i.IndirectParamTys) == len(i.Args) && len(i.IndirectReturnTys) > 0 {
					typed = true
				}
			}
		}
	}
	if !typed {
		t.Fatal("native proof lost semantic indirect signature")
	}
	allowed := map[uint16]bool{}
	for _, f := range steps.MIR2Module.Funcs {
		types := map[mir2.Reg]mir2.Ty{}
		for _, p := range f.Contract.Params {
			types[p.Reg] = p.Ty
		}
		for _, b := range f.Blocks {
			for _, i := range b.Insts {
				types[i.Dst] = i.Ty
				for n, r := range i.ExtraRets {
					if n < len(i.ExtraRetTys) {
						types[r] = i.ExtraRetTys[n]
					}
				}
			}
		}
		for r, ty := range types {
			if addr, ok := a.Symbols[mir2.Z80SpillLabel(f.Name, r)]; ok {
				for off := 0; off < mir2.NativeSpillWidth(ty); off++ {
					allowed[uint16(addr+off)] = true
				}
			}
		}
	}
	for _, tc := range []struct {
		name string
		want uint16
	}{{"test_basic_fp", 7}, {"test_reassign_fp", 20}, {"test_addr_of_func", 13}, {"test_unary_fp", 42}, {"test_callback", 20}, {"test_callback_negate", 65529}} {
		z := emulator.NewRemogattoZ80()
		z.LoadMemory(uint16(a.Origin), a.Binary)
		addr, ok := a.Symbols[tc.name]
		if !ok {
			t.Fatal("missing function", tc.name)
		}
		z.LoadMemory(0x7000, []byte{0xcd, byte(addr), byte(addr >> 8), 0x00})
		z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xeff0, A: 0x5a, F: 0xa5, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468})
		before := append([]byte(nil), z.Memory()...)
		done := false
		minSP := z.GetSP()
		for n := 0; n < 100000; n++ {
			if z.GetPC() == 0x7003 && z.GetSP() == 0xeff0 {
				done = true
				break
			}
			z.Step()
			if z.GetSP() < minSP {
				minSP = z.GetSP()
			}
		}
		if !done {
			t.Fatalf("%s did not return to original CALL/SP: %+v", tc.name, z.GetRegisters())
		}
		if got := z.GetRegisters().HL; got != tc.want {
			t.Fatalf("%s got%d want%d", tc.name, got, tc.want)
		}
		for addr, v := range before {
			if z.Memory()[addr] != v && !(addr >= int(minSP) && addr < 0xeff0) && !allowed[uint16(addr)] {
				t.Fatalf("%s changed non-stack/non-spill byte %04x", tc.name, addr)
			}
		}
	}
}

// Native support is conditioned on a semantic signature. Erasing it from the
// unchanged fixture must keep the conservative live-caller rejection in force.
func TestIndirectUnresolvedLiveCallerGuard(t *testing.T) {
	src, err := os.ReadFile("../../../examples/c89/func_ptr.c")
	if err != nil {
		t.Fatal(err)
	}
	hm, err := c89.Compile(string(src), "func_ptr.c")
	if err != nil {
		t.Fatal(err)
	}
	steps, err := pipeline.CompileHIRSteps(hm, pipeline.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if err := mir2.ValidateZ80IndirectCalls(steps.MIR2Module); err != nil {
		t.Fatal("typed fixture refused", err)
	}
	for _, f := range steps.MIR2Module.Funcs {
		for _, b := range f.Blocks {
			for _, i := range b.Insts {
				if i.Op == mir2.OpCallIndirect {
					i.IndirectParamTys = nil
					i.IndirectReturnTys = nil
				}
			}
		}
	}
	if err := mir2.ValidateZ80IndirectCalls(steps.MIR2Module); err == nil || !strings.Contains(err.Error(), "caller preservation for the indirect ABI is unsupported") {
		t.Fatalf("unresolved live caller lost guard: %v", err)
	}
}
