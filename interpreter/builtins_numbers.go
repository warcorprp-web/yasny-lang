package interpreter

import (
	"fmt"
	"math"
)

// Числа, типы, длина, диапазон.

func init() {
	builtins["длина"] = &Builtin{
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
	}
	builtins["тип"] = &Builtin{
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return builtinErrorWrongArgCount("тип", 1, len(args))
			}
			return &String{Value: args[0].Type()}
		},
	}
	builtins["округл"] = &Builtin{
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
	}
	builtins["строка"] = &Builtin{
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return builtinErrorWrongArgCount("строка", 1, len(args))
			}
			return &String{Value: args[0].Inspect()}
		},
	}
	builtins["число"] = &Builtin{
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
	}
	builtins["мин"] = &Builtin{
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
	}
	builtins["макс"] = &Builtin{
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
	}
	builtins["степень"] = &Builtin{
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
	}
	builtins["корень"] = &Builtin{
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
	}
	builtins["абс"] = &Builtin{
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return builtinErrorWrongArgCount("абс", 1, len(args))
			}
			// Сохраняем тип: целое остаётся целым, дробное — дробным.
			switch v := args[0].(type) {
			case *Integer:
				if v.Value < 0 {
					return &Integer{Value: -v.Value}
				}
				return v
			case *Float:
				return &Float{Value: math.Abs(v.Value)}
			default:
				return ErrorWithHint(currentCallToken, "аргумент должен быть числом", "Передайте числовое значение (INTEGER или FLOAT).")
			}
		},
	}
	builtins["диапазон"] = &Builtin{
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
	}
	builtins["размер"] = &Builtin{
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
	}
}
