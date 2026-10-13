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
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Original ASM executes; only console input and SQLite port services are modeled.
func TestOriginalZSQLZXRealMainNative(t *testing.T) {
	testOriginalZSQLZXMain(t, "zsql_zx_real", "zsql-zx-capacity", "MINZ_ZSQL_ZX_REAL_EXPORT", false)
}
func TestOriginalZSQLZXEmbeddedMainNative(t *testing.T) {
	testOriginalZSQLZXMain(t, "zsql_zx", "zsql-zx-embedded-main", "MINZ_ZSQL_ZX_EMBEDDED_EXPORT", true)
}
func testOriginalZSQLZXMain(t *testing.T, sourceName, packetName, exportEnv string, embedded bool) {
	packet := "../../../reports/" + packetName + "/frozen/"
	bufferName := "_inbuf"
	if embedded {
		bufferName = "_zx_inbuf"
	}
	read := func(n string) []byte {
		b, e := os.ReadFile(packet + n)
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	src := read("source.nanz")
	current, e := os.ReadFile("../../../examples/nanz/" + sourceName + ".nanz")
	if e != nil || !bytes.Equal(src, current) {
		t.Fatal("source", e)
	}
	var f struct {
		SHA       string   `json:"source_sha256"`
		SQLiteSHA string   `json:"sqlite_sha256"`
		Inputs    []string `json:"inputs"`
		Commands  []int    `json:"commands"`
		Texts     []string `json:"texts"`
		OutputSHA string   `json:"output_sha256"`
	}
	if e = json.Unmarshal(read("freeze.json"), &f); e != nil {
		t.Fatal(e)
	}
	out := read("output.bin")
	lib, e := os.ReadFile("../../../stdlib/sql/sqlite.nanz")
	if e != nil || !bytes.Equal(lib, read("sqlite.nanz")) || fmt.Sprintf("%x", sha256.Sum256(lib)) != f.SQLiteSHA || fmt.Sprintf("%x", sha256.Sum256(src)) != f.SHA || fmt.Sprintf("%x", sha256.Sum256(out)) != f.OutputSHA {
		t.Fatal("freeze", e)
	}
	save := func(n string, v any) {
		if d := os.Getenv(exportEnv); d != "" {
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
			h, e := nanz.ParseWithOpts(string(src), sourceName+".nanz", nanz.ParseOpts{BaseDir: "../../../examples/nanz", StdlibDir: "../../../stdlib"})
			if e != nil {
				t.Fatal(e)
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
					t.Fatal("symbol", n)
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
			for ch := 32; ch < 128; ch++ {
				for line := 0; line < 8; line++ {
					mem[0x3d00+(ch-32)*8+line] = byte((ch*17 + line*29) ^ 0x5a)
				}
			}
			font := append([]byte(nil), mem[0x3d00:0x4000]...)
			if embedded {
				start := strings.Index(string(src), "fun _font_anchor()")
				if start < 0 {
					t.Fatal("font source")
				}
				font = nil
				for _, m := range regexp.MustCompile(`(?m)^\s*DB (.+)$`).FindAllStringSubmatch(string(src)[start:], -1) {
					for _, literal := range strings.Split(m[1], ",") {
						v, err := strconv.ParseUint(strings.TrimPrefix(strings.TrimSpace(literal), "$"), 16, 8)
						if err != nil {
							t.Fatal(err)
						}
						font = append(font, byte(v))
					}
				}
				base := sym("_font_anchor")
				if !bytes.Equal(mem[base:int(base)+len(font)], font) {
					t.Fatal("assembled font")
				}
			}
			p := sym("main")
			copy(mem[0x7000:], []byte{0xcd, byte(p), byte(p >> 8), 0})
			want := append([]byte(nil), mem...)
			for p := 0x4000; p < 0x5800; p++ {
				want[p] = 0
			}
			for p := 0x5800; p < 0x5b00; p++ {
				want[p] = 0x47
			}
			for p := 0x5800; p < 0x5840; p++ {
				want[p] = 0x4f
			}
			if embedded {
				for p := 0x5820; p < 0x5840; p++ {
					want[p] = 0x4e
				}
			}
			row, col := 0, 0
			for _, ch := range out {
				if ch == 13 || ch == 10 {
					row++
					col = 0
					continue
				}
				for line := 0; line < 8; line++ {
					p := 0x4000 | ((row & 0x18) << 8) | (line << 8) | ((row & 7) << 5) | col
					if int(ch) < 32 || (int(ch)-32)*8+line >= len(font) {
						t.Fatal("font character", ch)
					}
					want[p] = font[(int(ch)-32)*8+line]
				}
				col++
				if col == 32 {
					col = 0
					row++
				}
			}
			want[sym("_zx_row")] = byte(row)
			want[sym("_zx_col")] = byte(col)
			want[sym("db")] = 1
			want[sym("db")+1] = 0
			want[sym("_sel_stmt")] = 1
			want[sym("_sel_stmt")+1] = 0
			if len(f.Inputs) != 16 || len(f.Inputs[0]) == 0 || len(f.Inputs[1]) == 0 || len(f.Inputs[2]) == 0 {
				t.Fatal("frozen dialogue")
			}
			capacity := 0
			for _, g := range h.Globals {
				if g.Name == bufferName {
					if a, ok := g.Ty.(*mir2.ArrayTy); ok {
						capacity = a.Len
					}
				}
			}
			if capacity != 61 {
				t.Fatal("source input storage layout")
			}
			for _, line := range f.Inputs {
				if len(line) > 60 {
					t.Fatal("workload exceeds 60-character storage")
				}
			}
			inputWant := make([]byte, 61)
			var input []byte
			for _, line := range f.Inputs {
				input = append(input, []byte(line)...)
				input = append(input, 13)
				copy(inputWant, line)
				inputWant[len(line)] = 0
			}
			copy(want[sym(bufferName):], inputWant)
			buf := sym("sql__sqlite___sql_buf")
			for _, text := range f.Texts {
				copy(want[buf:], append([]byte(text), 0))
			}
			host := &sqliteNativeHost{t: t}
			defer func() {
				if host.rows != nil {
					host.rows.Close()
				}
				if host.db != nil {
					host.db.Close()
				}
			}()
			inputPos, borders := 0, 0
			var ports []map[string]any
			z.SetIOHandlers(func(p uint16) byte {
				if byte(p>>8) != z.GetRegisters().A {
					t.Fatal("IN high", p)
				}
				var v byte
				if byte(p) == 0x23 {
					if inputPos >= len(input) {
						t.Fatal("extra console input")
					}
					v = input[inputPos]
					inputPos++
				} else {
					v = host.read(p)
				}
				ports = append(ports, map[string]any{"in": int(p), "value": int(v)})
				return v
			}, func(p uint16, v byte) {
				ports = append(ports, map[string]any{"out": int(p), "value": int(v)})
				if byte(p) == 0xfe {
					if borders != 0 || v != 1 || byte(p>>8) != v {
						t.Fatal("border", p, v)
					}
					borders++
				} else {
					host.write(p, v)
				}
			})
			z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000, A: 0x71, F: 0x85, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x2468, IY: 0x1357})
			minSP := uint16(0xf000)
			printed := 0
			halted := false
			column, offset := 0, 0
			for n := 0; n < 2000000; n++ {
				pc := z.GetPC()
				r := z.GetRegisters()
				if pc != 0x7000 && (pc < 0x8000 || int(pc) >= 0x8000+len(a.Binary)) {
					t.Fatal("foreign fetch", pc)
				}
				if pc == sym("_zxch") {
					if printed >= len(out) || r.A != out[printed] {
						t.Fatalf("output %d got %02x", printed, r.A)
					}
					printed++
				}
				if pc >= sym("sql__sqlite___sql_recv_str") && pc < sym("sql__sqlite___sql_recv_str")+20 && mem[pc] == 0x77 {
					if column >= len(f.Texts) {
						t.Fatal("extra receive")
					}
					w := append([]byte(f.Texts[column]), 0)
					if r.HL != buf+uint16(offset) || r.A != w[offset] {
						t.Fatal("receive write", column, offset, r)
					}
					offset++
					if offset == len(w) {
						column++
						offset = 0
					}
				}
				if mem[pc] == 0x76 {
					if mem[pc-1] != 0xf3 || r.SP != 0xeffe {
						t.Fatal("HALT contract", r)
					}
					halted = true
					break
				}
				z.Step()
				if z.GetSP() < minSP {
					minSP = z.GetSP()
				}
				if minSP < 0xe000 || z.GetSP() > 0xf000 {
					t.Fatal("stack bounds")
				}
			}
			save(mode+"/commands.json", host.commands)
			save(mode+"/ports.json", ports)
			save(mode+"/ram.bin", mem)
			if !halted || printed != len(out) || inputPos != len(input) || borders != 1 || column != len(f.Texts) || offset != 0 || len(host.commands) != len(f.Commands) || host.rows != nil || host.db != nil {
				t.Fatal("incomplete", halted, printed, inputPos, column, len(host.commands))
			}
			stringsSeen := []string{}
			textsSeen := []string{}
			for _, c := range host.commands {
				if text, ok := c["string"].(string); ok {
					stringsSeen = append(stringsSeen, text)
				}
				if text, ok := c["text"].(string); ok {
					textsSeen = append(textsSeen, text)
				}
			}
			if !bytes.Equal([]byte(strings.Join(stringsSeen, "\x00")), []byte(strings.Join(append([]string{":memory:"}, f.Inputs[:3]...), "\x00"))) || strings.Join(textsSeen, "\x00") != strings.Join(f.Texts, "\x00") {
				t.Fatal("SQL strings/column results", stringsSeen, textsSeen)
			}
			for i, c := range host.commands {
				if c["command"].(int) != f.Commands[i] {
					t.Fatal("command order", i, c)
				}
			}
			if uint16(mem[0xeffe])|uint16(mem[0xefff])<<8 != 0x7003 {
				t.Fatal("loader return word")
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
			save(mode+"/screen.bin", mem[0x4000:0x5b00])
			save(mode+"/summary.json", map[string]any{"bytes": len(a.Binary), "T_states": z.GetCycles(), "output_bytes": printed, "row": row, "column": col, "minimum_sp": minSP, "full_ram": true, "termination": "original DI/HALT"})
			t.Log(len(a.Binary), z.GetCycles(), printed, "screen/SQL/full RAM PASS")
		})
	}
}
