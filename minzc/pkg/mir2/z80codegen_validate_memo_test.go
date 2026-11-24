package mir2

import (
	"crypto/sha256"
	"reflect"
	"testing"

	"github.com/minz/minzc/pkg/z80validate"
)

// The cache answers only the exact text it validated; any other text, however
// close, is validated afresh. A nil cache validates directly.
func TestValidationCacheKeyedByText(t *testing.T) {
	valid := "f:\n    LD A, B\n    RET\n"
	invalid := "f:\n    LD HL, (DE)\n    RET\n"
	if len(z80validate.Validate(invalid)) == 0 {
		t.Fatal("control: invalid text validated clean")
	}
	for _, asm := range []string{valid, invalid} {
		want := z80validate.Validate(asm)
		var none *validationCache
		if got := none.validate(asm); !reflect.DeepEqual(got, want) {
			t.Fatalf("nil cache: %v, want %v", got, want)
		}
		c := &validationCache{}
		if got := c.validate(asm); !reflect.DeepEqual(got, want) {
			t.Fatalf("first lookup: %v, want %v", got, want)
		}
		if got, ok := c.m[sha256.Sum256([]byte(asm))]; !ok || !reflect.DeepEqual(got, want) {
			t.Fatalf("not memoised: %v %v", got, ok)
		}
	}
	// A seeded answer is returned for its own text only.
	c := &validationCache{m: map[[sha256.Size]byte][]z80validate.Error{
		sha256.Sum256([]byte(valid)): z80validate.Validate(invalid),
	}}
	if got := c.validate(valid); len(got) == 0 {
		t.Fatal("cache not consulted for identical text")
	}
	if got := c.validate(valid + "\n"); len(got) != 0 {
		t.Fatalf("cache answered different text: %v", got)
	}
}

// A full cache starts afresh instead of growing without bound.
func TestValidationCacheBounded(t *testing.T) {
	c := &validationCache{m: map[[sha256.Size]byte][]z80validate.Error{}}
	for i := 0; i < validationCacheLimit; i++ {
		c.m[[sha256.Size]byte{byte(i), byte(i >> 8), byte(i >> 16), 1}] = nil
	}
	c.validate("f:\n    RET\n")
	if len(c.m) != 1 {
		t.Fatalf("cache holds %d texts after reaching the limit", len(c.m))
	}
}

// Z80Codegen validates through the shared cache in every emission, and the
// product equals an emission validated without it.
func TestZ80CodegenUsesValidationCache(t *testing.T) {
	s := buildExactSite(t, "C", "B", []string{"D", "E", "H", "L"}, false)
	saved := z80CodegenEmit
	defer func() { z80CodegenEmit = saved }()
	var caches []*validationCache
	var clobbers []map[string]map[string]bool
	z80CodegenEmit = func(m *Module, ar *AllocResult, cache map[string]map[string]bool, opts ...Z80CodegenOptions) (string, error) {
		caches = append(caches, opts[0].validation)
		clobbers = append(clobbers, cache)
		return z80CodegenWithLeafWrites(m, ar, cache, opts...)
	}
	asm, err := Z80Codegen(s.m, s.ar)
	if err != nil {
		t.Fatal(err)
	}
	if len(caches) != 2 || caches[0] != sharedValidation || caches[1] != sharedValidation {
		t.Fatalf("emissions do not use the shared cache: %v", caches)
	}
	plain, err := z80CodegenWithLeafWrites(s.m, s.ar, clobbers[1])
	if err != nil || plain != asm {
		t.Fatalf("cached product differs from plain emission (err %v)\n%s\n---\n%s", err, asm, plain)
	}
}
