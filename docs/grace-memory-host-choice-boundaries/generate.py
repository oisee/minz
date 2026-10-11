#!/usr/bin/env python3
import base64,copy,json
from pathlib import Path
p=Path(__file__).resolve().parent
b64=lambda b:base64.b64encode(b).decode()
word=lambda n:b64(n.to_bytes(2,"little"))
base={"schema":"minz.memory-host-workload/v1","target_model":"cpm-z80-v1","hosts":{"peek":"read-u8-at-u16","poke":"write-u8-at-u16"},"regions":{n:{"address":a,"bytes":b64(bytes(sz)),"writable":True} for n,a,sz in [("nodes",32768,4096),("strings",36864,8192),("source",49152,4096)]},"globals":{"str_bump":word(36864)},"calls":[],"layout":{"code_origin":1024,"stack_low":57344,"stack_high":61424,"return_pc":256},"max_steps":10000000,"max_memory":65536}
def call(fn,args,patches=None):
 x={"function":fn,"arguments":args}
 if patches:x["patches"]=patches
 return x
def patch(region,data,offset=0):return {"region":region,"offset":offset,"bytes":b64(data)}
work={}
w=copy.deepcopy(base);w["calls"]=[call("region_match",[36864,0,0]),call("intern_region",[0,0])];work["zero-body"]=w
for name,right in [("first-equal",97),("first-mismatch",98)]:
 w=copy.deepcopy(base);w["calls"]=[call("region_match",[36864,0,1],[patch("strings",b"\x00a"),patch("source",bytes([right]))]),call("intern_region",[0,0])];work[name]=w
w=copy.deepcopy(base);w["globals"]["str_bump"]=word(36866);w["calls"]=[call("region_match",[36864,0,1],[patch("strings",b"\x01a"),patch("source",b"a")]),call("intern_region",[0,1])];work["unobserved-append"]=w
w=copy.deepcopy(base);w["regions"]["source"]["bytes"]=b64(b"a");w["calls"]=[call("region_match",[36864,0,0]),call("intern_region",[0,2])];work["source-tail-denial"]=w
w=copy.deepcopy(base);w["regions"]["strings"]["bytes"]=b64(bytes(2));w["calls"]=[call("region_match",[36864,0,0]),call("intern_region",[0,2],[patch("source",b"ab")])];work["destination-tail-denial"]=w
w=copy.deepcopy(base);w["regions"]["strings"]["writable"]=False;w["calls"]=[call("region_match",[36864,0,0]),call("intern_region",[0,1])];work["read-only-header-denial"]=w
w=copy.deepcopy(work["zero-body"]);w["max_steps"]=1;work["instruction-budget"]=w
d=p/"workloads";d.mkdir(exist_ok=True)
for name,w in work.items():(d/(name+".json")).write_text(json.dumps(w,indent=2)+"\n")
print(list(work))
