package nanz_test

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"testing"
)

func TestInputLoopProfileCountsAndSemantics(t *testing.T) {
	src := `global effects:u8
fun test(n:u8)->u8 {
 var i:u8=0
 while i<n {
  i=i+1
  if i==2 {continue}
  if i==4 {break}
  effects=effects+1
 }
 return i
}`
	h, err := nanz.Parse(src, "profile.nanz")
	if err != nil {
		t.Fatal(err)
	}
	before := h.Dump()
	p, err := hir.NewLoopProfiler(h)
	if err != nil {
		t.Fatal(err)
	}
	plain := mir2.NewVM(hir.LowerModule(h))
	totalBody := uint64(0)
	for _, n := range []int64{0, 1, 3, 5, 255} {
		want := n
		if want > 4 {
			want = 4
		}
		totalBody += uint64(want)
		got, err := p.Call("test", []mir2.Value{{I: n}})
		if err != nil || len(got) != 1 || got[0].I != want {
			t.Fatalf("n=%d got=%v err=%v", n, got, err)
		}
		original, e := plain.Call("test", []mir2.Value{{I: n}})
		if e != nil || original[0] != got[0] {
			t.Fatalf("original=%v err=%v", original, e)
		}
		if string(p.VM.ReadHeap(p.VM.GlobalAddr("effects"), 1)) != string(plain.ReadHeap(plain.GlobalAddr("effects"), 1)) {
			t.Fatal("effects changed")
		}
	}
	snapshot := p.Snapshot()
	if snapshot.CompletedCalls != 5 || snapshot.FailedCalls != 0 || len(snapshot.Counts) != 1 || snapshot.Counts[0].Entries != 5 || snapshot.Counts[0].BodyEntries != totalBody {
		t.Fatalf("counts: %+v", snapshot)
	}
	if h.Dump() != before {
		t.Fatal("input HIR mutated")
	}
	snapshot.Counts[0].Entries = 999
	snapshot.Sites.Sites[0].ID = "changed"
	if p.Snapshot().Counts[0].Entries != 5 || p.Snapshot().Sites.Sites[0].ID == "changed" {
		t.Fatal("snapshot aliases internal state")
	}
	data, err := json.Marshal(p.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(data))
	// Partial observations must retain failure status, never look like complete trips.
	p.VM.MaxSteps = p.VM.StepsExecuted() + 1
	if _, err := p.Call("test", []mir2.Value{{I: 5}}); err == nil {
		t.Fatal("gas failure missing")
	}
	if p.Snapshot().FailedCalls != 1 {
		t.Fatal("failed sample unmarked")
	}
}

func TestNestedLoopProfile(t *testing.T) {
	h, err := nanz.Parse(`fun test(n:u8)->u8 {
 var i:u8=0
 var sum:u8=0
 while i<n {
  var j:u8=0
  while j<i {sum=sum+1
   j=j+1}
  i=i+1
 }
 return sum
}`, "nested.nanz")
	if err != nil {
		t.Fatal(err)
	}
	p, err := hir.NewLoopProfiler(h)
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.Call("test", []mir2.Value{{I: 4}})
	if err != nil || got[0].I != 6 {
		t.Fatalf("got=%v err=%v", got, err)
	}
	s := p.Snapshot()
	if len(s.Counts) != 2 {
		t.Fatalf("sites=%+v", s)
	}
	for i, site := range s.Sites.Sites {
		c := s.Counts[i]
		if site.Origin.Line == 4 {
			if c.Entries != 1 || c.BodyEntries != 4 {
				t.Fatalf("outer=%+v", c)
			}
		} else {
			if c.Entries != 4 || c.BodyEntries != 6 {
				t.Fatalf("inner=%+v", c)
			}
		}
	}
}

func TestRangeAndEachLoopProfile(t *testing.T) {
	for _, src := range []string{
		`fun test(n:u8)->u8 {var sum:u8=0
 for i in 0..n {sum=sum+1}
 return sum}`,
		`global buf:[u8;5]=[1,2,3,4,5]
fun test(n:u8)->u8 {var sum:u8=0
 for b:u8 in buf[0..n] {sum=sum+1}
 return sum}`,
	} {
		h, err := nanz.Parse(src, "structured.nanz")
		if err != nil {
			t.Fatal(err)
		}
		p, err := hir.NewLoopProfiler(h)
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range []int64{0, 1, 5} {
			got, err := p.Call("test", []mir2.Value{{I: n}})
			if err != nil || got[0].I != n {
				t.Fatalf("got=%v want=%d err=%v", got, n, err)
			}
		}
		s := p.Snapshot()
		if len(s.Counts) != 1 || s.Counts[0].Entries != 3 || s.Counts[0].BodyEntries != 6 {
			t.Fatalf("counts: %+v", s)
		}
	}
}

func TestLoopProfileMissingOriginsAndHostCollision(t *testing.T) {
	src := `fun __minz_loop_profile_0_entry()->u8 {return 7}
fun test(n:u8)->u8 {while n>0 {n=n-1}
 return __minz_loop_profile_0_entry()}`
	h, err := nanz.Parse(src, "collision.nanz")
	if err != nil {
		t.Fatal(err)
	}
	for _, mapped := range []bool{true, false} {
		if !mapped {
			h.SourceLocations = nil
		}
		p, err := hir.NewLoopProfiler(h)
		if err != nil {
			t.Fatal(err)
		}
		got, err := p.Call("test", []mir2.Value{{I: 3}})
		if err != nil || got[0].I != 7 {
			t.Fatalf("got=%v err=%v", got, err)
		}
		s := p.Snapshot()
		if mapped {
			if len(s.Counts) != 1 || s.Counts[0].BodyEntries != 3 {
				t.Fatalf("counts=%+v", s)
			}
		} else {
			if len(s.Counts) != 0 || s.Sites.UnmappedLoops != 1 {
				t.Fatalf("invented origins=%+v", s)
			}
		}
	}
}
