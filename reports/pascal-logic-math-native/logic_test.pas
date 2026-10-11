program LogicTest;

{ Pure arithmetic — no comparison in return path }

function MulBy3(X: Byte): Byte;
begin
  MulBy3 := X * 3;
end;

function MulBy5(X: Byte): Byte;
begin
  MulBy5 := X * 5;
end;

function Halve(X: Byte): Byte;
begin
  Halve := X div 2;
end;

function Add3(A, B, C: Byte): Byte;
begin
  Add3 := A + B + C;
end;

function Diff(A, B: Byte): Byte;
begin
  Diff := A - B;
end;

begin
  assert MulBy3(0) = 0;
  assert MulBy3(1) = 3;
  assert MulBy3(10) = 30;
  assert MulBy3(85) = 255;
  assert MulBy5(0) = 0;
  assert MulBy5(1) = 5;
  assert MulBy5(10) = 50;
  assert MulBy5(51) = 255;
  assert Add3(1, 2, 3) = 6;
  assert Add3(0, 0, 0) = 0;
  assert Add3(100, 100, 55) = 255;
  assert Diff(10, 3) = 7;
  assert Diff(255, 255) = 0;
  assert Diff(100, 0) = 100;
end.
