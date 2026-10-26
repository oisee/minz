"""Offline article; generated listings are extracted verbatim from saved artifacts."""
from pathlib import Path
import hashlib, html, json, re
HERE=Path(__file__).resolve().parent
ART=HERE/'artifacts'

def function(text, name, assembly=False):
    marker=name+':' if assembly else 'fun @'+name+'('
    start=text.index(marker)
    end=text.find('\n; fun ' if assembly else '\nfun @', start)
    return text[start:end if end>=0 else None].rstrip()

asm=(ART/'meta-arena.a80').read_text()
hir=(ART/'meta-arena.hir').read_text()
mir=(ART/'meta-arena.mir2').read_text()
repeat=function(asm,'meta_repeat_u8_3_7',True)
reader=function(asm,'meta_layout_byte_8_7',True)
zero=function(asm,'meta_layout_byte_3_0',True)
entry=function(asm,'rec_meta_arena_memory_0',True)
count=lambda text:sum(bool(line.strip()) and not line.lstrip().startswith(';') and not line.rstrip().endswith(':') for line in text.splitlines())
source=(ART/'meta_arena_showcase.cr').read_text()
example=source[source.index('def meta_arena_memory'):source.index('\ndef meta_arena_copy')].rstrip()
old=(HERE.parent/'crystal-ecosystems-2026-10-25/template.html').read_text()
css=old.split('<style>',1)[1].split('</style>',1)[0]
values={
 'CSS':css,'SOURCE':example,'REPEAT_ASM':repeat,'READ_ASM':reader,'ZERO_ASM':zero,
 'ENTRY_ASM':entry,'READ_HIR':function(hir,'meta_layout_byte_8_7'),
 'READ_MIR':function(mir,'meta_layout_byte_8_7'),
 'REPEAT_COUNT':str(count(repeat)),'READ_COUNT':str(count(reader)),
 'ZERO_COUNT':str(count(zero)),'ENTRY_COUNT':str(count(entry)),
 'FRAME_DOWN':str(len(re.findall(r'^\s+DEC SP\s*$',entry,re.M))),
 'FRAME_UP':str(len(re.findall(r'^\s+INC SP.*$',entry,re.M))),
 'RING_RECEIPT':(ART/'iteration11-results.json').read_text(),
 'RING_FRAME_BYTES':str(next(r['bytes'] for r in json.loads((ART/'iteration11-layout.json').read_text())['layouts'] if r['name'].endswith('$frame'))),
 'BUFFER_BRIDGE_RECEIPT':(ART/'iteration10-results.json').read_text(),
 'BUFFER_CLEAR_ASM':function((ART/'byte-library.a80').read_text(),'ByteBuffer_clear',True),
 'BUFFER_PUSH_COUNT':str(count(function((ART/'byte-library.a80').read_text(),'ByteBuffer_try_push',True))),
 'BUFFER_PUSH_ASM':function((ART/'byte-library.a80').read_text(),'ByteBuffer_try_push',True),
 'BUFFER_CLEAR_COUNT':str(count(function((ART/'byte-library.a80').read_text(),'ByteBuffer_clear',True))),
 'BUFFER_RECEIPT':(ART/'iteration9-library-results.json').read_text(),
 'GEN_RECEIPT':(ART/'iteration8-results.json').read_text(),
 'GEN_BYTE_FRAME':str(next(r['bytes'] for r in json.loads((ART/'iteration8-layout.json').read_text())['layouts'] if r['name'].startswith('rec_byte_stream_'))),
 'GEN_RLE_FRAME':str(next(r['bytes'] for r in json.loads((ART/'iteration8-layout.json').read_text())['layouts'] if r['name'].startswith('rec_rle_stream_'))),
 'VIEW_RECEIPT':(ART/'iteration7-results.json').read_text(),
 'GO_RECEIPT':(ART/'iteration5-results.json').read_text(),
 'GO_SOURCE':'func fn_cr_three_iterations_2'+(ART/'scalar-meta-generated.go.txt').read_text().split('func fn_cr_three_iterations_2',1)[1].split('\n}',1)[0]+'\n}',
 'RECEIPT':(ART/'iteration4-results.json').read_text(),
}
manifest={'ring_bridge_commit':'b6951b11f','ring_library_commit':'afe7e685b','byte_bridge_commit':'802c61573','field_predicate_fix_commit':'f0b316b84','byte_library_commit':'0a0eb3d2c','assembly_experiment_commit':'cf4e395ff','go_probe_commit':'0503fd753','borrowed_view_commit':'7267b91bb','yield_bridge_commit':'be9c0121e','generator_fixes':['94cc4337b','2d7fa7904'],'target':'z80','register_allocator':'PBQP',
          'files':{p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(ART.iterdir()) if p.is_file()},
          'listing_instruction_counts':{name:count(value) for name,value in [('byte_buffer_clear',values['BUFFER_CLEAR_ASM']),('byte_buffer_try_push',values['BUFFER_PUSH_ASM']),('repeat',repeat),('reader_offset_7',reader),('reader_offset_0',zero),('arena_entry',entry)]}}
(HERE/'manifest.json').write_text(json.dumps(manifest,indent=2)+'\n')
values['MANIFEST']=json.dumps(manifest,indent=2)
text=(HERE/'template.html').read_text()
for key,value in values.items(): text=text.replace('@@'+key+'@@',value if key=='CSS' else html.escape(value))
assert not re.search(r'@@[A-Z_]+@@',text)
(HERE/'index.html').write_text(text)
