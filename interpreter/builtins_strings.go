package interpreter

import (
	"strings"
)

// Операции со строками.

func init() {
	builtins["разделить"] = &Builtin{
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
	}
	builtins["соединить"] = &Builtin{
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
	}
	builtins["заменить"] = &Builtin{
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
	}
	builtins["верхний"] = &Builtin{
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return builtinErrorWrongArgCount("верхний", 1, len(args))
			}
			if args[0].Type() != "STRING" {
				return builtinErrorWrongArgType("верхний", 1, "STRING (строка)", args[0].Type())
			}
			return &String{Value: strings.ToUpper(args[0].(*String).Value)}
		},
	}
	builtins["нижний"] = &Builtin{
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return builtinErrorWrongArgCount("нижний", 1, len(args))
			}
			if args[0].Type() != "STRING" {
				return builtinErrorWrongArgType("нижний", 1, "STRING (строка)", args[0].Type())
			}
			return &String{Value: strings.ToLower(args[0].(*String).Value)}
		},
	}
	builtins["подстрока"] = &Builtin{
		Fn: func(args ...Object) Object {
			if len(args) != 3 {
				return builtinErrorWrongArgCount("подстрока", 3, len(args))
			}
			if args[0].Type() != "STRING" || args[1].Type() != "INTEGER" || args[2].Type() != "INTEGER" {
				return ErrorWithHint(currentCallToken, "аргументы должны быть: строка, целое число, целое число", "Проверьте типы аргументов.")
			}
			runes := []rune(args[0].(*String).Value)
			start := int(args[1].(*Integer).Value)
			end := int(args[2].(*Integer).Value)
			if start < 0 || end > len(runes) || start > end {
				return ErrorWithHint(currentCallToken, "неверные индексы", "Индексы должны быть в пределах строки и начало <= конец.")
			}
			return &String{Value: string(runes[start:end])}
		},
	}
	builtins["обрезать"] = &Builtin{
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return builtinErrorWrongArgCount("обрезать", 1, len(args))
			}
			if args[0].Type() != "STRING" {
				return builtinErrorWrongArgType("обрезать", 1, "STRING (строка)", args[0].Type())
			}
			return &String{Value: strings.TrimSpace(args[0].(*String).Value)}
		},
	}
	builtins["начинается_с"] = &Builtin{
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
	}
	builtins["заканчивается_на"] = &Builtin{
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
	}
	builtins["содержит"] = &Builtin{
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
	}
	builtins["повторить"] = &Builtin{
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
	}
}
