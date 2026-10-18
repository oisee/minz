import fs from 'node:fs';
import crypto from 'node:crypto';
import transpiler from '@abaplint/transpiler';
import runtime from '@abaplint/runtime';
// Executes the unchanged complete report. No harness FORM call or input mutation.
// Records every report-level DATA object (scalars, flat structures, tables),
// SY-TABIX/SUBRC/INDEX and MemoryConsole output.
const src=fs.readFileSync(process.argv[2],'utf8');
const names=[];
let procedure=false;
for(const line of src.split('\n')){
  const t=line.trim().toUpperCase();
  if(/^(FORM|METHOD|FUNCTION)\s/.test(t))procedure=true;
  if(/^(ENDFORM|ENDMETHOD|ENDFUNCTION)\b/.test(t)){procedure=false;continue;}
  const m=line.match(/^\s*DATA\s+(\w+)/i);
  if(m&&!procedure)names.push(m[1].toLowerCase());
}
const out=await new transpiler.Transpiler({addCommonJS:false,ignoreSourceMap:true}).runRaw([{filename:'zref.prog.abap',contents:src}]);
if(out.objects.length!==1)throw new Error('expected one program');
const code=out.objects[0].chunk.getCode();
const AsyncFunction=Object.getPrototypeOf(async function(){}).constructor;
const transcript=new runtime.MemoryConsole();
const abap=new runtime.ABAP({console:transcript});
// The runtime's table statements address the instance through globalThis.
globalThis.abap=abap;
const live=await new AsyncFunction('abap',code+'\nreturn {'+names.map(n=>n+':'+n).join(',')+'};')(abap);
const plain=v=>{
  if(v instanceof runtime.types.Table)return v.array().map(plain);
  if(v instanceof runtime.types.Structure){const o={};for(const[k,c]of Object.entries(v.get()))o[k]=plain(c);return o;}
  return v.get();
};
const values={};
for(const n of names)values[n]=plain(live[n]);
const sy=abap.builtin.sy.get();
console.log(JSON.stringify({source_sha256:crypto.createHash('sha256').update(src).digest('hex'),provider:'@abaplint/transpiler + @abaplint/runtime 2.14.8',node:process.version,entry:'full report',values,sy:{tabix:sy.tabix.get(),subrc:sy.subrc.get(),index:sy.index.get()},output:transcript.get()},null,2));
