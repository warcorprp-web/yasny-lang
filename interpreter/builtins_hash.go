package interpreter

// Доступ к словарям.

func init() {
	builtins["ключи"] = &Builtin{
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
	}
	builtins["значения"] = &Builtin{
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
	}
	builtins["получить"] = &Builtin{
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
	}
}
