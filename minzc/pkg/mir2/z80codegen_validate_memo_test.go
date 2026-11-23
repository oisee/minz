package mir2

import (
	"reflect"
	"testing"

	"github.com/minz/minzc/pkg/z80validate"
)

// The memo answers only the exact text it validated; any other text, however
// close, is validated afresh.
func TestValidateFunctionAsmMemoKeyedByText(t *testing.T) {
	valid := "f:\n    LD A, B\n    RET\n"
	invalid := "f:\n    LD HL, (DE)\n    RET\n"
	for _, asm := range []string{valid, invalid} {
		want := z80validate.Validate(asm)
		if got := validateFunctionAsm(asm, nil); !reflect.DeepEqual(got, want) {
			t.Fatalf("nil memo: %v, want %v", got, want)
		}
		memo := map[string][]z80validate.Error{}
		if got := validateFunctionAsm(asm, memo); !reflect.DeepEqual(got, want) {
			t.Fatalf("first lookup: %v, want %v", got, want)
		}
		if got, ok := memo[asm]; !ok || !reflect.DeepEqual(got, want) {
			t.Fatalf("not memoised: %v %v", got, ok)
		}
	}
	if len(z80validate.Validate(invalid)) == 0 {
		t.Fatal("control: invalid text validated clean")
	}
	// A seeded answer is returned for its own text only.
	memo := map[string][]z80validate.Error{valid: z80validate.Validate(invalid)}
	if got := validateFunctionAsm(valid, memo); len(got) == 0 {
		t.Fatal("memo not consulted for identical text")
	}
	if got := validateFunctionAsm(valid+"\n", memo); len(got) != 0 {
		t.Fatalf("memo answered different text: %v", got)
	}
}

// Every emission of one Z80Codegen call shares a memo, and the product is
// the same as an emission validated without it.
func TestZ80CodegenSharesValidateMemo(t *testing.T) {
	s := buildExactSite(t, "C", "B", []string{"D", "E", "H", "L"}, false)
	saved := z80CodegenEmit
	defer func() { z80CodegenEmit = saved }()
	var memos []map[string][]z80validate.Error
	var caches []map[string]map[string]bool
	z80CodegenEmit = func(m *Module, ar *AllocResult, cache map[string]map[string]bool, opts ...Z80CodegenOptions) (string, error) {
		memos = append(memos, opts[0].validateMemo)
		caches = append(caches, cache)
		return z80CodegenWithLeafWrites(m, ar, cache, opts...)
	}
	asm, err := Z80Codegen(s.m, s.ar)
	if err != nil {
		t.Fatal(err)
	}
	if len(memos) != 2 || memos[0] == nil || reflect.ValueOf(memos[0]).Pointer() != reflect.ValueOf(memos[1]).Pointer() {
		t.Fatalf("emissions do not share one memo: %d emissions", len(memos))
	}
	plain, err := z80CodegenWithLeafWrites(s.m, s.ar, caches[1])
	if err != nil || plain != asm {
		t.Fatalf("memoised product differs from plain emission (err %v)\n%s\n---\n%s", err, asm, plain)
	}
}
