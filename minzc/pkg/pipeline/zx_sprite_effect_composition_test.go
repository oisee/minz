package pipeline

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"strings"
	"testing"
)

func TestZXSpriteEffectCompositionMachine(t *testing.T) {
	wave := []int{0, 1, 1, 0, 0, -1, -1, 0}
	runZXTypedSpriteSceneMachine(t, "sprite-effect-composition", "MINZ_SPRITE_COMPOSITION_EXPORT", func(frame, row int) int {
		shift := wave[(frame+row)%8]
		if frame < 16 {
			return shift
		}
		hash := byte(frame*13+row*17) ^ 0x5a
		hash ^= hash >> 3
		if hash&3 == 0 {
			return shift + 1
		}
		if hash&3 == 1 {
			return shift - 1
		}
		return shift
	})
}

func TestZXSpriteEffectTypeAdmission(t *testing.T) {
	for _, field := range []string{`effect ""`, `effect "bad-type"`, `effect "TVSignal"
 effect "ByteWave"`} {
		source := "import zx.sprite\n@zx_sprite(\"Bad\") {\n size 8 1\n pixels \"########\"\n" + field + "\n}"
		_, err := nanz.ParseWithOpts(source, "bad-effect.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if err == nil || !strings.Contains(err.Error(), "effect") {
			t.Fatalf("%s: %v", field, err)
		}
	}
	source := `import zx.sprite
struct MissingOffset {value:u8}
@zx_sprite("Bad") {
 size 8 1
 pixels "########"
 effect "MissingOffset"
}
fun run()->bool {let actor:Bad=Bad{x:8,y:0} let effect:MissingOffset=MissingOffset{value:0} return actor.stage_effect(&effect)}`
	h, err := nanz.ParseWithOpts(source, "missing-effect-method.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if err == nil {
		_, err = CompileHIRSteps(h, Options{AssertMode: "none", ContractOpt: true})
	}
	if err == nil {
		t.Fatal("effect without offset method was accepted")
	}
	if strings.Contains(err.Error(), "unknown type") || !strings.Contains(err.Error(), "Bad_stage_effect") || !strings.Contains(err.Error(), "void incompatible with i8") {
		t.Fatalf("expected a missing offset contract, got: %v", err)
	}
}

func TestZXSpriteCallerEffectTypeNativeVM(t *testing.T) {
	source := `import zx.sprite
struct CustomShift {bias:i8}
fun CustomShift.offset(self:^CustomShift,row:u8)->i8 {return self.bias}
@zx_sprite("Local") {
 size 8 1
 erase
 pixels "#_______"
 effect "CustomShift"
}
global effect:CustomShift=CustomShift{bias:1}
fun run()->u8 {
 let actor:Local=Local{x:8,y:0}
 if actor.stage_effect(&effect)==false {return 0}
 if actor.publish()==false {return 0}
 let p:^u8=ptr(0x4000)
 if p[1]!=0 {return 0}
 if p[2]!=0x80 {return 0}
 return 1
}`
	parse := func() *hir.Module {
		h, err := nanz.ParseWithOpts(source, "caller-effect.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	for _, opt := range []Options{{ContractOpt: true, AssertMode: "none"}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true, OptSize: true}} {
		measureGraceZ80HIR(t, parse(), "run", opt, nil, nil, 1)
		steps, err := CompileHIRSteps(parse(), opt)
		if err != nil {
			t.Fatal(err)
		}
		for _, module := range []*mir2.Module{hir.LowerModule(parse()), steps.MIR2Module} {
			vm := mir2.NewVM(module)
			vm.EnsureHeap(0x6000)
			got, err := vm.Call("run", nil)
			if err != nil || len(got) != 1 || got[0].I != 1 {
				t.Fatal(got, err)
			}
		}
	}
}
