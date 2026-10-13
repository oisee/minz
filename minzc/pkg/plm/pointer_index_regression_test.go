package plm_test

import (
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/plm"
	"testing"
)

func TestDeclaredWordPointerIndexVMZ80(t *testing.T) {
	src := `M: DO;
 DECLARE CELL BYTE;
 DECLARE ARR(3) BYTE;
 TEST: PROCEDURE (X) BYTE;
 DECLARE X BYTE;
 DECLARE PTR WORD;
 DECLARE RAW BASED PTR BYTE;
 PTR = .CELL;
 RAW = X;
 ARR(0) = 11;
 ARR(1) = 73;
 PTR = .ARR(1);
 RETURN PTR(0) + X;
 END TEST;
 ASSERT TEST(0) = 73;
 ASSERT TEST(73) = 146;
 ASSERT TEST(255) = 72;
 END M;`
	h, err := plm.Compile(src)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pipeline.CompileHIR(h); err != nil {
		t.Fatal(err)
	}
}
