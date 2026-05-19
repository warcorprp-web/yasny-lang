package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"yasny-lang/ast"
	"yasny-lang/interpreter"
	"yasny-lang/lexer"
	"yasny-lang/parser"
	"yasny-lang/pkgmgr"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Ясный v0.46 - Язык программирования на русском")
		printUsage()
		fmt.Println()
		startREPL()
		return
	}

	cmd := os.Args[1]
	rest := os.Args[2:]

	switch cmd {
	case "инит":
		cmdInit(rest)
	case "подключить":
		cmdInstall(rest)
	case "удалить":
		cmdUninstall(rest)
	case "список":
		cmdList(rest)
	case "запустить":
		cmdRun(rest)
	case "помощь", "--help", "-h":
		printUsage()
	case "версия", "--version", "-v":
		fmt.Println("Ясный v0.46")
	default:
		// Если первый аргумент похож на путь к файлу — запускаем его
		// (обратная совместимость со старым поведением).
		if _, err := os.Stat(cmd); err == nil {
			content, err := os.ReadFile(cmd)
			if err != nil {
				fmt.Printf("Ошибка чтения файла: %v\n", err)
				os.Exit(1)
			}
			runWithFilename(string(content), cmd)
			return
		}
		fmt.Printf("Неизвестная команда: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

// printUsage печатает справку по командам.
func printUsage() {
	fmt.Println("Использование:")
	fmt.Println("  yasny <файл.ya>             — запустить файл")
	fmt.Println("  yasny запустить [файл]      — запустить точку входа из пакет.json")
	fmt.Println("  yasny инит [имя]            — создать пакет.json и главный.ya")
	fmt.Println("  yasny подключить URL[@вер]  — установить пакет с GitHub")
	fmt.Println("  yasny подключить            — установить все пакеты из манифеста")
	fmt.Println("  yasny удалить ИМЯ           — удалить пакет")
	fmt.Println("  yasny список                — показать установленные пакеты")
	fmt.Println("  yasny версия                — версия Ясного")
	fmt.Println()
	fmt.Println("Без аргументов запускается интерактивный режим (REPL).")
}

// cmdInit реализует команду 'инит'.
func cmdInit(args []string) {
	dir := "."
	if len(args) >= 1 {
		dir = args[0]
	}
	// Имя проекта — последний сегмент абсолютного пути целевой папки.
	abs, err := filepath.Abs(dir)
	if err != nil {
		fmt.Printf("Ошибка: %v\n", err)
		os.Exit(1)
	}
	name := filepath.Base(abs)
	if err := pkgmgr.Init(dir, name); err != nil {
		fmt.Printf("Ошибка: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Создан проект Ясного в %s\n", abs)
	fmt.Println("  пакет.json — манифест проекта")
	fmt.Println("  главный.ya — точка входа")
	fmt.Println("  .gitignore — исключает папку пакеты/")
	fmt.Println()
	fmt.Println("Запустить: yasny запустить")
}

// cmdInstall реализует команду 'подключить'.
func cmdInstall(args []string) {
	root, m, err := pkgmgr.LoadFromCwd()
	if err != nil {
		fmt.Printf("Ошибка: %v\n", err)
		fmt.Println("Подсказка: запустите 'yasny инит' для создания проекта.")
		os.Exit(1)
	}

	if len(args) == 0 {
		// Установить всё из манифеста.
		if err := pkgmgr.InstallAll(root, m); err != nil {
			fmt.Printf("Ошибка: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Готово.")
		return
	}

	// Установить указанные пакеты.
	for _, spec := range args {
		fmt.Printf("→ %s\n", spec)
		name, err := pkgmgr.Install(root, spec)
		if err != nil {
			fmt.Printf("Ошибка: %v\n", err)
			os.Exit(1)
		}
		// Имя по умолчанию = последний сегмент URL. Пользователь
		// может вручную переименовать в манифесте под свой алиас.
		m.Deps[name] = spec
	}
	if err := m.Save(filepath.Join(root, pkgmgr.ManifestFile)); err != nil {
		fmt.Printf("Ошибка записи манифеста: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Готово.")
}

// cmdUninstall реализует команду 'удалить'.
func cmdUninstall(args []string) {
	if len(args) < 1 {
		fmt.Println("Использование: yasny удалить ИМЯ")
		os.Exit(1)
	}
	root, m, err := pkgmgr.LoadFromCwd()
	if err != nil {
		fmt.Printf("Ошибка: %v\n", err)
		os.Exit(1)
	}
	for _, alias := range args {
		if err := pkgmgr.Uninstall(root, m, alias); err != nil {
			fmt.Printf("Ошибка: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Удалён: %s\n", alias)
	}
	if err := m.Save(filepath.Join(root, pkgmgr.ManifestFile)); err != nil {
		fmt.Printf("Ошибка записи манифеста: %v\n", err)
		os.Exit(1)
	}
}

// cmdList реализует команду 'список'.
func cmdList(args []string) {
	_ = args
	root, m, err := pkgmgr.LoadFromCwd()
	if err != nil {
		fmt.Printf("Ошибка: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Проект: %s (%s)\n", m.Name, m.Version)
	if len(m.Deps) == 0 {
		fmt.Println("Зависимостей нет.")
		return
	}
	fmt.Println("Зависимости:")
	for alias, spec := range m.Deps {
		pkgDir := filepath.Join(root, pkgmgr.PackagesDir, pkgmgr.PackageNameFromSpec(spec))
		status := "✓"
		if _, err := os.Stat(pkgDir); err != nil {
			status = "✗ (не установлен — выполните 'yasny подключить')"
		}
		fmt.Printf("  %s %s — %s\n", status, alias, spec)
	}
}

// cmdRun реализует команду 'запустить'.
func cmdRun(args []string) {
	if len(args) >= 1 {
		// Явно указан файл — запускаем его.
		filename := args[0]
		content, err := os.ReadFile(filename)
		if err != nil {
			fmt.Printf("Ошибка чтения файла: %v\n", err)
			os.Exit(1)
		}
		runWithFilename(string(content), filename)
		return
	}

	root, m, err := pkgmgr.LoadFromCwd()
	if err != nil {
		fmt.Printf("Ошибка: %v\n", err)
		os.Exit(1)
	}
	entry := m.Entry
	if entry == "" {
		entry = pkgmgr.DefaultEntryPoint
	}
	entryPath := filepath.Join(root, entry)
	content, err := os.ReadFile(entryPath)
	if err != nil {
		fmt.Printf("Ошибка чтения точки входа %s: %v\n", entryPath, err)
		os.Exit(1)
	}
	runWithFilename(string(content), entryPath)
}

func runWithFilename(code, filename string) {
	l := lexer.NewWithFilename(code, filename)
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) != 0 {
		fmt.Println("❌ Ошибки парсинга:")
		for _, msg := range p.Errors() {
			fmt.Printf("  - %s\n", msg)
		}
		return
	}

	env := interpreter.NewEnvironment()
	result := interpreter.Eval(program, env)

	if result != nil && result.Type() == "ERROR" {
		fmt.Println(result.Inspect())
		os.Exit(1)
	}
}

func run(code string) {
	runWithFilename(code, "")
}

func runDemo() {
	code := `пусть имя = "Вася"
пусть возраст = 25

вывод("=== Переменные и строки ===")
вывод("Привет, " + имя + "!")
вывод("Тебе " + возраст + " лет")
вывод("")

вывод("=== Арифметика ===")
пусть сумма = 10 + 20
вывод("10 + 20 = " + сумма)
вывод("5 * 6 = " + (5 * 6))
вывод("")

вывод("=== Функции ===")
функция привет(имя)
    вывод("Привет, " + имя + "!")
конец

привет("Маша")
привет("Петя")
вывод("")

вывод("=== Рекурсия ===")
функция факториал(н)
    если н <= 1
        вернуть 1
    конец
    вернуть н * факториал(н - 1)
конец
вывод("Факториал 5 = " + факториал(5))
вывод("")

вывод("=== Циклы ===")
вывод("От 1 до 5:")
для i от 1 до 5
    вывод("  " + i)
конец
вывод("")

вывод("=== Массивы ===")
пусть числа = [1, 2, 3, 4, 5]
вывод("Массив: " + числа)
вывод("Длина: " + длина(числа))
вывод("Первый элемент: " + числа[0])

вывод("Цикл по массиву:")
для число в числа
    вывод("  " + число)
конец
вывод("")

вывод("=== Словари ===")
пусть человек = {"имя": "Вася", "возраст": 25, "город": "Москва"}
вывод("Человек: " + человек)
вывод("Имя: " + человек["имя"])
вывод("Возраст: " + человек["возраст"])
вывод("")

вывод("=== Массив словарей ===")
пусть люди = [
    {"имя": "Вася", "возраст": 25},
    {"имя": "Маша", "возраст": 22},
    {"имя": "Петя", "возраст": 30}
]

для человек в люди
    вывод("  " + человек["имя"] + " - " + человек["возраст"] + " лет")
конец`

	fmt.Println("=== Демо программа ===")
	fmt.Println()
	run(code)
}

func startREPL() {
	env := interpreter.NewEnvironment()
	scanner := bufio.NewScanner(os.Stdin)
	
	fmt.Println("Введите 'выход' или 'exit' для выхода")
	fmt.Println()

	for {
		fmt.Print(">>> ")
		
		if !scanner.Scan() {
			return
		}

		line := strings.TrimSpace(scanner.Text())
		
		if line == "" {
			continue
		}
		
		if line == "выход" || line == "exit" {
			fmt.Println("До свидания!")
			return
		}

		// Парсим и выполняем
		l := lexer.New(line + "\n")
		p := parser.New(l)
		program := p.ParseProgram()

		if len(p.Errors()) != 0 {
			fmt.Println("Ошибки парсинга:")
			for _, msg := range p.Errors() {
				fmt.Printf("  %s\n", msg)
			}
			continue
		}

		// Проверяем тип statement для вывода результата
		shouldPrint := false
		if len(program.Statements) > 0 {
			switch program.Statements[0].(type) {
			case *ast.ExpressionStatement:
				shouldPrint = true
			}
		}

		result := interpreter.Eval(program, env)
		if result != nil {
			if result.Type() == "ERROR" {
				fmt.Println(result.Inspect())
			} else if shouldPrint && result.Type() != "NULL" {
				fmt.Println(result.Inspect())
			}
		}
	}
}
