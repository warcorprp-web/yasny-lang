package interpreter

// Множественные операции на массивах.

func init() {
	builtins["уникальные"] = &Builtin{
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
	}
	builtins["объединение"] = &Builtin{
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
	}
	builtins["пересечение"] = &Builtin{
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
	}
	builtins["разность"] = &Builtin{
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
	}
}
