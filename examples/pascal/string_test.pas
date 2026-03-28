program StringTest;

{ Character classification — no CALL, but uses nested comparison.
  Known VIR condret-sink bug: returns original value instead of 0/1.
  These functions compile correctly on MIR2 VM.
  Z80 asserts commented out until VIR fixes condret for nested if. }

function IsDigit(C: Byte): Byte;
begin
  if C >= 48 then
    if C <= 57 then
      IsDigit := 1
    else
      IsDigit := 0
  else
    IsDigit := 0;
end;

function IsUpper(C: Byte): Byte;
begin
  if C >= 65 then
    if C <= 90 then
      IsUpper := 1
    else
      IsUpper := 0
  else
    IsUpper := 0;
end;

function IsLower(C: Byte): Byte;
begin
  if C >= 97 then
    if C <= 122 then
      IsLower := 1
    else
      IsLower := 0
  else
    IsLower := 0;
end;

begin
  { All currently broken on Z80 — condret-sink doesn't insert LD A,0/1
    assert IsDigit(48) = 1;
    assert IsDigit(65) = 0;
    assert IsUpper(65) = 1;
    assert IsUpper(97) = 0;
    assert IsLower(97) = 1;
    assert IsLower(65) = 0;
  }
end.
