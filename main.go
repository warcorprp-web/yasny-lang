package main

import (
	"bufio"
	"fmt"
	"os"
	"yasny-lang/ast"
	"yasny-lang/interpreter"
	"yasny-lang/lexer"
	"yasny-lang/parser"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Ясный v0.46 - Язык программирования на русском")
		fmt.Println("Использование: yasny <файл.ya>")
		fmt.Println("Или запустите без аргументов для интерактивного режима (REPL)")
		fmt.Println()
		startREPL()
		return
	}

	// Читаем файл
	filename := os.Args[1]
	content, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("Ошибка чтения файла: %v\n", err)
		os.Exit(1)
	}

	// Выполняем
	runWithFilename(string(content), filename)
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

	fmt.Println("=== Демо программа ===\n")
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
