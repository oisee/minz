package nanz_test

import (
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
