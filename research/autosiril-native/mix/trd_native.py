"""Specialize the existing 16-bank Scorpion guard for five owned mix opens."""
def once(s,old,new):
    assert s.count(old)==1,old
    return s.replace(old,new,1)
def code(generated):
    old=(generated/'echo-vm.go').read_text()
    s=old[old.index('type sectorOperation struct'):]
    s=once(s,'linked:=placeArena(compilerAsm,module)','linked:=compilerAsm')
    s=once(s,'ORG 0x6000','ORG 0x4000')
    s=once(s,'var openWrites uint32','var leaseWrites [3]uint32\nvar leaseLive [3]bool')
    s=once(s,'map[string]int{"delay_one":2,"delay_two":2,"config_wide":3,"config_unvoiced":3,"source_modes":3}','map[string]int{"delay_one":2,"delay_two":2,"mix_counts_config":3,"mix_policies":3}')
    a=s.index(' now:=p.WriteCount[a.Symbols["input_open"]]');b=s.index('if m.CPU.SP() < minSP',a)
    s=s[:a]+''' for slot:=0;slot<3;slot++{
  at:=a.Symbols["input_live"]+slot;now:=p.WriteCount[at]
  if now!=leaseWrites[slot]{
   value:=m.Memory.Read(uint16(at));if value>1{panic("invalid input lease flag")}
   if value==1&&!leaseLive[slot]{opens++;leaseLive[slot]=true}
   if value==0&&leaseLive[slot]{closes++;leaseLive[slot]=false}
   leaseWrites[slot]=now
  }
 }
'''+s[b:]
    s=once(s,'check(d.Flush())','var lowerCanary [64]byte\nfor i:=range lowerCanary{lowerCanary[i]=m.Memory.Read(uint16(origin-64+i))}\ncheck(d.Flush())')
    s=once(s,'if m.Memory.Read(uint16(i)) != 0xa5 {','if (i<origin && m.Memory.Read(uint16(i))!=lowerCanary[i-(origin-64)]) || (i>=origin && m.Memory.Read(uint16(i))!=0xa5) {')
    s=once(s,'returned = m.CPU.HL()','opened=opens!=0;closed=opens==closes\nfor _,live:=range leaseLive{if live{panic("native owned cursor leaked")}}\nreturned = m.CPU.HL()')
    s=once(s,'for i:=0x47f3;i<0x4800;i++ {if m.Memory.Read(uint16(i))!=0xa5 {panic("fixed workspace tail canary")}}','')
    s=s.replace('steps >= 250000000','steps >= 1000000000')
    return 'type placedSpan struct{Address,Bytes int}\nvar fixedPlacement=map[string]placedSpan{}\n'+s
