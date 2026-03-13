import * as vscode from 'vscode';
import * as path from 'path';
import * as fs from 'fs';
import { exec } from 'child_process';

let outputChannel: vscode.OutputChannel;
let diagnosticCollection: vscode.DiagnosticCollection;
let lspClient: any; // LanguageClient instance (loaded dynamically)

export function activate(context: vscode.ExtensionContext) {
    outputChannel = vscode.window.createOutputChannel('MinZ');
    diagnosticCollection = vscode.languages.createDiagnosticCollection('minz');

    outputChannel.appendLine('MinZ Language Support activating...');
    outputChannel.appendLine(`Extension path: ${context.extensionPath}`);

    // Register commands
    context.subscriptions.push(
        vscode.commands.registerCommand('minz.compile', () => compileMinZ()),
        vscode.commands.registerCommand('minz.compileAndRun', () => compileAndRun()),
        vscode.commands.registerCommand('minz.compileToIR', () => compileToIR()),
        vscode.commands.registerCommand('minz.compileToMIR', () => compileToMIR()),
        vscode.commands.registerCommand('minz.compileToASM', () => compileToASM()),
        vscode.commands.registerCommand('minz.compileAll', () => compileAll()),
        vscode.commands.registerCommand('minz.compileOptimized', () => compileOptimized()),
        vscode.commands.registerCommand('minz.showAST', () => showAST()),
        vscode.commands.registerCommand('minz.debugBuild', () => debugBuild()),
        vscode.commands.registerCommand('minz.startDebugging', () => startDebugging()),
        vscode.commands.registerCommand('minz.compileNative', () => compileNative()),
        vscode.commands.registerCommand('minz.compileNativeC', () => compileNative('-c')),
        vscode.commands.registerCommand('minz.compileNativeQBE', () => compileNative('-q')),
        vscode.commands.registerCommand('minz.emitC', () => emitNativeIR('-emit-c')),
        vscode.commands.registerCommand('minz.emitQBE', () => emitNativeIR('-emit-qbe')),
        diagnosticCollection
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
            ['(', ')'],
            ['|', '|']
        ],
        autoClosingPairs: [
            { open: '{', close: '}' },
            { open: '[', close: ']' },
            { open: '(', close: ')' },
            { open: '"', close: '"' },
            { open: "'", close: "'" },
            { open: '|', close: '|' }
        ]
    });

    // Register debug configuration provider for DeZog
    context.subscriptions.push(
        vscode.debug.registerDebugConfigurationProvider('dezog', new MinZDebugConfigProvider())
    );

    // Compile on save for diagnostics
    vscode.workspace.onDidSaveTextDocument((document) => {
        const lang = document.languageId;
        const name = document.fileName;
        if (lang === 'minz' || lang === 'nanz' || lang === 'mir' ||
            name.endsWith('.minz') || name.endsWith('.mz') || name.endsWith('.nanz') || name.endsWith('.mir')) {
            compileForDiagnostics(document);
        }
    });

    // Try to start LSP server
    startLSP(context);

    outputChannel.appendLine('MinZ Language Support activated successfully');
}

export function deactivate() {
    if (lspClient) {
        lspClient.stop();
    }
    if (outputChannel) {
        outputChannel.dispose();
    }
    if (diagnosticCollection) {
        diagnosticCollection.dispose();
    }
}

// ---------- LSP Client ----------

async function startLSP(context: vscode.ExtensionContext) {
    const config = vscode.workspace.getConfiguration('minz');
    const lspPath = config.get<string>('lspPath', '');

    // Try to find mzlsp binary
    let lspBinary = lspPath;
    if (!lspBinary) {
        const compilerPath = config.get<string>('compilerPath', 'mz');
        const compilerDir = path.dirname(compilerPath);
        const candidates = [
            path.join(compilerDir, 'mzlsp'),
            'mzlsp',
        ];
        for (const candidate of candidates) {
            try {
                await new Promise<void>((resolve, reject) => {
                    exec(`which ${candidate}`, (err) => err ? reject(err) : resolve());
                });
                lspBinary = candidate;
                break;
            } catch {
                continue;
            }
        }
    }

    if (!lspBinary) {
        outputChannel.appendLine('LSP server (mzlsp) not found — diagnostics via compile-on-save');
        return;
    }

    try {
        // Dynamically load vscode-languageclient if available
        const { LanguageClient, TransportKind } = require('vscode-languageclient/node');

        const serverOptions = {
            command: lspBinary,
            args: [],
            transport: TransportKind.stdio,
        };

        const clientOptions = {
            documentSelector: [
                { scheme: 'file', language: 'minz' },
                { scheme: 'file', language: 'nanz' },
            ],
            outputChannel: outputChannel,
        };

        lspClient = new LanguageClient('minz', 'MinZ/Nanz Language Server', serverOptions, clientOptions);
        lspClient.start();
        outputChannel.appendLine(`LSP server started: ${lspBinary}`);
    } catch (e: any) {
        outputChannel.appendLine(`LSP client not available: ${e.message}`);
        outputChannel.appendLine('Install vscode-languageclient for full LSP support');
    }
}

// ---------- Diagnostics (compile-on-save fallback) ----------

function compileForDiagnostics(document: vscode.TextDocument) {
    // Skip if LSP is running (it handles diagnostics)
    if (lspClient) { return; }

    const config = vscode.workspace.getConfiguration('minz');
    const compilerPath = config.get<string>('compilerPath', 'mz');
    const filePath = document.fileName;

    const workspaceFolder = vscode.workspace.getWorkspaceFolder(document.uri);
    const workingDir = workspaceFolder?.uri.fsPath || path.dirname(filePath);

    // Compile with --check-only if available, otherwise compile to /dev/null
    exec(`${compilerPath} "${filePath}" -o /dev/null 2>&1`, { cwd: workingDir }, (error, stdout, stderr) => {
        const output = (stdout || '') + (stderr || '');
        const diagnostics = parseCompilerErrors(output, document);
        diagnosticCollection.set(document.uri, diagnostics);
    });
}

function parseCompilerErrors(output: string, document: vscode.TextDocument): vscode.Diagnostic[] {
    const diagnostics: vscode.Diagnostic[] = [];
    // Match: filename:line:col: error: message  OR  filename:line:col: warning: message
    const pattern = /([^:\s]+):(\d+):(\d+):\s*(error|warning|note):\s*(.+)/g;
    let match;

    while ((match = pattern.exec(output)) !== null) {
        const [, file, lineStr, colStr, severity, message] = match;
        const line = Math.max(0, parseInt(lineStr, 10) - 1);
        const col = Math.max(0, parseInt(colStr, 10) - 1);

        // Only show diagnostics for the current file
        if (!document.fileName.endsWith(file) && file !== path.basename(document.fileName)) {
            continue;
        }

        const range = new vscode.Range(line, col, line, col + 20);
        const diag = new vscode.Diagnostic(
            range,
            message,
            severity === 'error' ? vscode.DiagnosticSeverity.Error :
            severity === 'warning' ? vscode.DiagnosticSeverity.Warning :
            vscode.DiagnosticSeverity.Information
        );
        diag.source = 'minz';
        diagnostics.push(diag);
    }

    // Also match simpler format: "error: message" or "parse error: ..."
    const simplePattern = /^(?:parse |semantic )?(error):\s*(.+?)(?:\s+at\s+(\d+):(\d+))?$/gm;
    while ((match = simplePattern.exec(output)) !== null) {
        const [, , message, lineStr, colStr] = match;
        const line = lineStr ? Math.max(0, parseInt(lineStr, 10) - 1) : 0;
        const col = colStr ? Math.max(0, parseInt(colStr, 10) - 1) : 0;
        const range = new vscode.Range(line, col, line, col + 20);
        const diag = new vscode.Diagnostic(range, message, vscode.DiagnosticSeverity.Error);
        diag.source = 'minz';
        diagnostics.push(diag);
    }

    return diagnostics;
}

// ---------- Helper ----------

function getMinZContext(): { filePath: string; config: vscode.WorkspaceConfiguration; workingDir: string; compilerPath: string } | null {
    const activeEditor = vscode.window.activeTextEditor;
    if (!activeEditor) {
        vscode.window.showErrorMessage('No file is currently open');
        return null;
    }

    const name = activeEditor.document.fileName;
    const lang = activeEditor.document.languageId;
    const isMinzFile = name.endsWith('.minz') || name.endsWith('.mz') ||
                       name.endsWith('.nanz') || name.endsWith('.mir');
    if (!isMinzFile && lang !== 'minz' && lang !== 'nanz' && lang !== 'mir') {
        vscode.window.showErrorMessage('No MinZ, Nanz, or MIR file is currently open.');
        return null;
    }

    const config = vscode.workspace.getConfiguration('minz');
    const workspaceFolder = vscode.workspace.getWorkspaceFolder(activeEditor.document.uri);
    return {
        filePath: activeEditor.document.fileName,
        config,
        workingDir: workspaceFolder?.uri.fsPath || path.dirname(activeEditor.document.fileName),
        compilerPath: config.get<string>('compilerPath', 'mz'),
    };
}

async function ensureOutputDir(ctx: ReturnType<typeof getMinZContext>): Promise<string> {
    if (!ctx) { return './build'; }
    const outputDir = ctx.config.get<string>('outputDirectory', './build');
    const fullOutputDir = path.resolve(ctx.workingDir, outputDir);
    if (!fs.existsSync(fullOutputDir)) {
        fs.mkdirSync(fullOutputDir, { recursive: true });
    }
    return outputDir;
}

function runCompiler(ctx: ReturnType<typeof getMinZContext>, args: string[], label: string): Promise<{ success: boolean; stdout: string; stderr: string }> {
    return new Promise((resolve) => {
        if (!ctx) { resolve({ success: false, stdout: '', stderr: 'No context' }); return; }

        const cmd = `${ctx.compilerPath} ${args.join(' ')}`;
        outputChannel.clear();
        outputChannel.show();
        outputChannel.appendLine(`${label}: ${cmd}`);

        exec(cmd, { cwd: ctx.workingDir }, (error, stdout, stderr) => {
            if (stdout) { outputChannel.appendLine(stdout); }
            if (stderr) { outputChannel.appendLine(stderr); }

            // Parse errors for diagnostics
            const activeDoc = vscode.window.activeTextEditor?.document;
            if (activeDoc) {
                const diagnostics = parseCompilerErrors((stdout || '') + (stderr || ''), activeDoc);
                diagnosticCollection.set(activeDoc.uri, diagnostics);
            }

            if (error) {
                outputChannel.appendLine(`Error: ${error.message}`);
                resolve({ success: false, stdout: stdout || '', stderr: stderr || '' });
            } else {
                resolve({ success: true, stdout: stdout || '', stderr: stderr || '' });
            }
        });
    });
}

function openFileBeside(filePath: string) {
    if (fs.existsSync(filePath)) {
        vscode.workspace.openTextDocument(filePath).then(doc => {
            vscode.window.showTextDocument(doc, vscode.ViewColumn.Beside);
        });
    }
}

// ---------- Compile Commands ----------

async function compileMinZ() {
    const ctx = getMinZContext();
    if (!ctx) { return; }
    await vscode.window.activeTextEditor?.document.save();
    const outputDir = await ensureOutputDir(ctx);
    const fileName = path.basename(ctx.filePath, path.extname(ctx.filePath));
    const outputFile = path.join(outputDir, `${fileName}.a80`);
    const enableOpt = ctx.config.get<boolean>('enableOptimizations', true);

    const args = [`"${ctx.filePath}"`, '-o', `"${outputFile}"`];
    if (!enableOpt) { args.push('--disable-optimize'); }

    const result = await runCompiler(ctx, args, 'Compiling to Z80 assembly');
    if (result.success) {
        outputChannel.appendLine(`Compilation successful: ${outputFile}`);
        vscode.window.showInformationMessage(`MinZ compiled: ${outputFile}`);
        openFileBeside(path.resolve(ctx.workingDir, outputFile));
    } else {
        vscode.window.showErrorMessage('Compilation failed — see MinZ output');
    }
}

async function compileAndRun() {
    const ctx = getMinZContext();
    if (!ctx) { return; }
    await vscode.window.activeTextEditor?.document.save();
    const outputDir = await ensureOutputDir(ctx);
    const fileName = path.basename(ctx.filePath, path.extname(ctx.filePath));

    // Get target from settings (default: cpm)
    const target = ctx.config.get<string>('target', 'cpm');
    const binExt = target === 'cpm' ? '.com' : '.bin';

    const asmFile = path.join(outputDir, `${fileName}.a80`);
    const binFile = path.join(outputDir, `${fileName}${binExt}`);

    // Step 1: Compile MinZ → ASM
    const compileResult = await runCompiler(ctx, [`"${ctx.filePath}"`, '-t', target, '-o', `"${asmFile}"`], `Step 1: Compile (${target})`);
    if (!compileResult.success) {
        vscode.window.showErrorMessage('Compilation failed — see MinZ output');
        return;
    }

    // Step 2: Assemble ASM → binary
    const mzaPath = path.join(path.dirname(ctx.compilerPath), 'mza');
    const fullAsmFile = path.resolve(ctx.workingDir, asmFile);
    const fullBinFile = path.resolve(ctx.workingDir, binFile);
    const assembleCmd = `${mzaPath} -o "${fullBinFile}" "${fullAsmFile}"`;
    outputChannel.appendLine(`Step 2: Assemble: ${assembleCmd}`);

    const assembleOk = await new Promise<boolean>((resolve) => {
        exec(assembleCmd, { cwd: ctx.workingDir }, (error, stdout, stderr) => {
            if (stdout) { outputChannel.appendLine(stdout); }
            if (stderr) { outputChannel.appendLine(stderr); }
            if (error) {
                outputChannel.appendLine(`Assembly error: ${error.message}`);
                resolve(false);
            } else {
                resolve(true);
            }
        });
    });
    if (!assembleOk) {
        vscode.window.showErrorMessage('Assembly failed — see MinZ output');
        return;
    }

    // Step 3: Run in emulator
    const mzePath = path.join(path.dirname(ctx.compilerPath), 'mze');
    const mzxPath = path.join(path.dirname(ctx.compilerPath), 'mzx');
    const terminal = vscode.window.createTerminal({ name: `MinZ: ${fileName} (${target})`, cwd: ctx.workingDir });
    terminal.show();

    if (target === 'cpm') {
        terminal.sendText(`${mzePath} -t cpm "${fullBinFile}"`);
    } else if (target === 'zxspectrum') {
        // ZX Spectrum — use MZX graphical emulator
        terminal.sendText(`${mzxPath} "${fullBinFile}"`);
    } else {
        // Other targets — use mze with target flag
        terminal.sendText(`${mzePath} -t ${target} "${fullBinFile}"`);
    }
}

async function compileToIR() {
    const ctx = getMinZContext();
    if (!ctx) { return; }
    await vscode.window.activeTextEditor?.document.save();
    const outputDir = await ensureOutputDir(ctx);
    const fileName = path.basename(ctx.filePath, path.extname(ctx.filePath));
    const outputFile = path.join(outputDir, `${fileName}.ir`);

    const result = await runCompiler(ctx, [`"${ctx.filePath}"`, '--emit-ir', '-o', `"${outputFile}"`], 'Compiling to IR');
    if (result.success) {
        outputChannel.appendLine(`IR output: ${outputFile}`);
        openFileBeside(path.resolve(ctx.workingDir, outputFile));
    }
}

async function compileToMIR() {
    const ctx = getMinZContext();
    if (!ctx) { return; }
    await vscode.window.activeTextEditor?.document.save();

    const result = await runCompiler(ctx, [`"${ctx.filePath}"`, '--dump-mir'], 'Compiling to MIR');
    if (result.success) {
        // Show MIR output in a new untitled document beside the source
        const doc = await vscode.workspace.openTextDocument({
            content: result.stdout,
            language: 'minz-mir',
        });
        vscode.window.showTextDocument(doc, vscode.ViewColumn.Beside);
    }
}

async function compileToASM() {
    const ctx = getMinZContext();
    if (!ctx) { return; }
    await vscode.window.activeTextEditor?.document.save();
    const outputDir = await ensureOutputDir(ctx);
    const fileName = path.basename(ctx.filePath, path.extname(ctx.filePath));
    const outputFile = path.join(outputDir, `${fileName}.a80`);

    const result = await runCompiler(ctx, [`"${ctx.filePath}"`, '-o', `"${outputFile}"`], 'Compiling to Z80 ASM');
    if (result.success) {
        outputChannel.appendLine(`ASM output: ${outputFile}`);
        openFileBeside(path.resolve(ctx.workingDir, outputFile));
    }
}

async function compileAll() {
    const ctx = getMinZContext();
    if (!ctx) { return; }
    await vscode.window.activeTextEditor?.document.save();
    const outputDir = await ensureOutputDir(ctx);
    const fileName = path.basename(ctx.filePath, path.extname(ctx.filePath));

    outputChannel.clear();
    outputChannel.show();
    outputChannel.appendLine(`=== Compile All: ${ctx.filePath} ===`);

    // 1. MIR dump
    const mirResult = await runCompiler(ctx, [`"${ctx.filePath}"`, '--dump-mir'], 'Step 1: MIR');
    if (mirResult.success) {
        const mirFile = path.join(outputDir, `${fileName}.mir`);
        const fullMirPath = path.resolve(ctx.workingDir, mirFile);
        fs.writeFileSync(fullMirPath, mirResult.stdout);
        outputChannel.appendLine(`MIR saved: ${mirFile}`);
    }

    // 2. ASM
    const asmFile = path.join(outputDir, `${fileName}.a80`);
    const asmResult = await runCompiler(ctx, [`"${ctx.filePath}"`, '-o', `"${asmFile}"`], 'Step 2: Z80 ASM');
    if (asmResult.success) {
        outputChannel.appendLine(`ASM saved: ${asmFile}`);
        openFileBeside(path.resolve(ctx.workingDir, asmFile));
    }

    if (mirResult.success && asmResult.success) {
        vscode.window.showInformationMessage(`MinZ compile-all complete: MIR + ASM`);
    }
}

async function compileOptimized() {
    const ctx = getMinZContext();
    if (!ctx) { return; }
    await vscode.window.activeTextEditor?.document.save();
    const outputDir = await ensureOutputDir(ctx);
    const fileName = path.basename(ctx.filePath, path.extname(ctx.filePath));
    const outputFile = path.join(outputDir, `${fileName}_optimized.a80`);
    const enableSMC = ctx.config.get<boolean>('enableSMC', false);
    const enableTrueSMC = ctx.config.get<boolean>('enableTrueSMC', false);

    const args = [`"${ctx.filePath}"`, '-o', `"${outputFile}"`];
    if (enableSMC) { args.push('--enable-smc'); }
    if (enableTrueSMC) { args.push('--enable-true-smc'); }

    const result = await runCompiler(ctx, args, 'Compiling with full optimizations');
    if (result.success) {
        outputChannel.appendLine(`Optimized output: ${outputFile}`);
        vscode.window.showInformationMessage(`MinZ optimized: ${outputFile}`);
        openFileBeside(path.resolve(ctx.workingDir, outputFile));
    }
}

async function showAST() {
    const ctx = getMinZContext();
    if (!ctx) { return; }
    await vscode.window.activeTextEditor?.document.save();

    const result = await runCompiler(ctx, ['--dump-ast', `"${ctx.filePath}"`], 'Generating AST');
    if (result.success && result.stdout) {
        const doc = await vscode.workspace.openTextDocument({
            content: result.stdout,
            language: 'json',
        });
        vscode.window.showTextDocument(doc, vscode.ViewColumn.Beside);
    } else {
        vscode.window.showWarningMessage('AST generation not available');
    }
}

// ---------- Native Compilation (nanz2native) ----------

function getNanz2NativePath(ctx: ReturnType<typeof getMinZContext>): string {
    if (!ctx) { return 'nanz2native'; }
    // Try alongside the compiler binary, then fall back to PATH
    const compilerDir = path.dirname(ctx.compilerPath);
    const candidate = path.join(compilerDir, 'nanz2native');
    if (fs.existsSync(candidate)) { return candidate; }
    return 'nanz2native';
}

async function compileNative(backendFlag?: string) {
    const ctx = getMinZContext();
    if (!ctx) { return; }
    await vscode.window.activeTextEditor?.document.save();

    const nanz2native = getNanz2NativePath(ctx);
    const args = ['-run', '-disasm'];
    if (backendFlag) { args.push(backendFlag); }
    args.push(`"${ctx.filePath}"`);

    const cmd = `${nanz2native} ${args.join(' ')}`;
    outputChannel.clear();
    outputChannel.show();
    outputChannel.appendLine(`Compile to native: ${cmd}`);

    exec(cmd, { cwd: ctx.workingDir }, (error, stdout, stderr) => {
        if (stdout) { outputChannel.appendLine(stdout); }
        if (stderr) { outputChannel.appendLine(stderr); }
        if (error) {
            outputChannel.appendLine(`Error: ${error.message}`);
            vscode.window.showErrorMessage('Native compilation failed — see MinZ output');
        } else {
            const backend = backendFlag === '-c' ? 'C99' : backendFlag === '-q' ? 'QBE' : 'C99+QBE';
            vscode.window.showInformationMessage(`Native compilation (${backend}) complete`);
        }
    });
}

async function emitNativeIR(emitFlag: string) {
    const ctx = getMinZContext();
    if (!ctx) { return; }
    await vscode.window.activeTextEditor?.document.save();

    const nanz2native = getNanz2NativePath(ctx);
    const cmd = `${nanz2native} ${emitFlag} "${ctx.filePath}"`;

    exec(cmd, { cwd: ctx.workingDir }, async (error, stdout, stderr) => {
        if (error) {
            outputChannel.clear();
            outputChannel.show();
            if (stderr) { outputChannel.appendLine(stderr); }
            outputChannel.appendLine(`Error: ${error.message}`);
            return;
        }
        // Show generated code in a new tab beside the source
        const lang = emitFlag === '-emit-c' ? 'c' : 'plaintext';
        const doc = await vscode.workspace.openTextDocument({ content: stdout, language: lang });
        vscode.window.showTextDocument(doc, vscode.ViewColumn.Beside);
    });
}

// ---------- Debug Build + DeZog ----------

async function debugBuild() {
    const ctx = getMinZContext();
    if (!ctx) { return; }
    await vscode.window.activeTextEditor?.document.save();
    const outputDir = await ensureOutputDir(ctx);
    const fileName = path.basename(ctx.filePath, path.extname(ctx.filePath));
    const asmFile = path.join(outputDir, `${fileName}.a80`);

    const args = [`"${ctx.filePath}"`, '-o', `"${asmFile}"`, '--emit-sld'];

    const result = await runCompiler(ctx, args, 'Debug build (with SLD)');
    if (result.success) {
        const sldFile = asmFile.replace(/\.a80$/, '.sld');
        outputChannel.appendLine(`Debug build complete: ${asmFile}`);
        if (fs.existsSync(path.resolve(ctx.workingDir, sldFile))) {
            outputChannel.appendLine(`SLD source map: ${sldFile}`);
        }
        vscode.window.showInformationMessage(`Debug build complete — ready for DeZog`);
    } else {
        vscode.window.showErrorMessage('Debug build failed — see MinZ output');
    }
}

async function startDebugging() {
    const ctx = getMinZContext();
    if (!ctx) { return; }

    // Build first
    await debugBuild();

    const outputDir = ctx.config.get<string>('outputDirectory', './build');
    const fileName = path.basename(ctx.filePath, path.extname(ctx.filePath));
    const sldFile = path.join(outputDir, `${fileName}.sld`);
    const binFile = path.join(outputDir, `${fileName}.bin`);

    // Launch DeZog
    const debugConfig: vscode.DebugConfiguration = {
        type: 'dezog',
        request: 'launch',
        name: 'Debug MinZ',
        sjasmplus: [{
            path: path.resolve(ctx.workingDir, sldFile),
        }],
        topOfStack: '0xFFF0',
        load: path.resolve(ctx.workingDir, binFile),
        startAddress: '0x8000',
    };

    vscode.debug.startDebugging(vscode.workspace.getWorkspaceFolder(vscode.Uri.file(ctx.filePath)), debugConfig);
}

// ---------- DeZog Debug Configuration Provider ----------

class MinZDebugConfigProvider implements vscode.DebugConfigurationProvider {
    resolveDebugConfiguration(
        folder: vscode.WorkspaceFolder | undefined,
        config: vscode.DebugConfiguration,
        _token?: vscode.CancellationToken
    ): vscode.ProviderResult<vscode.DebugConfiguration> {
        // If no config, provide a default
        if (!config.type) {
            const activeEditor = vscode.window.activeTextEditor;
            if (!activeEditor || (!activeEditor.document.fileName.endsWith('.minz') && !activeEditor.document.fileName.endsWith('.mz') && !activeEditor.document.fileName.endsWith('.mir'))) {
                return undefined;
            }

            const minzConfig = vscode.workspace.getConfiguration('minz');
            const outputDir = minzConfig.get<string>('outputDirectory', './build');
            const fileName = path.basename(activeEditor.document.fileName, path.extname(activeEditor.document.fileName));

            config.type = 'dezog';
            config.request = 'launch';
            config.name = 'Debug MinZ';
            config.sjasmplus = [{
                path: `\${workspaceFolder}/${outputDir}/${fileName}.sld`,
            }];
            config.topOfStack = '0xFFF0';
            config.load = `\${workspaceFolder}/${outputDir}/${fileName}.bin`;
            config.startAddress = '0x8000';
        }

        return config;
    }
}
