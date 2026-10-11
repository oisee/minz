#!/usr/bin/env python3
"""Exact ordinary-ASM diagnostic wrapper; no string representation repair."""
from pathlib import Path
import hashlib,json,re,sys
root=Path(__file__).resolve().parent.parent
original=(root/'ordinary-0d6.a80').read_bytes()
assert hashlib.sha256(original).hexdigest()=='2459fdd8e4f699b3e1e48fe017d06024cbf62214b818512739657d82716194b3'
a=original.decode()
assert not re.search(r'^\s*(ORG|__cpm_smoke)',a,re.M)
assert a.count('\npeek:\n\n')==a.count('\npoke:\n\n')==1
assert '; fun peek(addr: u16 = HL) -> u8 = A' in a
assert '; fun poke(addr: u16 = HL, val: u8 = C)' in a
out_count=a.count('    OUT (0x23), A')
assert out_count==16
old='    XOR A\n    DI\n    HALT\n.main_trmp0:'
assert a.count(old)==1
# Only these four finite changes: extern bodies, known print opcodes, main exit.
a=a.replace('\npeek:\n\n','\npeek:\n    LD A, (HL)\n    RET\n\n')
a=a.replace('\npoke:\n\n','\npoke:\n    LD (HL), C\n    RET\n\n')
a=a.replace('    OUT (0x23), A','    CALL __cpm_smoke_putchar')
a=a.replace(old,'    XOR A\n    RET\n.main_trmp0:')
assert not re.search(r'^\s*OUT\b',a,re.M)
startup='''; Derived diagnostic CP/M COM: unchanged strings and ordinary print loops.
ORG 0x0100
__cpm_smoke_entry:
    LD (__cpm_smoke_saved_sp), SP
    LD SP, __cpm_smoke_stack_top
    CALL main
    LD SP, (__cpm_smoke_saved_sp)
    JP 0
__cpm_smoke_saved_sp:
    DW 0
'''
bridge='''
; CALL 5 function 2, preserve all six pairs around BDOS.
__cpm_smoke_putchar:
    PUSH AF
    PUSH BC
    PUSH DE
    PUSH HL
    PUSH IX
    PUSH IY
    LD E, A
    LD C, 2
    CALL 5
    POP IY
    POP IX
    POP HL
    POP DE
    POP BC
    POP AF
    RET
__cpm_smoke_code_end:
; Static assembler must verify code_end <= 6000 before trusting this section.
ORG 0x6000
__cpm_smoke_stack_low_canary:
    DW 0xA55A
__cpm_smoke_stack_low:
    DS 0x1000
__cpm_smoke_stack_top:
__cpm_smoke_stack_high_canary:
    DW 0x5AA5
__cpm_smoke_image_end:
'''
wrapped=(startup+a+bridge).encode()
(root/'diagnostic-wrapper.a80').write_bytes(wrapped)
report={'ordinary_sha256':hashlib.sha256(original).hexdigest(),'derived_assembly_sha256':hashlib.sha256(wrapped).hexdigest(),'print_out_replacements':out_count,'extern_bodies_supplied':['peek HL->A','poke HL,C->void'],'main_terminal_replacement':'exact XOR A; DI; HALT before .main_trmp0 -> XOR A; RET','string_pool_unchanged':True,'print_loop_instructions_unchanged_except_output_call':True,'origin':256,'stack_low_canary':24576,'stack_low':24578,'stack_top':28674,'stack_high_canary':28674,'node_arena':32768,'string_arena':36864,'source_buffer':49152,'source_capacity':4096,'modeled_bdos_boundary':57344,'execution_performed':False,'known_blocker':'SString/CString source mismatch and representation-blind @print remain diagnostic defects'}
(root/'wrapper-changes.json').write_text(json.dumps(report,indent=2)+'\n')
print(json.dumps(report,indent=2))
