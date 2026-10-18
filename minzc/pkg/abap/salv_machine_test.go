package abap

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Terminal decoder only: it consumes standard control bytes produced by the
// compiled program. Expected cell/layout screens come from the independent model.
type salvTerminal struct {
	cells               [24][80]byte
	x, y, state, clears int
	csi                 string
	cursor              bool
}

func newSalvTerminal() *salvTerminal { v := &salvTerminal{cursor: true}; v.clear(); return v }
func (v *salvTerminal) clear() {
	for y := range v.cells {
		for x := range v.cells[y] {
			v.cells[y][x] = ' '
		}
	}
}
func (v *salvTerminal) down() {
	v.y++
	if v.y == 24 {
		copy(v.cells[:23], v.cells[1:])
		for x := range v.cells[23] {
			v.cells[23][x] = ' '
		}
		v.y = 23
	}
}
func (v *salvTerminal) put(ch byte) {
	if v.state == 1 {
		v.state = 0
		if ch == '[' {
			v.state = 2
			v.csi = ""
		}
		return
	}
	if v.state == 2 {
		if ch < 0x40 || ch > 0x7e {
			v.csi += string(ch)
			return
		}
		v.state = 0
		nums := strings.Split(v.csi, ";")
		number := func(i, def int) int {
			if i >= len(nums) || nums[i] == "" {
				return def
			}
			n, e := strconv.Atoi(nums[i])
			if e != nil {
				return def
			}
			return n
		}
		switch ch {
		case 'H', 'f':
			v.y = number(0, 1) - 1
			v.x = number(1, 1) - 1
			if v.x < 0 || v.x >= 80 || v.y < 0 || v.y >= 24 {
				panic("invalid cursor")
			}
		case 'J':
			if number(0, 0) == 2 {
				v.clear()
				v.clears++
			}
		case 'K':
			if number(0, 0) == 2 {
				for x := range v.cells[v.y] {
					v.cells[v.y][x] = ' '
				}
			}
		case 'h':
			if v.csi == "?25" {
				v.cursor = true
			}
		case 'l':
			if v.csi == "?25" {
				v.cursor = false
			}
		}
		return
	}
	switch ch {
	case 27:
		v.state = 1
	case 13:
		v.x = 0
	case 10:
		v.down()
	default:
		if ch >= 32 && ch <= 126 {
			v.cells[v.y][v.x] = ch
			v.x++
			if v.x == 80 {
				v.x = 0
				v.down()
			}
		}
	}
}
func (v *salvTerminal) screen() string {
	var rows []string
	for _, row := range v.cells {
		rows = append(rows, string(row[:]))
	}
	return strings.Join(rows, "\n")
}
func salvModel(t *testing.T, name string) string {
	t.Helper()
	data, e := os.ReadFile("testdata/salv-model/fixtures.json")
	if e != nil {
		t.Fatal(e)
	}
	var m struct {
		Scope string `json:"scope"`
		Cases []struct {
			Name   string   `json:"name"`
			Screen []string `json:"screen"`
		} `json:"cases"`
	}
	if e := json.Unmarshal(data, &m); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(m.Scope, "independent") {
		t.Fatal("model provenance")
	}
	for _, c := range m.Cases {
		if c.Name == name {
			if len(c.Screen) != 24 {
				t.Fatal("model height")
			}
			for _, r := range c.Screen {
				if len(r) != 80 {
					t.Fatal("model width")
				}
			}
			return strings.Join(c.Screen, "\n")
		}
	}
	t.Fatal("missing independent model", name)
	return ""
}

type salvCase struct {
	name, source string
	keys         []byte
	files        map[string]string
	entries      []string
	screens      []string
	objects      map[string][]byte
	// Typed binary storage inputs, separate from the ASCII-only ABAP source.
	// Replaces an exact literal payload before all three engines lower it.
	literalInputs map[string][]byte
}

func runSalvCase(t *testing.T, c salvCase) {
	t.Helper()
	hm, e := Compile(c.source, "zsalv")
	if e != nil {
		t.Fatal(e)
	}
	for old, data := range c.literalInputs {
		found := false
		for i, value := range hm.Strings {
			if value == old {
				if len(value) != len(data) {
					t.Fatal("binary input width differs")
				}
				hm.Strings[i] = string(data)
				found = true
			}
		}
		if !found {
			t.Fatal("missing binary storage input literal", old)
		}
	}
	if e := hir.Verify(hm); e != nil {
		t.Fatal(e)
	}
	opts := pipeline.DefaultOptions()
	opts.AssertMode = "none"
	steps, e := pipeline.CompileHIRSteps(hm, opts)
	if e != nil {
		t.Fatal(e)
	}
	text, e := mir2.Z80Codegen(steps.MIR2Module, steps.Allocation, mir2.Z80CodegenOptions{CPMApplicationEntry: true, UnsignedPairComparePolicy: steps.UnsignedPairComparePolicy})
	if e != nil {
		t.Fatal(e)
	}
	a, e := z80asm.NewAssembler().AssembleString(text)
	if e != nil || len(a.Errors) != 0 {
		t.Fatalf("assembly %v %v", e, a.Errors)
	}
	raw := hir.LowerModule(hm)
	check := func(t *testing.T, read func(string, int) []byte, frames []string) {
		t.Helper()
		for name, want := range c.objects {
			if got := read(name, len(want)); !bytes.Equal(got, want) {
				t.Fatalf("%s=%x want%x", name, got, want)
			}
		}
		if read("_abap_error_code", 1)[0] != 0 {
			t.Fatal("runtime fault", read("_abap_error_code", 1))
		}
		if len(frames) != len(c.screens) {
			t.Fatalf("frames%d want%d", len(frames), len(c.screens))
		}
		for i, want := range c.screens {
			if frames[i] != want {
				var differences []string
				gotRows := strings.Split(frames[i], "\n")
				wantRows := strings.Split(want, "\n")
				for r := range gotRows {
					if gotRows[r] != wantRows[r] {
						differences = append(differences, fmt.Sprintf("row%d got%q want%q", r+1, gotRows[r], wantRows[r]))
					}
				}
				t.Fatalf("frame%d\n%s", i, strings.Join(differences, "\n"))
			}
		}
	}
	observe := func(term *salvTerminal, frames *[]string) {
		if term.clears == 0 {
			return
		}
		s := term.screen()
		if len(*frames) == 0 || (*frames)[len(*frames)-1] != s {
			*frames = append(*frames, s)
		}
	}
	for index, module := range []*mir2.Module{raw, steps.MIR2Module} {
		t.Run(fmt.Sprintf("vm-%d", index), func(t *testing.T) {
			v := mir2.NewVM(module)
			v.MaxSteps = 10000000
			p := frontendProbe{files: c.files, entries: c.entries, keys: append([]byte{}, c.keys...), dma: 0x80}
			term := newSalvTerminal()
			var frames []string
			put := func(b byte) { term.put(b) }
			v.Hosts["_abap_num_putchar"] = func(args []mir2.Value) ([]mir2.Value, error) { put(byte(args[0].I)); return nil, nil }
			v.Hosts["abap_write_str"] = func(args []mir2.Value) ([]mir2.Value, error) {
				for addr := args[0].I; ; addr++ {
					b := v.ReadHeap(addr, 1)[0]
					if b == 0 {
						break
					}
					put(b)
				}
				return nil, nil
			}
			v.Hosts["sel_show"] = func([]mir2.Value) ([]mir2.Value, error) { return []mir2.Value{{I: 0}}, nil }
			v.Hosts["sel_register_c"] = func([]mir2.Value) ([]mir2.Value, error) { return nil, nil }
			v.Hosts["_abap_input_line"] = func([]mir2.Value) ([]mir2.Value, error) { return []mir2.Value{{I: 0}}, nil }
			for name, fn := range map[string]byte{"open": 15, "close": 16, "read": 20, "first": 17, "next": 18, "key": 6} {
				fn := fn
				v.Hosts["_abap_fs_bdos_"+name] = func([]mir2.Value) ([]mir2.Value, error) {
					if fn == 20 || fn == 17 || fn == 18 {
						p.dma = uint16(v.GlobalAddr("_abap_fs_dma_buffer"))
					}
					rc := p.bdos(fn, uint16(v.GlobalAddr("_abap_fs_fcb_buffer")), func(a uint16) byte { return v.ReadHeap(int64(a), 1)[0] }, func(a uint16, b byte) { v.WriteHeapBytes(int64(a), []byte{b}) })
					if fn == 20 || fn == 17 || fn == 18 {
						p.dma = 0x80
					}
					return []mir2.Value{{I: int64(rc)}}, nil
				}
			}
			v.Hosts["_abap_salv_key_poll"] = func([]mir2.Value) ([]mir2.Value, error) {
				observe(term, &frames)
				rc := p.bdos(6, 255, nil, nil)
				return []mir2.Value{{I: int64(rc)}}, nil
			}
			if _, e := v.Call("main", nil); e != nil {
				t.Fatalf("VM %v", e)
			}
			if p.opened != "" {
				t.Fatal("file leaked")
			}
			if !term.cursor {
				t.Fatal("cursor not restored")
			}
			check(t, func(name string, n int) []byte { return v.ReadHeap(v.GlobalAddr(name), n) }, frames)
		})
	}
	t.Run("native", func(t *testing.T) {
		z := emulator.NewRemogattoZ80WithScreen()
		z.SetROMEnd(0)
		mem := z.Memory()
		copy(mem, bytes.Repeat([]byte{0xa5}, 65536))
		copy(mem[0x100:], a.Binary)
		binary.LittleEndian.PutUint16(mem[6:8], 0xfe00)
		binary.LittleEndian.PutUint16(mem[0x80:0x82], 0xf100)
		bottom, top := a.Symbols["__cpm_app_stack"], a.Symbols["__cpm_app_stack_top"]
		minSP := top
		z.SetRegisters(emulator.Registers{PC: 0x100, SP: 0x80})
		p := frontendProbe{files: c.files, entries: c.entries, keys: append([]byte{}, c.keys...), dma: 0x80}
		term := newSalvTerminal()
		var frames []string
		var output strings.Builder
		for i := 0; i < 10000000 && z.GetPC() != 0xf100 && !z.IsHalted(); i++ {
			sp := int(z.GetSP())
			if sp != 0x80 && (sp < bottom || sp > top) {
				t.Fatalf("SP%04x outside resident stack", sp)
			}
			if sp >= bottom && sp < minSP {
				minSP = sp
			}
			if z.GetPC() != 5 {
				z.Step()
				continue
			}
			r := z.GetRegisters()
			rc := byte(0)
			switch byte(r.BC) {
			case 2:
				term.put(byte(r.DE))
				output.WriteByte(byte(r.DE))
			case 10:
				mem[r.DE+1] = 0
			case 6:
				observe(term, &frames)
				rc = p.bdos(6, r.DE, nil, nil)
			case 15, 16, 17, 18, 20, 26:
				rc = p.bdos(byte(r.BC), r.DE, func(a uint16) byte { return mem[a] }, func(a uint16, b byte) { mem[a] = b })
			default:
				t.Fatalf("BDOS%d", byte(r.BC))
			}
			r.PC = binary.LittleEndian.Uint16(mem[r.SP : r.SP+2])
			r.SP += 2
			r.A, r.F, r.BC, r.DE, r.HL = rc, 0, 0x1234, 0xabcd, 0xbeef
			z.SetRegisters(r)
		}
		if z.GetPC() != 0xf100 || z.GetSP() != 0x82 || z.IsHalted() {
			t.Fatalf("return PC%04x SP%04x halted%t", z.GetPC(), z.GetSP(), z.IsHalted())
		}
		if mem[a.Symbols["__cpm_app_stack_canary"]] != 0xa5 || minSP < bottom || minSP >= top {
			t.Fatal("resident stack/canary")
		}
		if binary.LittleEndian.Uint16(mem[0x80:0x82]) != 0xf100 {
			t.Fatal("outer return damaged")
		}
		if p.opened != "" || p.dma != 0x80 {
			t.Fatal("file/DMA leak")
		}
		if !term.cursor {
			t.Fatal("cursor not restored")
		}
		check(t, func(name string, n int) []byte {
			address, ok := nativeSymbol(a.Symbols, name)
			if !ok {
				t.Fatal("missing symbol", name)
			}
			return mem[address : address+n]
		}, frames)
		t.Logf("COM%d imageEnd%04x stack%04x..%04x minSP%04x used%d PCf100 SP0082 canaryA5", len(a.Binary), 0x100+len(a.Binary), bottom, top, minSP, top-minSP)
		if root := os.Getenv("MINZ_ABAP_SALV_ARTIFACTS"); root != "" {
			if e := os.MkdirAll(root, 0755); e != nil {
				t.Fatal(e)
			}
			meta, _ := json.MarshalIndent(map[string]any{"com_bytes": len(a.Binary), "image_end": 0x100 + len(a.Binary), "stack_bottom": bottom, "stack_top": top, "min_sp": minSP, "pc": z.GetPC(), "sp": z.GetSP(), "canary": 165, "symbols": a.Symbols}, "", "  ")
			files := map[string][]byte{"source.abap": []byte(c.source), "a80": []byte(text), "com": a.Binary, "hir": []byte(hm.Dump()), "mir2-raw": []byte(raw.Dump()), "mir2-final": []byte(steps.MIR2Module.Dump()), "stdout": []byte(output.String()), "native.json": meta}
			for i, frame := range frames {
				files[fmt.Sprintf("frame-%d.txt", i)] = []byte(frame + "\n")
			}
			for ext, data := range files {
				if e := os.WriteFile(filepath.Join(root, c.name+"."+ext), data, 0644); e != nil {
					t.Fatal(e)
				}
			}
		}
	})
}

func TestSALVFrozenReportMachines(t *testing.T) {
	src, e := os.ReadFile("../../../examples/abap/zfleet_csv_alv.abap")
	if e != nil {
		t.Fatal(e)
	}
	files := map[string]string{}
	for _, name := range []string{"EXTRA.CSV", "FLEET.CSV"} {
		data, e := os.ReadFile("../../../examples/abap/fleet/" + name)
		if e != nil {
			t.Fatal(e)
		}
		files[name] = string(data)
	}
	line := func(name, route string, seats uint32) []byte {
		return binary.LittleEndian.AppendUint32([]byte(fmt.Sprintf("%-20s%-20s", name, route)), seats)
	}
	// F4 READ sets TABIX1; existing report LOOP boundaries restore that outer
	// value. DISPLAY must preserve it (separate before/after cases follow).
	runSalvCase(t, salvCase{name: "frozen-fleet", source: string(src), keys: []byte("\x1bOS\x1b[B\r\rq"), files: files, entries: []string{"EXTRA.CSV", "FLEET.CSV"}, screens: []string{salvModel(t, "fleet")}, objects: map[string][]byte{"fleet": append(line("Atlas", "North", 40), line("Beacon", "South", 32)...), "_itab_count_fleet": {2, 0, 0, 0}, "total": {72, 0, 0, 0}, "_abap_subrc_value": {0, 0, 0, 0}, "_abap_tabix_value": {1, 0, 0, 0}}})
}
