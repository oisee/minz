program Factorial;
var
  N, Result: Integer;

function Fact(X: Integer): Integer;
begin
  if X <= 1 then
    Fact := 1
  else
    Fact := X * Fact(X - 1);
end;

begin
  Result := Fact(7);
  WriteLn(Result);
end.
