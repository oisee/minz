package hir_test

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"testing"
)

func TestLexicalSourcePersistentVMZ80(t *testing.T) {
	const src = `fun plus_one(v:u16)->u16{return v+1}
 fun plus_two(v:u16)->u16{return v+2}
 fun scoped(x:u16,n:u8,buf:^u8)->u16 {
 if n>0 {var temp:u16=plus_two(x)}
 var i:u8=0
 while i<n {if i==1 {var x:u8=99
 i=i+1
 continue}
 x=x+2
 i=i+1}
 var length:u16=3
 for x in buf[0..length] {var inner:u8=x}
 for x in 0..3 {var x:u8=99}
 return plus_one(x)
 }`
	m, e := nanz.Parse(src, "lexical-source.nanz")
	if e != nil {
		t.Fatal(e)
	}
	literal := func(n int64) hir.ProfileArgument { return hir.ProfileArgument{Value: &n} }
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 1000000, MaxMemory: 65536, ExactDataAccess: true, Resources: map[string][]byte{"buf": {1, 2, 3}}, Globals: map[string][]byte{}}
	expected := []int64{}
	for _, n := range []int64{0, 1, 2, 3, 5} {
		w.Calls = append(w.Calls, hir.ProfileInvocation{Function: "scoped", Arguments: []hir.ProfileArgument{literal(7), literal(n), {Resource: "buf"}}})
		count := n
		if n > 1 {
			count--
		}
		expected = append(expected, 8+2*count)
	}
	raw, e := hir.RunLoopProfile(m, w)
	if e != nil {
		t.Fatal(e)
	}
	for i, want := range expected {
		if raw.Results[i][0].I != want {
			t.Fatalf("sourceHIR call%dgot%v want%d", i, raw.Results[i], want)
		}
	}
	for _, grace := range []bool{false, true} {
		m, e = nanz.Parse(src, "lexical-source.nanz")
		if e != nil {
			t.Fatal(e)
		}
		steps, e := pipeline.CompileHIRSteps(m, pipeline.Options{AssertMode: "none", UseHIRGrace: grace})
		if e != nil {
			t.Fatal(e)
		}
		run, e := pipeline.RunZ80ProfileWorkload(steps, m, w, *raw)
		if e != nil {
			t.Fatal(e)
		}
		for i, want := range expected {
			if run.Invocations[i].Results[0].I != want {
				t.Fatalf("machinecall%dgot%v want%d", i, run.Invocations[i].Results, want)
			}
		}
	}
}
