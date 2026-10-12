package nanz_test

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"strings"
	"testing"
)

func TestLambdaCaptureMetadata(t *testing.T) {
	src := `global values:[u8;3]=[1,2,3]
fun test()->u8 {
 var sum:u8=0
 values.forEach(|x:u8| {sum=sum+x},3)
 return sum
}`
	m, err := nanz.Parse(src, "captures")
	if err != nil {
		t.Fatal(err)
	}
	f := m.FuncByName("lambda_0")
	if f == nil || len(f.Captures) != 1 || f.Captures[0].Name != "sum" {
		t.Fatalf("captures %+v", f)
	}
	if err := hir.Verify(m); err != nil {
		t.Fatal(err)
	}
	got, err := mir2.NewVM(hir.LowerModule(m)).Call("test", nil)
	if err != nil || len(got) != 1 || got[0].I != 6 {
		t.Fatalf("got %v err %v", got, err)
	}
	f.Captures = nil
	if err := hir.Verify(m); err == nil {
		t.Fatal("missing capture accepted")
	}
}
func TestLambdaUnknownReferenceRejected(t *testing.T) {
	_, err := nanz.Parse(`fun test()->u8 { let f=|x:u8| x+typo
 return f(1) }`, "typo")
	if err == nil || !strings.Contains(err.Error(), "unknown variable") {
		t.Fatalf("got %v", err)
	}
}
func TestPartialApplicationCapturesBoundVariable(t *testing.T) {
	m, err := nanz.Parse(`fun add(a:u8,b:u8)->u8{return a+b}
fun test(n:u8)->u8 {let f=add(n,_)
 return f(3)}`, "partial_capture")
	if err != nil {
		t.Fatal(err)
	}
	f := m.FuncByName("lambda_0")
	if len(f.Captures) != 1 || f.Captures[0].Name != "n" {
		t.Fatalf("captures %v", f.Captures)
	}
	if err := hir.Verify(m); err == nil || !strings.Contains(err.Error(), "inlined iterator context") {
		t.Fatalf("got %v", err)
	}
}
