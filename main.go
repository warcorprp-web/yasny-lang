package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"yasny-lang/ast"
	"yasny-lang/formatter"
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
	case "формат":
		cmdFormat(rest)
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

// printUsage печатает подробную справку по командам с примерами.
func printUsage() {
	fmt.Println("Использование:")
	fmt.Println("  yasny <файл.ya>             — запустить файл")
	fmt.Println("  yasny запустить [файл]      — запустить точку входа из пакет.json")
	fmt.Println("  yasny инит [имя]            — создать пакет.json и главный.ya")
	fmt.Println("  yasny подключить ИМЯ        — установить пакет по короткому имени")
	fmt.Println("  yasny подключить URL[@вер]  — установить пакет по полному URL")
	fmt.Println("  yasny подключить            — установить все пакеты из манифеста")
	fmt.Println("  yasny удалить ИМЯ           — удалить пакет")
	fmt.Println("  yasny список                — показать установленные пакеты")
	fmt.Println("  yasny формат файл.ya        — отформатировать (вывести в stdout)")
	fmt.Println("  yasny формат -в файл.ya     — отформатировать и переписать файл")
	fmt.Println("  yasny формат -в .           — отформатировать все .ya в папке")
	fmt.Println("  yasny помощь                — показать эту справку")
	fmt.Println("  yasny версия                — версия Ясного")
	fmt.Println()
	fmt.Println("Примеры:")
	fmt.Println("  yasny инит мой_проект")
	fmt.Println("  cd мой_проект")
	fmt.Println("  yasny подключить матем            # короткое имя из реестра")
	fmt.Println("  yasny подключить github.com/у/п@v1.0.0    # полный URL и тег")
	fmt.Println("  yasny запустить")
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
//
// Поведение:
//   - Без аргументов: ставит всё из манифеста (использует lock).
//   - С URL/именем: ставит указанный пакет.
//   - Если в текущей папке нет пакет.json — авто-создаёт минимальный
//     (как 'npm install foo' в пустой папке).
//   - Короткое имя без слешей резолвится через реестр коротких имён.
func cmdInstall(args []string) {
	root, m, err := pkgmgr.LoadFromCwd()
	if err != nil {
		// Авто-инициализация: первый 'подключить' в папке без манифеста
		// должен сразу работать, без отдельного 'инит'.
		if len(args) == 0 {
			fmt.Println("Ошибка: пакет.json не найден.")
			fmt.Println("Подсказка: создайте проект командой 'yasny инит',")
			fmt.Println("           или укажите пакет: 'yasny подключить ИМЯ'.")
			os.Exit(1)
		}
		cwd, _ := os.Getwd()
		mNew, errInit := pkgmgr.InitMinimal(cwd, filepath.Base(cwd))
		if errInit != nil {
			fmt.Printf("Ошибка авто-инициализации: %v\n", errInit)
			os.Exit(1)
		}
		fmt.Printf("Создан %s в текущей папке.\n", pkgmgr.ManifestFile)
		root = cwd
		m = mNew
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
	var registry *pkgmgr.Registry
	lock, _ := pkgmgr.LoadLock(root)

	for _, spec := range args {
		// Имя для папки пакеты/ — короткое имя пользователя или
		// последний сегмент URL, если URL.
		nameForFolder := ""

		// Если это короткое имя — резолвим через реестр.
		resolvedSpec := spec
		if pkgmgr.IsShortName(spec) {
			if registry == nil {
				fmt.Printf("Ищу '%s' в реестре...\n", spec)
				r, err := pkgmgr.FetchRegistry()
				if err != nil {
					fmt.Printf("Ошибка реестра: %v\n", err)
					fmt.Printf("Подсказка: укажите полный URL — 'yasny подключить github.com/.../%s'\n", spec)
					os.Exit(1)
				}
				registry = r
			}
			shortName, version := pkgmgr.ParseDependency(spec)
			url := registry.ResolveName(shortName)
			if url == "" {
				fmt.Printf("Имя '%s' не найдено в реестре.\n", shortName)
				fmt.Println("Подсказка: укажите полный URL — github.com/чей_то/пакет")
				os.Exit(1)
			}
			if version != "" {
				resolvedSpec = url + "@" + version
			} else {
				resolvedSpec = url
			}
			nameForFolder = shortName
			fmt.Printf("✓ Найдено: %s\n", resolvedSpec)
		} else {
			nameForFolder = pkgmgr.PackageNameFromSpec(resolvedSpec)
		}

		fmt.Printf("→ Устанавливаю %s в пакеты/%s\n", resolvedSpec, nameForFolder)
		_, commit, err := pkgmgr.InstallAs(root, resolvedSpec, nameForFolder)
		if err != nil {
			fmt.Printf("Ошибка: %v\n", err)
			os.Exit(1)
		}
		// В манифест: ключ = nameForFolder, значение = ровно как ввёл пользователь.
		m.Deps[nameForFolder] = spec

		// В lock — точный коммит и реальный источник.
		source, version := pkgmgr.ParseDependency(resolvedSpec)
		lock.Set(&pkgmgr.LockEntry{
			Name:    nameForFolder,
			Source:  source,
			Version: version,
			Commit:  commit,
		})
	}

	if err := m.Save(filepath.Join(root, pkgmgr.ManifestFile)); err != nil {
		fmt.Printf("Ошибка записи манифеста: %v\n", err)
		os.Exit(1)
	}
	if err := lock.Save(root); err != nil {
		fmt.Printf("Ошибка записи lock-файла: %v\n", err)
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

// cmdFormat реализует команду 'формат'.
//
// Без флагов: выводит отформатированный код на stdout.
// С флагом -в: переписывает файл(ы) на месте.
// Если указана папка: рекурсивно форматирует все .ya внутри.
func cmdFormat(args []string) {
	if len(args) == 0 {
		fmt.Println("Использование:")
		fmt.Println("  yasny формат файл.ya         — вывести в stdout")
		fmt.Println("  yasny формат -в файл.ya      — переписать файл")
		fmt.Println("  yasny формат -в папка/       — все .ya в папке (рекурсивно)")
		os.Exit(1)
	}

	writeMode := false
	paths := []string{}
	for _, a := range args {
		if a == "-в" || a == "-w" || a == "--write" {
			writeMode = true
			continue
		}
		paths = append(paths, a)
	}

	if len(paths) == 0 {
		fmt.Println("Ошибка: укажите файл или папку.")
		os.Exit(1)
	}

	changed := 0
	checked := 0
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			fmt.Printf("Ошибка: %v\n", err)
			os.Exit(1)
		}
		if info.IsDir() {
			err = filepath.Walk(path, func(p string, fi os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if fi.IsDir() {
					// Пропускаем пакеты/ и .git/
					name := fi.Name()
					if name == "пакеты" || name == ".git" || name == "node_modules" {
						return filepath.SkipDir
					}
					return nil
				}
				if strings.HasSuffix(p, ".ya") {
					checked++
					if formatOneFile(p, writeMode) {
						changed++
					}
				}
				return nil
			})
			if err != nil {
				fmt.Printf("Ошибка обхода: %v\n", err)
				os.Exit(1)
			}
		} else {
			checked++
			if formatOneFile(path, writeMode) {
				changed++
			}
		}
	}

	if writeMode {
		if changed == 0 {
			fmt.Printf("Проверено файлов: %d. Изменений не нужно.\n", checked)
		} else {
			fmt.Printf("Отформатировано: %d из %d файлов.\n", changed, checked)
		}
	}
}

// formatOneFile форматирует один файл. В writeMode переписывает,
// иначе печатает в stdout. Возвращает true, если содержимое
// изменилось.
func formatOneFile(path string, writeMode bool) bool {
	source, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("Ошибка чтения %s: %v\n", path, err)
		return false
	}
	formatted, err := formatter.Format(string(source))
	if err != nil {
		fmt.Printf("Ошибка форматирования %s:\n%v\n", path, err)
		return false
	}
	changed := string(source) != formatted
	if !writeMode {
		// Без флага -в: всегда печатаем результат, чтобы пользователь
		// увидел что получится.
		fmt.Print(formatted)
		return changed
	}
	if !changed {
		return false
	}
	if err := os.WriteFile(path, []byte(formatted), 0644); err != nil {
		fmt.Printf("Ошибка записи %s: %v\n", path, err)
		return false
	}
	fmt.Printf("✓ %s\n", path)
	return true
}
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
