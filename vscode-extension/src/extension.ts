import * as vscode from 'vscode';
import * as path from 'path';
import { exec } from 'child_process';

export function activate(context: vscode.ExtensionContext) {
    console.log('Расширение "Простой" активировано');

    // Команда: Запустить программу
    let runCommand = vscode.commands.registerCommand('yasny.run', () => {
        const editor = vscode.window.activeTextEditor;
        if (!editor) {
            vscode.window.showErrorMessage('Нет открытого файла');
            return;
        }

        const document = editor.document;
        if (document.languageId !== 'yasny') {
            vscode.window.showErrorMessage('Это не файл .pr');
            return;
        }

        // Сохраняем файл перед запуском
        document.save().then(() => {
            runProstoyFile(document.fileName);
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
            vscode.window.showErrorMessage('Это не файл .pr');
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

    context.subscriptions.push(runCommand, runInTerminalCommand, replCommand);
}

function runProstoyFile(filePath: string) {
    const config = vscode.workspace.getConfiguration('yasny');
    const yasnyPath = config.get<string>('executablePath', 'yasny');

    const outputChannel = vscode.window.createOutputChannel('Простой');
    outputChannel.show();
    outputChannel.clear();
    outputChannel.appendLine(`Запуск: ${path.basename(filePath)}`);
    outputChannel.appendLine('─'.repeat(50));

    exec(`"${yasnyPath}" "${filePath}"`, (error, stdout, stderr) => {
        if (stdout) {
            outputChannel.appendLine(stdout);
        }
        if (stderr) {
            outputChannel.appendLine(stderr);
        }
        if (error) {
            outputChannel.appendLine(`\nОшибка выполнения: ${error.message}`);
            vscode.window.showErrorMessage(`Ошибка: ${error.message}`);
        } else {
            outputChannel.appendLine('\n✅ Программа завершена');
        }
    });
}

function runInTerminal(filePath: string) {
    const config = vscode.workspace.getConfiguration('yasny');
    const yasnyPath = config.get<string>('executablePath', 'yasny');

    const terminal = vscode.window.createTerminal('Простой');
    terminal.show();
    terminal.sendText(`"${yasnyPath}" "${filePath}"`);
}

function openREPL() {
    const config = vscode.workspace.getConfiguration('yasny');
    const yasnyPath = config.get<string>('executablePath', 'yasny');

    const terminal = vscode.window.createTerminal('Простой REPL');
    terminal.show();
    terminal.sendText(`"${yasnyPath}"`);
}

export function deactivate() {}
