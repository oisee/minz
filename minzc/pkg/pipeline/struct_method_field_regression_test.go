package pipeline

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"testing"
)

// Pointer-to-small-struct method arguments retain the pointer's ABI width;
// offset4+field addresses must preserve live BC cap and the HL base.
func TestStructMethodPointerAndFieldScratchNative(t *testing.T) {
	const source = `struct Small { value: u8 }
struct Writer { ptr: ^u8 len: u16 cap: u16 overflow: u8 }
global padding:[u8;257]
global item:Small
global writer:Writer
global output:[u8;32]
fun Writer.init(self:^Writer,p:^u8,cap:u16) { self.ptr=p
 self.len=0
 self.cap=cap
 self.overflow=0 }
fun Small.read(self:^Small)->u8 { return self.value }
fun check(cap:u16,value:u8)->u16 { writer.init(&output,cap)
 item.value=value
 let v:u8=item.read()
 return writer.cap+u16(v)+u16(writer.overflow) }
`
	h, e := nanz.Parse(source, "struct-method-regression.nanz")
	if e != nil {
		t.Fatal(e)
	}
	for _, cap := range []int64{0, 1, 4, 32, 255, 256, 32768, 65535} {
		for _, v := range []int64{0, 1, 127, 128, 175, 255} {
			h.Asserts = append(h.Asserts, hir.Assert{FuncName: "check", Args: []int64{cap, v}, Expected: (cap + v) & 65535})
		}
	}
	if _, e = CompileHIRSteps(h); e != nil {
		t.Fatal(e)
	}
	// The old MIR copy of Small width8 silently hid pointer truncation in VM.
	// The formal pointer type is required in the explicit class-coercion Move.
	raw := hir.LowerModule(h)
	for _, f := range raw.Funcs {
		for _, b := range f.Blocks {
			for _, in := range b.Insts {
				if in.Op == mir2.OpMove {
					if _, ok := in.Ty.(*mir2.StructTy); ok {
						t.Fatalf("struct storage type used for argument move in%s", f.Name)
					}
				}
			}
		}
	}
	t.Log("48 independent cap+byte results native/VM, formal pointer width and field scratch PASS")
}
