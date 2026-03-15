program Sieve;
const
  Size = 100;
var
  I, K, Prime, Count: Integer;
  Flags: array[0..100] of Boolean;
begin
  Count := 0;
  for I := 0 to Size do
    Flags[I] := 1;
  for I := 0 to Size do
  begin
    if Flags[I] = 1 then
    begin
      Prime := I + I + 3;
      K := I + Prime;
      while K <= Size do
      begin
        Flags[K] := 0;
        K := K + Prime;
      end;
      Count := Count + 1;
    end;
  end;
  WriteLn(Count);
end.
