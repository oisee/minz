package pascal_test

import (
	"github.com/minz/minzc/pkg/pascal"
	"github.com/minz/minzc/pkg/pipeline"
	"testing"
)

func TestRecordFieldOffsetsAndArrayStoreVMZ80(t *testing.T) {
	src := `program Test;
 type Point = record X: Byte; Y: Byte; end;
 var P: Point; Arr: array[0..2] of Byte;
 function PointSum(A: Point): Byte;
 begin PointSum := A.X + A.Y; end;
 function Check(V: Byte): Byte;
 begin
 P.X := V; P.Y := 4;
 Arr[0] := 100; Arr[1] := 23; Arr[2] := 17;
 Check := PointSum(P) + Arr[1];
 end;
 begin
 assert Check(3) = 30;
 assert Check(0) = 27;
 assert Check(255) = 26;
 end.`
	h, err := pascal.Compile(src, "aggregate")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pipeline.CompileHIR(h); err != nil {
		t.Fatal(err)
	}
}
