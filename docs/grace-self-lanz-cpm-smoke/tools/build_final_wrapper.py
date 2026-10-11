#!/usr/bin/env python3
"""Finite CP/M adapter derivation from freshly compiled explicit-CString source; no CPU."""
from pathlib import Path
import hashlib,json,re
p=Path(__file__).resolve().parent.parent;d=p/'final-e518';src=p/'derived/self_lanz_parser_cstring.nanz'
assert hashlib.sha256(src.read_bytes()).hexdigest()=='574cef717965dbe6fcf6937ae35935b7b7e26f2051e49e7862d1826cff76f42d'
a=(d/'ordinary.a80').read_text();old=(p/'derived/guarded-contiguous-wrapper.a80').read_text()
start=old[:old.index('; ── compilation summary')];bridge=old[old.index('\n__cpm_smoke_putchar:'):]
assert not re.search(r'^\s*(ORG|__cpm_smoke)',a,re.M)
assert a.count('\npeek:\n\n')==a.count('\npoke:\n\n')==1
assert '; fun peek(addr: u16 = HL) -> u8 = A' in a and '; fun poke(addr: u16 = HL, val: u8 = C)' in a
terminal='    XOR A\n    DI\n    HALT\n.main_trmp0:';assert a.count(terminal)==1
n=a.count('    OUT (0x23), A');assert n>0
wrapped=start+a.replace('\npeek:\n\n','\npeek:\n    LD A, (HL)\n    RET\n\n').replace('\npoke:\n\n','\npoke:\n    LD (HL), C\n    RET\n\n').replace('    OUT (0x23), A','    CALL __cpm_smoke_putchar').replace(terminal,'    XOR A\n    RET\n.main_trmp0:')+bridge
assert len(re.findall(r'^\s*ORG\b',wrapped,re.M))==1 and not re.search(r'^\s*OUT\b',wrapped,re.M)
(d/'assembled.a80').write_text(wrapped)
report={'source_sha256':hashlib.sha256(src.read_bytes()).hexdigest(),'ordinary_assembly_sha256':hashlib.sha256(a.encode()).hexdigest(),'assembled_sha256':hashlib.sha256(wrapped.encode()).hexdigest(),'print_out_replacements':n,'extern_contracts':['peek HL -> A','poke HL,C -> void'],'main_exit':'exact known XOR A/DI/HALT block before main_trmp0 -> XOR A/RET','startup_and_bridge':'unchanged guarded single-ORG startup and six-pair BDOS2 bridge','execution_performed':False}
(d/'wrapper-identity.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))
