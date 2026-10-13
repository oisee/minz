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

// Whole original main: real imported ASM executes; only BDOS console services
// are test-owned runtime bindings. Corrected source literals follow the existing NUL-terminated API contract.
func TestPublicCPMTetrisNative(t *testing.T) {
	src, e := os.ReadFile("../../../examples/nanz/tetris_cpm.nanz")
	if e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile("../../../reports/tetris-main-native/freeze-corrected.json")
	if e != nil {
		t.Fatal(e)
	}
	var freeze struct {
		SHA           string `json:"source_sha256"`
		DependencySHA string `json:"dependency_sha256"`
	}
	if e = json.Unmarshal(data, &freeze); e != nil || fmt.Sprintf("%x", sha256.Sum256(src)) != freeze.SHA {
		t.Fatal("freeze", e)
	}
	dep, e := os.ReadFile("../../../stdlib/tui/render.nanz")
	if e != nil || fmt.Sprintf("%x", sha256.Sum256(dep)) != freeze.DependencySHA {
		t.Fatal("dependency", e)
	}
	save := func(n string, v any) {
		root := os.Getenv("MINZ_CPM_TETRIS_ARTIFACTS")
		if root == "" {
			return
		}
		p := filepath.Join(root, n)
		os.MkdirAll(filepath.Dir(p), 0755)
		var b []byte
		switch v := v.(type) {
		case string:
			b = []byte(v)
		case []byte:
			b = v
		default:
			b, _ = json.MarshalIndent(v, "", "  ")
		}
		if e = os.WriteFile(p, b, 0644); e != nil {
			t.Fatal(e)
		}
	}
	var geometry struct {
		Geometry map[string][][][][]int `json:"geometry"`
	}
	g, e := os.ReadFile("../../../reports/tetris-family-native/freeze.json")
	if e != nil || json.Unmarshal(g, &geometry) != nil {
		t.Fatal("geometry", e)
	}
	// Independent source state machine: two initial queue advances, z rotation,
	// quit still runs gravity/tick increment, then final acknowledgement.
	in := tetrisFamilyState{make([]byte, 200), map[string]int64{"cur_type": 0, "cur_rot": 0, "cur_x": 3, "cur_y": 0, "rng_state": 0xace1, "score": 0, "lines_cleared": 0, "drop_timer": 0, "drop_speed": 8, "tick": 0, "game_over": 0, "next_type": 0, "hold_type": 255, "hold_used": 0}}
	next, in, _ := tetrisSequenceModel("random_piece", nil, in, geometry.Geometry["tetris_cpm"])
	in.Globals["next_type"] = next
	_, in, _ = tetrisSequenceModel("spawn_piece", nil, in, geometry.Geometry["tetris_cpm"])
	frames := []tetrisFamilyState{tetrisFamilyCopy(in)}
	_, in, _ = tetrisSequenceModel("try_rotate", nil, in, geometry.Geometry["tetris_cpm"])
	in.Globals["drop_timer"]++
	in.Globals["tick"]++
	frames = append(frames, tetrisFamilyCopy(in))
	in.Globals["game_over"] = 1
	in.Globals["drop_timer"]++
	in.Globals["tick"]++
	// Exact ANSI oracle follows documented imported APIs (decimal coordinates are
	// two digits); intended text oracle excludes the source's SString prefix bytes.
	buildOutput := func(exact bool) []byte {
		var out bytes.Buffer
		gotoXY := func(x, y int) { fmt.Fprintf(&out, "\x1b[%02d;%02dH", y+1, x+1) }
		color := func(fg, bg, bright int) {
			if bright != 0 {
				fg += 60
				bg += 60
			}
			fmt.Fprintf(&out, "\x1b[%d;%dm", 30+fg, 40+bg)
		}
		puts := func(s string) {
			if exact {
				out.WriteByte(byte(len(s)))
			}
			out.WriteString(s)
		}
		out.WriteString("\x1b[2J\x1b[H")
		fgs := []int{6, 3, 5, 2, 1, 3, 4}
		brs := []int{0, 0, 0, 0, 0, 1, 0}
		for _, f := range frames {
			typ, rot := int(f.Globals["cur_type"]), int(f.Globals["cur_rot"])
			for _, xy := range geometry.Geometry["tetris_cpm"][typ][rot] {
				gotoXY(12+2*(int(f.Globals["cur_x"])+xy[0]), 1+int(f.Globals["cur_y"])+xy[1])
				color(0, fgs[typ], brs[typ])
				out.WriteString("  ")
			}
			color(7, 0, 0)
			gotoXY(1, 23)
			puts("Arrows:Move Z:Rot X:Drop C:Hold Q:Quit")
			out.WriteString("\x1b[0m")
		}
		color(7, 0, 1)
		gotoXY(16, 10)
		puts("GAME OVER!")
		gotoXY(14, 12)
		puts("Score: ")
		out.WriteString("00000\x1b[0m")
		gotoXY(1, 23)
		return out.Bytes()
	}
	wantBytes := buildOutput(false)
	save("expected-exact-stdout.bin", wantBytes)
	save("expected-intended-stdout.bin", buildOutput(false))
	save("expected-state.json", in)
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		h, e := nanz.ParseWithOpts(string(src), "tetris_cpm.nanz", nanz.ParseOpts{BaseDir: "../../../examples/nanz", StdlibDir: "../../../stdlib"})
		if e != nil {
			t.Fatal(e)
		}
		opt := Options{AssertMode: "none", ContractOpt: true, UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size", Runtime: mir2.Z80RuntimeCPM, RuntimeStackBytes: 1024}
		s, e := CompileHIRSteps(h, opt)
		if e != nil {
			t.Fatal(e)
		}
		asm := s.Assembly
		a, e := z80asm.NewAssembler().AssembleString(asm)
		if e != nil || len(a.Errors) > 0 {
			t.Fatal(e, a.Errors)
		}
		save(mode+"/program.a80", asm)
		save(mode+"/program.com", a.Binary)
		save(mode+"/final.mir", s.MIR2Module.Dump())
		z := emulator.NewRemogattoZ80WithScreen()
		z.SetROMEnd(0)
		mem := z.Memory()
		copy(mem, bytes.Repeat([]byte{0xa5}, 65536))
		copy(mem[0x100:], a.Binary)
		copy(mem[0x7000:], []byte{0xcd, 0x00, 0x01, 0x00})
		before := append([]byte(nil), mem...)
		after := append([]byte(nil), before...)
		for name, v := range in.Globals {
			p, ok := a.Symbols[name]
			if !ok {
				t.Fatal("global", name)
			}
			w := 1
			if name == "score" || name == "lines_cleared" || name == "rng_state" || name == "tick" {
				w = 2
			}
			for j := 0; j < w; j++ {
				after[p+j] = byte(v >> uint(j*8))
			}
		}
		copy(after[a.Symbols["board"]:], in.Board)
		z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000, A: 0x71, F: 0x85, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x2468, IY: 0x1357})
		keys := []byte{'z', 'q', 'a'}
		keyIndex := 0
		var output []byte
		var ledger []map[string]any
		stackTop := a.Symbols["__cpm_stack_top"]
		minSP := uint16(stackTop)
		done := false
		for step := 0; step < 2000000; step++ {
			pc := z.GetPC()
			if pc == 5 {
				r := z.GetRegisters()
				fn := byte(r.BC)
				switch fn {
				case 1:
					if keyIndex == len(keys) {
						t.Fatal("extra blocking input")
					}
					r.A = keys[keyIndex]
					ledger = append(ledger, map[string]any{"BDOS": 1, "input": r.A})
					keyIndex++
				case 2:
					output = append(output, byte(r.DE))
					ledger = append(ledger, map[string]any{"BDOS": 2, "output": byte(r.DE)})
				default:
					t.Fatal("unbound BDOS", fn)
				}
				r.F = 0xc7
				r.BC = 0xdead
				r.DE = 0xbeef
				r.HL = 0x1357
				r.IX = 0x9876
				r.IY = 0x2468
				r.PC = uint16(mem[r.SP]) | uint16(mem[r.SP+1])<<8
				r.SP += 2
				z.SetRegisters(r)
				continue
			}
			if pc == 0x7003 {
				done = true
				break
			}
			if pc != 0x7000 && (pc < 0x100 || int(pc) >= 0x100+len(a.Binary)) {
				t.Fatal("foreign fetch", pc)
			}
			z.Step()
			if z.GetSP() <= uint16(stackTop) && z.GetSP() < minSP {
				minSP = z.GetSP()
			}
			if int(minSP) < a.Symbols["__cpm_stack_bottom"] {
				t.Fatal("stack bound", mode, z.GetRegisters(), minSP, keyIndex)
			}
		}
		save(mode+"/actual-stdout.bin", output)
		save(mode+"/bdos-ledger.json", ledger)
		save(mode+"/registers.json", z.GetRegisters())
		if !done || keyIndex != 3 || z.GetSP() != 0xf000 {
			t.Fatal("main termination/input/SP", done, keyIndex, z.GetRegisters())
		}
		if !bytes.Equal(output, wantBytes) {
			i := 0
			for i < len(output) && i < len(wantBytes) && output[i] == wantBytes[i] {
				i++
			}
			t.Fatalf("%s stdout firstdiff%d got%q want%q", mode, i, output, wantBytes)
		}
		copy(after[minSP:stackTop], mem[minSP:stackTop])
		copy(after[0xeffe:0xf000], mem[0xeffe:0xf000])
		if mem[a.Symbols["__cpm_status"]] != 0 {
			t.Fatal("runtime status", mem[a.Symbols["__cpm_status"]])
		}
		for j := 0; j < 2; j++ {
			after[a.Symbols["__cpm_saved_sp"]+j] = mem[a.Symbols["__cpm_saved_sp"]+j]
		}
		for _, l := range a.Listing {
			if strings.HasPrefix(l.Label, "_spill_") {
				for j := range l.Bytes {
					after[l.Address+j] = mem[l.Address+j]
				}
			}
		}
		if !bytes.Equal(after, mem) {
			for p := range mem {
				if after[p] != mem[p] {
					t.Fatalf("%s RAM%04x got%d want%d", mode, p, mem[p], after[p])
				}
			}
		}
		save(mode+"/summary.json", map[string]any{"result": "PASS", "inputs": keyIndex, "output_bytes": len(output), "source_sha256": freeze.SHA, "dependency_sha256": freeze.DependencySHA, "image_bytes": len(a.Binary), "t_states": z.GetCycles(), "bdos_register_clobber_stress": true, "minimum_sp": minSP, "terminal_return_pc": "0x7003", "full_ram_guard": true, "native_whole_main": true, "intended_text_equal": bytes.Equal(output, buildOutput(false)), "scope": "original main+importedASM, public guarded runtime, host-bound BDOS1/2"})
		t.Log(mode, "public runtime exact stdout/state/OSreturn/SP PASS")
	}
}
