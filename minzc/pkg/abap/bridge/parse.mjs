// parse.mjs — ABAP→JSON AST bridge using @abaplint/core
//
// Usage:  node parse.mjs <file.abap>
//         echo "WRITE 'hello'." | node parse.mjs --stdin
//
// Output: JSON to stdout with structure:
//   { "statements": [...], "errors": [...] }
//
// Each statement: { "type": "Write"|"Data"|"If"|..., "tokens": [...], "expressions": [...] }

import { MemoryFile, Registry, ABAPFile } from "@abaplint/core";
import { readFileSync } from "fs";

// ── Read source ──────────────────────────────────────────────────────────────

let src;
let filename = "input.abap";

if (process.argv.includes("--stdin")) {
  src = readFileSync(0, "utf-8");
} else if (process.argv[2]) {
  filename = process.argv[2];
  src = readFileSync(filename, "utf-8");
} else {
  console.error("Usage: node parse.mjs <file.abap>  or  --stdin");
  process.exit(1);
}

// abaplint requires filename pattern like "name.prog.abap" for REPORT programs,
// "name.clas.abap" for classes, etc.  If the filename doesn't match, wrap it.
const base = filename.replace(/^.*[\\/]/, "").replace(/\.abap$/i, "");
if (!filename.includes(".prog.") && !filename.includes(".clas.") && !filename.includes(".intf.")) {
  // Auto-detect: if source has REPORT → prog, CLASS → clas, INTERFACE → intf
  const upper = src.trimStart().toUpperCase();
  if (upper.startsWith("CLASS ")) {
    filename = base + ".clas.abap";
  } else if (upper.startsWith("INTERFACE ")) {
    filename = base + ".intf.abap";
  } else {
    filename = base + ".prog.abap";
  }
}

// ── Parse via abaplint ───────────────────────────────────────────────────────

const file = new MemoryFile(filename, src);
const reg = new Registry();
reg.addFile(file);
await reg.parseAsync();

// Try multiple ways to get the parsed ABAP file
let abapFile = null;
for (const obj of reg.getObjects()) {
  const files = obj.getABAPFiles?.();
  if (files && files.length > 0) {
    abapFile = files[0];
    break;
  }
}
// Fallback: try getABAPFiles directly on registry if available
if (!abapFile && reg.getABAPFiles) {
  const files = reg.getABAPFiles();
  if (files.length > 0) abapFile = files[0];
}
if (!abapFile) {
  // Debug: show what objects exist
  const objs = [];
  for (const obj of reg.getObjects()) {
    objs.push({ type: obj.getType(), name: obj.getName() });
  }
  console.log(JSON.stringify({ statements: [], errors: ["no ABAP file parsed", JSON.stringify(objs)] }));
  process.exit(0);
}

// ── Walk AST → JSON ─────────────────────────────────────────────────────────

function tokenToJSON(tok) {
  // Tokens may have getStr/getRow/getCol methods, or be plain objects
  if (typeof tok.getStr === "function") {
    return { str: tok.getStr(), row: tok.getRow(), col: tok.getCol() };
  }
  return { str: String(tok), row: 0, col: 0 };
}

function nodeToJSON(node) {
  if (!node) return null;

  // If it's a token node (has getStr but no getChildren)
  if (typeof node.getStr === "function" && typeof node.getChildren !== "function") {
    return { type: "Token", str: node.getStr(), row: node.getRow(), col: node.getCol() };
  }

  const name = node.get?.()?.constructor?.name || node.constructor?.name || "Unknown";

  // Collect all tokens from this node
  const tokens = [];
  if (typeof node.getTokens === "function") {
    for (const tok of node.getTokens()) {
      tokens.push(tokenToJSON(tok));
    }
  } else if (typeof node.getAllTokens === "function") {
    for (const tok of node.getAllTokens()) {
      tokens.push(tokenToJSON(tok));
    }
  }

  // Recurse into children
  const children = [];
  if (typeof node.getChildren === "function") {
    for (const child of node.getChildren()) {
      const c = nodeToJSON(child);
      if (c) children.push(c);
    }
  }

  return { type: name, tokens, children };
}

// Try structured output first (preserves nesting), fall back to flat statements
const structure = abapFile.getStructure();
let result;
if (structure) {
  result = { structure: nodeToJSON(structure), errors: [] };
} else {
  const stmts = abapFile.getStatements().map(s => nodeToJSON(s));
  result = { statements: stmts, errors: [] };
}

console.log(JSON.stringify(result, null, 2));
