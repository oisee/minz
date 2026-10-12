package pipeline

import (
	"github.com/minz/minzc/pkg/hir"
	"strings"
	"testing"
)

// The v1 workload result schema contains scalar values, not object bytes.
// A source-level scalar caller remains usable after caller-buffer lowering.
func TestProfileScalarCallerWithOwnedStructResult(t *testing.T) {
	const source = `struct Pair { a:u8, b:u8 }
fun make(x:u8)->Pair { if x==0 {return Pair{a:0,b:1}}
return Pair{a:x,b:1} }
fun read(x:u8)->u8 {let p=make(x)
return p.a+p.b}`
	w := workloadSpec(hir.ProfileInvocation{Function: "read", Arguments: []hir.ProfileArgument{workloadLiteral(17)}})
	steps, input, oracle := workloadFixture(t, source, w)
	result, e := RunZ80ProfileWorkload(steps, input, w, oracle)
	if e != nil || result == nil {
		t.Fatalf("scalar caller: %v", e)
	}
	if len(oracle.Results) != 1 || len(oracle.Results[0]) != 1 || oracle.Results[0][0].I != 18 {
		t.Fatal(oracle.Results)
	}
}

func TestProfileAggregateEntryRequiresObjectOracle(t *testing.T) {
	const source = `struct Pair {a:u8,b:u8}
 fun make(x:u8)->Pair {if x==0 {return Pair{a:0,b:1}}
 return Pair{a:x,b:1}}`
	w := workloadSpec(hir.ProfileInvocation{Function: "make", Arguments: []hir.ProfileArgument{workloadLiteral(17)}})
	steps, input, oracle := workloadFixture(t, source, w)
	result, e := RunZ80ProfileWorkload(steps, input, w, oracle)
	if result != nil || e == nil || (!strings.Contains(e.Error(), "ABI") && !strings.Contains(e.Error(), "unsupported return")) {
		t.Fatalf("aggregate entry accepted: %v", e)
	}
}
