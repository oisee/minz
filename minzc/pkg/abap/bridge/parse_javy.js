// ABAP parser for Javy/QuickJS — stdin/stdout via WASI.
// Bundled with @abaplint/core via esbuild.

// Read stdin using WASI-compatible approach
function readStdin() {
  // Javy v8: readSync(fd, buffer) — read into provided Uint8Array
  if (typeof Javy !== "undefined" && Javy.IO && Javy.IO.readSync) {
    const chunks = [];
    const buf = new Uint8Array(65536);
    while (true) {
      const n = Javy.IO.readSync(0, buf);
      if (n === 0) break;
      chunks.push(buf.slice(0, n));
    }
    const total = chunks.reduce((s, c) => s + c.length, 0);
    const result = new Uint8Array(total);
    let off = 0;
    for (const c of chunks) { result.set(c, off); off += c.length; }
    return new TextDecoder().decode(result);
  }
  return "";
}

function writeStdout(s) {
  const encoded = new TextEncoder().encode(s);
  if (typeof Javy !== "undefined" && Javy.IO && Javy.IO.writeSync) {
    Javy.IO.writeSync(1, encoded);
    return;
  }
  if (typeof std !== "undefined" && std.out) {
    std.out.puts(s);
    return;
  }
  // Last resort
  console.log(s);
}

const src = readStdin();
if (!src) {
  writeStdout(JSON.stringify({ statements: [], errors: ["empty input"] }));
} else {
  const { MemoryFile, Registry } = require("@abaplint/core");

  const upper = src.trimStart().toUpperCase();
  let filename = "input.prog.abap";
  if (upper.startsWith("CLASS ")) filename = "input.clas.abap";
  else if (upper.startsWith("INTERFACE ")) filename = "input.intf.abap";

  const file = new MemoryFile(filename, src);
  const reg = new Registry();
  reg.addFile(file);
  reg.parse();

  let abapFile = null;
  for (const obj of reg.getObjects()) {
    const files = obj.getABAPFiles && obj.getABAPFiles();
    if (files && files.length > 0) { abapFile = files[0]; break; }
  }

  if (!abapFile) {
    writeStdout(JSON.stringify({ statements: [], errors: ["no ABAP file parsed"] }));
  } else {
    function n2j(node) {
      if (!node) return null;
      if (typeof node.getStr === "function" && typeof node.getChildren !== "function")
        return { type: "Token", str: node.getStr(), row: node.getRow(), col: node.getCol() };
      const name = (node.get && node.get() && node.get().constructor && node.get().constructor.name) || "Unknown";
      const tokens = [];
      if (typeof node.getTokens === "function")
        for (const tok of node.getTokens())
          tokens.push(typeof tok.getStr === "function"
            ? { str: tok.getStr(), row: tok.getRow(), col: tok.getCol() }
            : { str: String(tok), row: 0, col: 0 });
      const children = [];
      if (typeof node.getChildren === "function")
        for (const child of node.getChildren()) { const c = n2j(child); if (c) children.push(c); }
      return { type: name, tokens, children };
    }

    const structure = abapFile.getStructure();
    const result = structure
      ? { structure: n2j(structure), errors: [] }
      : { statements: abapFile.getStatements().map(s => n2j(s)), errors: [] };
    writeStdout(JSON.stringify(result));
  }
}
