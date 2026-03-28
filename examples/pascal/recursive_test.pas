program RecursiveTest;

{ --- Non-recursive helpers (safe, no CALL issues) --- }

function Inc2(X: Byte): Byte;
begin
  Inc2 := X + 2;
end;

function Dec2(X: Byte): Byte;
begin
  Dec2 := X - 2;
end;

function Negate(X: Byte): Byte;
begin
  Negate := 0 - X;
end;

function Identity(X: Byte): Byte;
begin
  Identity := X;
end;

function Zero(X: Byte): Byte;
begin
  Zero := 0;
end;

function IsPositive(X: Byte): Byte;
begin
  if X > 0 then
    IsPositive := 1
  else
    IsPositive := 0;
end;

{ Recursive — known Z80 bug (VIR CALL clobber), mir2 only }
function Fact(N: Byte): Byte;
begin
  if N <= 1 then
    Fact := 1
  else
    Fact := N * Fact(N - 1);
end;

function SumTo(N: Byte): Byte;
begin
  if N = 0 then
    SumTo := 0
  else
    SumTo := N + SumTo(N - 1);
end;

begin
  { Simple helpers — work on Z80 }
  assert Identity(0) = 0;
  assert Identity(42) = 42;
  assert Identity(255) = 255;
  assert Zero(42) = 0;
  assert Zero(0) = 0;
  assert Inc2(0) = 2;
  assert Inc2(10) = 12;
  assert Dec2(10) = 8;
  assert Dec2(2) = 0;
  assert Negate(1) = 255;
  assert Negate(0) = 0;
  assert IsPositive(1) = 1;
  assert IsPositive(42) = 1;
  assert IsPositive(0) = 0;

  { Recursive — currently broken on Z80 (VIR CALL save/restore)
    assert Fact(0) = 1;
    assert Fact(1) = 1;
    assert Fact(5) = 120;
    assert SumTo(0) = 0;
    assert SumTo(10) = 55;
  }
end.
