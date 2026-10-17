import fs from 'node:fs';
import crypto from 'node:crypto';
import transpiler from '@abaplint/transpiler';
import runtime from '@abaplint/runtime';
const src=fs.readFileSync(process.argv[2],'utf8');
const out=await new transpiler.Transpiler({addCommonJS:false,ignoreSourceMap:true}).runRaw([{filename:'zref.prog.abap',contents:src}]);
if(out.objects.length!==1)throw new Error('expected one program');
const code=out.objects[0].chunk.getCode();
const AsyncFunction=Object.getPrototypeOf(async function(){}).constructor;
// Execute the unchanged complete report. No harness FORM call or input mutation.
const transcript=new runtime.MemoryConsole();
const values=await new AsyncFunction('abap',code+'\nreturn {x:x.get(),y:y.get(),seen:seen.get()};')(new runtime.ABAP({console:transcript}));
console.log(JSON.stringify({source_sha256:crypto.createHash('sha256').update(src).digest('hex'),provider:'@abaplint/transpiler + @abaplint/runtime 2.14.8',node:process.version,entry:'full report',values,output:transcript.get()},null,2));
