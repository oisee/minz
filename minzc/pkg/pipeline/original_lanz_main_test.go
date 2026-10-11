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

// Book v7 section18.1: plain SString has a byte length prefix; CString has
// trailingNUL. Full originalmain is compiled, no entry/body surgery. Test-only
// extern implementations bind actual compiler ABI; OUT23 is native defaultIO.
func TestOriginalSelfLanzMainNativeFullState(t *testing.T) {
	for _, cpm := range []bool{false, true} {
		name := "port23"
		if cpm {
			name = "cpm-bdos2"
		}
		t.Run(name, func(t *testing.T) { originalSelfLanzMain(t, cpm) })
	}
}
func originalSelfLanzMain(t *testing.T, cpm bool) {
	const path = "../../../examples/nanz/self_lanz_parser.nanz"
	source, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	h, e := nanz.Parse(string(source), path)
	if e != nil {
		t.Fatal(e)
	}
	steps, e := CompileHIRSteps(h, Options{AssertMode: "none"})
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
	if cpm {
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
	a, e := z80asm.NewAssembler().AssembleString(fullAssembly)
	if e != nil || len(a.Errors) > 0 {
		t.Fatal(e, a.Errors)
	}
	z := emulator.NewRemogattoZ80WithScreen()
	z.SetROMEnd(0)
	z.MaxCycles = 10000000
	if e = z.LoadMemory(origin, a.Binary); e != nil {
		t.Fatal(e)
	}
	// Zero full modelregions; guard all adjacentbanks; two stackcanaries.
	for _, region := range [][2]uint16{{0x8000, 0x9000}, {0x9000, 0xb000}, {0xc000, 0xd000}} {
		for p := region[0]; p < region[1]; p++ {
			z.SetMemory(p, 0)
		}
	}
	for _, p := range []uint16{0x7fff, 0xb000, 0xbfff, 0xd000, 0xe000, 0xeff0} {
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
			t.Errorf("unexpectedport%x", port)
		}
		output = append(output, b)
	})
	if cpm {
		z.SetBDOSHandler(func(function byte, de uint16) (byte, uint16, bool) {
			if function != 2 {
				t.Fatalf("unsupported BDOS%d", function)
			}
			output = append(output, byte(de))
			return 0xa5, 0xc3c3, true
		})
	}
	runErr := z.Run()
	regs := z.GetRegisters()
	dir := os.Getenv("MINZ_ORIGINAL_LANZ_MAIN_ARTIFACTS")
	if dir != "" {
		dir = filepath.Join(dir, strings.TrimPrefix(t.Name(), "TestOriginalSelfLanzMainNativeFullState/"))
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
	save("capture.json", map[string]any{"registers": regs, "t_states": z.RemogattoZ80.GetCycles(), "error": fmt.Sprint(runErr), "globals": globals, "source": read(0xc000, 4096), "nodes": read(0x8000, 4096), "strings": read(0x9000, 8192)})
	if runErr != nil {
		t.Fatal(runErr)
	}
	input := "(fun add ((a u8) (b u8)) u8 (return (+ a b)))"
	host := newLanzHost()
	copy(host.ram[0xc000:], input)
	host.len = uint16(len(input))
	root := host.parse()
	var want []byte
	// Independent parsed-tree rendering checks structure and interning, not
	// parroting input as output.
	var render func(uint16) string
	word := func(p uint16) uint16 { return uint16(host.ram[p]) | uint16(host.ram[p+1])<<8 }
	render = func(p uint16) string {
		if p == 0 {
			return ""
		}
		switch host.ram[p] {
		case 1:
			s := word(p + 1)
			return string(host.ram[s+1 : s+1+uint16(host.ram[s])])
		case 3:
			return fmt.Sprint(byte(word(p + 1)))
		case 2:
			children := []string{}
			for c := word(p + 1); c != 0; c = word(c + 3) {
				children = append(children, render(c))
			}
			return "(" + strings.Join(children, " ") + ")"
		}
		t.Fatal("unexpectedtag")
		return ""
	}
	want = []byte(fmt.Sprintf("Input: %s\nParsed: %s\nNodes: %d Strings: %d bytes\n", input, render(root), (host.node-0x8000)/8, host.str-0x9000))
	save("oracle.json", map[string]any{"stdout": string(want), "source": host.ram[0xc000:0xd000], "nodes": host.ram[0x8000:0x9000], "strings": host.ram[0x9000:0xb000], "pos": host.pos, "src_len": host.len, "node_bump": host.node, "str_bump": host.str})
	if !bytes.Equal(output, want) || !bytes.Equal(read(0xc000, 4096), host.ram[0xc000:0xd000]) || !bytes.Equal(read(0x8000, 4096), host.ram[0x8000:0x9000]) || !bytes.Equal(read(0x9000, 8192), host.ram[0x9000:0xb000]) {
		t.Fatalf("originalmain output=%q expected=%q source=%v nodes=%v strings=%v", output, want, bytes.Equal(read(0xc000, 4096), host.ram[0xc000:0xd000]), bytes.Equal(read(0x8000, 4096), host.ram[0x8000:0x9000]), bytes.Equal(read(0x9000, 8192), host.ram[0x9000:0xb000]))
	}
	for name, v := range map[string]uint16{"pos": host.pos, "src_len": host.len, "node_bump": host.node, "str_bump": host.str, "src_buf": 0xc000, "node_arena": 0x8000, "str_arena": 0x9000, "SEXP_ATOM": 1, "SEXP_LIST": 2, "SEXP_INT": 3} {
		b := globals[name]
		got := uint16(b[0])
		if len(b) == 2 {
			got |= uint16(b[1]) << 8
		}
		if got != v {
			t.Fatal("global", name, b, v)
		}
	}
	if cpm {
		if regs.SP != 0xfe00 || regs.PC != 0 || z.GetMemory(uint16(a.Symbols["__oracle_status"])) != 1 {
			t.Fatalf("CP/M termination %+v status%d", regs, z.GetMemory(uint16(a.Symbols["__oracle_status"])))
		}
	} else if regs.SP != 0xeff0 {
		t.Fatal("unbalanced main stack", regs.SP)
	}
	for _, p := range []uint16{0x7fff, 0xb000, 0xbfff, 0xd000, 0xe000, 0xeff0} {
		if z.GetMemory(p) != 0xa5 {
			t.Fatalf("guardchanged%04x", p)
		}
	}
	// Only named source globals, explicit wrapper status/SP, and exact DB/DW
	// spill extents may mutate within the assembled image. Code/literals must
	// remain byte-for-byte intact even though the parser reads literal data.
	mutable := map[uint16]bool{}
	for _, g := range h.Globals {
		start := a.Symbols[g.Name]
		for j := 0; j < mir2.ByteWidth(g.Ty); j++ {
			mutable[uint16(start+j)] = true
		}
	}
	if cpm {
		for _, field := range []struct {
			name  string
			width int
		}{{"__oracle_status", 1}, {"__oracle_incoming_sp", 2}} {
			for j := 0; j < field.width; j++ {
				mutable[uint16(a.Symbols[field.name]+j)] = true
			}
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
	t.Logf("exact main %d stdout bytes all regions/globals/stack guards PASS %d T BIN %d SHA %x", len(output), z.RemogattoZ80.GetCycles(), len(a.Binary), sha256.Sum256(a.Binary))
}
