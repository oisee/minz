// Standalone ABAP parser — reads from stdin, writes JSON AST to stdout.
// Designed for bundling into a single file / Wasm module.

import { MemoryFile, Registry } from "@abaplint/core";

// Read all stdin
const chunks = [];
for await (const chunk of process.stdin) {
  chunks.push(chunk);
}
const src = Buffer.concat(chunks).toString("utf-8");

// Auto-detect filename pattern for abaplint
const upper = src.trimStart().toUpperCase();
let filename = "input.prog.abap";
if (upper.startsWith("CLASS ")) filename = "input.clas.abap";
else if (upper.startsWith("INTERFACE ")) filename = "input.intf.abap";

// Parse
const file = new MemoryFile(filename, src);
const reg = new Registry();
reg.addFile(file);
await reg.parseAsync();

let abapFile = null;
for (const obj of reg.getObjects()) {
  const files = obj.getABAPFiles?.();
  if (files && files.length > 0) { abapFile = files[0]; break; }
}

if (!abapFile) {
  process.stdout.write(JSON.stringify({ statements: [], errors: ["no ABAP file parsed"] }));
  process.exit(0);
}

// Walk AST → JSON
function nodeToJSON(node) {
  if (!node) return null;
  if (typeof node.getStr === "function" && typeof node.getChildren !== "function") {
    return { type: "Token", str: node.getStr(), row: node.getRow(), col: node.getCol() };
  }
  const name = node.get?.()?.constructor?.name || node.constructor?.name || "Unknown";
  const tokens = [];
  if (typeof node.getTokens === "function") {
    for (const tok of node.getTokens()) {
      tokens.push(typeof tok.getStr === "function"
        ? { str: tok.getStr(), row: tok.getRow(), col: tok.getCol() }
        : { str: String(tok), row: 0, col: 0 });
    }
  }
  const children = [];
  if (typeof node.getChildren === "function") {
    for (const child of node.getChildren()) {
      const c = nodeToJSON(child);
      if (c) children.push(c);
    }
  }
  return { type: name, tokens, children };
}

const structure = abapFile.getStructure();
const result = structure
  ? { structure: nodeToJSON(structure), errors: [] }
  : { statements: abapFile.getStatements().map(s => nodeToJSON(s)), errors: [] };

process.stdout.write(JSON.stringify(result));
