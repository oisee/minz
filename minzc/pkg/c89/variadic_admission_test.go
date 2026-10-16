package c89_test

import (
	"github.com/minz/minzc/pkg/c89"
	"strings"
	"testing"
)

func TestC89VariadicAdmission(t *testing.T) {
	for _, src := range []string{
		"int sum(int n,...){return n;}",
		"int sum(int,...); int check(void){return sum(2,3,4);}",
		"int check(int (*f)(int,...)){return f(2,3,4);}",
		"#include <stdarg.h>\nint sum(int n,...){va_list ap; va_start(ap,n); return va_arg(ap,int);}",
	} {
		if _, e := c89.Compile(src, "variadic.c"); e == nil || !strings.Contains(e.Error(), "unsupported Z80 varargs ABI") {
			t.Fatalf("expected explicit variadic ABI diagnostic, got %v for %s", e, src)
		}
	}
	if _, e := c89.Compile("int unused(int,...); int check(void){return 7;}", "unused.c"); e != nil {
		t.Fatal("unused header declaration must remain legal", e)
	}
}
