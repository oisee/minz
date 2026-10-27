"""Read-only fixture extraction from already verified third-stage native disks."""
from pathlib import Path
import hashlib,json,struct
from corpus import config,channels_from_wire,base_rows,paired_notes_from_wire

def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()

def read_trd_file(path,name):
    image=path.read_bytes();assert len(image)==655360
    matches=[at for at in range(0,2048,16) if image[at:at+9]==name]
    assert len(matches)==1,(path,name)
    at=matches[0];length=struct.unpack_from('<H',image,at+11)[0];sectors=image[at+13];position=image[at+15]*16+image[at+14]
    assert length<=sectors*256 and 16<=position and position+sectors<=2560
    chunks=[]
    for pos in range(position,position+sectors):
        track,sector=divmod(pos,16);physical=((track%80*2+track//80)*16+sector)*256
        chunks.append(image[physical:physical+256])
    return b''.join(chunks)[:length]

def native_cases(upstream):
    proof=json.loads((upstream/'trd/receipt.json').read_text());rows=[r for r in proof['arrange']['cases'] if r['engine']=='scorpion-z80' and r['case'].startswith('chain_')]
    assert len(rows)==20;cases=[]
    for r in rows:
        name=r['case'];short=name.removeprefix('chain_');stage=Path('trd/measured')/name/'scorpion-z80';disk=stage/'disk.trd'
        assert r['fault']==0 and sha(upstream/disk)==r['disk_sha256']
        original=(upstream/stage/'arrange.bin').read_bytes();notes=read_trd_file(upstream/disk,b'NOTES   N')
        assert original==read_trd_file(upstream/disk,b'ARRANGE A') and hashlib.sha256(original).hexdigest()==r['wire_sha256']
        cpm_arrange=Path('cpm/measured')/name/'z80/ARRANGE.BIN'
        cpm_notes=Path('cpm/measured')/short/'z80/NOTES.BIN'
        for path,logical in [(cpm_arrange,original),(Path('upstream/previous/producer')/cpm_notes,notes)]:
            expected=logical+b'\0'*((-len(logical))%128)
            assert (upstream/path).read_bytes()==expected,(name,path)
        cfg=config(modes=[109,112,109])
        cases.append(dict(name='native_chronos' if short=='chronos_full' else name,channels=channels_from_wire(original),config=cfg,base_rows=base_rows(notes,cfg),native_disk=str(disk),cpm_arrange=str(cpm_arrange),cpm_notes=str(cpm_notes),native_wire=original,native_notes=notes,paired_notes=paired_notes_from_wire(notes)))
    return cases
