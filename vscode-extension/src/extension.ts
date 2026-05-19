import * as vscode from 'vscode';
import { exec, execFile } from 'child_process';

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

    // Команда: Форматировать файл
    let formatCommand = vscode.commands.registerCommand('yasny.format', () => {
        const editor = vscode.window.activeTextEditor;
        if (!editor || editor.document.languageId !== 'yasny') {
            vscode.window.showErrorMessage('Откройте файл .ya');
            return;
        }
        vscode.commands.executeCommand('editor.action.formatDocument');
    });

    // Провайдер форматирования (через `yasny формат`)
    const formattingProvider = vscode.languages.registerDocumentFormattingEditProvider(
        { language: 'yasny' },
        {
            provideDocumentFormattingEdits(document: vscode.TextDocument): Thenable<vscode.TextEdit[]> {
                return new Promise((resolve, reject) => {
                    const config = vscode.workspace.getConfiguration('yasny');
                    const yasnyPath = config.get<string>('executablePath', 'yasny');
                    const text = document.getText();

                    const child = execFile(
                        yasnyPath,
                        ['формат', '-'],
                        { encoding: 'utf-8', maxBuffer: 32 * 1024 * 1024 },
                        (error, stdout, stderr) => {
                            if (error) {
                                vscode.window.showErrorMessage(
                                    'yasny формат: ' + (stderr || error.message).split('\n')[0]
                                );
                                reject(error);
                                return;
                            }
                            const fullRange = new vscode.Range(
                                document.positionAt(0),
                                document.positionAt(text.length)
                            );
                            resolve([vscode.TextEdit.replace(fullRange, stdout)]);
                        }
                    );
                    child.stdin?.write(text);
                    child.stdin?.end();
                });
            }
        }
    );

    context.subscriptions.push(runCommand);
    context.subscriptions.push(runInTerminalCommand);
    context.subscriptions.push(replCommand);
    context.subscriptions.push(formatCommand);
    context.subscriptions.push(formattingProvider);
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

export function deactivate() {}
