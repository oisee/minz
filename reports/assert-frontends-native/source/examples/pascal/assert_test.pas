program AssertTest;

function Double(X: Integer): Integer;
begin
  Double := X + X;
end;

function AddOne(X: Byte): Byte;
begin
  AddOne := X + 1;
end;

begin
  assert Double(21) = 42;
  assert AddOne(5) = 6;
end.
