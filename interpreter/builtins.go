package interpreter

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"
)

// OutputWriter - куда писать вывод (для WASM можно переопределить)
var OutputWriter io.Writer = os.Stdout

// Счётчики тестов
var testPasses, testFailures int

var ApplyFunctionCallback func(Object, []Object) Object

// Вспомогательные функции для ошибок встроенных функций
func builtinErrorWrongArgCount(funcName string, expected, got int) *Error {
	return ErrorWithHint(
		currentCallToken,
		fmt.Sprintf("функция '%s' ожидает %d аргумент(ов), получено %d", funcName, expected, got),
		fmt.Sprintf("Вызовите функцию правильно: %s(...)", funcName),
	)
}

func builtinErrorWrongArgType(funcName string, argNum int, expected, got string) *Error {
	return ErrorWithHint(
		currentCallToken,
		fmt.Sprintf("функция '%s': аргумент %d должен быть %s, получен %s", funcName, argNum, expected, got),
		fmt.Sprintf("Передайте значение типа %s", expected),
	)
}

func builtinErrorUnsupportedType(funcName string, got string) *Error {
	return ErrorWithHint(
		currentCallToken,
		fmt.Sprintf("функция '%s' не поддерживает тип %s", funcName, got),
		"Проверьте типы аргументов.",
	)
}

// newBuiltinError создает ошибку с номером строки из текущего вызова
func newBuiltinError(format string, a ...interface{}) *Error {
	return &Error{
		Message: fmt.Sprintf(format, a...),
		Line:    currentCallToken.Line,
		Column:  currentCallToken.Column,
	}
}

var builtins = map[string]*Builtin{
	"вывод": {
		Fn: func(args ...Object) Object {
			for _, arg := range args {
				fmt.Fprint(OutputWriter, arg.Inspect())
			}
			fmt.Fprintln(OutputWriter)
			return NULL
		},
	},
	"ввод": {
		Fn: func(args ...Object) Object {
			if len(args) > 0 {
				fmt.Print(args[0].Inspect())
			}
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				return &String{Value: scanner.Text()}
			}
			return NULL
		},
	},
	"длина": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return builtinErrorWrongArgCount("длина", 1, len(args))
			}

			switch arg := args[0].(type) {
			case *String:
				// Считаем Unicode codepoints (руны), а не байты
				return &Integer{Value: int64(len([]rune(arg.Value)))}
			case *Array:
				return &Integer{Value: int64(len(arg.Elements))}
			default:
				return builtinErrorUnsupportedType("длина", args[0].Type())
			}
		},
	},
	"добавить": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return builtinErrorWrongArgCount("добавить", 2, len(args))
			}
			if args[0].Type() != "ARRAY" {
				return builtinErrorWrongArgType("добавить", 1, "ARRAY (массив)", args[0].Type())
			}

			arr := args[0].(*Array)
			// Изменяем массив напрямую
			arr.Elements = append(arr.Elements, args[1])

			return arr
		},
	},
	"тип": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return builtinErrorWrongArgCount("тип", 1, len(args))
			}
			return &String{Value: args[0].Type()}
		},
	},
	"округл": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return builtinErrorWrongArgCount("округл", 1, len(args))
			}
			switch arg := args[0].(type) {
			case *Float:
				return &Integer{Value: int64(arg.Value + 0.5)}
			case *Integer:
				return arg
			default:
				return builtinErrorWrongArgType("округл", 1, "FLOAT или INTEGER (число)", args[0].Type())
			}
		},
	},
	"строка": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return builtinErrorWrongArgCount("строка", 1, len(args))
			}
			return &String{Value: args[0].Inspect()}
		},
	},
	"число": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return builtinErrorWrongArgCount("число", 1, len(args))
			}
			if args[0].Type() == "STRING" {
				str := args[0].(*String).Value
				// Пробуем распарсить как целое
				var num int64
				_, err := fmt.Sscanf(str, "%d", &num)
				if err == nil {
					return &Integer{Value: num}
				}
				// Пробуем как дробное
				var fnum float64
				_, err = fmt.Sscanf(str, "%f", &fnum)
				if err == nil {
					return &Float{Value: fnum}
				}
				return ErrorWithHint(
					currentCallToken,
					fmt.Sprintf("не удалось преобразовать '%s' в число", str),
					"Убедитесь, что строка содержит корректное число (например: \"123\" или \"45.67\").",
				)
			}
			return builtinErrorWrongArgType("число", 1, "STRING (строка)", args[0].Type())
		},
	},
	"ключи": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return builtinErrorWrongArgCount("ключи", 1, len(args))
			}
			if args[0].Type() != "HASH" {
				return builtinErrorWrongArgType("ключи", 1, "HASH (объект)", args[0].Type())
			}
			hash := args[0].(*Hash)
			pairs := hash.orderedPairs()
			keys := make([]Object, 0, len(pairs))
			for _, pair := range pairs {
				keys = append(keys, pair.Key)
			}
			return &Array{Elements: keys}
		},
	},
	"значения": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return builtinErrorWrongArgCount("значения", 1, len(args))
			}
			if args[0].Type() != "HASH" {
				return builtinErrorWrongArgType("значения", 1, "HASH (объект)", args[0].Type())
			}
			hash := args[0].(*Hash)
			pairs := hash.orderedPairs()
			values := make([]Object, 0, len(pairs))
			for _, pair := range pairs {
				values = append(values, pair.Value)
			}
			return &Array{Elements: values}
		},
	},
	"ошибка": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return builtinErrorWrongArgCount("ошибка", 1, len(args))
			}
			return ErrorWithHint(
				currentCallToken,
				args[0].Inspect(),
				"Эта ошибка была создана явно с помощью функции 'ошибка()'.",
			)
		},
	},
	"читать_файл": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return builtinErrorWrongArgCount("читать_файл", 1, len(args))
			}
			if args[0].Type() != "STRING" {
				return builtinErrorWrongArgType("читать_файл", 1, "STRING (строка с путём)", args[0].Type())
			}
			path := args[0].(*String).Value
			content, err := os.ReadFile(path)
			if err != nil {
				return ErrorFileNotFound(currentCallToken, path)
			}
			return &String{Value: string(content)}
		},
	},
	"записать_файл": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return builtinErrorWrongArgCount("записать_файл", 2, len(args))
			}
			if args[0].Type() != "STRING" {
				return builtinErrorWrongArgType("записать_файл", 1, "STRING (путь к файлу)", args[0].Type())
			}
			if args[1].Type() != "STRING" {
				return builtinErrorWrongArgType("записать_файл", 2, "STRING (содержимое)", args[1].Type())
			}
			path := args[0].(*String).Value
			content := args[1].(*String).Value
			err := os.WriteFile(path, []byte(content), 0644)
			if err != nil {
				return ErrorWithHint(
					currentCallToken,
					fmt.Sprintf("ошибка записи файла '%s': %s", path, err.Error()),
					"Проверьте права доступа и существование директории.",
				)
			}
			return NULL
		},
	},
	"существует_файл": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return builtinErrorWrongArgCount("существует_файл", 1, len(args))
			}
			if args[0].Type() != "STRING" {
				return builtinErrorWrongArgType("существует_файл", 1, "STRING (путь к файлу)", args[0].Type())
			}
			path := args[0].(*String).Value
			_, err := os.Stat(path)
			if err == nil {
				return TRUE
			}
			if os.IsNotExist(err) {
				return FALSE
			}
			return ErrorWithHint(
				currentCallToken,
				fmt.Sprintf("ошибка проверки файла: %s", err.Error()),
				"Проверьте права доступа к файлу.",
			)
		},
	},
	// Строки
	"разделить": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return builtinErrorWrongArgCount("разделить", 2, len(args))
			}
			if args[0].Type() != "STRING" || args[1].Type() != "STRING" {
				return ErrorWithHint(
					currentCallToken,
					"функция 'разделить' ожидает два аргумента типа STRING (строка)",
					"Используйте: разделить(\"текст\", \"разделитель\")",
				)
			}
			str := args[0].(*String).Value
			sep := args[1].(*String).Value
			parts := strings.Split(str, sep)
			elements := make([]Object, len(parts))
			for i, part := range parts {
				elements[i] = &String{Value: part}
			}
			return &Array{Elements: elements}
		},
	},
	"соединить": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return builtinErrorWrongArgCount("соединить", 2, len(args))
			}
			if args[0].Type() != "ARRAY" || args[1].Type() != "STRING" {
    return ErrorWithHint(currentCallToken, "первый аргумент должен быть массивом, второй - строкой", "Используйте: функция(массив, \"строка\")")
			}
			arr := args[0].(*Array)
			sep := args[1].(*String).Value
			parts := make([]string, len(arr.Elements))
			for i, elem := range arr.Elements {
				parts[i] = elem.Inspect()
			}
			return &String{Value: strings.Join(parts, sep)}
		},
	},
	"заменить": {
		Fn: func(args ...Object) Object {
			if len(args) != 3 {
				return builtinErrorWrongArgCount("заменить", 3, len(args))
			}
			if args[0].Type() != "STRING" || args[1].Type() != "STRING" || args[2].Type() != "STRING" {
    return ErrorWithHint(currentCallToken, "все аргументы должны быть строками", "Передайте строковые значения.")
			}
			str := args[0].(*String).Value
			old := args[1].(*String).Value
			new := args[2].(*String).Value
			return &String{Value: strings.ReplaceAll(str, old, new)}
		},
	},
	"верхний": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return builtinErrorWrongArgCount("верхний", 1, len(args))
			}
			if args[0].Type() != "STRING" {
				return builtinErrorWrongArgType("верхний", 1, "STRING (строка)", args[0].Type())
			}
			return &String{Value: strings.ToUpper(args[0].(*String).Value)}
		},
	},
	"нижний": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return builtinErrorWrongArgCount("нижний", 1, len(args))
			}
			if args[0].Type() != "STRING" {
				return builtinErrorWrongArgType("нижний", 1, "STRING (строка)", args[0].Type())
			}
			return &String{Value: strings.ToLower(args[0].(*String).Value)}
		},
	},
	"подстрока": {
		Fn: func(args ...Object) Object {
			if len(args) != 3 {
				return builtinErrorWrongArgCount("подстрока", 3, len(args))
			}
			if args[0].Type() != "STRING" || args[1].Type() != "INTEGER" || args[2].Type() != "INTEGER" {
    return ErrorWithHint(currentCallToken, "аргументы должны быть: строка, целое число, целое число", "Проверьте типы аргументов.")
			}
			str := args[0].(*String).Value
			start := int(args[1].(*Integer).Value)
			end := int(args[2].(*Integer).Value)
			if start < 0 || end > len(str) || start > end {
    return ErrorWithHint(currentCallToken, "неверные индексы", "Индексы должны быть в пределах строки и начало <= конец.")
			}
			return &String{Value: str[start:end]}
		},
	},
	"обрезать": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return builtinErrorWrongArgCount("обрезать", 1, len(args))
			}
			if args[0].Type() != "STRING" {
				return builtinErrorWrongArgType("обрезать", 1, "STRING (строка)", args[0].Type())
			}
			return &String{Value: strings.TrimSpace(args[0].(*String).Value)}
		},
	},
	"начинается_с": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return builtinErrorWrongArgCount("начинается_с", 2, len(args))
			}
			if args[0].Type() != "STRING" || args[1].Type() != "STRING" {
    return ErrorWithHint(currentCallToken, "все аргументы должны быть строками", "Передайте строковые значения.")
			}
			str := args[0].(*String).Value
			prefix := args[1].(*String).Value
			if strings.HasPrefix(str, prefix) {
				return TRUE
			}
			return FALSE
		},
	},
	"заканчивается_на": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return builtinErrorWrongArgCount("заканчивается_на", 2, len(args))
			}
			if args[0].Type() != "STRING" || args[1].Type() != "STRING" {
    return ErrorWithHint(currentCallToken, "все аргументы должны быть строками", "Передайте строковые значения.")
			}
			str := args[0].(*String).Value
			suffix := args[1].(*String).Value
			if strings.HasSuffix(str, suffix) {
				return TRUE
			}
			return FALSE
		},
	},
	"содержит": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return builtinErrorWrongArgCount("содержит", 2, len(args))
			}
			if args[0].Type() != "STRING" || args[1].Type() != "STRING" {
    return ErrorWithHint(currentCallToken, "все аргументы должны быть строками", "Передайте строковые значения.")
			}
			str := args[0].(*String).Value
			substr := args[1].(*String).Value
			if strings.Contains(str, substr) {
				return TRUE
			}
			return FALSE
		},
	},
	"повторить": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return builtinErrorWrongArgCount("повторить", 2, len(args))
			}
			if args[0].Type() != "STRING" || args[1].Type() != "INTEGER" {
    return ErrorWithHint(currentCallToken, "аргументы должны быть: строка и целое число", "Проверьте типы аргументов.")
			}
			str := args[0].(*String).Value
			count := int(args[1].(*Integer).Value)
			if count < 0 {
    return ErrorWithHint(currentCallToken, "количество повторений не может быть отрицательным", "Используйте положительное число.")
			}
			return &String{Value: strings.Repeat(str, count)}
		},
	},
	// Массивы
	"удалить": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return builtinErrorWrongArgCount("удалить", 2, len(args))
			}
			if args[0].Type() != "ARRAY" || args[1].Type() != "INTEGER" {
    return ErrorWithHint(currentCallToken, "первый аргумент должен быть массивом, второй - целым числом", "Используйте: функция(массив, индекс)")
			}
			arr := args[0].(*Array)
			idx := int(args[1].(*Integer).Value)
			if idx < 0 || idx >= len(arr.Elements) {
    return ErrorWithHint(currentCallToken, "индекс вне диапазона массива", "Проверьте, что индекс находится в пределах массива.")
			}
			// Изменяем массив напрямую
			arr.Elements = append(arr.Elements[:idx], arr.Elements[idx+1:]...)
			return arr
		},
	},
	"вставить": {
		Fn: func(args ...Object) Object {
			if len(args) != 3 {
				return builtinErrorWrongArgCount("вставить", 3, len(args))
			}
			if args[0].Type() != "ARRAY" || args[1].Type() != "INTEGER" {
    return ErrorWithHint(currentCallToken, "первый аргумент должен быть массивом, второй - целым числом", "Используйте: функция(массив, индекс)")
			}
			arr := args[0].(*Array)
			idx := int(args[1].(*Integer).Value)
			if idx < 0 || idx > len(arr.Elements) {
    return ErrorWithHint(currentCallToken, "индекс вне диапазона массива", "Проверьте, что индекс находится в пределах массива.")
			}
			// Изменяем массив напрямую
			newElements := make([]Object, 0, len(arr.Elements)+1)
			newElements = append(newElements, arr.Elements[:idx]...)
			newElements = append(newElements, args[2])
			newElements = append(newElements, arr.Elements[idx:]...)
			arr.Elements = newElements
			return arr
		},
	},
	// Математика
	"мин": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return builtinErrorWrongArgCount("мин", 2, len(args))
			}
			a := toFloat(args[0])
			b := toFloat(args[1])
			if a == nil || b == nil {
    return ErrorWithHint(currentCallToken, "все аргументы должны быть числами", "Передайте числовые значения (INTEGER или FLOAT).")
			}
			return &Float{Value: math.Min(*a, *b)}
		},
	},
	"макс": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return builtinErrorWrongArgCount("макс", 2, len(args))
			}
			a := toFloat(args[0])
			b := toFloat(args[1])
			if a == nil || b == nil {
    return ErrorWithHint(currentCallToken, "все аргументы должны быть числами", "Передайте числовые значения (INTEGER или FLOAT).")
			}
			return &Float{Value: math.Max(*a, *b)}
		},
	},
	"степень": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return builtinErrorWrongArgCount("степень", 2, len(args))
			}
			a := toFloat(args[0])
			b := toFloat(args[1])
			if a == nil || b == nil {
    return ErrorWithHint(currentCallToken, "все аргументы должны быть числами", "Передайте числовые значения (INTEGER или FLOAT).")
			}
			return &Float{Value: math.Pow(*a, *b)}
		},
	},
	"корень": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return builtinErrorWrongArgCount("корень", 1, len(args))
			}
			a := toFloat(args[0])
			if a == nil {
    return ErrorWithHint(currentCallToken, "аргумент должен быть числом", "Передайте числовое значение (INTEGER или FLOAT).")
			}
			return &Float{Value: math.Sqrt(*a)}
		},
	},
	"абс": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return builtinErrorWrongArgCount("абс", 1, len(args))
			}
			a := toFloat(args[0])
			if a == nil {
    return ErrorWithHint(currentCallToken, "аргумент должен быть числом", "Передайте числовое значение (INTEGER или FLOAT).")
			}
			return &Float{Value: math.Abs(*a)}
		},
	},
	"диапазон": {
		Fn: func(args ...Object) Object {
			if len(args) < 1 || len(args) > 2 {
				return builtinErrorWrongArgCount("диапазон", 1, len(args))
			}
			
			var start, end int64
			
			if len(args) == 1 {
				// диапазон(5) -> [0, 1, 2, 3, 4]
				if args[0].Type() != "INTEGER" {
					return builtinErrorWrongArgType("диапазон", 1, "INTEGER (целое число)", args[0].Type())
				}
				start = 0
				end = args[0].(*Integer).Value
			} else {
				// диапазон(1, 5) -> [1, 2, 3, 4]
				if args[0].Type() != "INTEGER" || args[1].Type() != "INTEGER" {
     return ErrorWithHint(currentCallToken, "аргументы должны быть целыми числами", "Передайте целые числа (INTEGER).")
				}
				start = args[0].(*Integer).Value
				end = args[1].(*Integer).Value
			}
			
			if end < start {
				return &Array{Elements: []Object{}}
			}
			
			elements := make([]Object, end-start)
			for i := start; i < end; i++ {
				elements[i-start] = &Integer{Value: i}
			}
			
			return &Array{Elements: elements}
		},
	},
	"сортировать": {
		Fn: func(args ...Object) Object {
			if len(args) < 1 || len(args) > 2 {
				return builtinErrorWrongArgCount("сортировать", 1, len(args))
			}
			if args[0].Type() != "ARRAY" {
    return ErrorWithHint(currentCallToken, "первый аргумент должен быть массивом", "Передайте массив в качестве первого аргумента.")
			}
			
			arr := args[0].(*Array)
			sorted := make([]Object, len(arr.Elements))
			copy(sorted, arr.Elements)
			
			// Если передана функция сравнения
			if len(args) == 2 {
				if args[1].Type() != "FUNCTION" {
     return ErrorWithHint(currentCallToken, "второй аргумент должен быть функцией", "Передайте функцию в качестве второго аргумента.")
				}
				
				compareFn := args[1].(*Function)
				
				// Сортировка пузырьком с пользовательским компаратором
				for i := 0; i < len(sorted); i++ {
					for j := i + 1; j < len(sorted); j++ {
						if ApplyFunctionCallback != nil {
							result := ApplyFunctionCallback(compareFn, []Object{sorted[i], sorted[j]})
							
							// Если функция вернула истина, значит нужно поменять местами
							if isTruthy(result) {
								sorted[i], sorted[j] = sorted[j], sorted[i]
							}
						}
					}
				}
			} else {
				// Сортировка по умолчанию
				for i := 0; i < len(sorted); i++ {
					for j := i + 1; j < len(sorted); j++ {
						var less bool
						
						if sorted[i].Type() == "INTEGER" && sorted[j].Type() == "INTEGER" {
							less = sorted[i].(*Integer).Value > sorted[j].(*Integer).Value
						} else if sorted[i].Type() == "FLOAT" && sorted[j].Type() == "FLOAT" {
							less = sorted[i].(*Float).Value > sorted[j].(*Float).Value
						} else if sorted[i].Type() == "STRING" && sorted[j].Type() == "STRING" {
							less = sorted[i].(*String).Value > sorted[j].(*String).Value
						}
						
						if less {
							sorted[i], sorted[j] = sorted[j], sorted[i]
						}
					}
				}
			}
			
			arr.Elements = sorted
			return arr
		},
	},
	"реверс": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return builtinErrorWrongArgCount("реверс", 1, len(args))
			}
			if args[0].Type() != "ARRAY" {
				return builtinErrorWrongArgType("реверс", 1, "ARRAY (массив)", args[0].Type())
			}
			
			arr := args[0].(*Array)
			n := len(arr.Elements)
			
			for i := 0; i < n/2; i++ {
				arr.Elements[i], arr.Elements[n-1-i] = arr.Elements[n-1-i], arr.Elements[i]
			}
			
			return arr
		},
	},
	"найти": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return builtinErrorWrongArgCount("найти", 2, len(args))
			}
			
			// Для строк - найти подстроку
			if args[0].Type() == "STRING" && args[1].Type() == "STRING" {
				str := args[0].(*String).Value
				substr := args[1].(*String).Value
				return &Integer{Value: int64(strings.Index(str, substr))}
			}
			
			// Для массивов - найти элемент по условию
			if args[0].Type() != "ARRAY" || args[1].Type() != "FUNCTION" {
    return ErrorWithHint(currentCallToken, "требуется массив и функция", "Используйте: функция(массив, функция)")
			}
			
			arr := args[0].(*Array)
			fn := args[1].(*Function)
			
			for _, elem := range arr.Elements {
				if ApplyFunctionCallback != nil {
					result := ApplyFunctionCallback(fn, []Object{elem})
					if isTruthy(result) {
						return elem
					}
				}
			}
			
			return NULL
		},
	},
	"найти_индекс": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return builtinErrorWrongArgCount("найти_индекс", 2, len(args))
			}
			if args[0].Type() != "ARRAY" || args[1].Type() != "FUNCTION" {
    return ErrorWithHint(currentCallToken, "аргументы должны быть: массив и функция", "Используйте: функция(массив, функция)")
			}
			
			arr := args[0].(*Array)
			fn := args[1].(*Function)
			
			for i, elem := range arr.Elements {
				if ApplyFunctionCallback != nil {
					result := ApplyFunctionCallback(fn, []Object{elem})
					if isTruthy(result) {
						return &Integer{Value: int64(i)}
					}
				}
			}
			
			return &Integer{Value: -1}
		},
	},
	"все": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return builtinErrorWrongArgCount("все", 2, len(args))
			}
			if args[0].Type() != "ARRAY" || args[1].Type() != "FUNCTION" {
    return ErrorWithHint(currentCallToken, "аргументы должны быть: массив и функция", "Используйте: функция(массив, функция)")
			}
			
			arr := args[0].(*Array)
			fn := args[1].(*Function)
			
			for _, elem := range arr.Elements {
				if ApplyFunctionCallback != nil {
					result := ApplyFunctionCallback(fn, []Object{elem})
					if !isTruthy(result) {
						return FALSE
					}
				}
			}
			
			return TRUE
		},
	},
	"любой": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return builtinErrorWrongArgCount("любой", 2, len(args))
			}
			if args[0].Type() != "ARRAY" || args[1].Type() != "FUNCTION" {
    return ErrorWithHint(currentCallToken, "аргументы должны быть: массив и функция", "Используйте: функция(массив, функция)")
			}
			
			arr := args[0].(*Array)
			fn := args[1].(*Function)
			
			for _, elem := range arr.Elements {
				if ApplyFunctionCallback != nil {
					result := ApplyFunctionCallback(fn, []Object{elem})
					if isTruthy(result) {
						return TRUE
					}
				}
			}
			
			return FALSE
		},
	},
	"сумма": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return builtinErrorWrongArgCount("сумма", 1, len(args))
			}
			if args[0].Type() != "ARRAY" {
				return builtinErrorWrongArgType("сумма", 1, "ARRAY (массив)", args[0].Type())
			}
			
			arr := args[0].(*Array)
			var sum float64 = 0
			
			for _, elem := range arr.Elements {
				switch v := elem.(type) {
				case *Integer:
					sum += float64(v.Value)
				case *Float:
					sum += v.Value
				default:
     return ErrorWithHint(currentCallToken, "массив должен содержать только числа", "Убедитесь, что все элементы массива - числа.")
				}
			}
			
			// Если все элементы были Integer, вернуть Integer
			allIntegers := true
			for _, elem := range arr.Elements {
				if elem.Type() != "INTEGER" {
					allIntegers = false
					break
				}
			}
			
			if allIntegers {
				return &Integer{Value: int64(sum)}
			}
			return &Float{Value: sum}
		},
	},
	"взять": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return builtinErrorWrongArgCount("взять", 2, len(args))
			}
			if args[1].Type() != "INTEGER" {
				return ErrorWithHint(currentCallToken, "второй аргумент должен быть целым числом", "взять(массив|генератор, n)")
			}
			count := int(args[1].(*Integer).Value)
			if count < 0 {
				count = 0
			}
			
			switch v := args[0].(type) {
			case *Array:
				if count > len(v.Elements) {
					count = len(v.Elements)
				}
				return &Array{Elements: append([]Object{}, v.Elements[:count]...)}
			case *Generator:
				result := []Object{}
				for i := 0; i < count; i++ {
					val, ok := v.Next()
					if !ok {
						break
					}
					result = append(result, val)
				}
				v.Close()
				return &Array{Elements: result}
			default:
				return builtinErrorWrongArgType("взять", 1, "массив или генератор", args[0].Type())
			}
		},
	},
	"пропустить": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return builtinErrorWrongArgCount("пропустить", 2, len(args))
			}
			if args[0].Type() != "ARRAY" || args[1].Type() != "INTEGER" {
    return ErrorWithHint(currentCallToken, "аргументы должны быть: массив и целое число", "Используйте: функция(массив, число)")
			}
			
			arr := args[0].(*Array)
			count := int(args[1].(*Integer).Value)
			
			if count < 0 {
				count = 0
			}
			if count > len(arr.Elements) {
				count = len(arr.Elements)
			}
			
			return &Array{Elements: arr.Elements[count:]}
		},
	},
	"загрузить": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return builtinErrorWrongArgCount("загрузить", 1, len(args))
			}
			if args[0].Type() != "STRING" {
				return builtinErrorWrongArgType("загрузить", 1, "STRING (строка)", args[0].Type())
			}
			// Эта функция будет вызываться из eval.go с доступом к env
   return ErrorWithHint(currentCallToken, "внутренняя ошибка: загрузить() вызвана неправильно", "Это внутренняя ошибка интерпретатора.")
		},
	},
	"установить": {
		Fn: func(args ...Object) Object {
			if len(args) != 3 {
				return builtinErrorWrongArgCount("установить", 3, len(args))
			}
			if args[0].Type() != "INSTANCE" {
    return ErrorWithHint(currentCallToken, "первый аргумент должен быть экземпляром класса", "Передайте объект класса.")
			}
			if args[1].Type() != "STRING" {
    return ErrorWithHint(currentCallToken, "второй аргумент должен быть строкой", "Передайте строку в качестве второго аргумента.")
			}
			
			inst := args[0].(*Instance)
			key := args[1].(*String).Value
			inst.Properties[key] = args[2]
			
			return NULL
		},
	},
	"преобразовать": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return builtinErrorWrongArgCount("преобразовать", 2, len(args))
			}
			if args[0].Type() != "ARRAY" {
    return ErrorWithHint(currentCallToken, "первый аргумент должен быть массивом", "Передайте массив в качестве первого аргумента.")
			}
			if args[1].Type() != "FUNCTION" {
    return ErrorWithHint(currentCallToken, "второй аргумент должен быть функцией", "Передайте функцию в качестве второго аргумента.")
			}
			
			arr := args[0].(*Array)
			fn := args[1]
			
			result := make([]Object, len(arr.Elements))
			for i, elem := range arr.Elements {
				fnArgs := []Object{elem, &Integer{Value: int64(i)}}
				evaluated := ApplyFunctionCallback(fn, fnArgs)
				result[i] = evaluated
			}
			
			return &Array{Elements: result}
		},
	},
	"фильтр": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return builtinErrorWrongArgCount("фильтр", 2, len(args))
			}
			if args[0].Type() != "ARRAY" {
    return ErrorWithHint(currentCallToken, "первый аргумент должен быть массивом", "Передайте массив в качестве первого аргумента.")
			}
			if args[1].Type() != "FUNCTION" {
    return ErrorWithHint(currentCallToken, "второй аргумент должен быть функцией", "Передайте функцию в качестве второго аргумента.")
			}
			
			arr := args[0].(*Array)
			fn := args[1]
			
			result := []Object{}
			for i, elem := range arr.Elements {
				fnArgs := []Object{elem, &Integer{Value: int64(i)}}
				evaluated := ApplyFunctionCallback(fn, fnArgs)
				
				if isTruthy(evaluated) {
					result = append(result, elem)
				}
			}
			
			return &Array{Elements: result}
		},
	},
	"дляКаждого": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return builtinErrorWrongArgCount("дляКаждого", 2, len(args))
			}
			if args[0].Type() != "ARRAY" {
    return ErrorWithHint(currentCallToken, "первый аргумент должен быть массивом", "Передайте массив в качестве первого аргумента.")
			}
			if args[1].Type() != "FUNCTION" {
    return ErrorWithHint(currentCallToken, "второй аргумент должен быть функцией", "Передайте функцию в качестве второго аргумента.")
			}
			
			arr := args[0].(*Array)
			fn := args[1]
			
			for i, elem := range arr.Elements {
				fnArgs := []Object{elem, &Integer{Value: int64(i)}}
				ApplyFunctionCallback(fn, fnArgs)
			}
			
			return NULL
		},
	},
	"свернуть": {
		Fn: func(args ...Object) Object {
			if len(args) < 2 || len(args) > 3 {
				return builtinErrorWrongArgCount("свернуть", 2, len(args))
			}
			if args[0].Type() != "ARRAY" {
    return ErrorWithHint(currentCallToken, "первый аргумент должен быть массивом", "Передайте массив в качестве первого аргумента.")
			}
			if args[1].Type() != "FUNCTION" {
    return ErrorWithHint(currentCallToken, "второй аргумент должен быть функцией", "Передайте функцию в качестве второго аргумента.")
			}
			
			arr := args[0].(*Array)
			fn := args[1]
			
			if len(arr.Elements) == 0 {
				if len(args) == 3 {
					return args[2]
				}
				return NULL
			}
			
			var accumulator Object
			startIdx := 0
			
			if len(args) == 3 {
				accumulator = args[2]
			} else {
				accumulator = arr.Elements[0]
				startIdx = 1
			}
			
			for i := startIdx; i < len(arr.Elements); i++ {
				fnArgs := []Object{accumulator, arr.Elements[i], &Integer{Value: int64(i)}}
				accumulator = ApplyFunctionCallback(fn, fnArgs)
			}
			
			return accumulator
		},
	},
	"объединить": {
		Fn: func(args ...Object) Object {
			result := []Object{}
			
			for _, arg := range args {
				if arg.Type() == "ARRAY" {
					arr := arg.(*Array)
					result = append(result, arr.Elements...)
				} else {
					result = append(result, arg)
				}
			}
			
			return &Array{Elements: result}
		},
	},
	// Set-операции на массивах
	"уникальные": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 || args[0].Type() != "ARRAY" {
				return builtinErrorWrongArgType("уникальные", 1, "массив", args[0].Type())
			}
			arr := args[0].(*Array)
			seen := make(map[string]bool)
			result := []Object{}
			for _, el := range arr.Elements {
				key := el.Inspect()
				if !seen[key] {
					seen[key] = true
					result = append(result, el)
				}
			}
			return &Array{Elements: result}
		},
	},
	"объединение": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 || args[0].Type() != "ARRAY" || args[1].Type() != "ARRAY" {
				return ErrorWithHint(currentCallToken, "объединение(a, b) требует двух массивов", "")
			}
			a := args[0].(*Array)
			b := args[1].(*Array)
			seen := make(map[string]bool)
			result := []Object{}
			for _, el := range a.Elements {
				key := el.Inspect()
				if !seen[key] {
					seen[key] = true
					result = append(result, el)
				}
			}
			for _, el := range b.Elements {
				key := el.Inspect()
				if !seen[key] {
					seen[key] = true
					result = append(result, el)
				}
			}
			return &Array{Elements: result}
		},
	},
	"пересечение": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 || args[0].Type() != "ARRAY" || args[1].Type() != "ARRAY" {
				return ErrorWithHint(currentCallToken, "пересечение(a, b) требует двух массивов", "")
			}
			a := args[0].(*Array)
			b := args[1].(*Array)
			inB := make(map[string]bool)
			for _, el := range b.Elements {
				inB[el.Inspect()] = true
			}
			seen := make(map[string]bool)
			result := []Object{}
			for _, el := range a.Elements {
				key := el.Inspect()
				if inB[key] && !seen[key] {
					seen[key] = true
					result = append(result, el)
				}
			}
			return &Array{Elements: result}
		},
	},
	"разность": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 || args[0].Type() != "ARRAY" || args[1].Type() != "ARRAY" {
				return ErrorWithHint(currentCallToken, "разность(a, b) требует двух массивов", "")
			}
			a := args[0].(*Array)
			b := args[1].(*Array)
			inB := make(map[string]bool)
			for _, el := range b.Elements {
				inB[el.Inspect()] = true
			}
			seen := make(map[string]bool)
			result := []Object{}
			for _, el := range a.Elements {
				key := el.Inspect()
				if !inB[key] && !seen[key] {
					seen[key] = true
					result = append(result, el)
				}
			}
			return &Array{Elements: result}
		},
	},
	// Map-операции на хешах
	"получить": {
		Fn: func(args ...Object) Object {
			if len(args) < 2 || len(args) > 3 {
				return ErrorWithHint(currentCallToken, "получить(хеш, ключ[, по_умолчанию])", "")
			}
			hash, ok := args[0].(*Hash)
			if !ok {
				return builtinErrorWrongArgType("получить", 1, "объект", args[0].Type())
			}
			hashable, ok := args[1].(Hashable)
			if !ok {
				return ErrorWithHint(currentCallToken, "ключ должен быть hashable", "")
			}
			if pair, found := hash.Pairs[hashable.HashKey()]; found {
				return pair.Value
			}
			if len(args) == 3 {
				return args[2]
			}
			return NULL
		},
	},
	// Размер - алиас для длина (более привычно для Set/Map)
	"размер": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return builtinErrorWrongArgCount("размер", 1, len(args))
			}
			switch v := args[0].(type) {
			case *Array:
				return &Integer{Value: int64(len(v.Elements))}
			case *Hash:
				return &Integer{Value: int64(len(v.Pairs))}
			case *String:
				return &Integer{Value: int64(len([]rune(v.Value)))}
			default:
				return builtinErrorWrongArgType("размер", 1, "массив/объект/строка", args[0].Type())
			}
		},
	},
	// Тестирование
	"__тест__": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return builtinErrorWrongArgCount("__тест__", 2, len(args))
			}
			name, _ := args[0].(*String)
			fn := args[1]
			result := ApplyFunctionCallback(fn, []Object{})
			if result != nil && result.Type() == "ERROR" {
				fmt.Fprintf(OutputWriter, "  ✗ %s\n    %s\n", name.Value, result.(*Error).Message)
				testFailures++
			} else {
				fmt.Fprintf(OutputWriter, "  ✓ %s\n", name.Value)
				testPasses++
			}
			return NULL
		},
	},
	"__проверить__": {
		Fn: func(args ...Object) Object {
			if len(args) < 1 {
				return ErrorWithHint(currentCallToken, "проверить требует условие", "")
			}
			cond := args[0]
			if cond.Type() == "BOOLEAN" && !cond.(*Boolean).Value {
				return ErrorWithHint(currentCallToken, "проверка не пройдена", "")
			}
			if cond.Type() == "NULL" {
				return ErrorWithHint(currentCallToken, "проверка не пройдена (значение пусто)", "")
			}
			return NULL
		},
	},
	// Асинхронность - ждать все Futures из массива
	"все_ждать": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 || args[0].Type() != "ARRAY" {
				return builtinErrorWrongArgType("все_ждать", 1, "массив futures", args[0].Type())
			}
			arr := args[0].(*Array)
			results := make([]Object, len(arr.Elements))
			for i, el := range arr.Elements {
				if fut, ok := el.(*Future); ok {
					results[i] = fut.Wait()
				} else {
					results[i] = el
				}
			}
			return &Array{Elements: results}
		},
	},
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func nativeToObject(data interface{}) Object {
	switch v := data.(type) {
	case nil:
		return NULL
	case bool:
		if v {
			return TRUE
		}
		return FALSE
	case float64:
		if v == float64(int64(v)) {
			return &Integer{Value: int64(v)}
		}
		return &Float{Value: v}
	case string:
		return &String{Value: v}
	case []interface{}:
		elements := make([]Object, len(v))
		for i, elem := range v {
			elements[i] = nativeToObject(elem)
		}
		return &Array{Elements: elements}
	case map[string]interface{}:
		// Сортируем ключи, чтобы порядок при разборе JSON был
		// стабильным (Go-карта итерируется в случайном порядке).
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		h := NewHash()
		for _, k := range keys {
			h.Set(&String{Value: k}, nativeToObject(v[k]))
		}
		return h
	default:
		return NULL
	}
}

func objectToNative(obj Object) interface{} {
	switch o := obj.(type) {
	case *Integer:
		return o.Value
	case *Float:
		return o.Value
	case *String:
		return o.Value
	case *Boolean:
		return o.Value
	case *Array:
		result := make([]interface{}, len(o.Elements))
		for i, elem := range o.Elements {
			result[i] = objectToNative(elem)
		}
		return result
	case *Hash:
		result := make(map[string]interface{})
		for _, pair := range o.Pairs {
			if key, ok := pair.Key.(*String); ok {
				result[key.Value] = objectToNative(pair.Value)
			}
		}
		return result
	case *Null:
		return nil
	default:
		return nil
	}
}

func toFloat(obj Object) *float64 {
	switch o := obj.(type) {
	case *Integer:
		f := float64(o.Value)
		return &f
	case *Float:
		return &o.Value
	default:
		return nil
	}
}
