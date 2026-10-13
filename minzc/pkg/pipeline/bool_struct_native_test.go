package pipeline

import (
	"bytes"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"testing"
)

// Book v7 struct fields are separately addressable typed values, just as bool arrays.
func TestBoolStructAddressableFieldsNative(t *testing.T) {
	const source = `struct Flags { first:bool
 second:bool
 third:bool
 value:u8 }
 global flags:Flags
 global guard:u8=165
 fun fill() {flags.first=true
 flags.second=false
 flags.third=true
 flags.value=90}`
	h, err := nanz.Parse(source, "bool-struct")
	if err != nil {
		t.Fatal(err)
	}
	steps, err := CompileHIRSteps(h)
	if err != nil {
		t.Fatal(err)
	}
	vm := mir2.NewVM(steps.MIR2Module)
	if _, err = vm.Call("fill", nil); err != nil {
		t.Fatal(err)
	}
	memory, symbols := nativeOracleRun(t, steps, []string{"fill"})
	want := []byte{1, 0, 1, 90}
	a := symbols["flags"]
	if !bytes.Equal(vm.ReadHeap(vm.GlobalAddr("flags"), 4), want) || !bytes.Equal(memory[a:a+4], want) {
		t.Fatalf("VM=%v Z80=%v want=%v", vm.ReadHeap(vm.GlobalAddr("flags"), 4), memory[a:a+4], want)
	}
	if vm.ReadHeap(vm.GlobalAddr("guard"), 1)[0] != 165 || memory[symbols["guard"]] != 165 {
		t.Fatal("struct writes overwrote next global")
	}
}
