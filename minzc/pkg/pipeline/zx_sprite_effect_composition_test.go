package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

func TestZXSpriteEffectCompositionMachine(t *testing.T) {
	source, err := os.ReadFile("../../../reports/advanced-demos/sprite-effect-composition/source.nanz")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(source), `effect "TVSignal"`) != 1 {
		t.Fatal("effect descriptor is not unique")
	}
	wave := []int{0, 1, 1, 0, 0, -1, -1, 0}
	shift := func(frame, row int) int {
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
	}
	root := os.Getenv("MINZ_SPRITE_COMPOSITION_EXPORT")
	for _, style := range []string{"default", "unrolled", "loop"} {
		t.Run(style, func(t *testing.T) {
			candidate := source
			if style != "default" {
				candidate = []byte(strings.Replace(string(source), `effect "TVSignal"`, `effect "TVSignal"`+"\n    effect_preparation "+`"`+style+`"`, 1))
				if root != "" {
					t.Setenv("MINZ_SPRITE_COMPOSITION_EXPORT", filepath.Join(root, style))
				}
			}
			runZXTypedSpriteSceneSourceMachine(t, candidate, "MINZ_SPRITE_COMPOSITION_EXPORT", shift)
		})
	}
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
	for _, field := range []string{
		`effect_preparation`, `effect_preparation ""`, `effect_preparation "unknown"`,
		`effect_preparation "loop" "unrolled"`,
		"effect_preparation \"loop\"\neffect_preparation \"unrolled\"",
		`effect_preparation "loop"`, // absent effect
	} {
		decl := ""
		if field != `effect_preparation "loop"` {
			decl = "effect \"ByteWave\"\n"
		}
		source := "import zx.sprite\n@zx_sprite(\"Bad\") {\nsize 8 1\npixels \"########\"\n" + decl + field + "\n}"
		_, err := nanz.ParseWithOpts(source, "bad-preparation.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if err == nil || !strings.Contains(err.Error(), "effect_preparation") {
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
	for _, style := range []string{"unrolled", "loop"} {
		t.Run("caller-"+style, func(t *testing.T) {
			candidate := strings.Replace(source, `effect "CustomShift"`, `effect "CustomShift"`+"\neffect_preparation "+`"`+style+`"`, 1)
			checkZXSpriteEffectSourceNativeVM(t, candidate, 1)
		})
		t.Run("ordered-instances-"+style, func(t *testing.T) {
			candidate := `import zx.sprite
struct OrderedEffect {digest:u8,count:u8}
fun OrderedEffect.offset(self:^OrderedEffect,row:u8)->i8 {
 self.digest=self.digest*3+row
 self.count=self.count+1
 return 0
}
@zx_sprite_instances("Ordered") {
 instances 2
 size 8 4
 pixels "#_______"
 pixels "_#______"
 pixels "__#_____"
 pixels "___#____"
 effect "OrderedEffect"
 effect_preparation "STYLE"
}
global effect:OrderedEffect=OrderedEffect{digest:7,count:0}
fun run()->u16 {
 let first:Ordered_0=Ordered_0{x:8,y:0}
 let second:Ordered_1=Ordered_1{x:24,y:8}
 if first.stage_effect(&effect)==false {return 0}
 if second.stage_effect(&effect)==false {return 0}
 return u16(effect.count)*256+u16(effect.digest)
}`
			candidate = strings.Replace(candidate, "STYLE", style, 1)
			// Independent ordered recurrence: two visits of rows 0,1,2,3.
			digest := byte(7)
			for instance := 0; instance < 2; instance++ {
				for row := byte(0); row < 4; row++ {
					digest = digest*3 + row
				}
			}
			checkZXSpriteEffectSourceNativeVM(t, candidate, 8*256+int64(digest))
		})
	}
}

func checkZXSpriteEffectSourceNativeVM(t *testing.T, source string, want int64) {
	t.Helper()
	parse := func() *hir.Module {
		h, err := nanz.ParseWithOpts(source, "caller-effect.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	for _, opt := range []Options{{ContractOpt: true, AssertMode: "none"}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true, OptSize: true}} {
		measureGraceZ80HIR(t, parse(), "run", opt, nil, nil, want)
		steps, err := CompileHIRSteps(parse(), opt)
		if err != nil {
			t.Fatal(err)
		}
		for _, module := range []*mir2.Module{hir.LowerModule(parse()), steps.MIR2Module} {
			vm := mir2.NewVM(module)
			vm.EnsureHeap(0x6000)
			got, err := vm.Call("run", nil)
			if err != nil || len(got) != 1 || got[0].I != want {
				t.Fatal(got, err)
			}
		}
	}
}
