package abap

import (
	"encoding/binary"
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

// The test-owned row model isolates scalar transport and SELECT control flow.
// It is not an SAP SQL or native database-server parity test.
func TestSQLScalarTransportVMAndZ80(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		row        bool
		want       int32
	}{
		{"zero", "0", true, 0}, {"wide", "65537", true, 65537},
		{"negative", "-65537", true, -65537}, {"max", "2147483647", true, 2147483647},
		{"min", "-2147483648", true, -2147483648}, {"missing", "999", false, 17},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := Compile("REPORT zsql. DATA x TYPE i VALUE 17. DATA y TYPE i. DATA seen TYPE i. SELECT SINGLE n FROM t INTO x. y = sy-subrc.", "zsql")
			if err != nil {
				t.Fatal(err)
			}
			row := int64(0)
			if tc.row {
				row = 1
			}
			installSQLRowModel(m, tc.text, row)
			status := int32(4)
			if tc.row {
				status = 0
			}
			verifySQLRowModel(t, m, map[string][]byte{"x": sqlI32(tc.want), "y": sqlI32(status), "seen": sqlI32(1), "_abap_subrc_value": sqlI32(status)})
		})
	}
	for _, tc := range []struct{ name, text, want string }{{"pad", "ab", "ab  "}, {"truncate", "abcdef", "abcd"}, {"empty", "", "    "}} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := Compile("REPORT zsql. DATA c TYPE c LENGTH 4 VALUE 'old'. DATA y TYPE i. DATA seen TYPE i. SELECT SINGLE n FROM t INTO c. y = sy-subrc.", "zsql")
			if err != nil {
				t.Fatal(err)
			}
			installSQLRowModel(m, tc.text, 1)
			verifySQLRowModel(t, m, map[string][]byte{"c": []byte(tc.want), "y": sqlI32(0), "seen": sqlI32(1)})
		})
	}
}

func TestSQLReferenceTargetTransport(t *testing.T) {
	m, err := Compile("REPORT zsql. DATA x TYPE i. DATA y TYPE i. DATA seen TYPE i. FORM read CHANGING v TYPE i. SELECT SINGLE n FROM t INTO @V. ENDFORM. PERFORM read CHANGING x. y = sy-subrc.", "zsql")
	if err != nil {
		t.Fatal(err)
	}
	installSQLRowModel(m, "65537", 1)
	verifySQLRowModel(t, m, map[string][]byte{"x": sqlI32(65537), "y": sqlI32(0), "seen": sqlI32(1)})
}
func TestSQLStringTransport(t *testing.T) {
	m, err := Compile("REPORT zsql. DATA c TYPE string VALUE 'old'. DATA y TYPE i. DATA seen TYPE i. SELECT SINGLE n FROM t INTO c. y = sy-subrc.", "zsql")
	if err != nil {
		t.Fatal(err)
	}
	installSQLRowModel(m, "ab", 1)
	verifySQLRowModel(t, m, map[string][]byte{"c": {2, 0, 3, 0, 97, 98}, "y": sqlI32(0), "seen": sqlI32(1)})
}
func TestSQLLoopExitTransport(t *testing.T) {
	for _, row := range []int64{0, 1} {
		t.Run(fmt.Sprint(row), func(t *testing.T) {
			m, err := Compile("REPORT zsql. DATA x TYPE i VALUE 17. DATA y TYPE i. DATA seen TYPE i. SELECT n FROM t INTO x. EXIT. ENDSELECT. y = sy-subrc.", "zsql")
			if err != nil {
				t.Fatal(err)
			}
			installSQLRowModel(m, "65537", row)
			x, status := int32(17), int32(4)
			if row == 1 {
				x = 65537
				status = 0
			}
			verifySQLRowModel(t, m, map[string][]byte{"x": sqlI32(x), "y": sqlI32(status), "seen": sqlI32(1)})
		})
	}
}

func sqlI32(v int32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(v))
	return b
}
func installSQLRowModel(m *hir.Module, text string, row int64) {
	m.Globals = append(m.Globals, mir2.Global{Name: "_sql_test_text", Ty: &mir2.ArrayTy{Elem: mir2.TyU8, Len: len(text) + 1}, Init: append([]byte(text), 0)})
	for _, f := range m.Funcs {
		if !f.IsExtern || !strings.HasPrefix(f.Name, "sqlite_") {
			continue
		}
		f.IsExtern = false
		var val hir.Expr = &hir.IntLitExpr{Val: 0, Ty: f.RetTy}
		switch f.Name {
		case "sqlite_open", "sqlite_query":
			val = &hir.IntLitExpr{Val: 1, Ty: f.RetTy}
		case "sqlite_step":
			val = &hir.IntLitExpr{Val: row, Ty: f.RetTy}
		case "sqlite_column_text":
			val = &hir.AddrOfExpr{Sym: "_sql_test_text"}
		}
		f.Body = &hir.Block{Body: []hir.Stmt{&hir.ReturnStmt{Val: val}}}
		if f.Name == "sqlite_finalize" {
			f.Body.Body = append([]hir.Stmt{&hir.AssignStmt{Target: &hir.VarRefExpr{Name: "seen", Ty: mir2.TyI32}, Val: &hir.BinExpr{Op: "+", L: &hir.VarRefExpr{Name: "seen", Ty: mir2.TyI32}, R: &hir.IntLitExpr{Val: 1, Ty: mir2.TyI32}, Ty: mir2.TyI32}}}, f.Body.Body...)
		}
	}
}
func verifySQLRowModel(t *testing.T, m *hir.Module, want map[string][]byte) {
	verifySQLRowModelWithFault(t, m, want, 0)
}
func verifySQLRowModelWithFault(t *testing.T, m *hir.Module, want map[string][]byte, fault byte) {
	t.Helper()
	if err := hir.Verify(m); err != nil {
		t.Fatal(err)
	}
	steps, err := pipeline.CompileHIRSteps(m, pipeline.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	check := func(read func(string, int) []byte) {
		for name, value := range want {
			if got := read(name, len(value)); string(got) != string(value) {
				t.Fatalf("%s=%x want %x", name, got, value)
			}
		}
	}
	for _, module := range []*mir2.Module{hir.LowerModule(m), steps.MIR2Module} {
		v := mir2.NewVM(module)
		v.MaxSteps = 3000000
		_, err := v.Call("main", nil)
		if (err != nil) != (fault != 0) {
			t.Fatalf("VM error %v want fault %d", err, fault)
		}
		if code := v.ReadHeap(v.GlobalAddr("_abap_error_code"), 1)[0]; code != fault {
			t.Fatalf("VM fault %d want %d", code, fault)
		}
		check(func(n string, c int) []byte { return v.ReadHeap(v.GlobalAddr(n), c) })
	}
	asm, err := mir2.Z80Codegen(steps.MIR2Module, steps.Allocation, mir2.Z80CodegenOptions{CalledMain: true, UnsignedPairComparePolicy: steps.UnsignedPairComparePolicy})
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
	if err != nil || len(a.Errors) > 0 {
		t.Fatalf("assemble: %v %v", err, a.Errors)
	}
	if dir := os.Getenv("MINZ_ABAP_SQL_ARTIFACTS"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		name := strings.ReplaceAll(t.Name(), "/", "-")
		for ext, data := range map[string][]byte{"a80": []byte("ORG 0x8000\n" + asm), "bin": a.Binary} {
			if err := os.WriteFile(filepath.Join(dir, name+"."+ext), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	e := emulator.NewRemogattoZ80()
	e.LoadMemory(0x8000, a.Binary)
	mem := e.Memory()
	mem[0xE000] = 0xA7
	mem[0xE001] = 0x39
	mem[0xEFFE] = 0
	mem[0xEFFF] = 0xF1
	entry, ok := a.Symbols["main"]
	if !ok {
		t.Fatal("no main")
	}
	e.SetPC(uint16(entry))
	e.SetSP(0xEFFE)
	for i := 0; i < 10000000 && e.GetPC() != 0xF100 && !e.IsHalted(); i++ {
		e.Step()
	}
	if fault == 0 {
		if e.GetPC() != 0xF100 || e.GetSP() != 0xF000 {
			t.Fatalf("return PC=%04x SP=%04x", e.GetPC(), e.GetSP())
		}
	} else if !e.IsHalted() || e.GetPC() == 0xF100 || e.GetIFF1() || e.GetSP() > 0xEFFE || e.GetSP() <= 0xE000 {
		t.Fatalf("native fault PC=%04x SP=%04x", e.GetPC(), e.GetSP())
	}
	if code := mem[a.Symbols["_abap_error_code"]]; code != fault {
		t.Fatalf("native fault %d want %d", code, fault)
	}
	if mem[0xE000] != 0xA7 || mem[0xE001] != 0x39 || mem[0xEFFE] != 0 || mem[0xEFFF] != 0xF1 {
		t.Fatal("stack/canary changed")
	}
	check(func(n string, c int) []byte {
		addr, ok := a.Symbols[n]
		if !ok { // c is reserved by assembler
			addr, ok = a.Symbols["v_"+n]
		}
		if !ok {
			t.Fatal(fmt.Sprintf("no global %s", n))
		}
		b := make([]byte, c)
		for i := range b {
			b[i] = mem[uint16(addr+i)]
		}
		return b
	})
}

func TestSQLScalarConversionFaults(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		code       byte
	}{
		{"overflow", "2147483648", 5}, {"invalid", "12x", 4},
		{"nonascii", string([]byte{128}), 6}, {"too-long", strings.Repeat("1", 256), 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := Compile("REPORT zsql. DATA x TYPE i VALUE 17. DATA y TYPE i. DATA seen TYPE i. SELECT SINGLE n FROM t INTO x. y = sy-subrc.", "zsql")
			if err != nil {
				t.Fatal(err)
			}
			installSQLRowModel(m, tc.text, 1)
			verifySQLRowModelWithFault(t, m, map[string][]byte{"x": sqlI32(17), "y": sqlI32(0), "seen": sqlI32(0)}, tc.code)
		})
	}
}

func TestSQLNullAndQueryFailure(t *testing.T) {
	for _, fn := range []string{"sqlite_column_text", "sqlite_query"} {
		t.Run(fn, func(t *testing.T) {
			m, err := Compile("REPORT zsql. DATA x TYPE i VALUE 17. DATA y TYPE i. DATA seen TYPE i. SELECT SINGLE n FROM t INTO x. y = sy-subrc.", "zsql")
			if err != nil {
				t.Fatal(err)
			}
			installSQLRowModel(m, "1", 1)
			for _, f := range m.Funcs {
				if f.Name == fn {
					f.Body = &hir.Block{Body: []hir.Stmt{&hir.ReturnStmt{Val: &hir.IntLitExpr{Val: 0, Ty: f.RetTy}}}}
				}
			}
			verifySQLRowModelWithFault(t, m, map[string][]byte{"x": sqlI32(17), "y": sqlI32(0), "seen": sqlI32(0)}, 6)
		})
	}
}

func TestSQLUndeclaredTargetAdmission(t *testing.T) {
	m, err := Compile("REPORT zsql. SELECT SINGLE n FROM t INTO missing.", "zsql")
	if err == nil || m != nil || !strings.Contains(err.Error(), "undeclared ABAP object") {
		t.Fatalf("module=%v error=%v", m, err)
	}
}
