const { execSync } = require('child_process');

// Run tree-sitter and get S-expression output
const output = execSync('tree-sitter parse examples/test_three_vars.minz', { encoding: 'utf8' });

console.log('Tree-sitter S-expression output:');
console.log(output);
console.log('\n---\n');

// Now let's test what happens with --json flag
try {
    const jsonOutput = execSync('tree-sitter parse examples/test_three_vars.minz --json', { encoding: 'utf8' });
    console.log('Tree-sitter JSON output:');
    console.log(jsonOutput);
} catch (e) {
    console.log('JSON output error:', e.message);
}