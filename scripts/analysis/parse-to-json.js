#!/usr/bin/env node

const Parser = require('tree-sitter');
const MinZ = require('./build/Release/tree_sitter_minz_binding');
const fs = require('fs');

if (process.argv.length < 3) {
    console.error('Usage: parse-to-json.js <filename>');
    process.exit(1);
}

const filename = process.argv[2];
const sourceCode = fs.readFileSync(filename, 'utf8');

const parser = new Parser();
parser.setLanguage(MinZ);

const tree = parser.parse(sourceCode);

// Convert tree to JSON
function nodeToJSON(node) {
    const result = {
        type: node.type,
        text: node.text,
        startPosition: {
            row: node.startPosition.row,
            column: node.startPosition.column
        },
        endPosition: {
            row: node.endPosition.row,
            column: node.endPosition.column
        }
    };
    
    if (node.childCount > 0) {
        result.children = [];
        for (let i = 0; i < node.childCount; i++) {
            result.children.push(nodeToJSON(node.child(i)));
        }
    }
    
    if (node.fields) {
        result.fields = {};
        for (const fieldName of Object.keys(node.fields)) {
            const fieldNode = node[fieldName];
            if (fieldNode) {
                result.fields[fieldName] = nodeToJSON(fieldNode);
            }
        }
    }
    
    return result;
}

const jsonAST = {
    rootNode: nodeToJSON(tree.rootNode)
};

console.log(JSON.stringify(jsonAST, null, 2));