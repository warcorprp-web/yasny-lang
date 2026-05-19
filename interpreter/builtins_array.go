package interpreter

import (
	"strings"
)

// Операции с массивами и функции высшего порядка.

func init() {
	builtins["добавить"] = &Builtin{
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
	}
	builtins["удалить"] = &Builtin{
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
	}
	builtins["вставить"] = &Builtin{
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
	}
	builtins["сортировать"] = &Builtin{
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
	}
	builtins["реверс"] = &Builtin{
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
	}
	builtins["найти"] = &Builtin{
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
	}
	builtins["найти_индекс"] = &Builtin{
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
	}
	builtins["все"] = &Builtin{
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
	}
	builtins["любой"] = &Builtin{
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
	}
	builtins["сумма"] = &Builtin{
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
	}
	builtins["взять"] = &Builtin{
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
	}
	builtins["пропустить"] = &Builtin{
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
	}
	builtins["преобразовать"] = &Builtin{
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
	}
	builtins["фильтр"] = &Builtin{
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
	}
	builtins["дляКаждого"] = &Builtin{
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
	}
	builtins["свернуть"] = &Builtin{
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
	}
	builtins["объединить"] = &Builtin{
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
	}
}
