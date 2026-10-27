"""CP/M physical-record framing and immediate-make visibility contracts."""
from copy import deepcopy
import base64


def make_cases(trd):
    cases=[]
    for old in trd['cases']:
        if old.get('fail_call') or old.get('disk_control'):
            continue
        c=deepcopy(old)
        for key in ['disk','published','disk_error','data_writes']:c.pop(key,None)
        name=c['name'];data=base64.b64decode(c['wire'])
        if name=='trailing_zero':
            continue  # This byte is allowed inside CP/M final-record padding.
        if name.startswith('truncated_'):
            n=int(name.split('_')[1])
            c['fault']=951 if n==0 else 950 if n<4 else 952 if n==4 else 953 if n<12 else 955 if n in [12,18] else 958 if n<18 else 956
            c['output_exists']=n>=12
            c['written_records']=0
            # Deliberately short host record: OS fixture fills unread bytes1A.
        else:
            data+=b'\0'*((-len(data))%128)
            c['output_exists']=not (name.startswith('bad_') and name.split('bad_',1)[1] in ['magic','version','kind','width','flags','channels','reserved','repeat_0','repeat_33'])
        c['wire']=base64.b64encode(data).decode()
        if name.startswith('chain_'):
            c['cpm_file']='measured/'+name[len('chain_'):]+'/z80/EVENTS.BIN'
        cases.append(c)
    single=deepcopy(next(c for c in cases if c['name']=='single'))
    padding=deepcopy(single);padding['name']='nonzero_physical_padding'
    b=bytearray(base64.b64decode(padding['wire']));b[27]=1
    padding.update(wire=base64.b64encode(b).decode(),fault=957,output_exists=True,written_records=0)
    for key in ['lnote_count','ornament_count','ornament_bytes']:padding.pop(key,None)
    cases.append(padding)
    for name,fn,occ,fault,exists,writes,complete in [
        ('input_open_failure',15,1,900,False,0,False),
        ('header_read_failure',20,1,904,False,0,False),
        ('create_failure',22,1,901,False,0,False),
        ('data_write_failure',21,1,902,True,0,False),
        ('input_close_failure',16,1,906,True,1,True),
        ('output_close_failure',16,2,903,True,1,True),
    ]:
        c=deepcopy(single);c.update(name=name,os_fault=dict(function=fn,occurrence=occ),fault=fault,output_exists=exists,written_records=writes,complete_output=complete)
        if not complete:
            for key in ['lnote_count','ornament_count','ornament_bytes']:c.pop(key,None)
        cases.append(c)
    existing=deepcopy(single);existing.update(name='preexisting_output',existing_output=base64.b64encode(bytes([167])*128).decode(),fault=901,output_exists=True,written_records=1)
    for key in ['lnote_count','ornament_count','ornament_bytes']:existing.pop(key,None)
    cases.append(existing)
    chronos=deepcopy(next(c for c in cases if c['name']=='chain_chronos_full'))
    for fn,occ,fault,writes in [(20,2,904,None),(20,51,904,None),(21,1,902,0),(21,140,902,139),(21,280,902,279)]:
        c=deepcopy(chronos)
        c.update(name='chronos_os_'+str(fn)+'_'+str(occ),os_fault=dict(function=fn,occurrence=occ),fault=fault,output_exists=True)
        if writes is not None:c['written_records']=writes
        for key in ['lnote_count','ornament_count','ornament_bytes']:c.pop(key,None)
        cases.append(c)
    assert len({c['name'] for c in cases})==len(cases)
    return dict(cases=cases)
