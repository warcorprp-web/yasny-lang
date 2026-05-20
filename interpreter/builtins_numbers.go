package interpreter

import (
	"fmt"
	"math"
)

// flattenNumericArgs принимает список аргументов и превращает в []float64.
// Поддерживает три формы:
//   мин(1, 2, 3)        — varargs
//   мин([1, 2, 3])      — один массив
//   мин(1, 2, [3, 4])   — смешанно
//
// Возвращает []float64 при успехе, allInts (true если все элементы были Integer)
// или Object (ошибку) при сбое.
func flattenNumericArgs(args []Object, fnName string) (nums []float64, allInts bool, err Object) {
	allInts = true
	for i, arg := range args {
		if arr, ok := arg.(*Array); ok {
			for j, el := range arr.Elements {
				if _, isInt := el.(*Integer); !isInt {
					allInts = false
				}
				v := toFloat(el)
				if v == nil {
					return nil, false, ErrorWithHint(currentCallToken,
						fmt.Sprintf("функция '%s': элемент %d массива не число", fnName, j),
						"Все элементы должны быть INTEGER или FLOAT.")
				}
				nums = append(nums, *v)
			}
			continue
		}
		if _, isInt := arg.(*Integer); !isInt {
			allInts = false
		}
		v := toFloat(arg)
		if v == nil {
			return nil, false, ErrorWithHint(currentCallToken,
				fmt.Sprintf("функция '%s': аргумент %d не число", fnName, i+1),
				"Передавайте числа или массивы чисел.")
		}
		nums = append(nums, *v)
	}
	return nums, allInts, nil
}

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
			case *Hash:
				return &Integer{Value: int64(len(arg.Pairs))}
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
			nums, allInts, err := flattenNumericArgs(args, "мин")
			if err != nil {
				return err
			}
			if len(nums) == 0 {
				return ErrorWithHint(currentCallToken, "функция 'мин' требует хотя бы одно число", "Используйте: мин(1, 2, 3) или мин([1, 2, 3])")
			}
			result := nums[0]
			for _, v := range nums[1:] {
				if v < result {
					result = v
				}
			}
			if allInts {
				return &Integer{Value: int64(result)}
			}
			return &Float{Value: result}
		},
	}
	builtins["макс"] = &Builtin{
		Fn: func(args ...Object) Object {
			nums, allInts, err := flattenNumericArgs(args, "макс")
			if err != nil {
				return err
			}
			if len(nums) == 0 {
				return ErrorWithHint(currentCallToken, "функция 'макс' требует хотя бы одно число", "Используйте: макс(1, 2, 3) или макс([1, 2, 3])")
			}
			result := nums[0]
			for _, v := range nums[1:] {
				if v > result {
					result = v
				}
			}
			if allInts {
				return &Integer{Value: int64(result)}
			}
			return &Float{Value: result}
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
			if len(args) < 1 || len(args) > 3 {
				return builtinErrorWrongArgCount("диапазон", 1, len(args))
			}

			var start, end, step int64
			step = 1

			if len(args) == 1 {
				// диапазон(5) -> [0, 1, 2, 3, 4]
				if args[0].Type() != "INTEGER" {
					return builtinErrorWrongArgType("диапазон", 1, "INTEGER (целое число)", args[0].Type())
				}
				start = 0
				end = args[0].(*Integer).Value
			} else {
				// диапазон(нач, кон) или диапазон(нач, кон, шаг)
				if args[0].Type() != "INTEGER" || args[1].Type() != "INTEGER" {
					return ErrorWithHint(currentCallToken, "аргументы должны быть целыми числами", "Передайте целые числа (INTEGER).")
				}
				start = args[0].(*Integer).Value
				end = args[1].(*Integer).Value
				if len(args) == 3 {
					if args[2].Type() != "INTEGER" {
						return builtinErrorWrongArgType("диапазон", 3, "INTEGER (целое число)", args[2].Type())
					}
					step = args[2].(*Integer).Value
					if step == 0 {
						return ErrorWithHint(currentCallToken, "шаг не может быть 0", "Используйте положительный или отрицательный шаг.")
					}
				}
			}

			elements := []Object{}
			if step > 0 {
				for i := start; i < end; i += step {
					elements = append(elements, &Integer{Value: i})
				}
			} else {
				for i := start; i > end; i += step {
					elements = append(elements, &Integer{Value: i})
				}
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
