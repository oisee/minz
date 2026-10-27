"""Chunk API contracts and directory success fixtures over duplex CP/M I/O."""
from copy import deepcopy

TINY='''
global contract_buffer:[u8;128]
global contract_at:u16
global contract_length:u16
fun read_chunk()->u32 {
    if contract_at == contract_length {
        let result:u32 = physical_read_chunk()
        if (result >> 16) != 0 { return result }
        contract_at = 0
        contract_length = u16(result)
        var index:u16 = 0
        while index < contract_length {
            contract_buffer[index] = input_chunk[index]
            index = index + 1
        }
    }
    var count:u16 = contract_length - contract_at
    if count > CHUNK { count = CHUNK }
    var index:u16 = 0
    while index < count {
        input_chunk[index] = contract_buffer[contract_at+index]
        index = index + 1
    }
    contract_at = contract_at + count
    return u32(count)
}
'''


def controls(text,fixtures):
    single=next(c for c in fixtures if c['name']=='single')
    drum=next(c for c in fixtures if c['name']=='all_drums_envelope_0')
    cases=[]
    for name,old,new,fault,created,writes in [
        ('empty_progress','fun read_chunk()->u32 {','global contract_calls:u8\nfun read_chunk()->u32 {\n    if contract_calls == 0 { contract_calls = 1\n        return 0 }',0,True,None),
        ('stalled','fun read_chunk()->u32 {','fun read_chunk()->u32 { return 0\n',912,False,0),
        ('eof_with_bytes','fun read_chunk()->u32 {','fun read_chunk()->u32 { return 65537\n',911,False,0),
        ('oversize_read','fun read_chunk()->u32 {','fun read_chunk()->u32 { return 257\n',210,False,0),
        ('unknown_status','fun read_chunk()->u32 {','fun read_chunk()->u32 { return 196608\n',904,False,0),
        ('short_write','return u32(count)','return u32(count-1)',902,True,1),
        ('zero_capacity','chunk_capacity = 128','chunk_capacity = 0',910,False,0),
        ('oversize_capacity','chunk_capacity = 128','chunk_capacity = 257',910,False,0),
        ('zero_padding_quantum','input_padding_quantum = 128','input_padding_quantum = 0',910,False,0),
        ('oversize_padding_quantum','input_padding_quantum = 128','input_padding_quantum = 257',910,False,0),
    ]:
        assert text.count(old)==1,old
        c=deepcopy(drum if name=='short_write' else single)
        c.update(name=name,fault=fault,output_exists=created)
        if fault:
            for key in ['lnote_count','ornament_count','ornament_bytes','cpm_file']:c.pop(key,None)
        if writes is not None:c['written_records']=writes
        cases.append((name,text.replace(old,new,1),[c]))
    # Include a multi-record case: output writes must not overwrite the saved
    # physical input record while it is being returned in tiny progress chunks.
    for size in [1,7]:
        name='tiny_'+str(size)
        altered=text.replace('fun read_chunk()->u32 {','fun physical_read_chunk()->u32 {',1)+TINY.replace('CHUNK',str(size))
        cases.append((name,altered,[deepcopy(single),deepcopy(drum)]))
    return cases
