const Parser = require('tree-sitter');
const MinZ = require('./bindings/node');
const fs = require('fs');

// Create parser
const parser = new Parser();
parser.setLanguage(MinZ.Language || MinZ);

// Read test file
const sourceCode = fs.readFileSync('test.minz', 'utf8');

// Parse the code
const tree = parser.parse(sourceCode);

// 1. S-expression format (default tree-sitter format)
console.log("=== S-EXPRESSION FORMAT ===");
console.log(tree.rootNode.toString());

// 2. JSON format with detailed node information
function nodeToJSON(node) {
  const result = {
    type: node.type,
    startPosition: node.startPosition,
    endPosition: node.endPosition,
    startIndex: node.startIndex,
    endIndex: node.endIndex,
    text: node.text.substring(0, 50) + (node.text.length > 50 ? '...' : ''),
  };

  if (node.childCount > 0) {
    result.children = [];
    for (let i = 0; i < node.childCount; i++) {
      result.children.push(nodeToJSON(node.child(i)));
    }
  }

  if (node.isNamed) {
    result.named = true;
  }

  if (node.isMissing) {
    result.missing = true;
  }

  if (node.hasError) {
    result.error = true;
  }

  return result;
}

console.log("\n=== JSON FORMAT ===");
console.log(JSON.stringify(nodeToJSON(tree.rootNode), null, 2));

// 3. Check for errors
console.log("\n=== PARSING ERRORS ===");
function findErrors(node, errors = []) {
  if (node.type === 'ERROR' || node.isMissing) {
    errors.push({
      type: node.type,
      position: `${node.startPosition.row}:${node.startPosition.column}`,
      text: node.text
    });
  }
  
  for (let i = 0; i < node.childCount; i++) {
    findErrors(node.child(i), errors);
  }
  
  return errors;
}

const errors = findErrors(tree.rootNode);
console.log(`Found ${errors.length} errors:`);
errors.forEach(err => console.log(`  - ${err.position}: ${err.type} "${err.text.substring(0, 20)}..."`));

// 4. Export to JSON file
fs.writeFileSync('ast.json', JSON.stringify(nodeToJSON(tree.rootNode), null, 2));
console.log("\n✓ AST exported to ast.json");

// 5. Simple statistics
function getStats(node, stats = {types: {}, depth: 0}) {
  stats.types[node.type] = (stats.types[node.type] || 0) + 1;
  
  let maxDepth = stats.depth;
  for (let i = 0; i < node.childCount; i++) {
    const childStats = getStats(node.child(i), {types: {}, depth: stats.depth + 1});
    Object.keys(childStats.types).forEach(type => {
      stats.types[type] = (stats.types[type] || 0) + childStats.types[type];
    });
    maxDepth = Math.max(maxDepth, childStats.maxDepth || childStats.depth);
  }
  
  return {...stats, maxDepth};
}

console.log("\n=== AST STATISTICS ===");
const stats = getStats(tree.rootNode);
console.log(`Max depth: ${stats.maxDepth}`);
console.log(`Node types: ${Object.keys(stats.types).length}`);
console.log("Most common nodes:");
Object.entries(stats.types)
  .sort((a, b) => b[1] - a[1])
  .slice(0, 10)
  .forEach(([type, count]) => console.log(`  ${type}: ${count}`));