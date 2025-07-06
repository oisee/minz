"use strict";
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || function (mod) {
    if (mod && mod.__esModule) return mod;
    var result = {};
    if (mod != null) for (var k in mod) if (k !== "default" && Object.prototype.hasOwnProperty.call(mod, k)) __createBinding(result, mod, k);
    __setModuleDefault(result, mod);
    return result;
};
Object.defineProperty(exports, "__esModule", { value: true });
exports.deactivate = exports.activate = void 0;
const vscode = __importStar(require("vscode"));
const path = __importStar(require("path"));
const fs = __importStar(require("fs"));
const child_process_1 = require("child_process");
let outputChannel;
function activate(context) {
    outputChannel = vscode.window.createOutputChannel('MinZ');
    // Register commands
    context.subscriptions.push(vscode.commands.registerCommand('minz.compile', () => compileMinZ()), vscode.commands.registerCommand('minz.compileToIR', () => compileToIR()), vscode.commands.registerCommand('minz.compileOptimized', () => compileOptimized()), vscode.commands.registerCommand('minz.showAST', () => showAST()));
    // Register language configuration
    vscode.languages.setLanguageConfiguration('minz', {
        comments: {
            lineComment: '//',
            blockComment: ['/*', '*/']
        },
        brackets: [
            ['{', '}'],
            ['[', ']'],
            ['(', ')']
        ],
        autoClosingPairs: [
            { open: '{', close: '}' },
            { open: '[', close: ']' },
            { open: '(', close: ')' },
            { open: '"', close: '"' },
            { open: "'", close: "'" }
        ]
    });
    outputChannel.appendLine('MinZ Language Support activated');
}
exports.activate = activate;
function deactivate() {
    if (outputChannel) {
        outputChannel.dispose();
    }
}
exports.deactivate = deactivate;
async function compileMinZ() {
    const activeEditor = vscode.window.activeTextEditor;
    if (!activeEditor || activeEditor.document.languageId !== 'minz') {
        vscode.window.showErrorMessage('No MinZ file is currently open');
        return;
    }
    const filePath = activeEditor.document.fileName;
    const config = vscode.workspace.getConfiguration('minz');
    const compilerPath = config.get('compilerPath', 'minzc');
    const outputDir = config.get('outputDirectory', './build');
    const enableOptimizations = config.get('enableOptimizations', true);
    // Save the file first
    await activeEditor.document.save();
    // Ensure output directory exists
    const workspaceFolder = vscode.workspace.getWorkspaceFolder(activeEditor.document.uri);
    if (workspaceFolder) {
        const fullOutputDir = path.join(workspaceFolder.uri.fsPath, outputDir);
        if (!fs.existsSync(fullOutputDir)) {
            fs.mkdirSync(fullOutputDir, { recursive: true });
        }
    }
    const fileName = path.basename(filePath, '.minz');
    const outputFile = path.join(outputDir, `${fileName}.a80`);
    let args = [filePath, '-o', outputFile];
    if (enableOptimizations) {
        args.push('-O');
    }
    outputChannel.clear();
    outputChannel.show();
    outputChannel.appendLine(`Compiling ${filePath} to Z80 assembly...`);
    outputChannel.appendLine(`Command: ${compilerPath} ${args.join(' ')}`);
    const workingDir = workspaceFolder?.uri.fsPath || path.dirname(filePath);
    (0, child_process_1.exec)(`${compilerPath} ${args.join(' ')}`, { cwd: workingDir }, (error, stdout, stderr) => {
        if (error) {
            outputChannel.appendLine(`Error: ${error.message}`);
            vscode.window.showErrorMessage(`Compilation failed: ${error.message}`);
            return;
        }
        if (stderr) {
            outputChannel.appendLine(`Warning: ${stderr}`);
        }
        if (stdout) {
            outputChannel.appendLine(stdout);
        }
        outputChannel.appendLine(`✓ Compilation successful! Output: ${outputFile}`);
        vscode.window.showInformationMessage(`MinZ compiled successfully to ${outputFile}`);
        // Open the generated assembly file
        const fullOutputPath = path.resolve(workingDir, outputFile);
        if (fs.existsSync(fullOutputPath)) {
            vscode.workspace.openTextDocument(fullOutputPath).then(doc => {
                vscode.window.showTextDocument(doc, vscode.ViewColumn.Beside);
            });
        }
    });
}
async function compileToIR() {
    const activeEditor = vscode.window.activeTextEditor;
    if (!activeEditor || activeEditor.document.languageId !== 'minz') {
        vscode.window.showErrorMessage('No MinZ file is currently open');
        return;
    }
    const filePath = activeEditor.document.fileName;
    const config = vscode.workspace.getConfiguration('minz');
    const compilerPath = config.get('compilerPath', 'minzc');
    const outputDir = config.get('outputDirectory', './build');
    await activeEditor.document.save();
    const workspaceFolder = vscode.workspace.getWorkspaceFolder(activeEditor.document.uri);
    if (workspaceFolder) {
        const fullOutputDir = path.join(workspaceFolder.uri.fsPath, outputDir);
        if (!fs.existsSync(fullOutputDir)) {
            fs.mkdirSync(fullOutputDir, { recursive: true });
        }
    }
    const fileName = path.basename(filePath, '.minz');
    const outputFile = path.join(outputDir, `${fileName}.ir`);
    const args = [filePath, '--emit-ir', '-o', outputFile];
    outputChannel.clear();
    outputChannel.show();
    outputChannel.appendLine(`Compiling ${filePath} to IR...`);
    outputChannel.appendLine(`Command: ${compilerPath} ${args.join(' ')}`);
    const workingDir = workspaceFolder?.uri.fsPath || path.dirname(filePath);
    (0, child_process_1.exec)(`${compilerPath} ${args.join(' ')}`, { cwd: workingDir }, (error, stdout, stderr) => {
        if (error) {
            outputChannel.appendLine(`Error: ${error.message}`);
            vscode.window.showErrorMessage(`IR compilation failed: ${error.message}`);
            return;
        }
        if (stderr) {
            outputChannel.appendLine(`Warning: ${stderr}`);
        }
        if (stdout) {
            outputChannel.appendLine(stdout);
        }
        outputChannel.appendLine(`✓ IR compilation successful! Output: ${outputFile}`);
        vscode.window.showInformationMessage(`MinZ compiled to IR: ${outputFile}`);
        // Open the generated IR file
        const fullOutputPath = path.resolve(workingDir, outputFile);
        if (fs.existsSync(fullOutputPath)) {
            vscode.workspace.openTextDocument(fullOutputPath).then(doc => {
                vscode.window.showTextDocument(doc, vscode.ViewColumn.Beside);
            });
        }
    });
}
async function compileOptimized() {
    const activeEditor = vscode.window.activeTextEditor;
    if (!activeEditor || activeEditor.document.languageId !== 'minz') {
        vscode.window.showErrorMessage('No MinZ file is currently open');
        return;
    }
    const filePath = activeEditor.document.fileName;
    const config = vscode.workspace.getConfiguration('minz');
    const compilerPath = config.get('compilerPath', 'minzc');
    const outputDir = config.get('outputDirectory', './build');
    const enableSMC = config.get('enableSMC', false);
    await activeEditor.document.save();
    const workspaceFolder = vscode.workspace.getWorkspaceFolder(activeEditor.document.uri);
    if (workspaceFolder) {
        const fullOutputDir = path.join(workspaceFolder.uri.fsPath, outputDir);
        if (!fs.existsSync(fullOutputDir)) {
            fs.mkdirSync(fullOutputDir, { recursive: true });
        }
    }
    const fileName = path.basename(filePath, '.minz');
    const outputFile = path.join(outputDir, `${fileName}_optimized.a80`);
    let args = [filePath, '-O', '-o', outputFile];
    if (enableSMC) {
        args.push('--enable-smc');
    }
    outputChannel.clear();
    outputChannel.show();
    outputChannel.appendLine(`Compiling ${filePath} with full optimizations...`);
    outputChannel.appendLine(`Command: ${compilerPath} ${args.join(' ')}`);
    const workingDir = workspaceFolder?.uri.fsPath || path.dirname(filePath);
    (0, child_process_1.exec)(`${compilerPath} ${args.join(' ')}`, { cwd: workingDir }, (error, stdout, stderr) => {
        if (error) {
            outputChannel.appendLine(`Error: ${error.message}`);
            vscode.window.showErrorMessage(`Optimized compilation failed: ${error.message}`);
            return;
        }
        if (stderr) {
            outputChannel.appendLine(`Warning: ${stderr}`);
        }
        if (stdout) {
            outputChannel.appendLine(stdout);
        }
        outputChannel.appendLine(`✓ Optimized compilation successful! Output: ${outputFile}`);
        vscode.window.showInformationMessage(`MinZ compiled with optimizations: ${outputFile}`);
        // Open the generated assembly file
        const fullOutputPath = path.resolve(workingDir, outputFile);
        if (fs.existsSync(fullOutputPath)) {
            vscode.workspace.openTextDocument(fullOutputPath).then(doc => {
                vscode.window.showTextDocument(doc, vscode.ViewColumn.Beside);
            });
        }
    });
}
async function showAST() {
    const activeEditor = vscode.window.activeTextEditor;
    if (!activeEditor || activeEditor.document.languageId !== 'minz') {
        vscode.window.showErrorMessage('No MinZ file is currently open');
        return;
    }
    const filePath = activeEditor.document.fileName;
    await activeEditor.document.save();
    outputChannel.clear();
    outputChannel.show();
    outputChannel.appendLine(`Generating AST for ${filePath}...`);
    const workspaceFolder = vscode.workspace.getWorkspaceFolder(activeEditor.document.uri);
    const workingDir = workspaceFolder?.uri.fsPath || path.dirname(filePath);
    // Use tree-sitter to parse and show AST
    (0, child_process_1.exec)(`npx tree-sitter parse "${filePath}"`, { cwd: workingDir }, (error, stdout, stderr) => {
        if (error) {
            outputChannel.appendLine(`Error: ${error.message}`);
            vscode.window.showErrorMessage(`AST generation failed: ${error.message}`);
            return;
        }
        if (stderr) {
            outputChannel.appendLine(`Warning: ${stderr}`);
        }
        if (stdout) {
            outputChannel.appendLine('=== Abstract Syntax Tree ===');
            outputChannel.appendLine(stdout);
        }
        vscode.window.showInformationMessage('AST generated successfully! Check the MinZ output channel.');
    });
}
//# sourceMappingURL=extension.js.map