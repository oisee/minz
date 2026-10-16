import core from '@abaplint/core';
import parserError from '@abaplint/core/build/src/rules/parser_error.js';
const {Registry,MemoryFile}=core;
const {ParserError}=parserError;
import {readFileSync} from 'node:fs';
const raw=readFileSync(process.argv[2],'utf8');
const reg=new Registry();reg.addFile(new MemoryFile('zminz.prog.abap',raw));reg.parse();
const obj=reg.getFirstObject();const file=obj.getABAPFiles()[0];
const token=t=>({str:t.getStr(),row:t.getRow(),col:t.getCol()});
function node(v){
 if(!v)return null;
 if(v.constructor.name==='TokenNode'){return {type:'Token',...token(v.get())}}
 const n={type:v.get?.()?.constructor?.name||v.constructor.name};
 const ts=v.getTokens?.()||v.getAllTokens?.()||[];
 if(ts.length)n.tokens=ts.map(token);
 const cs=v.getChildren?.()||[];
 if(cs.length)n.children=cs.map(node);
 return n;
}
const diagnostics=new ParserError().initialize(reg).run(obj).map(i=>({key:i.getKey(),message:i.getMessage(),filename:i.getFilename(),row:i.getStart().getRow(),col:i.getStart().getCol(),endRow:i.getEnd().getRow(),endCol:i.getEnd().getCol()}));
console.log(JSON.stringify({structure:node(file.getStructure()),statements:file.getStatements().map(node),diagnostics}));
