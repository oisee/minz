program BubbleSort;
{ Classic bubble sort on Z80 — arrays, nested loops, swap }

const
  N = 10;

var
  Arr: array[0..9] of Byte;
  I, J, Temp: Byte;
  Swapped: Boolean;

{ Initialize array with descending values }
procedure InitArray;
begin
  Arr[0] := 50;
  Arr[1] := 30;
  Arr[2] := 80;
  Arr[3] := 10;
  Arr[4] := 60;
  Arr[5] := 40;
  Arr[6] := 90;
  Arr[7] := 20;
  Arr[8] := 70;
  Arr[9] := 100;
end;

{ Bubble sort: O(n^2) but simple and correct }
procedure Sort;
begin
  repeat
    Swapped := false;
    for I := 0 to N - 2 do
    begin
      if Arr[I] > Arr[I + 1] then
      begin
        Temp := Arr[I];
        Arr[I] := Arr[I + 1];
        Arr[I + 1] := Temp;
        Swapped := true;
      end;
    end;
  until not Swapped;
end;

{ Print array }
procedure PrintArray;
begin
  for I := 0 to N - 1 do
  begin
    Write(Arr[I]);
    Write(' ');
  end;
  WriteLn('');
end;

begin
  InitArray;
  WriteLn('Before:');
  PrintArray;
  Sort;
  WriteLn('After:');
  PrintArray;
end.
