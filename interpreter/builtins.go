package interpreter

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

var ApplyFunctionCallback func(Object, []Object) Object

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
				fmt.Print(arg.Inspect())
			}
			fmt.Println()
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
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=1", len(args))
			}

			switch arg := args[0].(type) {
			case *String:
				return &Integer{Value: int64(len(arg.Value))}
			case *Array:
				return &Integer{Value: int64(len(arg.Elements))}
			default:
				return newBuiltinError("аргумент для длина не поддерживается, получен %s", args[0].Type())
			}
		},
	},
	"добавить": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=2", len(args))
			}
			if args[0].Type() != "ARRAY" {
				return newBuiltinError("аргумент для добавить должен быть ARRAY, получен %s", args[0].Type())
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
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=1", len(args))
			}
			return &String{Value: args[0].Type()}
		},
	},
	"округл": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=1", len(args))
			}
			switch arg := args[0].(type) {
			case *Float:
				return &Integer{Value: int64(arg.Value + 0.5)}
			case *Integer:
				return arg
			default:
				return newBuiltinError("аргумент для округл должен быть числом, получен %s", args[0].Type())
			}
		},
	},
	"строка": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=1", len(args))
			}
			return &String{Value: args[0].Inspect()}
		},
	},
	"число": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=1", len(args))
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
				return newBuiltinError("не удалось преобразовать '%s' в число", str)
			}
			return newBuiltinError("аргумент для число должен быть строкой, получен %s", args[0].Type())
		},
	},
	"ключи": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=1", len(args))
			}
			if args[0].Type() != "HASH" {
				return newBuiltinError("аргумент для ключи должен быть HASH, получен %s", args[0].Type())
			}
			hash := args[0].(*Hash)
			keys := make([]Object, 0, len(hash.Pairs))
			for _, pair := range hash.Pairs {
				keys = append(keys, pair.Key)
			}
			return &Array{Elements: keys}
		},
	},
	"значения": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=1", len(args))
			}
			if args[0].Type() != "HASH" {
				return newBuiltinError("аргумент для значения должен быть HASH, получен %s", args[0].Type())
			}
			hash := args[0].(*Hash)
			values := make([]Object, 0, len(hash.Pairs))
			for _, pair := range hash.Pairs {
				values = append(values, pair.Value)
			}
			return &Array{Elements: values}
		},
	},
	"ошибка": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=1", len(args))
			}
			return newBuiltinError(args[0].Inspect())
		},
	},
	"читать_файл": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=1", len(args))
			}
			if args[0].Type() != "STRING" {
				return newBuiltinError("аргумент должен быть STRING, получен %s", args[0].Type())
			}
			path := args[0].(*String).Value
			content, err := os.ReadFile(path)
			if err != nil {
				return newBuiltinError("ошибка чтения файла: %s", err.Error())
			}
			return &String{Value: string(content)}
		},
	},
	"записать_файл": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=2", len(args))
			}
			if args[0].Type() != "STRING" {
				return newBuiltinError("первый аргумент должен быть STRING, получен %s", args[0].Type())
			}
			if args[1].Type() != "STRING" {
				return newBuiltinError("второй аргумент должен быть STRING, получен %s", args[1].Type())
			}
			path := args[0].(*String).Value
			content := args[1].(*String).Value
			err := os.WriteFile(path, []byte(content), 0644)
			if err != nil {
				return newBuiltinError("ошибка записи файла: %s", err.Error())
			}
			return NULL
		},
	},
	"существует_файл": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=1", len(args))
			}
			if args[0].Type() != "STRING" {
				return newBuiltinError("аргумент должен быть STRING, получен %s", args[0].Type())
			}
			path := args[0].(*String).Value
			_, err := os.Stat(path)
			if err == nil {
				return TRUE
			}
			if os.IsNotExist(err) {
				return FALSE
			}
			return newBuiltinError("ошибка проверки файла: %s", err.Error())
		},
	},
	// Строки
	"разделить": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=2", len(args))
			}
			if args[0].Type() != "STRING" || args[1].Type() != "STRING" {
				return newBuiltinError("аргументы должны быть STRING")
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
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=2", len(args))
			}
			if args[0].Type() != "ARRAY" || args[1].Type() != "STRING" {
				return newBuiltinError("первый аргумент должен быть ARRAY, второй STRING")
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
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=3", len(args))
			}
			if args[0].Type() != "STRING" || args[1].Type() != "STRING" || args[2].Type() != "STRING" {
				return newBuiltinError("все аргументы должны быть STRING")
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
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=1", len(args))
			}
			if args[0].Type() != "STRING" {
				return newBuiltinError("аргумент должен быть STRING")
			}
			return &String{Value: strings.ToUpper(args[0].(*String).Value)}
		},
	},
	"нижний": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=1", len(args))
			}
			if args[0].Type() != "STRING" {
				return newBuiltinError("аргумент должен быть STRING")
			}
			return &String{Value: strings.ToLower(args[0].(*String).Value)}
		},
	},
	"подстрока": {
		Fn: func(args ...Object) Object {
			if len(args) != 3 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=3", len(args))
			}
			if args[0].Type() != "STRING" || args[1].Type() != "INTEGER" || args[2].Type() != "INTEGER" {
				return newBuiltinError("аргументы должны быть STRING, INTEGER, INTEGER")
			}
			str := args[0].(*String).Value
			start := int(args[1].(*Integer).Value)
			end := int(args[2].(*Integer).Value)
			if start < 0 || end > len(str) || start > end {
				return newBuiltinError("неверные индексы")
			}
			return &String{Value: str[start:end]}
		},
	},
	"обрезать": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=1", len(args))
			}
			if args[0].Type() != "STRING" {
				return newBuiltinError("аргумент должен быть STRING")
			}
			return &String{Value: strings.TrimSpace(args[0].(*String).Value)}
		},
	},
	"начинается_с": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=2", len(args))
			}
			if args[0].Type() != "STRING" || args[1].Type() != "STRING" {
				return newBuiltinError("аргументы должны быть STRING")
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
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=2", len(args))
			}
			if args[0].Type() != "STRING" || args[1].Type() != "STRING" {
				return newBuiltinError("аргументы должны быть STRING")
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
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=2", len(args))
			}
			if args[0].Type() != "STRING" || args[1].Type() != "STRING" {
				return newBuiltinError("аргументы должны быть STRING")
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
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=2", len(args))
			}
			if args[0].Type() != "STRING" || args[1].Type() != "INTEGER" {
				return newBuiltinError("аргументы должны быть STRING, INTEGER")
			}
			str := args[0].(*String).Value
			count := int(args[1].(*Integer).Value)
			if count < 0 {
				return newBuiltinError("количество повторений не может быть отрицательным")
			}
			return &String{Value: strings.Repeat(str, count)}
		},
	},
	// Массивы
	"удалить": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=2", len(args))
			}
			if args[0].Type() != "ARRAY" || args[1].Type() != "INTEGER" {
				return newBuiltinError("первый аргумент должен быть ARRAY, второй INTEGER")
			}
			arr := args[0].(*Array)
			idx := int(args[1].(*Integer).Value)
			if idx < 0 || idx >= len(arr.Elements) {
				return newBuiltinError("индекс вне диапазона")
			}
			// Изменяем массив напрямую
			arr.Elements = append(arr.Elements[:idx], arr.Elements[idx+1:]...)
			return arr
		},
	},
	"вставить": {
		Fn: func(args ...Object) Object {
			if len(args) != 3 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=3", len(args))
			}
			if args[0].Type() != "ARRAY" || args[1].Type() != "INTEGER" {
				return newBuiltinError("первый аргумент должен быть ARRAY, второй INTEGER")
			}
			arr := args[0].(*Array)
			idx := int(args[1].(*Integer).Value)
			if idx < 0 || idx > len(arr.Elements) {
				return newBuiltinError("индекс вне диапазона")
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
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=2", len(args))
			}
			a := toFloat(args[0])
			b := toFloat(args[1])
			if a == nil || b == nil {
				return newBuiltinError("аргументы должны быть числами")
			}
			return &Float{Value: math.Min(*a, *b)}
		},
	},
	"макс": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=2", len(args))
			}
			a := toFloat(args[0])
			b := toFloat(args[1])
			if a == nil || b == nil {
				return newBuiltinError("аргументы должны быть числами")
			}
			return &Float{Value: math.Max(*a, *b)}
		},
	},
	"степень": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=2", len(args))
			}
			a := toFloat(args[0])
			b := toFloat(args[1])
			if a == nil || b == nil {
				return newBuiltinError("аргументы должны быть числами")
			}
			return &Float{Value: math.Pow(*a, *b)}
		},
	},
	"корень": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=1", len(args))
			}
			a := toFloat(args[0])
			if a == nil {
				return newBuiltinError("аргумент должен быть числом")
			}
			return &Float{Value: math.Sqrt(*a)}
		},
	},
	"абс": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=1", len(args))
			}
			a := toFloat(args[0])
			if a == nil {
				return newBuiltinError("аргумент должен быть числом")
			}
			return &Float{Value: math.Abs(*a)}
		},
	},
	"случайное": {
		Fn: func(args ...Object) Object {
			if len(args) == 0 {
				return &Float{Value: rand.Float64()}
			}
			if len(args) == 2 {
				if args[0].Type() != "INTEGER" || args[1].Type() != "INTEGER" {
					return newBuiltinError("аргументы должны быть INTEGER")
				}
				min := int(args[0].(*Integer).Value)
				max := int(args[1].(*Integer).Value)
				return &Integer{Value: int64(rand.Intn(max-min+1) + min)}
			}
			return newBuiltinError("неверное количество аргументов. получено=%d, нужно=0 или 2", len(args))
		},
	},
	"диапазон": {
		Fn: func(args ...Object) Object {
			if len(args) < 1 || len(args) > 2 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=1 или 2", len(args))
			}
			
			var start, end int64
			
			if len(args) == 1 {
				// диапазон(5) -> [0, 1, 2, 3, 4]
				if args[0].Type() != "INTEGER" {
					return newBuiltinError("аргумент должен быть INTEGER")
				}
				start = 0
				end = args[0].(*Integer).Value
			} else {
				// диапазон(1, 5) -> [1, 2, 3, 4]
				if args[0].Type() != "INTEGER" || args[1].Type() != "INTEGER" {
					return newBuiltinError("аргументы должны быть INTEGER")
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
			if len(args) != 1 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=1", len(args))
			}
			if args[0].Type() != "ARRAY" {
				return newBuiltinError("аргумент должен быть ARRAY")
			}
			
			arr := args[0].(*Array)
			// Создаем копию для сортировки
			sorted := make([]Object, len(arr.Elements))
			copy(sorted, arr.Elements)
			
			// Простая сортировка пузырьком для чисел
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
			
			arr.Elements = sorted
			return arr
		},
	},
	"реверс": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=1", len(args))
			}
			if args[0].Type() != "ARRAY" {
				return newBuiltinError("аргумент должен быть ARRAY")
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
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=2", len(args))
			}
			
			// Для строк - найти подстроку
			if args[0].Type() == "STRING" && args[1].Type() == "STRING" {
				str := args[0].(*String).Value
				substr := args[1].(*String).Value
				return &Integer{Value: int64(strings.Index(str, substr))}
			}
			
			// Для массивов - найти элемент по условию
			if args[0].Type() != "ARRAY" || args[1].Type() != "FUNCTION" {
				return newBuiltinError("для массива нужны ARRAY и FUNCTION")
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
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=2", len(args))
			}
			if args[0].Type() != "ARRAY" || args[1].Type() != "FUNCTION" {
				return newBuiltinError("аргументы должны быть ARRAY и FUNCTION")
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
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=2", len(args))
			}
			if args[0].Type() != "ARRAY" || args[1].Type() != "FUNCTION" {
				return newBuiltinError("аргументы должны быть ARRAY и FUNCTION")
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
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=2", len(args))
			}
			if args[0].Type() != "ARRAY" || args[1].Type() != "FUNCTION" {
				return newBuiltinError("аргументы должны быть ARRAY и FUNCTION")
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
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=1", len(args))
			}
			if args[0].Type() != "ARRAY" {
				return newBuiltinError("аргумент должен быть ARRAY")
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
					return newBuiltinError("массив должен содержать только числа")
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
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=2", len(args))
			}
			if args[0].Type() != "ARRAY" || args[1].Type() != "INTEGER" {
				return newBuiltinError("аргументы должны быть ARRAY и INTEGER")
			}
			
			arr := args[0].(*Array)
			count := int(args[1].(*Integer).Value)
			
			if count < 0 {
				count = 0
			}
			if count > len(arr.Elements) {
				count = len(arr.Elements)
			}
			
			return &Array{Elements: arr.Elements[:count]}
		},
	},
	"пропустить": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=2", len(args))
			}
			if args[0].Type() != "ARRAY" || args[1].Type() != "INTEGER" {
				return newBuiltinError("аргументы должны быть ARRAY и INTEGER")
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
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=1", len(args))
			}
			if args[0].Type() != "STRING" {
				return newBuiltinError("аргумент должен быть STRING")
			}
			// Эта функция будет вызываться из eval.go с доступом к env
			return newBuiltinError("загрузить() должна вызываться через evalLoadCall")
		},
	},
	"http_получить": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=1", len(args))
			}
			if args[0].Type() != "STRING" {
				return newBuiltinError("аргумент должен быть STRING")
			}
			url := args[0].(*String).Value
			
			resp, err := http.Get(url)
			if err != nil {
				return newBuiltinError("ошибка HTTP запроса: %s", err.Error())
			}
			defer resp.Body.Close()
			
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return newBuiltinError("ошибка чтения ответа: %s", err.Error())
			}
			
			return &String{Value: string(body)}
		},
	},
	"http_сервер": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=2 (порт, обработчик)", len(args))
			}
			if args[0].Type() != "INTEGER" {
				return newBuiltinError("порт должен быть INTEGER")
			}
			if args[1].Type() != "FUNCTION" {
				return newBuiltinError("обработчик должен быть FUNCTION")
			}
			
			port := args[0].(*Integer).Value
			handler := args[1].(*Function)
			
			http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				// Создаем объект запроса
				request := &Hash{
					Pairs: map[HashKey]HashPair{
						(&String{Value: "путь"}).HashKey(): {
							Key:   &String{Value: "путь"},
							Value: &String{Value: r.URL.Path},
						},
						(&String{Value: "метод"}).HashKey(): {
							Key:   &String{Value: "метод"},
							Value: &String{Value: r.Method},
						},
					},
				}
				
				// Вызываем обработчик
				if ApplyFunctionCallback != nil {
					result := ApplyFunctionCallback(handler, []Object{request})
					
					// Отправляем ответ
					if result.Type() == "STRING" {
						w.Header().Set("Content-Type", "text/html; charset=utf-8")
						fmt.Fprint(w, result.(*String).Value)
					} else {
						w.Header().Set("Content-Type", "text/plain; charset=utf-8")
						fmt.Fprint(w, result.Inspect())
					}
				}
			})
			
			addr := fmt.Sprintf(":%d", port)
			fmt.Printf("🚀 Сервер запущен на http://localhost:%d\n", port)
			
			if err := http.ListenAndServe(addr, nil); err != nil {
				return newBuiltinError("ошибка запуска сервера: %s", err.Error())
			}
			
			return NULL
		},
	},
	"json_разобрать": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=1", len(args))
			}
			if args[0].Type() != "STRING" {
				return newBuiltinError("аргумент должен быть STRING")
			}
			
			var data interface{}
			err := json.Unmarshal([]byte(args[0].(*String).Value), &data)
			if err != nil {
				return newBuiltinError("ошибка парсинга JSON: %s", err.Error())
			}
			
			return nativeToObject(data)
		},
	},
	"json_создать": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=1", len(args))
			}
			
			native := objectToNative(args[0])
			bytes, err := json.Marshal(native)
			if err != nil {
				return newBuiltinError("ошибка создания JSON: %s", err.Error())
			}
			
			return &String{Value: string(bytes)}
		},
	},
	"установить": {
		Fn: func(args ...Object) Object {
			if len(args) != 3 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=3", len(args))
			}
			if args[0].Type() != "INSTANCE" {
				return newBuiltinError("первый аргумент должен быть INSTANCE")
			}
			if args[1].Type() != "STRING" {
				return newBuiltinError("второй аргумент должен быть STRING")
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
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=2", len(args))
			}
			if args[0].Type() != "ARRAY" {
				return newBuiltinError("первый аргумент должен быть ARRAY")
			}
			if args[1].Type() != "FUNCTION" {
				return newBuiltinError("второй аргумент должен быть FUNCTION")
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
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=2", len(args))
			}
			if args[0].Type() != "ARRAY" {
				return newBuiltinError("первый аргумент должен быть ARRAY")
			}
			if args[1].Type() != "FUNCTION" {
				return newBuiltinError("второй аргумент должен быть FUNCTION")
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
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=2", len(args))
			}
			if args[0].Type() != "ARRAY" {
				return newBuiltinError("первый аргумент должен быть ARRAY")
			}
			if args[1].Type() != "FUNCTION" {
				return newBuiltinError("второй аргумент должен быть FUNCTION")
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
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=2 или 3", len(args))
			}
			if args[0].Type() != "ARRAY" {
				return newBuiltinError("первый аргумент должен быть ARRAY")
			}
			if args[1].Type() != "FUNCTION" {
				return newBuiltinError("второй аргумент должен быть FUNCTION")
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
	"regex_найти": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=2", len(args))
			}
			if args[0].Type() != "STRING" || args[1].Type() != "STRING" {
				return newBuiltinError("аргументы должны быть STRING")
			}
			
			text := args[0].(*String).Value
			pattern := args[1].(*String).Value
			
			re, err := regexp.Compile(pattern)
			if err != nil {
				return newBuiltinError("неверное регулярное выражение: %s", err.Error())
			}
			
			matches := re.FindAllString(text, -1)
			result := make([]Object, len(matches))
			for i, match := range matches {
				result[i] = &String{Value: match}
			}
			
			return &Array{Elements: result}
		},
	},
	"regex_заменить": {
		Fn: func(args ...Object) Object {
			if len(args) != 3 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=3", len(args))
			}
			if args[0].Type() != "STRING" || args[1].Type() != "STRING" || args[2].Type() != "STRING" {
				return newBuiltinError("аргументы должны быть STRING")
			}
			
			text := args[0].(*String).Value
			pattern := args[1].(*String).Value
			replacement := args[2].(*String).Value
			
			re, err := regexp.Compile(pattern)
			if err != nil {
				return newBuiltinError("неверное регулярное выражение: %s", err.Error())
			}
			
			result := re.ReplaceAllString(text, replacement)
			return &String{Value: result}
		},
	},
	"regex_совпадает": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return newBuiltinError("неверное количество аргументов. получено=%d, нужно=2", len(args))
			}
			if args[0].Type() != "STRING" || args[1].Type() != "STRING" {
				return newBuiltinError("аргументы должны быть STRING")
			}
			
			text := args[0].(*String).Value
			pattern := args[1].(*String).Value
			
			re, err := regexp.Compile(pattern)
			if err != nil {
				return newBuiltinError("неверное регулярное выражение: %s", err.Error())
			}
			
			if re.MatchString(text) {
				return TRUE
			}
			return FALSE
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
		pairs := make(map[HashKey]HashPair)
		for key, val := range v {
			keyObj := &String{Value: key}
			valObj := nativeToObject(val)
			pairs[keyObj.HashKey()] = HashPair{Key: keyObj, Value: valObj}
		}
		return &Hash{Pairs: pairs}
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
