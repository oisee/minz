package nanz_test

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

func TestUserMetafunctionGeneratedReturnType(t *testing.T) {
	const src = `
fun @make_value() -> void {
    emit(c"fun generated() -> u8 { return 46 }")
}
@make_value()
fun main() -> u8 { return generated() }
`
	m, err := nanz.Parse(src, "generated_return.nanz")
	if err != nil {
		t.Fatal(err)
	}
	vm := mir2.NewVM(hir.LowerModule(m))
	values, err := vm.Call("main", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0].I != 46 {
		t.Fatalf("generated u8 result = %v, want 46", values)
	}
}

func TestUserMetafunctionNumericBlockArguments(t *testing.T) {
	const src = `
fun @make_value() -> void {
    let n: u8 = node_arg_int(0, 0)
    emit(str_concat(c"fun generated() -> u8 { return ", str_concat(str_from_int(n), c" }")))
}
@make_value() { value 46 }
fun main() -> u8 { return generated() }
`
	m, err := nanz.Parse(src, "numeric_block.nanz")
	if err != nil {
		t.Fatal(err)
	}
	vm := mir2.NewVM(hir.LowerModule(m))
	values, err := vm.Call("main", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0].I != 46 {
		t.Fatalf("numeric block result = %v, want 46", values)
	}

	bad := strings.Replace(src, "value 46", `value "forty-six"`, 1)
	if _, err := nanz.Parse(bad, "bad_numeric_block.nanz"); err == nil || !strings.Contains(err.Error(), "node_arg_int") {
		t.Fatalf("invalid numeric argument should fail at compile time, got %v", err)
	}
}
