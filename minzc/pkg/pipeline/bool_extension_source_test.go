package pipeline

import (
	"github.com/minz/minzc/pkg/nanz"
	"testing"
)

// Exercise the automatically allocated pending predicate used by the real
// local-width diagnosis. The explicit cast isolates this backend contract from
// the separate implicit declaration-width repair.
func TestBoolExtensionSourceVMZ80(t *testing.T) {
	source := `fun decl(v:u16)->u16 {let x:u8=u8(v)
return u16(x)+u16(x==0)*1000}
fun double_bool(x:u8)->u16{let b:bool=x==0
return u16(b)+u16(b)*1000}
fun later_bool(x:u8,y:u8)->u16{let b:bool=x<=y
let first:u16=u16(b)
let other:u16=u16(x!=y)
return first+u16(b)*1000+other*10}
fun signed(v:i8)->u16{return u16(i16(v))}
assert decl(0)==1000
assert decl(255)==255
assert decl(256)==1000
assert decl(257)==1
assert double_bool(0)==1001
assert double_bool(1)==0
assert later_bool(0,0)==1001
assert later_bool(0,1)==1011
assert later_bool(1,0)==10
assert later_bool(255,255)==1001
assert signed(-128)==65408
assert signed(-1)==65535
assert signed(0)==0
assert signed(127)==127`
	for _, grace := range []bool{false, true} {
		m, e := nanz.Parse(source, "bool-extension.nanz")
		if e != nil {
			t.Fatal(e)
		}
		if _, e = CompileHIRSteps(m, Options{AssertMode: "all", UseHIRGrace: grace}); e != nil {
			t.Fatal(e)
		}
	}
}
