"""Explicit sector/BDOS failure and disk-publication contracts for the fourth stage."""
from copy import deepcopy

def trd_cases(musical,positive):
    single=next(c for c in musical if c['name']=='equal_delays_second_is_first_inserted');rows=[]
    def add(name,**kw):
        c=deepcopy(single);c.update(name=name);c.update(kw);rows.append(c);return c
    for call in range(1,12):
        fault=900 if call in [1,2,4,5] else 904 if call in [3,6] else 902 if call==7 else 903
        add('sector_failure_'+str(call),fail_call=call,fault=fault,disk_error=2,input_opens=0 if call<=2 else 1 if call<=5 else 2,published=call>=10,data_writes=0 if call<=7 else 1)
    for part in ['notes','arrange']:
        for control,error,fault in [('missing_input',7,900),('duplicate_input',6,900),('bad_source_extent',4,900),('source_length_under',0,951),('source_length_over',0,957),('source_length_exceeds_extent',4,900)]:
            opens=2 if part=='arrange' else 1
            if fault==900:opens=1 if part=='arrange' and control in ['missing_input','duplicate_input'] else 0
            add(part+'_'+control,disk_control=part+':'+control,disk_error=error,fault=fault,input_opens=opens,data_writes=0)
    for control,error,fault,opens in [
        ('existing_output',5,900,0),('bad_geometry',3,900,0),('bad_directory_count',3,900,0),('directory_count_mismatch',8,900,0),('free_overlap',8,900,0),
        ('no_space',9,901,2),('directory_full',9,901,2),('last_sector',0,0,2),('full_disk',9,901,2),('bad_full_sentinel',3,900,0),
        ('directory_slot_15',0,0,2),('directory_slot_16',0,0,2),('directory_slot_127',0,0,2)]:
        add(control,disk_control=control,disk_error=error,fault=fault,input_opens=opens,published=control=='existing_output',data_writes=1 if fault==0 else 0)
    chronos=next(c for c in musical if c['name']=='native_chronos')
    for control,fault,writes in [('space_exact',0,215),('space_one_short',902,214)]:
        c=deepcopy(chronos);c.update(name='chronos_'+control,disk_control=control,fault=fault,disk_error=0,data_writes=writes);rows.append(c)
    ops=next(r for r in positive['echo']['cases'] if r['engine']=='scorpion-z80')['operations']
    data=[op for op in ops if op['op']==6 and op['position']>=16];assert len(data)==215
    for index in [0,107,214]:
        c=deepcopy(chronos);c.update(name='chronos_data_failure_'+str(index),fail_call=data[index]['call'],fault=902,disk_error=2,data_writes=index);rows.append(c)
    for op in ops[-4:]:
        c=deepcopy(chronos);c.update(name='chronos_publication_failure_'+str(op['call']),fail_call=op['call'],fault=903,disk_error=2,data_writes=215,published=op['position']==8);rows.append(c)
    return rows

def cpm_cases(musical):
    single=next(c for c in musical if c['name']=='equal_delays_second_is_first_inserted');rows=[]
    for name,fn,occ,fault,opens,exists,records,complete in [
        ('notes_open_failure',15,1,900,0,False,0,False),('notes_header_failure',20,1,904,1,False,0,False),('notes_close_failure',16,1,906,1,False,0,False),
        ('arrange_open_failure',15,2,900,1,False,0,False),('arrange_header_failure',20,3,904,2,False,0,False),('create_failure',22,1,901,2,False,0,False),
        ('data_write_failure',21,1,902,2,True,0,False),('arrange_close_failure',16,2,906,2,True,1,True),('output_close_failure',16,3,903,2,True,1,True)]:
        c=deepcopy(single);c.update(name=name,os_fault=dict(function=fn,occurrence=occ),fault=fault,input_opens=opens,output_exists=exists,written_records=records,complete_output=complete);rows.append(c)
    import base64
    c=deepcopy(single);c.update(name='preexisting_output',existing_output=base64.b64encode(bytes([167])*128).decode(),fault=901,input_opens=2,output_exists=True,written_records=1);rows.append(c)
    chronos=next(c for c in musical if c['name']=='native_chronos')
    for name,fn,occ,fault,opens,exists,writes in [
        ('notes_middle_read',20,41,904,1,False,0),('arrange_middle_read',20,200,904,2,True,None),
        ('write_first',21,1,902,2,True,0),('write_middle',21,215,902,2,True,214),('write_last',21,429,902,2,True,428)]:
        c=deepcopy(chronos);c.update(name='chronos_'+name,os_fault=dict(function=fn,occurrence=occ),fault=fault,input_opens=opens,output_exists=exists)
        if writes is not None:c['written_records']=writes
        rows.append(c)
    return rows
