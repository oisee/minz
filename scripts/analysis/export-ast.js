const { execSync } = require('child_process');
const fs = require('fs');

// Run tree-sitter parse and capture output
let output;
try {
  output = execSync('npx tree-sitter parse test.minz 2>/dev/null', { encoding: 'utf8' });
} catch (e) {
  // tree-sitter returns non-zero exit code even on successful parse with errors
  output = e.stdout || '';
}

// Parse S-expression to JSON
function parseSExp(sexp) {
  let pos = 0;
  
  function skipWhitespace() {
    while (pos < sexp.length && /\s/.test(sexp[pos])) pos++;
  }
  
  function parseNode() {
    skipWhitespace();
    
    if (pos >= sexp.length) return null;
    
    if (sexp[pos] === '(') {
      pos++; // skip '('
      skipWhitespace();
      
      // Parse node type
      let nodeType = '';
      while (pos < sexp.length && !/[\s\(\)\[]/.test(sexp[pos])) {
        nodeType += sexp[pos++];
      }
      
      const node = { type: nodeType };
      
      // Parse position if present
      skipWhitespace();
      if (sexp[pos] === '[') {
        const posMatch = sexp.slice(pos).match(/^\[(\d+), (\d+)\] - \[(\d+), (\d+)\]/);
        if (posMatch) {
          node.start = { line: parseInt(posMatch[1]), column: parseInt(posMatch[2]) };
          node.end = { line: parseInt(posMatch[3]), column: parseInt(posMatch[4]) };
          pos += posMatch[0].length;
        }
      }
      
      // Parse children and fields
      const children = [];
      const fields = {};
      
      skipWhitespace();
      while (pos < sexp.length && sexp[pos] !== ')') {
        // Check for field
        const fieldMatch = sexp.slice(pos).match(/^(\w+):/);
        if (fieldMatch) {
          const fieldName = fieldMatch[1];
          pos += fieldMatch[0].length;
          skipWhitespace();
          const fieldValue = parseNode();
          fields[fieldName] = fieldValue;
        } else {
          const child = parseNode();
          if (child) children.push(child);
        }
        skipWhitespace();
      }
      
      if (children.length > 0) node.children = children;
      if (Object.keys(fields).length > 0) node.fields = fields;
      
      pos++; // skip ')'
      return node;
    } else {
      // Parse literal or identifier
      let value = '';
      if (sexp[pos] === '"') {
        // String literal
        pos++; // skip opening quote
        while (pos < sexp.length && sexp[pos] !== '"') {
          if (sexp[pos] === '\\' && pos + 1 < sexp.length) {
            value += sexp[pos++];
          }
          value += sexp[pos++];
        }
        pos++; // skip closing quote
        return { type: 'string', value };
      } else {
        // Other token
        while (pos < sexp.length && !/[\s\(\)]/.test(sexp[pos])) {
          value += sexp[pos++];
        }
        return { type: 'token', value };
      }
    }
  }
  
  return parseNode();
}

const ast = parseSExp(output);

// Save full AST
fs.writeFileSync('ast-full.json', JSON.stringify(ast, null, 2));

// Create simplified AST without position info
function simplifyAST(node) {
  if (!node) return null;
  
  const simple = { type: node.type };
  
  if (node.children) {
    simple.children = node.children.map(simplifyAST).filter(Boolean);
  }
  
  if (node.fields) {
    simple.fields = {};
    for (const [key, value] of Object.entries(node.fields)) {
      simple.fields[key] = simplifyAST(value);
    }
  }
  
  if (node.value !== undefined) {
    simple.value = node.value;
  }
  
  return simple;
}

const simpleAST = simplifyAST(ast);
fs.writeFileSync('ast-simple.json', JSON.stringify(simpleAST, null, 2));

// Statistics
function collectStats(node, stats = { nodeTypes: {}, maxDepth: 0, totalNodes: 0 }, depth = 0) {
  if (!node) return stats;
  
  stats.totalNodes++;
  stats.nodeTypes[node.type] = (stats.nodeTypes[node.type] || 0) + 1;
  stats.maxDepth = Math.max(stats.maxDepth, depth);
  
  if (node.children) {
    for (const child of node.children) {
      collectStats(child, stats, depth + 1);
    }
  }
  
  if (node.fields) {
    for (const field of Object.values(node.fields)) {
      collectStats(field, stats, depth + 1);
    }
  }
  
  return stats;
}

const stats = collectStats(ast);

console.log('AST Export Summary:');
console.log('==================');
console.log(`Total nodes: ${stats.totalNodes}`);
console.log(`Max depth: ${stats.maxDepth}`);
console.log(`Unique node types: ${Object.keys(stats.nodeTypes).length}`);
console.log('\nMost common node types:');
Object.entries(stats.nodeTypes)
  .sort((a, b) => b[1] - a[1])
  .slice(0, 10)
  .forEach(([type, count]) => {
    console.log(`  ${type}: ${count}`);
  });

console.log('\nFiles created:');
console.log('  - ast-full.json (with position info)');
console.log('  - ast-simple.json (structure only)');

// Check for errors
const errors = [];
function findErrors(node, path = '') {
  if (!node) return;
  
  if (node.type === 'ERROR' || node.type === 'MISSING') {
    errors.push({
      type: node.type,
      path,
      start: node.start,
      end: node.end
    });
  }
  
  if (node.children) {
    node.children.forEach((child, i) => {
      findErrors(child, `${path}/[${i}]`);
    });
  }
  
  if (node.fields) {
    Object.entries(node.fields).forEach(([key, value]) => {
      findErrors(value, `${path}/${key}`);
    });
  }
}

findErrors(ast);

if (errors.length > 0) {
  console.log(`\nFound ${errors.length} parse errors:`);
  errors.slice(0, 5).forEach(err => {
    console.log(`  - ${err.type} at ${err.start ? `${err.start.line}:${err.start.column}` : 'unknown'}`);
  });
  if (errors.length > 5) {
    console.log(`  ... and ${errors.length - 5} more`);
  }
}