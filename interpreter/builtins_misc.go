package interpreter

import (
	"fmt"
)

// Прочее: ошибки, импорт, тесты, async.

func init() {
	builtins["ошибка"] = &Builtin{
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
	}
	builtins["загрузить"] = &Builtin{
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
	}
	builtins["установить"] = &Builtin{
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
	}
	builtins["__тест__"] = &Builtin{
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
	}
	builtins["__проверить__"] = &Builtin{
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
	}
	builtins["все_ждать"] = &Builtin{
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
	}
}
