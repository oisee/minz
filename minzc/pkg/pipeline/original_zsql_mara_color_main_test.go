package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Static demonstration, not a SQL workload: original main has no SQL imports.
func TestOriginalZSQLMaraColorMainNative(t *testing.T) {
	const packet = "../../../reports/zsql-mara-color-main/frozen/"
	src, e := os.ReadFile("../../../examples/abap/zsql_mara_color_zx.nanz")
	if e != nil {
		t.Fatal(e)
	}
	font, e := os.ReadFile(packet + "font.bin")
	if e != nil || len(font) != 760 {
		t.Fatal("font", e)
	}
	var freeze struct {
		SourceSHA  string   `json:"source_sha256"`
		FontSHA    string   `json:"font_sha256"`
		OutputSHA  string   `json:"output_sha256"`
		Lines      []string `json:"lines"`
		Attributes [][2]int `json:"attributes"`
		Initial    byte     `json:"initial_attribute"`
	}
	b, e := os.ReadFile(packet + "freeze.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(b, &freeze); e != nil {
		t.Fatal(e)
	}
	frozenSource, err := os.ReadFile(packet + "source.nanz")
	if err != nil || !bytes.Equal(src, frozenSource) || fmt.Sprintf("%x", sha256.Sum256(src)) != freeze.SourceSHA || fmt.Sprintf("%x", sha256.Sum256(font)) != freeze.FontSHA {
		t.Fatal("source/font freeze", err)
	}
	output, e := os.ReadFile(packet + "output.bin")
	if e != nil || fmt.Sprintf("%x", sha256.Sum256(output)) != freeze.OutputSHA || string(output) != strings.Join(freeze.Lines, "\r")+"\r" {
		t.Fatal("output freeze", e)
	}
	save := func(n string, v any) {
		if d := os.Getenv("MINZ_MARA_COLOR_EXPORT"); d != "" {
			var b []byte
			switch x := v.(type) {
			case []byte:
				b = x
			case string:
				b = []byte(x)
			default:
				var e error
				b, e = json.MarshalIndent(v, "", "  ")
				if e != nil {
					t.Fatal(e)
				}
			}
			p := filepath.Join(d, n)
			if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(p, b, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		t.Run(mode, func(t *testing.T) {
			h, e := nanz.Parse(string(src), "zsql_mara_color_zx.nanz")
			if e != nil {
				t.Fatal(e)
			}
			capacity := 0
			for _, g := range h.Globals {
				if g.Name == "_zx_font" {
					capacity = g.Ty.Width() / 8
				}
			}
			if capacity != len(font) {
				t.Fatalf("source font capacity %d bytes, printable font needs %d", capacity, len(font))
			}
			s, e := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none", UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
			if e != nil {
				t.Fatal(e)
			}
			asm := "ORG 0x8000\n" + s.Assembly
			a, e := z80asm.NewAssembler().AssembleString(asm)
			if e != nil || len(a.Errors) > 0 {
				t.Fatal(e, a.Errors)
			}
			sym := func(n string) uint16 {
				p, ok := a.Symbols[n]
				if !ok {
					t.Fatal(n)
				}
				return uint16(p)
			}
			save(mode+"/program.a80", asm)
			save(mode+"/program.bin", a.Binary)
			save(mode+"/final.mir", s.MIR2Module.Dump())
			save(mode+"/allocation.json", s.Allocation)
			z := emulator.NewRemogattoZ80WithScreen()
			z.SetROMEnd(0)
			z.Hooks = nil
			mem := z.Memory()
			copy(mem, bytes.Repeat([]byte{0xa5}, 65536))
			copy(mem[0x8000:], a.Binary)
			if !bytes.Equal(mem[sym("_zx_font"):int(sym("_zx_font"))+len(font)], font) {
				t.Fatal("font bytes")
			}
			p := sym("main")
			copy(mem[0x7000:], []byte{0xcd, byte(p), byte(p >> 8), 0})
			want := append([]byte(nil), mem...)
			for p := 0x4000; p < 0x5800; p++ {
				want[p] = 0
			}
			for p := 0x5800; p < 0x5b00; p++ {
				want[p] = freeze.Initial
			}
			for _, a := range freeze.Attributes {
				for col := 0; col < 32; col++ {
					want[0x5800+a[0]*32+col] = byte(a[1])
				}
			}
			row, col := 0, 0
			for _, ch := range output {
				if ch == 13 {
					row++
					col = 0
					continue
				}
				if ch < 32 || ch > 126 {
					t.Fatal("glyph", ch)
				}
				for line := 0; line < 8; line++ {
					addr := 0x4000 | ((row & 0x18) << 8) | (line << 8) | ((row & 7) << 5) | col
					want[addr] = font[(int(ch)-32)*8+line]
				}
				col++
				if col == 32 {
					row++
					col = 0
				}
			}
			want[sym("_zx_row")] = byte(row)
			want[sym("_zx_col")] = byte(col)
			borders := 0
			z.SetIOHandlers(func(p uint16) byte { t.Fatal("unexpected input", p); return 0 }, func(p uint16, v byte) {
				if byte(p) != 0xfe || byte(p>>8) != 1 || v != 1 || borders != 0 {
					t.Fatal("border", p, v)
				}
				borders++
			})
			z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000, A: 0x71, F: 0x85, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x2468, IY: 0x1357})
			minSP := uint16(0xf000)
			printed := 0
			attr := 0
			halt := false
			for step := 0; step < 2000000; step++ {
				pc := z.GetPC()
				r := z.GetRegisters()
				if pc != 0x7000 && (pc < 0x8000 || int(pc) >= 0x8000+len(a.Binary)) {
					t.Fatal("foreign fetch", pc)
				}
				if pc == sym("_zxch") {
					if printed >= len(output) || r.A != output[printed] {
						t.Fatal("text byte", printed, r.A)
					}
					printed++
				}
				if pc == sym("zx_attr_row") {
					if attr >= len(freeze.Attributes) || int(r.A) != freeze.Attributes[attr][0] || int(byte(r.BC)) != freeze.Attributes[attr][1] {
						t.Fatal("attribute call", attr, r)
					}
					attr++
				}
				if mem[pc] == 0x76 {
					if mem[pc-1] != 0xf3 || r.SP != 0xeffe {
						t.Fatal("HALT", r)
					}
					halt = true
					break
				}
				z.Step()
				if z.GetSP() < minSP {
					minSP = z.GetSP()
				}
				if minSP < 0xe000 || z.GetSP() > 0xf000 {
					t.Fatal("stack")
				}
			}
			if !halt || printed != len(output) || attr != len(freeze.Attributes) || borders != 1 || uint16(mem[0xeffe])|uint16(mem[0xefff])<<8 != 0x7003 {
				t.Fatal("incomplete", halt, printed, attr, borders)
			}
			copy(want[minSP:0xf000], mem[minSP:0xf000])
			for _, l := range a.Listing {
				if strings.HasPrefix(l.Label, "_spill_") {
					copy(want[l.Address:l.Address+len(l.Bytes)], mem[l.Address:l.Address+len(l.Bytes)])
				}
			}
			if !bytes.Equal(want, mem) {
				for p := range mem {
					if want[p] != mem[p] {
						t.Fatalf("RAM %04x got %02x want %02x", p, mem[p], want[p])
					}
				}
			}
			save(mode+"/ram.bin", mem)
			save(mode+"/screen.bin", mem[0x4000:0x5b00])
			save(mode+"/summary.json", map[string]any{"bytes": len(a.Binary), "T_states": z.GetCycles(), "text_bytes": printed, "attribute_calls": attr, "minimum_sp": minSP, "row": row, "column": col, "full_ram": true, "SQL_executed": false})
			t.Log(len(a.Binary), z.GetCycles(), printed, attr, "static screen/full RAM PASS")
		})
	}
}
