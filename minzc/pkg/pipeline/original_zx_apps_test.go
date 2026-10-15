package pipeline

import (
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"testing"
)

func TestOriginalZXAppsNative(t *testing.T) {
	paths := []string{"examples/abap/hello_zx.nanz", "examples/abap/sap_report_zx.nanz", "examples/abap/zsql_mara_color_zx.nanz", "examples/nanz/zsql_zx.nanz", "examples/nanz/zsql_zx_real.nanz"}
	export := func(name string, b []byte) {
		if d := os.Getenv("ZX_APPS_EXPORT"); d != "" {
			p := filepath.Join(d, name)
			os.MkdirAll(filepath.Dir(p), 0755)
			if e := os.WriteFile(p, b, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
	for i, path := range paths {
		if i == 2 {
			// The complete byte-array font and text overflow are covered by the
			// frozen full-memory oracle, replacing the obsolete u16-font probe.
			t.Run("source2/full-main", TestOriginalZSQLMaraColorMainNative)
			continue
		}
		for _, mode := range []struct {
			name string
			o    Options
		}{{"ordinary", Options{ContractOpt: true, AssertMode: "none"}}, {"grace-size", Options{ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true, OptSize: true}}, {"grace-speed", Options{ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true}}} {
			t.Run(fmt.Sprintf("source%d/%s", i, mode.name), func(t *testing.T) {
				b, e := os.ReadFile("../../../" + path)
				if e != nil {
					t.Fatal(e)
				}
				prefix := fmt.Sprintf("source%d/%s", i, mode.name)
				result := map[string]any{"source": path, "mode": mode.name}
				defer func() { out, _ := json.MarshalIndent(result, "", "  "); export(prefix+".json", out) }()
				h, e := nanz.ParseWithOpts(string(b), path, nanz.ParseOpts{BaseDir: filepath.Dir("../../../" + path), StdlibDir: "../../../stdlib"})
				if e != nil {
					result["parse_error"] = e.Error()
					t.Log(e)
					return
				}
				s, e := CompileHIRSteps(h, mode.o)
				if e != nil {
					result["compile_error"] = e.Error()
					t.Log(e)
					return
				}
				export(prefix+".a80", []byte(s.Assembly))
				export(prefix+".mir", []byte(s.MIR2Module.Dump()))
				a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
				if e != nil || a == nil || len(a.Errors) > 0 {
					result["assemble_errors"] = fmt.Sprint(e, a.Errors)
					t.Log(e, a.Errors)
					return
				}
				export(prefix+".bin", a.Binary)
				result["bytes"] = len(a.Binary)
				abi := map[string]any{}
				for _, f := range s.MIR2Module.Funcs {
					p := map[string]string{}
					for _, v := range f.Contract.Params {
						p[v.Name] = s.Allocation.Locs[v.Reg].Name
					}
					abi[f.Name] = p
				}
				result["actual_parameter_homes"] = abi
				if i >= 3 {
					result["native_scope"] = "not executed: requires real SQLite port bridge model"
					return
				}
				z := emulator.NewRemogattoZ80WithScreen()
				z.SetROMEnd(0)
				prof := emulator.NewProfiler()
				z.SetProfiler(prof)
				for n := range z.Memory() {
					z.Memory()[n] = byte(n*41 + 29)
				}
				z.LoadMemory(0x8000, a.Binary)
				for ch := 32; ch < 128; ch++ {
					for line := 0; line < 8; line++ {
						z.Memory()[0x3d00+(ch-32)*8+line] = byte((ch*17 + line*29) ^ 0x5a)
					}
				}
				reads := []uint16{}
				writes := []uint16{}
				values := []byte{}
				z.SetIOHandlers(func(p uint16) byte {
					reads = append(reads, p)
					if len(reads) > 8 && byte(p>>8) == 0xfd {
						return 0xfe
					}
					return 0xff
				}, func(p uint16, v byte) { writes = append(writes, p); values = append(values, v) })
				target := a.Symbols["main"]
				z.LoadMemory(0x7000, []byte{0xcd, byte(target), byte(target >> 8), 0x76})
				z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000, A: 0x97, F: 0x45, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468})
				before := append([]byte(nil), z.Memory()...)
				done := false
				halt := false
				cycles := int64(0)
				depth := uint16(0xf000)
				for n := 0; n < 2000000; n++ {
					if z.GetPC() == 0x7003 && z.GetSP() == 0xf000 {
						done = true
						break
					}
					if z.Memory()[z.GetPC()] == 0x76 {
						halt = true
						break
					}
					cycles += int64(z.Step())
					if z.GetSP() < depth {
						depth = z.GetSP()
					}
				}
				result["returned"] = done
				result["halted"] = halt
				result["registers"] = z.GetRegisters()
				result["min_sp"] = depth
				result["t_states_before_terminal"] = cycles
				result["read_ports"] = reads
				result["write_ports"] = writes
				result["write_values"] = values
				want := append([]byte(nil), before[0x4000:0x5b00]...)
				switch i {
				case 0:
					for n := range want {
						want[n] = 0
					}
					for n := 0x1800; n < len(want); n++ {
						want[n] = 15
					}
					row, col, attr := 0, 0, byte(15)
					put := func(text string) {
						for _, ch := range []byte(text) {
							if ch == 10 {
								row++
								col = 0
								continue
							}
							for line := 0; line < 8; line++ {
								want[((row&0x18)<<8)|(line<<8)|((row&7)<<5)|col] = byte((int(ch)*17 + line*29) ^ 0x5a)
							}
							want[0x1800+row*32+col] = attr
							col++
						}
					}
					attr = 14
					put("=== Z80 System Report ===\n\n")
					for _, pair := range [][2]string{{"CPU:      ", "Z80A"}, {"Clock:    ", "3.5 MHz"}, {"RAM:      ", "48 KB"}, {"ROM:      ", "16 KB"}, {"Kernel:   ", "MinZ v0.21"}, {"Runtime:  ", "ABAP on Z80"}} {
						attr = 15
						put(pair[0])
						attr = 11
						put(pair[1] + "\n")
					}
					put("\n")
					attr = 14
					put("=========================\n")
					attr = 10
					put("STATUS: ALL SYSTEMS GO\n\n")
					attr = 7
					put("Press any key...\nHello from ABAP on ZX!\n")
					result["expected_row"] = row
					result["expected_col"] = col

				case 1:
					for n := 0x1800; n < len(want); n++ {
						want[n] = 0
					}
					colors := map[int]byte{0: 0x4f, 1: 0x4e, 3: 0x44, 4: 0x44, 5: 0x44, 7: 0x47, 9: 0x47, 10: 0x45, 11: 0x46, 12: 0x46, 13: 0x46, 15: 0x47, 21: 0x4f}
					for row, v := range colors {
						for col := 0; col < 32; col++ {
							want[0x1800+row*32+col] = v
						}
					}
					fill := func(row, lo, hi int) {
						for col := lo; col <= hi; col++ {
							for line := 0; line < 8; line++ {
								want[((row&0x18)<<8)|(line<<8)|((row&7)<<5)|col] = 0xff
							}
						}
					}
					fill(0, 2, 27)
					fill(1, 2, 24)
					fill(21, 0, 29)
					for row := 11; row <= 13; row++ {
						fill(row, 1, 8)
					}
					fill(7, 2, 10)
					fill(7, 12, 17)

				}
				export(prefix+".screen", z.Memory()[0x4000:0x5b00])
				export(prefix+".expected-screen", want)
				mismatch := -1
				for n, v := range want {
					if z.Memory()[0x4000+n] != v {
						mismatch = 0x4000 + n
						break
					}
				}
				result["first_screen_mismatch"] = mismatch
				outside := []int{}
				for n := range z.Memory() {
					allowed := (n >= 0x4000 && n <= 0x5aff) || (n >= int(depth) && n < 0xf000)
					if i == 0 && (n == int(a.Symbols["zx__console__con_col"]) || n == int(a.Symbols["zx__console__con_row"]) || n == int(a.Symbols["zx__console__con_attr"])) {
						allowed = true
					}

					if !allowed && (z.Memory()[n] != before[n] || prof.WriteCount[n] != 0) {
						outside = append(outside, n)
					}
				}
				result["outside_allowed_writes"] = outside
				if i == 0 {
					// Release scan, then key scan: A is found in row FD. The console's
					// printable-key mapping checks SYMBOL SHIFT in row 7F before the
					// remaining rows; con_getchar checks CAPS SHIFT in row FE last.
					// Keep the exact ordered port oracle, including both modifier reads.
					expected := []uint16{
						0xfefe, 0xfdfe, 0xfbfe, 0xf7fe, 0xeffe, 0xdffe, 0xbffe, 0x7ffe,
						0xfefe, 0xfdfe, 0x7ffe, 0xfbfe, 0xf7fe, 0xeffe, 0xdffe, 0xbffe, 0x7ffe,
						0xfefe,
					}
					result["expected_read_ports"] = expected
					if !done || mismatch >= 0 || len(outside) > 0 || fmt.Sprint(reads) != fmt.Sprint(expected) || len(writes) > 0 {
						t.Fatal("corrected hello model", result)
					}
					result["native_pass"] = true
					return
				}
				result["native_pass"] = halt && z.GetSP() == 0xeffe && mismatch < 0 && len(outside) == 0 && fmt.Sprint(writes) == "[510]" && fmt.Sprint(values) == "[1]" && len(reads) == 0
				t.Logf("original app halted=%v mismatch=%04x outside=%d SP=%04x", halt, mismatch, len(outside), z.GetSP())
				if i == 1 && result["native_pass"] != true {
					t.Fatal("native model", result)
				}
			})
		}
	}
}
