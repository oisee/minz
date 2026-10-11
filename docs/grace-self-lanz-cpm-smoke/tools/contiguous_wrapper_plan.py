#!/usr/bin/env python3
"""Prepare a separate contiguous diagnostic; do not execute or repair literals."""
from pathlib import Path
import hashlib,json
root=Path(__file__).resolve().parent.parent
old=(root/'diagnostic-wrapper.a80').read_text()
old_stack='''ORG 0x6000
__cpm_smoke_stack_low_canary:
    DW 0xA55A
__cpm_smoke_stack_low:
    DS 0x1000
__cpm_smoke_stack_top:
__cpm_smoke_stack_high_canary:
    DW 0x5AA5
__cpm_smoke_image_end:
'''
assert old.count(old_stack)==1
new=old.replace(old_stack,'''__cpm_smoke_image_end:
__cpm_smoke_stack_low_canary EQU 0x6000
__cpm_smoke_stack_low EQU 0x6002
__cpm_smoke_stack_top EQU 0x7002
__cpm_smoke_stack_high_canary EQU 0x7002
''')
new=new.replace('    LD SP, __cpm_smoke_stack_top\n', '''    ; Refuse a BDOS boundary below the complete source-buffer extent.
    LD HL, (6)
    LD DE, 0xD000
    OR A
    SBC HL, DE
    JP C, __cpm_smoke_refuse
    LD HL, 0xA55A
    LD (__cpm_smoke_stack_low_canary), HL
    LD HL, 0x5AA5
    LD (__cpm_smoke_stack_high_canary), HL
    LD SP, __cpm_smoke_stack_top
''')
new=new.replace('__cpm_smoke_saved_sp:\n', '__cpm_smoke_refuse:\n    JP 0\n__cpm_smoke_saved_sp:\n',1)
assert new.count('ORG ')==1
(root/'contiguous-diagnostic-wrapper.a80').write_text(new)
(root/'CONTIGUOUS-PLAN.md').write_text('''# Contiguous CP/M diagnostic\n\nThis static variant preserves the old ordinary assembly and original literals. It is not a runnable faithful main demonstration: the known source CString assumption remains unresolved. No execution has occurred.\n\nA single ORG 0100 emits only code/data. Stack addresses are EQU constants; startup initializes both canaries in RAM. Before any arena or stack writes, the BDOS pointer at 0006 must cover D000, the end of the source region. The final reviewed runnable wrapper will also check incoming SP against all owned regions and check canaries after main.\n\nAssembly review must verify every listing byte equals COM[address-0100], COM size equals image_end-0100, and code_end remains below 6000. The historical multi-ORG image remains preserved as a rejected mapping diagnostic.\n''')
print(hashlib.sha256(new.encode()).hexdigest())
