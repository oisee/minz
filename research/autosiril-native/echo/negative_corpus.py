"""Explicit invalid-frame/resource contracts, independent of the target decoder."""
from copy import deepcopy
import base64,struct
from build_matrix import notes_wire
from encode_reference import cell_payload

def frame(kind,width,metadata,records):
    return b'ASIR'+bytes([1,kind,width,0])+struct.pack('<HH',metadata,0)+b''.join(b'\1'+r for r in records)+b'\xff'+struct.pack('<I',len(records))+b'END!'

def definitions(values):
    records=[]
    for ident,data in enumerate(values):
        for at in range(0,len(data),8):
            p=bytearray(16);p[0]=1;struct.pack_into('<HHH',p,1,ident,len(data),at);piece=data[at:at+8];p[7]=len(piece);p[8:8+len(piece)]=piece;records.append(bytes(p))
    return records

def output_without_cells(records,extent):
    records=list(records)
    for ch in range(3):
        p=bytearray(16);p[0]=2;p[1]=ch;struct.pack_into('<I',p,2,extent);records.append(bytes(p))
    return frame(4,16,3,records)

def cases(musical):
    base=deepcopy(next(c for c in musical if c['name']=='equal_delays_second_is_first_inserted'))
    rows=[]
    def add(name,field,data,fault,cpm_fault=None,**kw):
        c=deepcopy(base);c.update(name=name,fault=fault,input_opens=1 if field=='notes' else 2)
        c.pop('disk',None);c[field]=base64.b64encode(data).decode()
        # Header rejection happens before creating output; all payload/END errors
        # happen after make, but this small fixture has no full output record.
        c.update(output_exists=field!='notes' and fault not in [950,951,952,953],written_records=0)
        if cpm_fault is not None:c['cpm_fault']=cpm_fault
        c.update(kw);rows.append(c);return c
    for field in ['notes','wire']:
        original=base64.b64decode(base[field])
        for name,at,value,fault in [('magic',0,0,950),('version',4,2,952),('kind',5,9,953),('width',6,17,953),('flags',7,1,953),('reserved_low',10,1,953),('reserved_high',11,1,953),('tag',12,2,955),('count',len(original)-8,255,956),('end_magic',len(original)-1,0,956)]:
            data=bytearray(original);data[at]=value;add(field+'_'+name,field,data,fault)
        add(field+'_truncated_magic',field,original[:1],951,950)
        add(field+'_nonzero_tail',field,original+b'\1',957)
        add(field+'_extra_physical_record',field,original+b'\0'*128,957)
    # Source count / contents / order, before any output is made.
    original=bytearray(base64.b64decode(base['notes']))
    data=original.copy();struct.pack_into('<H',data,8,65);add('notes_source_count_65','notes',data,954)
    data=original.copy();data[17]=128;add('notes_pitch_128','notes',data,958)
    data=original.copy();data[18]=1;add('notes_source_outside_metadata','notes',data,958)
    add('notes_off_before_start','notes',frame(1,6,1,[struct.pack('<HHBB',2,1,60,0)]),105)
    add('notes_source_order','notes',frame(1,6,1,[struct.pack('<HHBB',2,2,60,0),struct.pack('<HHBB',1,3,60,0)]),106)
    for count in [2046,2047]:
        records=[struct.pack('<HHBB',i,i,60,0) for i in range(count)]
        c=add('notes_capacity_'+str(count),'notes',frame(1,6,1,records),0 if count==2046 else 959)
        c['wire']=base64.b64encode(frame(3,16,3,definitions([[0,0]]))).decode()
        c['expected']=base64.b64encode(output_without_cells(definitions([[0,0]]),count-1+7)).decode()
        if count==2046:c.update(input_opens=2,output_exists=True,written_records=1,counts=[0]*3,extents=[count-1+7]*3)
    # Every mutable LNote field whose domain is constrained by the producer.
    original=base64.b64decode(base['wire']);cell_at=30 # header12 + definition17 + tag1
    assert original[cell_at]==0
    for name,offset,value,fault in [('role',0,2,985),('channel',1,3,958),('pitch',4,128,958),('volume',9,1,984),('type',11,0,958),('kind',12,120,958),('reserved',15,1,958),('non_envelope_form',6,1,984),('non_envelope_note',13,0,984)]:
        data=bytearray(original);data[cell_at+offset]=value;add('lnote_'+name,'wire',data,fault)
    data=bytearray(original);struct.pack_into('<H',data,cell_at+2,20);add('lnote_beyond_companion','wire',data,986)
    data=bytearray(original);data[cell_at+12]=112;struct.pack_into('<H',data,cell_at+7,1)
    c=add('lnote_undefined_poly_ornament','wire',data,985);c['config']['modes']=[112,109,109]
    records=[original[at+1:at+17] for at in range(12,len(original)-9,17)]
    add('lnote_duplicate_row','wire',frame(3,16,3,records+[records[1]]),970)
    # Definition headers, canonical fragments, expanded order and zero padding.
    for name,offset,value in [('id',1,1),('zero_length',3,0),('offset',5,1),('count',7,1),('first_nonzero',8,1),('unused_nonzero',10,1)]:
        data=bytearray(original);data[13+offset]=value;add('definition_'+name,'wire',data,985)
    add('missing_definition','wire',frame(3,16,3,[]),985)
    rec=bytearray(definitions([[0]*9])[0]);add('unfinished_definition','wire',frame(3,16,3,[rec]),985)
    defs=definitions([[0]]+[list(range(64))+[63+i] for i in range(1,64)])
    assert sum([1]+[65]*63)==4096
    for extra in [0,1]:
        values=[[0]]+[list(range(64))+[63+i] for i in range(1,64)]
        if extra:values[-1].append(127)
        records=definitions(values);wire=frame(3,16,3,records)
        c=add('definition_bytes_'+str(4096+extra),'wire',wire,985 if extra else 0,written_records=None)
        c['expected']=base64.b64encode(output_without_cells(records,26)).decode()
        if not extra:c.update(counts=[0]*3,extents=[26]*3)
    values=[[0,0]]+[[0,0,i,i] for i in range(1,65)]
    records=definitions(values);c=add('definition_count_65','wire',frame(3,16,3,records),985,written_records=None)
    c['expected']=base64.b64encode(output_without_cells(records,26)).decode()
    for name,row,first,second in [('first_row_overflow',65535,1,0),('second_row_overflow',65535,0,1),('wide_row_overflow',65504,32,32)]:
        p=bytearray(records[0]);p=cell_payload(0,dict(row=row,note=dict(note=60,sample=2,envelope=0,ornament=0,volume=1500,type='s',kind='m',enote=-1)))
        c=add(name,'wire',frame(3,16,3,definitions([[0,0]])+[p]),980)
        c['notes']=base64.b64encode(notes_wire(65535)).decode();c['config'].update(first=first,second=second,wide=[name.startswith('wide'),False,False])
    # Target rejects input #1025, without conflating delayed output with input cap.
    c=deepcopy(next(c for c in musical if c['name']=='nominal_input_limit'));wire=base64.b64decode(c['wire']);records=[wire[at+1:at+17] for at in range(12,len(wire)-9,17)]
    p=bytearray(records[-1]);struct.pack_into('<H',p,2,3072);records.append(bytes(p))
    c.update(name='lnote_input_1025',wire=base64.b64encode(frame(3,16,3,records)).decode(),notes=base64.b64encode(notes_wire(3072)).decode(),fault=200,input_opens=2,output_exists=True)
    rows.append(c)
    for field in ['notes','wire']:
        original=bytearray(base64.b64decode(base[field]));original[-6]=1
        add(field+'_end_high_count',field,original,956)
    for name,field,value,fault in [('arrange_channel_count_0','wire',0,953),('arrange_channel_count_4','wire',4,953),('notes_no_declared_sources','notes',0,958)]:
        original=bytearray(base64.b64decode(base[field]));struct.pack_into('<H',original,8,value);add(name,field,original,fault)
    from corpus import channels_from_wire
    env=deepcopy(channels_from_wire(base64.b64decode(next(c for c in musical if c['name']=='native_chronos')['wire']))[0][0]);env['row']=0
    for name,offset,value in [('sample',5,3),('form',6,0),('enote',13,0)]:
        p=bytearray(cell_payload(0,env));p[offset]=value;add('envelope_'+name,'wire',frame(3,16,3,definitions([[0,0]])+[p]),984)
    for name,values in [('repeat_33',[[0]*33]),('repeat_mismatch',[[0,0],[0,0,1]]),('decreasing_block',[[0,0],[0,0,2,1]]),('nonincreasing_step',[[0,0],[0,0,0,0]])]:
        add('definition_'+name,'wire',frame(3,16,3,definitions(values)),985)
    for name,key,value in [('delay_one_33','first',33),('delay_two_33','second',33),('unknown_source_mode','modes',[120,109,109])]:
        c=deepcopy(base);c.update(name=name,fault=976,input_opens=0,output_exists=False,written_records=0);c['config'][key]=value;rows.append(c)
    assert len({c['name'] for c in rows})==len(rows)
    return rows
