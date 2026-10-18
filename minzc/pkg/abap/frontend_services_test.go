package abap

import (
	"encoding/binary"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"strings"
	"testing"
)

// The provider supplies CP/M records, never line or SPLIT semantics. Both VMs
// execute the same compiled helpers as native code; native tests intercept CALL 5.
type frontendProbe struct {
	files      map[string]string
	matches    []string
	failRead   bool
	closes     int
	entries    []string
	keys       []byte
	pos, index int
	opened     string
	dma        uint16
}

func (p *frontendProbe) bdos(fn byte, de uint16, read func(uint16) byte, write func(uint16, byte)) byte {
	name := func() string {
		var b []byte
		for i := uint16(1); i < 9; i++ {
			if c := read(de + i); c != ' ' {
				b = append(b, c)
			}
		}
		b = append(b, '.')
		for i := uint16(9); i < 12; i++ {
			if c := read(de + i); c != ' ' {
				b = append(b, c)
			}
		}
		return string(b)
	}
	switch fn {
	case 15:
		p.opened = ""
		p.pos = 0
		if _, ok := p.files[name()]; !ok {
			return 255
		}
		p.opened = name()
		return 0
	case 16:
		p.opened = ""
		p.closes++
		return 0
	case 26:
		p.dma = de
		return 0
	case 20:
		if p.failRead {
			return 2
		}
		data := p.files[p.opened]
		if p.pos >= len(data) {
			return 1
		}
		for i := 0; i < 128; i++ {
			b := byte(26)
			if p.pos < len(data) {
				b = data[p.pos]
				p.pos++
			}
			write(p.dma+uint16(i), b)
		}
		return 0
	case 17, 18:
		if fn == 17 {
			p.index = 0
			p.matches = nil
			for _, entry := range p.entries {
				var encoded [11]byte
				for i := range encoded {
					encoded[i] = ' '
				}
				parts := strings.Split(entry, ".")
				copy(encoded[:8], parts[0])
				if len(parts) == 2 {
					copy(encoded[8:], parts[1])
				}
				match := true
				for i, b := range encoded {
					pattern := read(de+1+uint16(i)) & 127
					if pattern != '?' && pattern != b {
						match = false
					}
				}
				if match {
					p.matches = append(p.matches, entry)
				}
			}
		}
		if p.index == len(p.matches) {
			return 255
		}
		slot := p.index % 4
		entry := p.dma + uint16(slot*32)
		for i := uint16(0); i < 32; i++ {
			write(entry+i, 0)
		}
		parts := strings.Split(p.matches[p.index], ".")
		for i := 0; i < 11; i++ {
			write(entry+1+uint16(i), ' ')
		}
		for i, b := range []byte(parts[0]) {
			write(entry+1+uint16(i), b)
		}
		if len(parts) == 2 {
			for i, b := range []byte(parts[1]) {
				write(entry+9+uint16(i), b)
			}
		}
		p.index++
		return byte(slot)
	case 6:
		if len(p.keys) == 0 {
			return 0
		}
		b := p.keys[0]
		p.keys = p.keys[1:]
		return b
	}
	return 255
}

type frontendCase struct {
	name, src   string
	files       map[string]string
	keys        []byte
	entries     []string
	text        map[string]string
	ints        map[string]int32
	output      string
	fault       byte
	failRead    bool
	application bool
}

func runFrontendCase(t *testing.T, c frontendCase) {
	t.Helper()
	m, err := Compile(c.src, "zfiles")
	if err != nil {
		t.Fatal(err)
	}
	opts := pipeline.DefaultOptions()
	opts.AssertMode = "none"
	steps, err := pipeline.CompileHIRSteps(m, opts)
	if err != nil {
		t.Fatal(err)
	}
	text, err := mir2.Z80Codegen(steps.MIR2Module, steps.Allocation, mir2.Z80CodegenOptions{CalledMain: true, CPMApplicationEntry: c.application, UnsignedPairComparePolicy: steps.UnsignedPairComparePolicy})
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x100\n" + text)
	if err != nil || len(a.Errors) > 0 {
		t.Fatalf("assemble %v %v", err, a.Errors)
	}
	if len(a.Binary) > 0xd000 {
		t.Fatalf("image too large: %d", len(a.Binary))
	}
	check := func(read func(string, int) []byte, output, wantOutput string) {
		t.Helper()
		for name, want := range c.text {
			if got := string(read(name, len(want))); got != want {
				t.Fatalf("%s=%q want %q", name, got, want)
			}
		}
		for name, want := range c.ints {
			if got := int32(binary.LittleEndian.Uint32(read(name, 4))); got != want {
				t.Fatalf("%s=%d want %d", name, got, want)
			}
		}
		if got := read("_abap_error_code", 1)[0]; got != c.fault {
			t.Fatalf("fault=%d want %d", got, c.fault)
		}
		if output != wantOutput {
			t.Fatalf("output=%q want %q", output, wantOutput)
		}
	}
	for i, module := range []*mir2.Module{hir.LowerModule(m), steps.MIR2Module} {
		t.Run(fmt.Sprintf("vm-%d", i), func(t *testing.T) {
			v := mir2.NewVM(module)
			v.MaxSteps = 10000000
			p := frontendProbe{dma: 0x80, files: c.files, entries: c.entries, failRead: c.failRead, keys: append([]byte{}, c.keys...)}
			var out strings.Builder
			v.Hosts["sel_show"] = func([]mir2.Value) ([]mir2.Value, error) { return []mir2.Value{{I: 0}}, nil }
			v.Hosts["sel_register_c"] = func([]mir2.Value) ([]mir2.Value, error) { return nil, nil }
			v.Hosts["_abap_num_putchar"] = func(args []mir2.Value) ([]mir2.Value, error) { out.WriteByte(byte(args[0].I)); return nil, nil }
			v.Hosts["abap_write_str"] = func(args []mir2.Value) ([]mir2.Value, error) {
				for addr := args[0].I; ; addr++ {
					b := v.ReadHeap(addr, 1)[0]
					if b == 0 {
						break
					}
					out.WriteByte(b)
				}
				return nil, nil
			}
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
			_, err := v.Call("main", nil)
			if (err != nil) != (c.fault != 0) {
				t.Fatalf("VM error %v", err)
			}
			if p.opened != "" {
				t.Fatal("VM leaked open file", p.opened)
			}
			check(func(name string, n int) []byte { return v.ReadHeap(v.GlobalAddr(name), n) }, out.String(), c.output)
		})
	}
	t.Run("z80", func(t *testing.T) {
		z := emulator.NewRemogattoZ80WithScreen()
		z.SetROMEnd(0)
		mem := z.Memory()
		copy(mem[0x100:], a.Binary)
		mem[0xeffe], mem[0xefff] = 0, 0xf1
		mem[0xe000] = 0xa5
		entry := uint16(a.Symbols["main"])
		minSP := uint16(0xeffe)
		if c.application {
			entry = 0x100
			mem[6], mem[7] = 0, 0xfe
			minSP = uint16(a.Symbols["__cpm_app_stack_top"])
		}
		z.SetRegisters(emulator.Registers{PC: entry, SP: 0xeffe})
		p := frontendProbe{dma: 0x80, files: c.files, entries: c.entries, failRead: c.failRead, keys: append([]byte{}, c.keys...)}
		var out strings.Builder
		for i := 0; i < 10000000 && z.GetPC() != 0xf100 && !z.IsHalted(); i++ {
			if c.application && z.GetSP() < minSP {
				minSP = z.GetSP()
			}
			if z.GetPC() != 5 {
				z.Step()
				continue
			}
			r := z.GetRegisters()
			rc := byte(0)
			switch byte(r.BC) {
			case 2:
				out.WriteByte(byte(r.DE))
			case 10:
				mem[r.DE+1] = 0
			case 6, 15, 16, 17, 18, 20, 26:
				rc = p.bdos(byte(r.BC), r.DE, func(a uint16) byte { return mem[a] }, func(a uint16, b byte) { mem[a] = b })
			default:
				t.Fatalf("BDOS %d", byte(r.BC))
			}
			r.PC = binary.LittleEndian.Uint16(mem[r.SP : r.SP+2])
			r.SP += 2
			r.A, r.F, r.BC, r.DE, r.HL = rc, 0, 0x1234, 0xabcd, 0xbeef
			z.SetRegisters(r)
		}
		if c.fault == 0 || c.application {
			if z.GetPC() != 0xf100 || z.GetSP() != 0xf000 {
				t.Fatalf("PC=%04x SP=%04x", z.GetPC(), z.GetSP())
			}
		} else if !z.IsHalted() {
			t.Fatalf("expected fault PC=%04x", z.GetPC())
		}
		if mem[0xe000] != 0xa5 || binary.LittleEndian.Uint16(mem[0xeffe:0xf000]) != 0xf100 {
			t.Fatal("stack canary/return word")
		}
		if p.opened != "" {
			t.Fatal("native leaked open file", p.opened)
		}
		if p.dma != 0x80 {
			t.Fatalf("native left shared DMA %04x", p.dma)
		}
		wantOutput := c.output
		if c.application {
			bottom, top := a.Symbols["__cpm_app_stack"], a.Symbols["__cpm_app_stack_top"]
			if int(minSP) < bottom || int(minSP) >= top || mem[a.Symbols["__cpm_app_stack_canary"]] != 0xa5 {
				t.Fatalf("resident stack min=%04x bounds=%04x..%04x", minSP, bottom, top)
			}
			if c.fault != 0 {
				wantOutput += fmt.Sprintf("ABAP runtime error 0x%02X\r\n", c.fault)
				if z.GetRegisters().A != c.fault {
					t.Fatal("lost terminal fault A")
				}
			}
			t.Logf("application imageEnd=%04x stack=%04x..%04x minSP=%04x returnPC=%04x SP=%04x canary=A5", 0x100+len(a.Binary), bottom, top, minSP, z.GetPC(), z.GetSP())
		}
		check(func(name string, n int) []byte {
			addr, ok := a.Symbols[name]
			if !ok {
				addr, ok = a.Symbols["v_"+name]
			}
			if !ok {
				t.Fatalf("missing %s", name)
			}
			return mem[addr : addr+n]
		}, out.String(), wantOutput)
	})
}

func frontendUploadSource(capacity int, tail string) string {
	return fmt.Sprintf(`REPORT zupload.
TYPES line_type TYPE c LENGTH 4.
DATA rows TYPE STANDARD TABLE OF line_type WITH EMPTY KEY INITIAL SIZE %d.
DATA line TYPE line_type.
DATA suffix TYPE i.
cl_gui_frontend_services=>gui_upload(
 EXPORTING filename = 'FLEET.CSV' filetype = 'ASC'
 CHANGING data_tab = rows
 EXCEPTIONS file_open_error = 1 OTHERS = 2 ).
%s`, capacity, tail)
}

func TestFrontendUploadRecords(t *testing.T) {
	for _, c := range []frontendCase{
		{name: "CRLF-LF-EOF", src: frontendUploadSource(4, "LOOP AT rows INTO line. WRITE |{ line };|. ENDLOOP."), files: map[string]string{"FLEET.CSV": "AB\r\nCD\nE\x1aignored"}, text: map[string]string{"rows": "AB  CD  E   "}, ints: map[string]int32{"_itab_count_rows": 3}, output: "AB;CD;E;"},
		{name: "long-line-truncation", src: frontendUploadSource(2, "LOOP AT rows INTO line. WRITE |{ line };|. ENDLOOP."), files: map[string]string{"FLEET.CSV": "ABCDEF\r\nZ"}, text: map[string]string{"rows": "ABCDZ   "}, ints: map[string]int32{"_itab_count_rows": 2}, output: "ABCD;Z;"},
		{name: "CRLF-record-boundary", src: frontendUploadSource(2, "WRITE |{ sy-subrc }|."), files: map[string]string{"FLEET.CSV": strings.Repeat("A", 127) + "\r\nZ\n"}, text: map[string]string{"rows": "AAAAZ   "}, ints: map[string]int32{"_itab_count_rows": 2}, output: "0"},
		{name: "empty-file", src: frontendUploadSource(2, "WRITE |{ sy-subrc }|."), files: map[string]string{"FLEET.CSV": ""}, ints: map[string]int32{"_itab_count_rows": 0}, output: "0"},
		{name: "missing-file", src: frontendUploadSource(2, "WRITE |{ sy-subrc }|."), files: map[string]string{}, ints: map[string]int32{"_itab_count_rows": 0, "_abap_subrc_value": 1}, output: "1"},
		{name: "read-error", src: frontendUploadSource(2, "WRITE |{ sy-subrc }|."), files: map[string]string{"FLEET.CSV": "AB\r\n"}, failRead: true, ints: map[string]int32{"_itab_count_rows": 0, "_abap_subrc_value": 2}, output: "2"},
		{name: "capacity-fault", application: true, src: frontendUploadSource(2, "suffix = 99."), files: map[string]string{"FLEET.CSV": "A\nB\nC\n"}, text: map[string]string{"rows": "A   B   "}, ints: map[string]int32{"_itab_count_rows": 2, "suffix": 0}, fault: 7},
	} {
		t.Run(c.name, func(t *testing.T) { runFrontendCase(t, c) })
	}
}

func TestFrontendDirectoryCatalog(t *testing.T) {
	src := `REPORT zcatalog.
TYPES: BEGIN OF ty_file, filename TYPE c LENGTH 12, END OF ty_file.
DATA files TYPE STANDARD TABLE OF ty_file WITH EMPTY KEY INITIAL SIZE 4.
DATA total TYPE i.
cl_gui_frontend_services=>directory_list_files(
 EXPORTING directory = '' filter = '*.CSV'
 CHANGING file_table = files count = total ).
WRITE |{ total }:{ sy-subrc }|.`
	runFrontendCase(t, frontendCase{name: "catalog", src: src, entries: []string{"A.CSV", "B.CSV", "README.TXT"}, ints: map[string]int32{"total": 2, "_itab_count_files": 2}, text: map[string]string{"files": "A.CSV       B.CSV       "}, output: "2:0"})
}

func TestFrontendExampleCompile(t *testing.T) {
	src, err := os.ReadFile("../../../examples/abap/zpickfile_cpm.abap")
	if err != nil {
		t.Fatal(err)
	}
	m, err := Compile(string(src), "zpickfile_cpm")
	if err != nil {
		t.Fatal(err)
	}
	if err := hir.Verify(m); err != nil {
		t.Fatal(err)
	}
}

// EXPECT = the admitted frontend contract (frontend-contract.md), not an SAP
// execution trace. The file bytes are independent fixtures shared by real MZE.
func TestFrontendF4Report(t *testing.T) {
	src, err := os.ReadFile("../../../examples/abap/zpickfile_cpm.abap")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, name := range []string{"EXTRA.CSV", "FLEET.CSV"} {
		data, err := os.ReadFile("../../../examples/abap/fleet/" + name)
		if err != nil {
			t.Fatal(err)
		}
		files[name] = string(data)
	}
	const prompt = "P_FILE [EXTRA.CSV]: "
	const menu = "Choose CSV\r\n>1 EXTRA.CSV\r\n 2 FLEET.CSV\r\n"
	const down = "\r\n 1 EXTRA.CSV\r\n>2 FLEET.CSV\r\n"
	for _, c := range []frontendCase{
		{name: "F4-down-select", keys: []byte("\x1bOS\x1b[B\r\r"), output: prompt + menu + down + "P_FILE [FLEET.CSV]: \r\n\r\n\r\nAtlas North 40\r\nBeacon South 32\r\nTotal seats: 72", text: map[string]string{"p_file": "FLEET.CSV   "}, ints: map[string]int32{"total": 72, "_itab_count_fleet": 2, "_itab_count_lt_lines": 2, "lv_rc": 1, "lv_action": 0}},
		{name: "F4-cancel", keys: []byte("\x1bOS\x1b\r"), output: prompt + menu + prompt + "\r\n\r\n\r\nSpare Depot 8\r\nTotal seats: 8", text: map[string]string{"p_file": "EXTRA.CSV   "}, ints: map[string]int32{"total": 8, "_itab_count_fleet": 1, "_itab_count_lt_files": 0, "lv_rc": 0, "lv_action": 1}},
		{name: "missing-file", keys: []byte("MISSING.CSV\r"), output: prompt + "MISSING.CSV\r\n\r\n\r\nFile not found", ints: map[string]int32{"total": 0, "_itab_count_fleet": 0, "_abap_subrc_value": 1}},
	} {
		c.application = true
		c.src = string(src)
		c.files = files
		c.entries = []string{"EXTRA.CSV", "FLEET.CSV", "README.TXT"}
		t.Run(c.name, func(t *testing.T) { runFrontendCase(t, c) })
	}
}

func TestFrontendUploadWideTableWithoutWorkarea(t *testing.T) {
	src := `REPORT zwide.
TYPES line_type TYPE c LENGTH 1536.
DATA rows TYPE STANDARD TABLE OF line_type WITH EMPTY KEY INITIAL SIZE 1.
cl_gui_frontend_services=>gui_upload( EXPORTING filename = 'FLEET.CSV' CHANGING data_tab = rows ).`
	data := strings.Repeat("A", 1536)
	runFrontendCase(t, frontendCase{src: src, files: map[string]string{"FLEET.CSV": data + "\n"}, text: map[string]string{"rows": data}, ints: map[string]int32{"_itab_count_rows": 1}})
}

func TestFrontendRuntimePreservesReportGlobals(t *testing.T) {
	src := strings.Replace(frontendUploadSource(2, "WRITE |{ n }:{ i }:{ count }|."), "DATA suffix TYPE i.", "DATA suffix TYPE i. DATA n TYPE i VALUE 77. DATA i TYPE i VALUE 77. DATA count TYPE i VALUE 77.", 1)
	runFrontendCase(t, frontendCase{src: src, files: map[string]string{"FLEET.CSV": "A\n"}, ints: map[string]int32{"n": 77, "i": 77, "count": 77, "_itab_count_rows": 1}, output: "77:77:77"})
}

func TestFrontendDirectoryCapacityBeforePublication(t *testing.T) {
	src := `REPORT zcatalog.
TYPES: BEGIN OF ty_file, filename TYPE c LENGTH 12, END OF ty_file.
DATA files TYPE STANDARD TABLE OF ty_file WITH EMPTY KEY INITIAL SIZE 1.
DATA total TYPE i.
DATA suffix TYPE i.
cl_gui_frontend_services=>directory_list_files( EXPORTING filter = '*.CSV' CHANGING file_table = files count = total ).
suffix = 99.`
	runFrontendCase(t, frontendCase{src: src, entries: []string{"A.CSV", "B.CSV"}, text: map[string]string{"files": strings.Repeat("\x00", 12)}, ints: map[string]int32{"total": 0, "_itab_count_files": 0, "suffix": 0}, fault: 7, application: true})
}
