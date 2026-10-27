"""Common chunk semantics, independently injected in each of the two inputs."""
from copy import deepcopy
from compose import globals_first

TINY='''
global contract_buffer:[u8;PHYSICAL]
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

def once(text,old,new):
    assert text.count(old)==1,old
    return text.replace(old,new,1)

def controls(source,fixtures,transport):
    single=next(c for c in fixtures if c['name']=='equal_delays_second_is_first_inserted')
    dense=next(c for c in fixtures if c['name']=='maximum_window_dense')
    physical=128 if transport=='cpm' else 256
    common='global contract_phase:u8\n'+once(source,'fun select_arrangement() {','fun select_arrangement() {\n    contract_phase = 1')
    rows=[]
    for phase in [0,1]:
        for name,result,fault in [('stalled',0,912),('eof_with_bytes',65537,911),('oversize_read',257,210),('unknown_status',196608,904)]:
            text=once(common,'fun read_chunk()->u32 {',f'fun read_chunk()->u32 {{\n    if contract_phase == {phase} {{ return u32({result}) }}')
            c=deepcopy(single);c.update(name=name+'_'+str(phase),fault=fault,input_opens=phase+1,output_exists=False,written_records=0)
            rows.append((c['name'],text,[c]))
        text='global contract_once:u8\n'+once(common,'fun read_chunk()->u32 {',f'fun read_chunk()->u32 {{\n    if contract_phase == {phase} && contract_once == 0 {{ contract_once = 1\n        return 0 }}')
        c=deepcopy(single);c['name']='empty_progress_'+str(phase);rows.append((c['name'],text,[c]))
        cap_controls=[('zero_capacity','chunk_capacity',physical,0),('oversize_capacity','chunk_capacity',physical,257),('oversize_padding_quantum','input_padding_quantum',128 if transport=='cpm' else 0,257)]
        if transport=='cpm':cap_controls += [('zero_padding_quantum','input_padding_quantum',128,0)]
        for name,var,old,new in cap_controls:
            text=once(common,f'    {var} = {old}\n',f'    {var} = {old}\n    if contract_phase == {phase} {{ {var} = {new} }}\n')
            c=deepcopy(single);c.update(name=name+'_'+str(phase),fault=910,input_opens=phase+1,output_exists=False,written_records=0)
            rows.append((c['name'],text,[c]))
    at=source.index('fun write_chunk(');end=source.index('\nfun ',at+1)
    write=once(source[at:end],'return u32(count)','return u32(count-1)')
    text=source[:at]+write+source[end:];c=deepcopy(dense);c.update(name='short_write',fault=902,input_opens=2,output_exists=True,written_records=1,data_writes=1)
    rows.append(('short_write',text,[c]))
    for size in [1,7]:
        text=once(source,'fun read_chunk()->u32 {','fun physical_read_chunk()->u32 {')+TINY.replace('PHYSICAL',str(physical)).replace('CHUNK',str(size))
        rows.append(('tiny_'+str(size),globals_first(text),[deepcopy(single),deepcopy(dense)]))
    return rows
