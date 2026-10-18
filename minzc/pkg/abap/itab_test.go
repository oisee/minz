package abap

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

// itabLayout derives the byte image of report objects from the unchanged
// source declarations: TYPE c bytes, then TYPE i little-endian, in order.
type itabLayout struct {
	types map[string]*TypesDecl
	data  map[string]*DataDecl
}

func newItabLayout(t *testing.T, src string) itabLayout {
	t.Helper()
	p, err := Parse(src, "zref")
	if err != nil {
		t.Fatal(err)
	}
	l := itabLayout{types: map[string]*TypesDecl{}, data: map[string]*DataDecl{}}
	for _, d := range p.Decls {
		switch d := d.(type) {
		case *TypesDecl:
			l.types[d.Name] = d
		case *DataDecl:
			l.data[strings.ToLower(d.Name)] = d
		}
	}
	return l
}

func (l itabLayout) encodeLine(t *testing.T, ty string, raw json.RawMessage) []byte {
	t.Helper()
	td := l.types[ty]
	if td == nil {
		t.Fatalf("unknown structure type %s", ty)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != len(td.Components) {
		t.Fatalf("reference %s has %d components, source %d", ty, len(fields), len(td.Components))
	}
	var out []byte
	for _, c := range td.Components {
		v, ok := fields[c.Name]
		if !ok {
			t.Fatalf("reference lacks %s-%s", ty, c.Name)
		}
		switch c.AbapTy {
		case "i":
			var n int32
			if err := json.Unmarshal(v, &n); err != nil {
				t.Fatal(err)
			}
			out = binary.LittleEndian.AppendUint32(out, uint32(n))
		case "c":
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				t.Fatal(err)
			}
			if len(s) != characterLength(c.Length) {
				t.Fatalf("%s-%s reference length %d", ty, c.Name, len(s))
			}
			out = append(out, s...)
		}
	}
	return out
}

const itabOrigin = "ORG 0x4000\n"

type itabExpectation struct {
	objects map[string][]byte // symbol -> exact bytes
	output  string
	sy      map[string]int32
	fault   byte
	// pastEnd names a table whose next pastLen bytes after pastOffset must
	// keep their pre-run image unless they belong to an exactly checked object.
	pastEnd             string
	pastOffset, pastLen int
}

// checkPastEnd proves no byte past a table changed except checked objects.
func checkPastEnd(t *testing.T, want itabExpectation, addr func(string) int, before, after []byte) {
	t.Helper()
	start := addr(want.pastEnd) + want.pastOffset
	for i := range before {
		owned := false
		for symbol, image := range want.objects {
			if a := addr(symbol); start+i >= a && start+i < a+len(image) {
				owned = true
			}
		}
		for symbol := range want.sy {
			if a := addr(symbol); start+i >= a && start+i < a+4 {
				owned = true
			}
		}
		if !owned && before[i] != after[i] {
			t.Fatalf("byte %d past %s changed: % x -> % x", i, want.pastEnd, before, after)
		}
	}
}

func itabReferenceExpectation(t *testing.T, path string) (string, itabExpectation) {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(strings.TrimSuffix(path, ".abap") + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var ref struct {
		SHA      string                     `json:"source_sha256"`
		Provider string                     `json:"provider"`
		Entry    string                     `json:"entry"`
		Output   string                     `json:"output"`
		Values   map[string]json.RawMessage `json:"values"`
		SY       map[string]int32           `json:"sy"`
	}
	if err := json.Unmarshal(data, &ref); err != nil {
		t.Fatal(err)
	}
	if ref.SHA != fmt.Sprintf("%x", sha256.Sum256(src)) || ref.Entry != "full report" || ref.Provider != "@abaplint/transpiler + @abaplint/runtime 2.14.8" {
		t.Fatal("reference identity changed")
	}
	layout := newItabLayout(t, string(src))
	want := itabExpectation{objects: map[string][]byte{}, output: ref.Output, sy: map[string]int32{}}
	tables := 0
	for name, raw := range ref.Values {
		d := layout.data[name]
		if d == nil {
			t.Fatalf("reference object %s is not a report DATA object", name)
		}
		switch {
		case d.Table != nil:
			var rows []json.RawMessage
			if err := json.Unmarshal(raw, &rows); err != nil {
				t.Fatal(err)
			}
			if len(rows) == 0 || len(rows) > d.Table.Capacity {
				t.Fatalf("table %s: %d reference lines (fixture must keep lines within capacity %d)", name, len(rows), d.Table.Capacity)
			}
			var image []byte
			for _, row := range rows {
				image = append(image, layout.encodeLine(t, d.AbapTy, row)...)
			}
			want.objects[d.Name] = image
			want.objects["_itab_count_"+d.Name] = binary.LittleEndian.AppendUint32(nil, uint32(len(rows)))
			tables++
		case layout.types[d.AbapTy] != nil:
			want.objects[d.Name] = layout.encodeLine(t, d.AbapTy, raw)
		case d.AbapTy == "i":
			var n int32
			if err := json.Unmarshal(raw, &n); err != nil {
				t.Fatal(err)
			}
			want.objects[d.Name] = binary.LittleEndian.AppendUint32(nil, uint32(n))
		default:
			t.Fatalf("unsupported reference object %s TYPE %s", name, d.AbapTy)
		}
	}
	if tables == 0 || len(ref.SY) != 3 {
		t.Fatal("reference must observe a table and SY-TABIX/SUBRC/INDEX")
	}
	for field, value := range ref.SY {
		want.sy["_abap_"+field+"_value"] = value
	}
	return string(src), want
}

func nativeSymbol(symbols map[string]int, name string) (int, bool) {
	if addr, ok := symbols[name]; ok {
		return addr, true
	}
	addr, ok := symbols["v_"+name] // documented register-name escaping
	return addr, ok
}

// verifyItabReport runs one unchanged report on raw MIR VM, final MIR VM and
// assembled Z80 (bare HALT and called-main), checking output bytes, the exact
// object images, table counts, SY fields, fault code, PC/SP and the canary.
func verifyItabReport(t *testing.T, name, src string, want itabExpectation) {
	t.Helper()
	m, err := Compile(src, "zref")
	if err != nil {
		t.Fatal(err)
	}
	if err := hir.Verify(m); err != nil {
		t.Fatal(err)
	}
	steps, err := pipeline.CompileHIRSteps(m, pipeline.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	check := func(t *testing.T, read func(string, int) []byte) {
		t.Helper()
		for symbol, image := range want.objects {
			if got := read(symbol, len(image)); !bytes.Equal(got, image) {
				t.Fatalf("%s = % x want % x", symbol, got, image)
			}
		}
		for symbol, value := range want.sy {
			if got := int32(binary.LittleEndian.Uint32(read(symbol, 4))); got != value {
				t.Fatalf("%s = %d want %d", symbol, got, value)
			}
		}
		if got := read("_abap_error_code", 1)[0]; got != want.fault {
			t.Fatalf("fault %d want %d", got, want.fault)
		}
	}
	for i, module := range []*mir2.Module{hir.LowerModule(m), steps.MIR2Module} {
		t.Run([]string{"raw-vm", "final-vm"}[i], func(t *testing.T) {
			v := mir2.NewVM(module)
			v.MaxSteps = 30000000
			var out []byte
			v.Hosts["_abap_num_putchar"] = func(args []mir2.Value) ([]mir2.Value, error) { out = append(out, byte(args[0].I)); return nil, nil }
			vmAddr := func(name string) int { return int(v.GlobalAddr(name)) }
			var before []byte
			if want.pastEnd != "" {
				before = append(before, v.ReadHeap(int64(vmAddr(want.pastEnd)+want.pastOffset), want.pastLen)...)
			}
			_, err := v.Call("main", nil)
			if want.pastEnd != "" {
				checkPastEnd(t, want, vmAddr, before, v.ReadHeap(int64(vmAddr(want.pastEnd)+want.pastOffset), want.pastLen))
			}
			if (err != nil) != (want.fault != 0) {
				t.Fatalf("VM termination %v, want fault %d", err, want.fault)
			}
			check(t, func(name string, n int) []byte { return v.ReadHeap(v.GlobalAddr(name), n) })
			if string(out) != want.output {
				t.Fatalf("VM output %q want %q", out, want.output)
			}
		})
	}
	called, err := mir2.Z80Codegen(steps.MIR2Module, steps.Allocation, mir2.Z80CodegenOptions{CalledMain: true, UnsignedPairComparePolicy: steps.UnsignedPairComparePolicy})
	if err != nil {
		t.Fatal(err)
	}
	for _, image := range []struct {
		name, text string
		called     bool
	}{{"bare", steps.Assembly, false}, {"called", called, true}} {
		t.Run(image.name, func(t *testing.T) {
			// Whole reports with i32 tables exceed the 24 KiB between 0x8000 and
			// the 0xE000 canary. Load at 0x4000 (the emulator's first writable
			// page) and prove the image stays below the canary.
			a, err := z80asm.NewAssembler().AssembleString(itabOrigin + image.text)
			if err != nil || len(a.Errors) > 0 {
				t.Fatalf("assemble: %v %v", err, a.Errors)
			}
			if 0x4000+len(a.Binary) > 0xe000 {
				t.Fatalf("image of %d bytes reaches the canary", len(a.Binary))
			}
			if dir := os.Getenv("MINZ_ABAP_ITAB_ARTIFACTS"); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				for ext, data := range map[string][]byte{"abap": []byte(src), "a80": []byte(itabOrigin + image.text), "bin": a.Binary} {
					if err := os.WriteFile(filepath.Join(dir, name+"-"+image.name+"."+ext), data, 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
			z := emulator.NewRemogattoZ80()
			mem := z.Memory()
			copy(mem[0x4000:], a.Binary)
			sp := uint16(0xf000)
			if image.called {
				sp = 0xeffe
				mem[sp], mem[sp+1] = 0, 0xf1
			}
			mem[0xe000] = 0xa5
			z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["main"]), SP: sp})
			nativeAddr := func(name string) int {
				addr, ok := nativeSymbol(a.Symbols, name)
				if !ok {
					t.Fatalf("missing native object %s", name)
				}
				return addr
			}
			var before []byte
			if want.pastEnd != "" {
				start := nativeAddr(want.pastEnd) + want.pastOffset
				before = append(before, mem[start:start+want.pastLen]...)
			}
			var out []byte
			for i := 0; i < 60000000 && z.GetPC() != 0xf100 && !z.IsHalted(); i++ {
				if z.GetPC() == 5 {
					r := z.GetRegisters()
					if byte(r.BC) != 2 {
						t.Fatalf("unexpected BDOS %d", byte(r.BC))
					}
					out = append(out, byte(r.DE))
					if len(out) > len(want.output) || string(out) != want.output[:len(out)] || r.SP >= 0xf000 {
						t.Fatalf("BDOS prefix=%q want=%q SP=%04x", out, want.output, r.SP)
					}
					r.PC = binary.LittleEndian.Uint16(mem[r.SP : r.SP+2])
					r.SP += 2
					z.SetRegisters(r)
				} else {
					z.Step()
				}
			}
			r := z.GetRegisters()
			halted := z.IsHalted() && (mem[r.PC] == 0x76 || mem[r.PC-1] == 0x76)
			if want.fault != 0 {
				// A terminal fault halts inside the report; it never returns.
				if !halted || r.PC == 0xf100 || r.SP > sp || r.SP <= 0xe000 || mem[0xe000] != 0xa5 {
					t.Fatalf("fault termination PC=%04x SP=%04x halted=%v canary=%02x", r.PC, r.SP, z.IsHalted(), mem[0xe000])
				}
			} else {
				termination := r.PC == 0xf100
				if !image.called {
					termination = halted
				}
				if !termination || r.SP != 0xf000 || mem[0xe000] != 0xa5 {
					t.Fatalf("termination %s PC=%04x SP=%04x canary=%02x", image.name, r.PC, r.SP, mem[0xe000])
				}
			}
			if want.pastEnd != "" {
				start := nativeAddr(want.pastEnd) + want.pastOffset
				checkPastEnd(t, want, nativeAddr, before, mem[start:start+want.pastLen])
			}
			if image.called && (mem[0xeffe] != 0 || mem[0xefff] != 0xf1) {
				t.Fatal("outer return word damaged")
			}
			check(t, func(name string, n int) []byte {
				addr, ok := nativeSymbol(a.Symbols, name)
				if !ok {
					t.Fatalf("missing native object %s", name)
				}
				return mem[addr : addr+n]
			})
			if string(out) != want.output {
				t.Fatalf("native output %q want %q", out, want.output)
			}
		})
	}
}

func TestInternalTableFullReportReference(t *testing.T) {
	paths, err := filepath.Glob("testdata/itab-reference/*.abap")
	if err != nil || len(paths) != 4 {
		t.Fatalf("reference sources: %v count=%d", err, len(paths))
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			src, want := itabReferenceExpectation(t, path)
			verifyItabReport(t, strings.TrimSuffix(filepath.Base(path), ".abap"), src, want)
		})
	}
}

// Specification oracle: the reference runtime grows tables without bound, so
// it cannot observe MinZ's fixed capacity. The third APPEND must fault (code
// 7) before writing; the first two lines, the count and later state remain.
func TestInternalTableCapacityFault(t *testing.T) {
	src := `REPORT zover.
TYPES: BEGIN OF ty_pair,
         tag TYPE c LENGTH 2,
         n   TYPE i,
       END OF ty_pair.
DATA work TYPE ty_pair.
DATA pairs TYPE STANDARD TABLE OF ty_pair WITH DEFAULT KEY INITIAL SIZE 2.
DATA seen TYPE i.
work-tag = 'A1'.
work-n = 1.
APPEND work TO pairs.
work-tag = 'B2'.
work-n = 2.
APPEND work TO pairs.
seen = 1.
work-tag = 'C3'.
work-n = 3.
APPEND work TO pairs.
seen = 2.`
	line := func(tag string, n uint32) []byte { return binary.LittleEndian.AppendUint32([]byte(tag), n) }
	want := itabExpectation{
		objects: map[string][]byte{
			"pairs":             append(line("A1", 1), line("B2", 2)...),
			"_itab_count_pairs": {2, 0, 0, 0},
			"seen":              {1, 0, 0, 0},
			"work":              line("C3", 3),
		},
		sy:    map[string]int32{"_abap_tabix_value": 2},
		fault: tableCapacityFault,
	}
	// Nothing is written past the two-line array: the 6 bytes a third line
	// would occupy keep their image unless they are an exactly checked object.
	want.pastEnd, want.pastOffset, want.pastLen = "pairs", 12, 6
	verifyItabReport(t, "capacity-fault", src, want)
}

// LOOP INTO copies: changing the work area never changes the stored line.
func TestInternalTableLoopIntoCopies(t *testing.T) {
	src := `REPORT zcopy.
TYPES: BEGIN OF ty_cell,
         label TYPE c LENGTH 2,
         v     TYPE i,
       END OF ty_cell.
DATA cell TYPE ty_cell.
DATA cells TYPE STANDARD TABLE OF ty_cell INITIAL SIZE 2.
cell-label = 'P'.
cell-v = 5.
APPEND cell TO cells.
LOOP AT cells INTO cell.
  cell-label = 'QQ'.
  cell-v = cell-v * 3.
ENDLOOP.
READ TABLE cells INTO cell INDEX 1.`
	line := binary.LittleEndian.AppendUint32([]byte("P "), 5)
	verifyItabReport(t, "loop-copy", src, itabExpectation{
		objects: map[string][]byte{"cells": line, "cell": line, "_itab_count_cells": {1, 0, 0, 0}},
		sy:      map[string]int32{"_abap_tabix_value": 1, "_abap_subrc_value": 0},
	})
}

// Every refused form goes through the embedded parser and the full compiler.
func TestInternalTableAdmission(t *testing.T) {
	const head = "REPORT zbad.\nTYPES: BEGIN OF ty, a TYPE i, b TYPE c LENGTH 2, END OF ty.\nTYPES: BEGIN OF tz, a TYPE i, END OF tz.\nDATA ls TYPE ty.\nDATA lz TYPE tz.\nDATA n TYPE i.\nDATA lt TYPE STANDARD TABLE OF ty.\n"
	for _, c := range []struct{ src, want string }{
		{"TYPES: BEGIN OF tn, a TYPE i, BEGIN OF inner, b TYPE i, END OF inner, END OF tn.", "nested"},
		{"TYPES: BEGIN OF tn, a TYPE ty, END OF tn.", "not admitted"},
		{"TYPES: BEGIN OF tn, s TYPE string, END OF tn.", "deep"},
		{"TYPES: BEGIN OF tn, t TYPE STANDARD TABLE OF ty, END OF tn.", "table components"},
		{"TYPES: BEGIN OF tn, a TYPE i.\nINCLUDE TYPE ty.\nTYPES: END OF tn.", "INCLUDE"},
		{"TYPES: BEGIN OF tn, p TYPE p, END OF tn.", "not admitted"},
		{"TYPES ti TYPE i.", "only flat BEGIN OF"},
		{"DATA ls2 TYPE ty VALUE 1.", ""},
		{"DATA ls3 LIKE ls.", "LIKE"},
		{"DATA t2 TYPE SORTED TABLE OF ty WITH UNIQUE KEY a.", "SORTED TABLE"},
		{"DATA t2 TYPE HASHED TABLE OF ty WITH UNIQUE KEY a.", "HASHED TABLE"},
		{"DATA t2 TYPE STANDARD TABLE OF i.", "tables of elementary"},
		{"DATA t2 TYPE STANDARD TABLE OF ty WITH KEY a.", "WITH KEY a"},
		{"DATA t2 TYPE STANDARD TABLE OF ty WITH NON-UNIQUE KEY a.", "is not implemented"},
		{"DATA t2 TYPE STANDARD TABLE OF ty WITH HEADER LINE.", "HEADER LINE"},
		{"DATA t2 TYPE STANDARD TABLE OF ty INITIAL SIZE 0.", "INITIAL SIZE"},
		{"DATA t2 TYPE STANDARD TABLE OF ty INITIAL SIZE 9999.", "table limit"},
		{"DATA r TYPE REF TO ty.", "REF TO"},
		{"FORM f. DATA lx TYPE ty. ENDFORM.", "procedure-local"},
		{"FORM f. DATA lx TYPE STANDARD TABLE OF ty. ENDFORM.", "procedure-local"},
		{"FORM f. TYPES: BEGIN OF tl, a TYPE i, END OF tl. ENDFORM.", "inside procedures"},
		{"FORM f USING p TYPE ty. ENDFORM.", "not implemented precisely"},
		{"LOOP AT lt INTO ls WHERE a = 1. ENDLOOP.", "WHERE"},
		{"FIELD-SYMBOLS <fs> TYPE ty.", "FIELD-SYMBOLS"},
		{"LOOP AT lt ASSIGNING FIELD-SYMBOL(<fs>). ENDLOOP.", "ASSIGNING"},
		{"LOOP AT lt REFERENCE INTO DATA(r). ENDLOOP.", "REFERENCE INTO"},
		{"LOOP AT lt INTO DATA(row). ENDLOOP.", "inline"},
		{"LOOP AT lt INTO ls FROM 2. ENDLOOP.", "FROM"},
		{"LOOP AT lt INTO lz. ENDLOOP.", "conversions"},
		{"LOOP AT ls INTO ls. ENDLOOP.", "not a declared internal table"},
		{"MODIFY lt FROM ls INDEX 1.", "MODIFY"},
		{"DELETE lt INDEX 1.", "DELETE"},
		{"INSERT ls INTO lt INDEX 1.", "INSERT"},
		{"SORT lt BY a.", "SORT"},
		{"COLLECT ls INTO lt.", "COLLECT"},
		{"APPEND LINES OF lt TO lt.", "APPEND LINES OF"},
		{"APPEND ls TO lt ASSIGNING FIELD-SYMBOL(<fs>).", "ASSIGNING"},
		{"APPEND ls TO lt REFERENCE INTO DATA(r).", "REFERENCE INTO"},
		{"APPEND ls TO lt SORTED BY a.", "SORTED BY"},
		{"APPEND lz TO lt.", "conversions"},
		{"APPEND n TO lt.", "structure variable"},
		{"READ TABLE lt INTO ls WITH KEY a = 1.", "WITH KEY"},
		{"READ TABLE lt INTO ls WITH TABLE KEY a = 1.", "WITH TABLE KEY"},
		{"READ TABLE lt INTO ls INDEX 1 TRANSPORTING a.", "TRANSPORTING"},
		{"READ TABLE lt ASSIGNING FIELD-SYMBOL(<fs>) INDEX 1.", "ASSIGNING"},
		{"READ TABLE lt REFERENCE INTO DATA(r) INDEX 1.", "REFERENCE INTO"},
		{"READ TABLE lt INDEX 1 TRANSPORTING NO FIELDS.", "TRANSPORTING"},
		{"READ TABLE lt INTO ls INDEX ls-b.", "character-to-integer"},
		{"DESCRIBE TABLE lt.", "SY-TFILL"},
		{"DESCRIBE TABLE lt LINES ls-b.", "TYPE i target"},
		{"DESCRIBE FIELD n LENGTH n IN CHARACTER MODE.", "DESCRIBE FIELD"},
		{"REFRESH ls.", "not a declared internal table"},
		{"FREE lt.", "FREE"},
		{"ls = n.", "same TYPES type"},
		{"ls = lz.", "same TYPES type"},
		{"n = ls.", "elementary operand"},
		{"lt = lt.", "internal table"},
		{"n = ls-b.", "character conversion"},
		{"ls-a = 'X'.", "character-to-integer"},
		{"ls-zz = 1.", "no component zz"},
		{"n-a = 1.", "not a declared structure"},
		{"WRITE ls.", "elementary operand"},
		{"WRITE lt.", "elementary operand"},
		{"IF ls = lz. ENDIF.", "elementary operand"},
		{"n = strlen( ls-b ).", "strlen( ) is not implemented"},
		{"n = lines( ls ).", "declared internal table"},
		{"MOVE-CORRESPONDING ls TO lz.", "MOVE-CORRESPONDING"},
		{"SELECT * FROM t INTO TABLE lt.", "INTO TABLE"},
	} {
		module, err := Compile(head+c.src, "zbad")
		if module != nil || err == nil {
			t.Fatalf("admitted %q", c.src)
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%q: diagnostic %q lacks %q", c.src, err, c.want)
		}
	}
	// TYPE TABLE OF is the ABAP spelling of a standard table.
	for _, src := range []string{"DATA t3 TYPE TABLE OF ty.", "DATA t3 TYPE STANDARD TABLE OF ty WITH EMPTY KEY INITIAL SIZE 3."} {
		if _, err := Compile(head+src+" APPEND ls TO t3.", "zok"); err != nil {
			t.Fatalf("%s: %v", src, err)
		}
	}
}

// SY-TABIX in nested loops through a FORM: RETURN inside the callee's LOOP
// restores the caller's row number; an empty LOOP leaves SY-TABIX unchanged
// and sets SY-SUBRC 4 (reference runtime loop.ts finally/early-exit paths).
func TestInternalTableTabixAcrossProcedures(t *testing.T) {
	src := `REPORT ztabix.
TYPES: BEGIN OF ty_row,
         v TYPE i,
       END OF ty_row.
DATA row TYPE ty_row.
DATA inner TYPE ty_row.
DATA rows TYPE STANDARD TABLE OF ty_row INITIAL SIZE 3.
DATA none TYPE STANDARD TABLE OF ty_row INITIAL SIZE 1.
DATA x TYPE i.
FORM scan.
  LOOP AT rows INTO inner.
    IF sy-tabix = 2.
      RETURN.
    ENDIF.
  ENDLOOP.
ENDFORM.
row-v = 1.
APPEND row TO rows.
APPEND row TO rows.
APPEND row TO rows.
LOOP AT rows INTO row.
  PERFORM scan.
  x = x * 10 + sy-tabix.
  LOOP AT none INTO inner.
    x = 99.
  ENDLOOP.
  x = x * 10 + sy-subrc.
  x = x * 10 + sy-tabix.
ENDLOOP.`
	verifyItabReport(t, "tabix-procedures", src, itabExpectation{
		// Each step: tabix, empty-loop subrc 4, unchanged tabix.
		objects: map[string][]byte{"x": binary.LittleEndian.AppendUint32(nil, 141242343)},
		sy:      map[string]int32{"_abap_tabix_value": 3, "_abap_subrc_value": 0},
	})
}
