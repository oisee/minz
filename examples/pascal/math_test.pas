program MathTest;

{ --- Pure arithmetic --- }

function Double(X: Integer): Integer;
begin
  Double := X + X;
end;

function AddOne(X: Byte): Byte;
begin
  AddOne := X + 1;
end;

function Square(X: Byte): Byte;
begin
  Square := X * X;
end;

function Triple(X: Byte): Byte;
begin
  Triple := X + X + X;
end;

{ --- Comparison-based --- }

function Max(A, B: Byte): Byte;
begin
  if A > B then
    Max := A
  else
    Max := B;
end;

function Min(A, B: Byte): Byte;
begin
  if A < B then
    Min := A
  else
    Min := B;
end;

function IsEven(X: Byte): Byte;
begin
  if X mod 2 = 0 then
    IsEven := 1
  else
    IsEven := 0;
end;

function AbsDiff(A, B: Byte): Byte;
begin
  if A > B then
    AbsDiff := A - B
  else
    AbsDiff := B - A;
end;

begin
  { Pure arithmetic }
  assert Double(0) = 0;
  assert Double(21) = 42;
  assert Double(100) = 200;
  assert AddOne(0) = 1;
  assert AddOne(5) = 6;
  assert AddOne(254) = 255;
  assert Square(0) = 0;
  assert Square(1) = 1;
  assert Square(5) = 25;
  assert Square(10) = 100;
  assert Triple(0) = 0;
  assert Triple(3) = 9;
  assert Triple(10) = 30;

  { Comparison — previously broken by TermCondRet bug }
  assert Max(3, 7) = 7;
  assert Max(10, 2) = 10;
  assert Max(5, 5) = 5;
  assert Min(3, 7) = 3;
  assert Min(10, 2) = 2;
  assert IsEven(0) = 1;
  assert IsEven(4) = 1;
  assert IsEven(7) = 0;
  assert AbsDiff(10, 3) = 7;
  assert AbsDiff(3, 10) = 7;
  assert AbsDiff(5, 5) = 0;
end.
