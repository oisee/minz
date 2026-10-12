package main

import (
 "fmt"
 "testing"
 "github.com/minz/minzc/pkg/spectrum"
)

// Exercise the actual registered provider devices, not a copied protocol model.
// Numeric/string results are independent frozen SQLite expectations.
func TestMZXSQLiteProviderPublicProtocol(t *testing.T){
 newPorts:=func()*spectrum.Ports{m,err:=spectrum.New48K(make([]byte,16384));if err!=nil{t.Fatal(err)};setupMZXSQLitePorts(m,false);return m.Ports}
 out:=func(p *spectrum.Ports,port,v byte){p.WritePort(uint16(v)<<8|uint16(port),v)}
 cmd:=func(p *spectrum.Ports,v byte)byte{out(p,0x41,v);return p.ReadPort(0xa543)}
 str:=func(p *spectrum.Ports,s string){for _,b:=range append([]byte(s),0){out(p,0x45,b)}}
 handle:=func(p *spectrum.Ports,h byte){out(p,0x45,h)}
 query:=func(p *spectrum.Ports,db byte,s string)byte{out(p,0x47,db);str(p,s);return cmd(p,3)}
 text:=func(p *spectrum.Ports,sh,col byte)string{handle(p,sh);out(p,0x47,col);cmd(p,5);var b []byte;for i:=0;i<256;i++{v:=p.ReadPort(0xc145);if v==0{return string(b)};b=append(b,v)};t.Fatal("unterminated bounded response");return ""}
 integer:=func(p *spectrum.Ports,sh,col byte)uint16{handle(p,sh);out(p,0x47,col);cmd(p,6);return uint16(p.ReadPort(0x7845))|uint16(p.ReadPort(0x9847))<<8}
 t.Run("handles_status_and_fresh_query",func(t *testing.T){p:=newPorts();str(p,":memory:");db:=cmd(p,1);if db!=1{t.Fatalf("open status43 got%d want1",db)}
  for _,s:=range []string{"CREATE TABLE t(name TEXT,n INTEGER)","INSERT INTO t VALUES('Alice',-32768),('Bob',65535)"}{out(p,0x47,db);str(p,s);if got:=cmd(p,8);got!=0{t.Fatal("exec",got,s)}}
  sh:=query(p,db,"SELECT name,n FROM t ORDER BY rowid");if sh!=1{t.Fatalf("query status43 got%d want1",sh)}
  for i,want:=range []struct{s string;n uint16}{{"Alice",32768},{"Bob",65535}}{handle(p,sh);if got:=cmd(p,4);got!=1{t.Fatal("row status",i,got)};if got:=text(p,sh,0);got!=want.s{t.Fatal("text",got,want.s)};if got:=integer(p,sh,1);got!=want.n{t.Fatal("int",got,want.n)}}
  handle(p,sh);if got:=cmd(p,4);got!=0{t.Fatal("done",got)};handle(p,sh);cmd(p,7)
  // Handle-only step/finalize must not prefix their bytes to the next SQL.
  second:=query(p,db,"SELECT 256");if second!=2{t.Fatal("second query status",second)};handle(p,second);if got:=cmd(p,4);got!=1{t.Fatal("second step",got)};if got:=integer(p,second,0);got!=256{t.Fatal("second int",got)};handle(p,second);cmd(p,7);handle(p,db);cmd(p,2)
  str(p,":memory:");if got:=cmd(p,1);got!=2{t.Fatal("reopen handle",got)};handle(p,2);cmd(p,2)
 })
 t.Run("errors_invalid_columns_and_integer_bounds",func(t *testing.T){p:=newPorts();str(p,":memory:");cmd(p,1)
  out(p,0x47,1);str(p,"NOT SQL");if got:=cmd(p,8);got!=1{t.Fatal("exec error",got)}
  if got:=query(p,1,"NOT SQL");got!=0{t.Fatal("query error must return handle0",got)}
  if got:=text(p,255,255);got!=""{t.Fatal("invalid text",got)};if got:=integer(p,255,255);got!=0{t.Fatal("invalid int",got)}
  for _,n:=range []int{0,1,255,256,32767,32768,65535,-1,-32768}{sh:=query(p,1,fmt.Sprintf("SELECT %d",n));if sh==0{t.Fatal("query",n)};handle(p,sh);if got:=cmd(p,4);got!=1{t.Fatal("step",n,got)};if got:=integer(p,sh,0);got!=uint16(n){t.Fatal("integer",n,got)};if got:=text(p,sh,255);got!=""{t.Fatal("invalid column",got)};if got:=integer(p,sh,255);got!=0{t.Fatal("invalid column int",got)};handle(p,sh);cmd(p,7)}
  handle(p,1);cmd(p,2)
 })
}
