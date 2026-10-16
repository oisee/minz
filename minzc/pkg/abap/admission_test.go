package abap

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestUnsupportedStatementsAreNeverDropped(t *testing.T) {
	bad := &ASTChild{Type: "Delete", Tokens: []*Token{{Str: "DELETE", Row: 7, Col: 3}}}
	wrap := func(typ string, child *ASTChild) *ASTChild { return &ASTChild{Type: typ, Children: []*ASTChild{child}} }
	for _, kind := range []string{"flat", "report", "form", "function", "while", "if", "do", "select", "method", "case"} {
		t.Run(kind, func(t *testing.T) {
			r := &ParseResult{}
			switch kind {
			case "flat":
				r.Statements = []*StmtNode{{Type: bad.Type, Tokens: bad.Tokens}}
			case "report":
				r.Structure = &StructNode{Type: "Any", Children: []*ASTChild{wrap("Normal", bad)}}
			case "method":
				r.Structure = &StructNode{Children: []*ASTChild{wrap("ClassImplementation", wrap("Method", wrap("Body", bad)))}}
			default:
				typ := map[string]string{"form": "Form", "function": "FunctionModule", "while": "While", "if": "If", "do": "Do", "select": "Select", "case": "Case"}[kind]
				r.Structure = &StructNode{Children: []*ASTChild{wrap(typ, wrap("Body", bad))}}
			}
			_, err := buildProgram("admission.abap", r)
			if err == nil {
				t.Fatal("unsupported executable statement disappeared")
			}
			if kind != "case" && !strings.Contains(err.Error(), "7:3") {
				t.Fatalf("missing source location: %v", err)
			}
		})
	}
}

func TestLoweringFailureDoesNotDropProcedure(t *testing.T) {
	for _, kind := range []string{"form", "function", "method", "main", "event"} {
		t.Run(kind, func(t *testing.T) {
			body := []Stmt_{&ClearStmt{Target: "missing"}}
			var d Decl
			switch kind {
			case "form":
				d = &FormDecl{Name: "probe", Body: body}
			case "function":
				d = &FunctionDecl{Name: "probe", Body: body}
			case "main":
				d = &formBodyDecl{stmt: body[0]}
			case "event":
				d = &EventDecl{Event: "START-OF-SELECTION", Body: body}
			case "method":
				d = &ClassDecl{Name: "test", Methods: []*MethodDecl{{Name: "probe", Body: body}}}
			}
			prog := &Program{Name: "bad", Decls: []Decl{d}}
			if kind == "event" {
				prog.Events = map[string][]Stmt_{"START-OF-SELECTION": body}
			}
			_, err := LowerProgram(prog)
			if err == nil || !strings.Contains(err.Error(), "unknown target") {
				t.Fatalf("procedure silently omitted: %v", err)
			}
		})
	}
}

// The harness explicitly invokes probe. A report that merely declares this FORM
// must leave lv_x=7; after the call CLEAR must set the actual global object to 0.
func TestClearGlobalAfterExplicitCallVMAndZ80(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // ordinary parsing and execution must need no Node/npm
	src := `REPORT zclear.
DATA lv_x TYPE i VALUE 7.
FORM probe.
 CLEAR lv_x.
ENDFORM.`
	parse := func() *hir.Module {
		t.Helper()
		m, e := Compile(src, "zclear")
		if e != nil {
			t.Fatal(e)
		}
		if e = hir.Verify(m); e != nil {
			t.Fatal(e)
		}
		return m
	}
	check := func(m *mir2.Module) {
		t.Helper()
		vm := mir2.NewVM(m)
		addr := vm.GlobalAddr("lv_x")
		if got := binary.LittleEndian.Uint16(vm.ReadHeap(addr, 2)); got != 7 {
			t.Fatalf("before call: %d", got)
		}
		if _, e := vm.Call("probe", nil); e != nil {
			t.Fatal(e)
		}
		if got := binary.LittleEndian.Uint16(vm.ReadHeap(addr, 2)); got != 0 {
			t.Fatalf("after call: %d", got)
		}
	}
	check(hir.LowerModule(parse()))
	steps, e := pipeline.CompileHIRSteps(parse(), pipeline.DefaultOptions())
	if e != nil {
		t.Fatal(e)
	}
	check(steps.MIR2Module)
	asm, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if e != nil || len(asm.Errors) > 0 {
		t.Fatalf("assemble: %v %v", e, asm.Errors)
	}
	z := emulator.NewRemogattoZ80()
	mem := z.Memory()
	copy(mem[0x8000:], asm.Binary)
	addr, ok := asm.Symbols["lv_x"]
	if !ok {
		t.Fatal("missing global symbol")
	}
	if got := binary.LittleEndian.Uint16(mem[addr : addr+2]); got != 7 {
		t.Fatalf("native before call: %d", got)
	}
	mem[0xeffe], mem[0xefff] = 0, 0xf1
	z.SetRegisters(emulator.Registers{PC: uint16(asm.Symbols["probe"]), SP: 0xeffe})
	for i := 0; i < 100000 && z.GetPC() != 0xf100; i++ {
		z.Step()
	}
	r := z.GetRegisters()
	if r.PC != 0xf100 || r.SP != 0xf000 {
		t.Fatalf("native return PC=%04x SP=%04x", r.PC, r.SP)
	}
	if got := binary.LittleEndian.Uint16(mem[addr : addr+2]); got != 0 {
		t.Fatalf("native after call: %d", got)
	}
}
