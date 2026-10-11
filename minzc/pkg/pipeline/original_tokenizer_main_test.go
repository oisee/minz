package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Exact original tokenizer main, including print_all_tokens. Book v7 section
// 18.1 printing string literals uses their tagged length/NUL representation;
// source input here is explicit poke bytes, no inferred CString conversion.
func TestOriginalTokenizerMainNativeFullState(t *testing.T) {
	for _, cpm := range []bool{false, true} {
		name := "port23"
		if cpm {
			name = "cpm-bdos2"
		}
		t.Run(name, func(t *testing.T) { originalTokenizerMain(t, cpm) })
	}
}
func TestCPMRuntimeOriginalTokenizerMain(t *testing.T) { originalTokenizerRuntimeMain(t, true, true) }
func originalTokenizerMain(t *testing.T, cpm bool)     { originalTokenizerRuntimeMain(t, cpm, false) }
func originalTokenizerRuntimeMain(t *testing.T, cpm, production bool) {
	const path = "../../../examples/nanz/self_tokenizer.nanz"
	source, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	h, e := nanz.Parse(string(source), path)
	if e != nil {
		t.Fatal(e)
	}
	opts := Options{AssertMode: "none"}
	if production {
		opts.Runtime = mir2.Z80RuntimeCPM
		opts.ContractOpt = true
	}
	steps, e := CompileHIRSteps(h, opts)
	if e != nil {
		t.Fatal(e)
	}
	peek, poke := steps.MIR2Module.FuncByName("peek"), steps.MIR2Module.FuncByName("poke")
	if assertParamLocation(peek.Contract.Params[0], steps.Allocation).Name != "HL" || assertParamLocation(poke.Contract.Params[0], steps.Allocation).Name != "HL" || assertParamLocation(poke.Contract.Params[1], steps.Allocation).Name != "C" {
		t.Fatal("extern ABI drift")
	}
	asm := strings.Replace(steps.Assembly, "peek:\n", "peek:\n LD A,(HL)\n RET\n", 1)
	asm = strings.Replace(asm, "poke:\n", "poke:\n LD (HL),C\n RET\n", 1)
	origin := uint16(0x400)
	fullAssembly := "ORG 0x400\n" + asm
	if cpm && !production {
		// Test runtime translation only: exact source/HIR/main remain intact.
		// Preserve every main branch/label, replace output effects and process
		// termination; the explicit wrapper records status and warm-boots.
		if strings.Count(asm, "    DI\n    HALT") != 1 {
			t.Fatal("main termination shape drift")
		}
		asm = strings.ReplaceAll(asm, "OUT (0x23), A", "CALL __oracle_bdos_putchar")
		asm = strings.Replace(asm, "    DI\n    HALT", "    RET", 1)
		asm += `
__oracle_bdos_putchar:
 PUSH AF
 PUSH BC
 PUSH DE
 PUSH HL
 PUSH IX
 PUSH IY
 LD E,A
 LD C,2
 CALL 5
 POP IY
 POP IX
 POP HL
 POP DE
 POP BC
 POP AF
 RET
`
		origin = 0x100
		fullAssembly = `ORG 0x100
__oracle_cpm_entry:
 LD (__oracle_incoming_sp),SP
 LD SP,0xeff0
 CALL main
 LD A,1
 LD (__oracle_status),A
 LD SP,(__oracle_incoming_sp)
 RET
__oracle_status:
 DB 0
__oracle_incoming_sp:
 DW 0
` + asm
	}
	if production {
		origin = 0x100
		fullAssembly = steps.Assembly
	}
	a, e := z80asm.NewAssembler().AssembleString(fullAssembly)
	if e != nil || len(a.Errors) > 0 {
		t.Fatal(e, a.Errors)
	}
	if production && os.Getenv("MINZ_CPM_TOKENIZER_COM") != "" {
		bin, e := os.ReadFile(os.Getenv("MINZ_CPM_TOKENIZER_COM"))
		if e != nil || !bytes.Equal(bin, a.Binary) {
			t.Fatal("ordinary CLI COM differs from runtime compiler", e)
		}
	}
	z := emulator.NewRemogattoZ80WithScreen()
	z.SetROMEnd(0)
	z.MaxCycles = 10000000
	if e = z.LoadMemory(origin, a.Binary); e != nil {
		t.Fatal(e)
	}
	// Zero full modelregions; guard all adjacentbanks; two stackcanaries.
	for _, region := range [][2]uint16{{0x9000, 0xb000}, {0xb000, 0xc000}, {0xc000, 0xd000}} {
		for p := region[0]; p < region[1]; p++ {
			z.SetMemory(p, 0)
		}
	}
	for _, p := range []uint16{0x8fff, 0xd000, 0xe000, 0xeff0} {
		z.SetMemory(p, 0xa5)
	}
	if cpm {
		z.SetMemory(0xfdfe, 0)
		z.SetMemory(0xfdff, 0)
		z.SetRegisters(emulator.Registers{PC: 0x100, SP: 0xfdfe})
	} else {
		z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["main"]), SP: 0xeff0})
	}
	var output []byte
	z.SetIOHandlers(nil, func(port uint16, b byte) {
		if byte(port) != 0x23 {
			t.Errorf("unexpected port %x", port)
		}
		output = append(output, b)
	})
	if cpm {
		z.SetBDOSHandler(func(function byte, de uint16) (byte, uint16, bool) {
			if function != 2 {
				t.Fatalf("unsupported BDOS%d", function)
			}
			output = append(output, byte(de))
			if production {
				r := z.GetRegisters()
				r.A = 0xa5
				r.F = 0xff
				r.BC = 0x1234
				r.DE = 0x5678
				r.HL = 0x9abc
				r.IX = 0x2468
				r.IY = 0x1357
				z.SetRegisters(r)
			}
			return 0xa5, 0xc3c3, true
		})
	}
	var before [65536]byte
	if production {
		for p := range before {
			before[p] = z.GetMemory(uint16(p))
		}
	}
	runErr := z.Run()
	regs := z.GetRegisters()
	dir := os.Getenv("MINZ_ORIGINAL_TOKENIZER_MAIN_ARTIFACTS")
	if dir != "" {
		dir = filepath.Join(dir, strings.TrimPrefix(t.Name(), "TestOriginalTokenizerMainNativeFullState/"))
	}
	save := func(name string, v any) {
		if dir == "" {
			return
		}
		os.MkdirAll(dir, 0755)
		var b []byte
		switch x := v.(type) {
		case []byte:
			b = x
		case string:
			b = []byte(x)
		default:
			b, _ = json.MarshalIndent(v, "", "  ")
		}
		if e := os.WriteFile(filepath.Join(dir, name), b, 0644); e != nil {
			t.Fatal(e)
		}
	}
	read := func(start uint16, n int) []byte {
		b := make([]byte, n)
		for i := range b {
			b[i] = z.GetMemory(start + uint16(i))
		}
		return b
	}
	globals := map[string][]byte{}
	for _, g := range h.Globals {
		globals[g.Name] = read(uint16(a.Symbols[g.Name]), mir2.ByteWidth(g.Ty))
	}
	save("source.nanz", source)
	save("target.a80", fullAssembly)
	save("target.bin", a.Binary)
	save("stdout.txt", output)
	save("capture.json", map[string]any{"registers": regs, "t_states": z.RemogattoZ80.GetCycles(), "error": fmt.Sprint(runErr), "globals": globals, "source": read(0xc000, 4096), "tokens": read(0xb000, 4096), "strings": read(0x9000, 8192)})
	if runErr != nil {
		t.Fatal(runErr)
	}
	input := "fun add(a: u8, b: u8) -> u8 { return a + b }"
	model := newOriginalTokenizerModel()
	clear(model.arena)
	clear(model.tokens)
	model.init()
	model.run(input)
	initial := map[string][]byte{}
	for _, g := range h.Globals {
		data := make([]byte, mir2.ByteWidth(g.Ty))
		copy(data, g.Init)
		initial[g.Name] = data
	}
	wantGlobals := model.globals(uint16(len(input)), initial)
	// Independently render model tokens, rather than invoking original tok_name
	// or trusting compiler/VM output. KW pointers map back to host keyword text.
	tokenNames := map[byte]string{1: "id", 2: "num", 3: "(", 4: ")", 5: "{", 6: "}", 7: ":", 8: ",", 9: "->", 10: "=", 11: "==", 12: "+", 13: "-", 14: "*", 21: "str"}
	var list strings.Builder
	for i := 0; i < int(model.count); i++ {
		token := model.tokens[i*4 : i*4+4]
		name := tokenNames[token[0]]
		if token[0] == 30 {
			pointer := uint16(token[1]) | uint16(token[2])<<8
			for word, p := range model.keywords {
				if p == pointer {
					name = strings.ToLower(strings.TrimPrefix(word, "KW_"))
				}
			}
		}
		list.WriteString(name)
		list.WriteByte(' ')
	}
	want := []byte(fmt.Sprintf("Source: %s\nTokens: %d\n%s\nInterned strings: %d bytes\n", input, model.count, list.String(), model.bump-0x9000))
	wantSource := make([]byte, 4096)
	copy(wantSource, input)
	save("oracle.json", map[string]any{"stdout": string(want), "source": wantSource, "tokens": model.tokens, "strings": model.arena, "globals": wantGlobals})
	if !bytes.Equal(output, want) || !bytes.Equal(read(0xc000, 4096), wantSource) || !bytes.Equal(read(0xb000, 4096), model.tokens) || !bytes.Equal(read(0x9000, 8192), model.arena) {
		t.Fatalf("fullmain output=%q expected=%q source=%v tokens=%v strings=%v", output, want, bytes.Equal(read(0xc000, 4096), wantSource), bytes.Equal(read(0xb000, 4096), model.tokens), bytes.Equal(read(0x9000, 8192), model.arena))
	}
	for name, want := range wantGlobals {
		if !bytes.Equal(globals[name], want) {
			t.Fatalf("global %s got%x expected%x", name, globals[name], want)
		}
	}
	if cpm {
		if regs.SP != 0xfe00 || regs.PC != 0 || (!production && z.GetMemory(uint16(a.Symbols["__oracle_status"])) != 1) {
			t.Fatalf("CP/M termination %+v status%d", regs, z.GetMemory(uint16(a.Symbols["__oracle_status"])))
		}
	} else if regs.SP != 0xeff0 {
		t.Fatal("unbalanced main stack", regs.SP)
	}
	for _, p := range []uint16{0x8fff, 0xd000, 0xe000, 0xeff0} {
		if z.GetMemory(p) != 0xa5 {
			t.Fatalf("guard changed %04x", p)
		}
	}
	// Only named source globals, explicit wrapper status/SP, and exact DB/DW
	// spill extents may mutate within the assembled image. Code/literals must
	// remain byte-for-byte intact including literals used by printing.
	mutable := map[uint16]bool{}
	for _, g := range h.Globals {
		start := a.Symbols[g.Name]
		for j := 0; j < mir2.ByteWidth(g.Ty); j++ {
			mutable[uint16(start+j)] = true
		}
	}
	if cpm && !production {
		for _, field := range []struct {
			name  string
			width int
		}{{"__oracle_status", 1}, {"__oracle_incoming_sp", 2}} {
			for j := 0; j < field.width; j++ {
				mutable[uint16(a.Symbols[field.name]+j)] = true
			}
		}
	}
	if production {
		for _, r := range [][2]int{{a.Symbols["__cpm_saved_sp"], a.Symbols["__cpm_saved_sp"] + 2}, {a.Symbols["__cpm_stack"], a.Symbols["__cpm_stack_top"]}} {
			for p := r[0]; p < r[1]; p++ {
				mutable[uint16(p)] = true
			}
		}
		if z.GetMemory(uint16(a.Symbols["__cpm_stack_guard"])) != 0xa5 {
			t.Fatal("resident stack guard")
		}
	}
	for _, line := range a.Listing {
		if strings.HasPrefix(strings.ToLower(line.Label), "_spill_") || line.Label == "__oracle_status" || line.Label == "__oracle_incoming_sp" {
			for j := range line.Bytes {
				mutable[uint16(line.Address+j)] = true
			}
		}
	}
	for j, b := range a.Binary {
		p := origin + uint16(j)
		if !mutable[p] && z.GetMemory(p) != b {
			t.Fatalf("immutable image changed %04x", p)
		}
	}
	if production {
		for p, b := range before {
			address := uint16(p)
			if mutable[address] || (p >= 0x9000 && p < 0xd000) {
				continue
			}
			if z.GetMemory(address) != b {
				t.Fatalf("unexpected full-memory write %04x", p)
			}
		}
	}
	if production {
		min := a.Symbols["__cpm_stack_top"]
		for p := a.Symbols["__cpm_stack"]; p < min; p++ {
			if z.GetMemory(uint16(p)) != 0 {
				min = p
				break
			}
		}
		t.Logf("resident stack touched %d bytes", a.Symbols["__cpm_stack_top"]-min)
	}
	t.Logf("exact main %d stdout bytes all regions/globals/stack guards PASS %d T BIN %d SHA %x", len(output), z.RemogattoZ80.GetCycles(), len(a.Binary), sha256.Sum256(a.Binary))
}
