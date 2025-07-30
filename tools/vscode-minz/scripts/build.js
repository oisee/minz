#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

console.log('🚀 MinZ VSCode Extension Build Script');
console.log('=====================================');

// Read package.json
const packagePath = path.join(__dirname, '..', 'package.json');
const packageJson = JSON.parse(fs.readFileSync(packagePath, 'utf8'));

// Parse current version
const [major, minor, patch] = packageJson.version.split('.').map(Number);

// Increment patch version
const newVersion = `${major}.${minor}.${patch + 1}`;

// Update package.json
packageJson.version = newVersion;
fs.writeFileSync(packagePath, JSON.stringify(packageJson, null, 2) + '\n');

console.log(`📦 Version bumped from ${major}.${minor}.${patch} to ${newVersion}`);

// Compile TypeScript
console.log('🔨 Compiling TypeScript...');
try {
  execSync('npm run compile', { stdio: 'inherit', cwd: path.join(__dirname, '..') });
  console.log('✅ TypeScript compilation completed!');
} catch (error) {
  console.error('❌ TypeScript compilation failed:', error.message);
  process.exit(1);
}

// Package the extension
console.log('📦 Packaging VSCode extension...');
try {
  execSync('npm run package', { stdio: 'inherit', cwd: path.join(__dirname, '..') });
  console.log('✅ Extension packaged successfully!');
} catch (error) {
  console.error('❌ Extension packaging failed:', error.message);
  process.exit(1);
}

// Show completion message
console.log('');
console.log('🎉 Build completed successfully!');
console.log('================================');
console.log(`📁 Package: minz-language-support-${newVersion}.vsix`);
console.log('');
console.log('Next steps:');
console.log('  1. Install: make install-extension');
console.log('  2. Test with .minz files');
console.log('  3. Verify syntax highlighting and snippets');
console.log('');
console.log('🔧 Features included:');
console.log('  ✓ Modern "fun" syntax highlighting');
console.log('  ✓ "loop at" iterator syntax');
console.log('  ✓ Bitwise operators (~, &)');
console.log('  ✓ All MinZ keywords and operators');
console.log('  ✓ Code snippets and autocompletion');