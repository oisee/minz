program CaseTest;
var
  Ch: Byte;
  Done: Boolean;
begin
  Done := false;
  repeat
    Ch := 66; { 'B' }
    case Ch of
      65: WriteLn('Alpha');
      66: WriteLn('Beta');
      67: WriteLn('Gamma');
    else
      WriteLn('Other');
    end;
    Done := true;
  until Done;
end.
