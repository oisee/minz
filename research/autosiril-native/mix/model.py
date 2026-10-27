"""Planning-only bounded control state; input/output collections are host fixtures."""
from copy import deepcopy

def fold(earlier,incoming,policy,stats):
    assert policy in ['+','-']
    previous=-1
    for c in incoming:
        assert c['row']>previous;previous=c['row']
    source=iter(earlier);head=next(source,None)
    values=[None]*4;tags=[None]*4
    def put(row,note):
        slot=row%4
        assert tags[slot] is None or tags[slot]==row,'alias of a different live row'
        tags[slot]=row;values[slot]=deepcopy(note)
        stats['peak']=max(stats['peak'],sum(t is not None for t in tags))
    def drain(before=None):
        nonlocal head
        while True:
            pending=min((t for t in tags if t is not None),default=None)
            row=head['row'] if head is not None else None
            if pending is None and row is None:return
            assert pending is None or row is None or pending!=row,'earlier cell loaded twice'
            if row is not None and (pending is None or row<pending):
                if before is not None and row>=before:return
                cell=head;head=next(source,None);yield deepcopy(cell)
            else:
                if before is not None and pending>=before:return
                slot=pending%4;yield dict(row=pending,note=deepcopy(values[slot]));tags[slot]=None;values[slot]=None
    for cell in incoming:
        row,note=cell['row'],cell['note']
        yield from drain(row)
        while head is not None and head['row']<=row+3:
            assert head['row']>=row and tags[head['row']%4] is None
            put(head['row'],head['note']);head=next(source,None)
        def current(at):return values[at%4] if tags[at%4]==at else None
        old=current(row)
        if old is None or old['type']=='r' or old['volume']<note['volume']:
            put(row,note)
        elif policy=='-':
            # The reference's repeated current-row volume condition is false
            # after the first branch failed. Spill candidates use nil/release.
            for offset in range(1,4):
                target=current(row+offset)
                if target is None or target['type']=='r':put(row+offset,note);break
    yield from drain()
    assert all(t is None for t in tags)

def downmix(case):
    counts=case['config']['counts'];policies=case['config']['policies']
    assert len(counts)==len(policies)==3 and sum(counts)==3 and all(0<=c<=3 for c in counts)
    result=[];source=0;stats=dict(peak=0)
    for count in counts:
        mixed=[]
        for _ in range(count):
            mixed=list(fold(mixed,case['channels'][source],policies[source],stats));source+=1
        result.append(mixed)
    return dict(channels=result,lengths=[ch[-1]['row']+1 if ch else 0 for ch in result],peak=stats['peak'])
