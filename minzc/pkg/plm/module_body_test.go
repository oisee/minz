package plm

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"strings"
	"testing"
)

func TestModuleEntryAndExactStringBytes(t *testing.T) {
	h, err := Compile("Demo: do; declare Result byte; Copy: procedure(P); declare P word; Result=P(0); end Copy; call Copy(.'a'); end Demo;")
	if err != nil {
		t.Fatal(err)
	}
	if h.FuncByName("main") == nil {
		t.Fatal("module statements lost")
	}
	if len(h.Globals) != 2 || string(h.Globals[1].Init) != "a" || mir2.ByteWidth(h.Globals[1].Ty) != 1 {
		t.Fatal("literal bytes/extent", h.Globals)
	}
	vm := mir2.NewVM(hir.LowerModule(h))
	if _, err = vm.Call("main", nil); err != nil {
		t.Fatal(err)
	}
	p, err := vm.ResolveSymbol("RESULT")
	if err != nil || vm.ReadHeap(p, 1)[0] != 'a' {
		t.Fatal(err, vm.ReadHeap(p, 1))
	}
	decl, err := Compile("Only: procedure; end Only;")
	if err != nil || decl.FuncByName("main") != nil {
		t.Fatal("declaration-only entry", err)
	}
	if _, err = Compile("Bad: do; call Only(.('a', 'b')); end Bad;"); err == nil {
		t.Fatal("unsupported literal list silently accepted")
	}
}

func TestPLMLiteralCaseAndMacros(t *testing.T) {
	const src = "/* user's comment */ Demo: do; declare Alias literally '0ffh'; declare Result byte; Result=Alias; end Demo;"
	h, e := Compile(src)
	if e != nil {
		t.Fatal(e)
	}
	vm := mir2.NewVM(hir.LowerModule(h))
	if _, e = vm.Call("main", nil); e != nil {
		t.Fatal(e)
	}
	p, e := vm.ResolveSymbol("RESULT")
	if e != nil || vm.ReadHeap(p, 1)[0] != 255 {
		t.Fatal(e, vm.ReadHeap(p, 1))
	}
	l, e := NewLexer("call f('aB''c!', 0ffh);")
	if e != nil {
		t.Fatal(e)
	}
	var literals []string
	for !l.IsKind(TokEOF) {
		v := l.Next()
		if v.Kind == TokString {
			literals = append(literals, v.Val)
		}
	}
	if len(literals) != 1 || literals[0] != "aB'c!" {
		t.Fatal(literals)
	}
	if got := Preprocess("/* don't fold strings */ declare X literally '7'; call F('aB''X');"); !strings.Contains(got, "'aB''X'") {
		t.Fatal(got)
	}
}
