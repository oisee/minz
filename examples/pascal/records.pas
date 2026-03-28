program RecordTest;
{ Records — structured data on Z80 }

type
  Point = record
    X: Byte;
    Y: Byte;
  end;

var
  P1, P2: Point;

function Manhattan(A, B: Point): Byte;
var
  DX, DY: Byte;
begin
  if A.X > B.X then DX := A.X - B.X
  else DX := B.X - A.X;
  if A.Y > B.Y then DY := A.Y - B.Y
  else DY := B.Y - A.Y;
  Manhattan := DX + DY;
end;

function PointSum(P: Point): Byte;
begin
  PointSum := P.X + P.Y;
end;

begin
  P1.X := 3;
  P1.Y := 4;
  WriteLn('Point sum:');
  WriteLn(PointSum(P1));
end.
