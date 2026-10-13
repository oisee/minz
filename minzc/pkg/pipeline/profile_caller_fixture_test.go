package pipeline

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"testing"
)

const profileCallerSource = `global data:[u8;5]=[1,2,3,4,5]
global output:u8
fun small_sum(ptr:^u8,n:u8)->u8 {
 var s:u8=0
 var i:u8=0
 while i<n {s=s+ptr[i]
 i=i+1}
 return s
}
fun big_sum(ptr:^u8,n:u8)->u8 {
 var s:u8=0
 var i:u8=0
 while i<n {s=s+ptr[i]
 i=i+1}
 return s
}
fun run()->u8 {
 var first:u8=small_sum(&data,1)
 var second:u8=big_sum(&data,5)
 output=first+second
 return output
}`

func TestOrdinaryCallerProfileOracle(t *testing.T) {
	m, err := nanz.Parse(profileCallerSource, "caller.nanz")
	if err != nil {
		t.Fatal(err)
	}
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 100000, MaxMemory: 65536, Calls: []hir.ProfileInvocation{{Function: "run"}, {Function: "run"}}}
	run, err := hir.RunLoopProfile(m, w)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Results) != 2 || run.Results[0][0].I != 16 || run.Results[1][0].I != 16 || run.Globals["output"][0] != 16 {
		t.Fatal(run)
	}
	if len(run.Profile.Guards) != 2 {
		t.Fatal(run.Profile.Guards)
	}
}
