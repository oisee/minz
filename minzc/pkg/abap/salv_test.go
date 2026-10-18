package abap

import (
	"crypto/sha256"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"os"
	"testing"
)

func TestSALVFrozenReportCompile(t *testing.T) {
	src, err := os.ReadFile("../../../examples/abap/zfleet_csv_alv.abap")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(src)) != "50a48e0726154b3d37115bfdcd09d4f6222c7c2e3380293128cb489c75fb7979" {
		t.Fatal("frozen source modified")
	}
	m, err := Compile(string(src), "zfleet_csv_alv")
	if err != nil {
		t.Fatal(err)
	}
	if err := hir.Verify(m); err != nil {
		t.Fatal(err)
	}
}
