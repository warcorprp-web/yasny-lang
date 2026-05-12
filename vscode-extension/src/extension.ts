import * as vscode from 'vscode';
import * as path from 'path';
import { exec } from 'child_process';

export function activate(context: vscode.ExtensionContext) {
    console.log('Расширение "Ясный" активировано');

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

    context.subscriptions.push(runCommand);
    context.subscriptions.push(runInTerminalCommand);
    context.subscriptions.push(replCommand);
}

function runYasnyFile(filePath: string) {
    const config = vscode.workspace.getConfiguration('yasny');
    const yasnyPath = config.get<string>('interpreterPath', 'yasny');

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
    const yasnyPath = config.get<string>('interpreterPath', 'yasny');

    const terminal = vscode.window.createTerminal('Ясный');
    terminal.show();
    terminal.sendText(`${yasnyPath} "${filePath}"`);
}

function openREPL() {
    const config = vscode.workspace.getConfiguration('yasny');
    const yasnyPath = config.get<string>('interpreterPath', 'yasny');

    const terminal = vscode.window.createTerminal('Ясный REPL');
    terminal.show();
    terminal.sendText(yasnyPath);
}

export function deactivate() {}
