import * as vscode from 'vscode';
import { exec, execFile } from 'child_process';
import { LanguageClient, LanguageClientOptions, ServerOptions } from 'vscode-languageclient/node';

let client: LanguageClient | undefined;

export function activate(context: vscode.ExtensionContext) {
    console.log('Расширение "Ясный" активировано');

    const config = vscode.workspace.getConfiguration('yasny');
    const yasnyPath = config.get<string>('executablePath', 'yasny');

    // === LSP Client ===
    const serverOptions: ServerOptions = {
        command: yasnyPath,
        args: ['lsp'],
    };

    const clientOptions: LanguageClientOptions = {
        documentSelector: [{ scheme: 'file', language: 'yasny' }],
    };

    client = new LanguageClient('yasny', 'Ясный Language Server', serverOptions, clientOptions);
    client.start();

    // Команда: Запустить программу
    let runCommand = vscode.commands.registerCommand('yasny.run', () => {
        const editor = vscode.window.activeTextEditor;
        if (!editor) {
            vscode.window.showErrorMessage('Нет открытого файла');
            return;
        }

        const document = editor.document;
        if (document.languageId !== 'yasny') {
            vscode.window.showErrorMessage('Это не файл .ya');
            return;
        }

        document.save().then(() => {
            runYasnyFile(document.fileName);
        });
    });

    // Команда: Запустить в терминале
    let runInTerminalCommand = vscode.commands.registerCommand('yasny.runInTerminal', () => {
        const editor = vscode.window.activeTextEditor;
        if (!editor) {
            vscode.window.showErrorMessage('Нет открытого файла');
            return;
        }

        const document = editor.document;
        if (document.languageId !== 'yasny') {
            vscode.window.showErrorMessage('Это не файл .ya');
            return;
        }

        document.save().then(() => {
            runInTerminal(document.fileName);
        });
    });

    // Команда: Открыть REPL
    let replCommand = vscode.commands.registerCommand('yasny.openREPL', () => {
        openREPL();
    });

    // Команда: Форматировать файл
    let formatCommand = vscode.commands.registerCommand('yasny.format', () => {
        const editor = vscode.window.activeTextEditor;
        if (!editor || editor.document.languageId !== 'yasny') {
            vscode.window.showErrorMessage('Откройте файл .ya');
            return;
        }
        vscode.commands.executeCommand('editor.action.formatDocument');
    });

    context.subscriptions.push(runCommand);
    context.subscriptions.push(runInTerminalCommand);
    context.subscriptions.push(replCommand);
    context.subscriptions.push(formatCommand);
}

function runYasnyFile(filePath: string) {
    const config = vscode.workspace.getConfiguration('yasny');
    const yasnyPath = config.get<string>('executablePath', 'yasny');

    const outputChannel = vscode.window.createOutputChannel('Ясный');
    outputChannel.clear();
    outputChannel.show();

    exec(`${yasnyPath} "${filePath}"`, (error, stdout, stderr) => {
        if (error) {
            outputChannel.appendLine(`Ошибка: ${error.message}`);
            return;
        }
        if (stderr) {
            outputChannel.appendLine(stderr);
        }
        if (stdout) {
            outputChannel.appendLine(stdout);
        }
    });
}

function runInTerminal(filePath: string) {
    const config = vscode.workspace.getConfiguration('yasny');
    const yasnyPath = config.get<string>('executablePath', 'yasny');

    const terminal = vscode.window.createTerminal('Ясный');
    terminal.show();
    terminal.sendText(`${yasnyPath} "${filePath}"`);
}

function openREPL() {
    const config = vscode.workspace.getConfiguration('yasny');
    const yasnyPath = config.get<string>('executablePath', 'yasny');

    const terminal = vscode.window.createTerminal('Ясный REPL');
    terminal.show();
    terminal.sendText(yasnyPath);
}

export function deactivate(): Thenable<void> | undefined {
    if (client) {
        return client.stop();
    }
    return undefined;
}
