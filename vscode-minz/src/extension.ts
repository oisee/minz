import * as vscode from 'vscode';
import * as path from 'path';
import * as fs from 'fs';
import { exec, spawn } from 'child_process';

let outputChannel: vscode.OutputChannel;

export function activate(context: vscode.ExtensionContext) {
    outputChannel = vscode.window.createOutputChannel('MinZ');
    
    // Register commands
    context.subscriptions.push(
        vscode.commands.registerCommand('minz.compile', () => compileMinZ()),
        vscode.commands.registerCommand('minz.compileToIR', () => compileToIR()),
        vscode.commands.registerCommand('minz.compileOptimized', () => compileOptimized()),
        vscode.commands.registerCommand('minz.showAST', () => showAST())
    );

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

export function deactivate() {
    if (outputChannel) {
        outputChannel.dispose();
    }
}

async function compileMinZ() {
    const activeEditor = vscode.window.activeTextEditor;
    if (!activeEditor || activeEditor.document.languageId !== 'minz') {
        vscode.window.showErrorMessage('No MinZ file is currently open');
        return;
    }

    const filePath = activeEditor.document.fileName;
    const config = vscode.workspace.getConfiguration('minz');
    const compilerPath = config.get<string>('compilerPath', 'minzc');
    const outputDir = config.get<string>('outputDirectory', './build');
    const enableOptimizations = config.get<boolean>('enableOptimizations', true);
    
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
    
    exec(`${compilerPath} ${args.join(' ')}`, { cwd: workingDir }, (error, stdout, stderr) => {
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
    const compilerPath = config.get<string>('compilerPath', 'minzc');
    const outputDir = config.get<string>('outputDirectory', './build');
    
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
    
    exec(`${compilerPath} ${args.join(' ')}`, { cwd: workingDir }, (error, stdout, stderr) => {
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
    const compilerPath = config.get<string>('compilerPath', 'minzc');
    const outputDir = config.get<string>('outputDirectory', './build');
    const enableSMC = config.get<boolean>('enableSMC', false);
    
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
    
    exec(`${compilerPath} ${args.join(' ')}`, { cwd: workingDir }, (error, stdout, stderr) => {
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
    const config = vscode.workspace.getConfiguration('minz');
    const compilerPath = config.get<string>('compilerPath', 'minzc');
    
    await activeEditor.document.save();
    
    outputChannel.clear();
    outputChannel.show();
    outputChannel.appendLine(`Generating AST for ${filePath}...`);

    const workspaceFolder = vscode.workspace.getWorkspaceFolder(activeEditor.document.uri);
    const workingDir = workspaceFolder?.uri.fsPath || path.dirname(filePath);
    
    // Use minzc compiler to show AST (if it supports --ast flag)
    exec(`${compilerPath} --ast "${filePath}"`, { cwd: workingDir }, (error, stdout, stderr) => {
        if (error) {
            // Fallback message if compiler doesn't support AST generation
            outputChannel.appendLine(`Note: AST generation is not available in the current MinZ compiler`);
            outputChannel.appendLine(`To view AST, use tree-sitter parse command directly`);
            vscode.window.showWarningMessage('AST generation not available in current MinZ compiler');
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