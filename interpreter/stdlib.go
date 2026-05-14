package interpreter

import (
	"math"
	"time"
)

// stdModules - стандартные модули, доступные глобально без импорта
var stdModules = map[string]*Hash{}

// makeHashFromBuiltins создаёт Hash из набора builtin-функций
func makeHashFromBuiltins(items map[string]func(args ...Object) Object) *Hash {
	pairs := make(map[HashKey]HashPair)
	for name, fn := range items {
		key := &String{Value: name}
		val := &Builtin{Fn: fn}
		pairs[key.HashKey()] = HashPair{Key: key, Value: val}
	}
	return &Hash{Pairs: pairs}
}

// makeHashFromValues - Hash с константами
func makeHashFromValues(items map[string]Object) *Hash {
	pairs := make(map[HashKey]HashPair)
	for name, val := range items {
		key := &String{Value: name}
		pairs[key.HashKey()] = HashPair{Key: key, Value: val}
	}
	return &Hash{Pairs: pairs}
}

func init() {
	// Модуль "время"
	timeFns := map[string]func(args ...Object) Object{
		"сейчас": func(args ...Object) Object {
			return &Integer{Value: time.Now().Unix()}
		},
		"метка": func(args ...Object) Object {
			return &Integer{Value: time.Now().UnixMilli()}
		},
		"метка_нс": func(args ...Object) Object {
			return &Integer{Value: time.Now().UnixNano()}
		},
		"строка": func(args ...Object) Object {
			t := time.Now()
			if len(args) >= 1 {
				if i, ok := args[0].(*Integer); ok {
					t = time.Unix(i.Value, 0)
				}
			}
			format := "2006-01-02 15:04:05"
			if len(args) >= 2 {
				if s, ok := args[1].(*String); ok {
					format = s.Value
				}
			}
			return &String{Value: t.Format(format)}
		},
		"год": func(args ...Object) Object {
			return &Integer{Value: int64(time.Now().Year())}
		},
		"месяц": func(args ...Object) Object {
			return &Integer{Value: int64(time.Now().Month())}
		},
		"день": func(args ...Object) Object {
			return &Integer{Value: int64(time.Now().Day())}
		},
		"час": func(args ...Object) Object {
			return &Integer{Value: int64(time.Now().Hour())}
		},
		"минута": func(args ...Object) Object {
			return &Integer{Value: int64(time.Now().Minute())}
		},
		"секунда": func(args ...Object) Object {
			return &Integer{Value: int64(time.Now().Second())}
		},
		"спать": func(args ...Object) Object {
			if len(args) != 1 {
				return ErrorWithHint(currentCallToken, "время.спать(миллисекунды)", "")
			}
			ms, ok := args[0].(*Integer)
			if !ok {
				return ErrorWithHint(currentCallToken, "ожидается целое число (миллисекунды)", "")
			}
			time.Sleep(time.Duration(ms.Value) * time.Millisecond)
			return NULL
		},
	}
	stdModules["время"] = makeHashFromBuiltins(timeFns)

	// Модуль "мат" - математические константы и функции
	matVals := map[string]Object{
		"пи":          &Float{Value: math.Pi},
		"е":           &Float{Value: math.E},
		"бесконечность": &Float{Value: math.Inf(1)},
	}
	matMod := makeHashFromValues(matVals)
	matFns := map[string]func(args ...Object) Object{
		"sin": func(args ...Object) Object {
			if len(args) != 1 {
				return ErrorWithHint(currentCallToken, "мат.sin(x)", "")
			}
			x := toFloat(args[0])
			if x == nil {
				return ErrorWithHint(currentCallToken, "ожидается число", "")
			}
			return &Float{Value: math.Sin(*x)}
		},
		"cos": func(args ...Object) Object {
			if len(args) != 1 {
				return ErrorWithHint(currentCallToken, "мат.cos(x)", "")
			}
			x := toFloat(args[0])
			if x == nil {
				return ErrorWithHint(currentCallToken, "ожидается число", "")
			}
			return &Float{Value: math.Cos(*x)}
		},
		"tan": func(args ...Object) Object {
			if len(args) != 1 {
				return ErrorWithHint(currentCallToken, "мат.tan(x)", "")
			}
			x := toFloat(args[0])
			if x == nil {
				return ErrorWithHint(currentCallToken, "ожидается число", "")
			}
			return &Float{Value: math.Tan(*x)}
		},
		"лог": func(args ...Object) Object {
			if len(args) != 1 {
				return ErrorWithHint(currentCallToken, "мат.лог(x) - натуральный логарифм", "")
			}
			x := toFloat(args[0])
			if x == nil {
				return ErrorWithHint(currentCallToken, "ожидается число", "")
			}
			return &Float{Value: math.Log(*x)}
		},
		"лог2": func(args ...Object) Object {
			if len(args) != 1 {
				return ErrorWithHint(currentCallToken, "мат.лог2(x)", "")
			}
			x := toFloat(args[0])
			if x == nil {
				return ErrorWithHint(currentCallToken, "ожидается число", "")
			}
			return &Float{Value: math.Log2(*x)}
		},
		"лог10": func(args ...Object) Object {
			if len(args) != 1 {
				return ErrorWithHint(currentCallToken, "мат.лог10(x)", "")
			}
			x := toFloat(args[0])
			if x == nil {
				return ErrorWithHint(currentCallToken, "ожидается число", "")
			}
			return &Float{Value: math.Log10(*x)}
		},
		"пол": func(args ...Object) Object {
			if len(args) != 1 {
				return ErrorWithHint(currentCallToken, "мат.пол(x) - округление вниз", "")
			}
			x := toFloat(args[0])
			if x == nil {
				return ErrorWithHint(currentCallToken, "ожидается число", "")
			}
			return &Float{Value: math.Floor(*x)}
		},
		"потолок": func(args ...Object) Object {
			if len(args) != 1 {
				return ErrorWithHint(currentCallToken, "мат.потолок(x) - округление вверх", "")
			}
			x := toFloat(args[0])
			if x == nil {
				return ErrorWithHint(currentCallToken, "ожидается число", "")
			}
			return &Float{Value: math.Ceil(*x)}
		},
	}
	// Добавляем функции в matMod
	for name, fn := range matFns {
		key := &String{Value: name}
		val := &Builtin{Fn: fn}
		matMod.Pairs[key.HashKey()] = HashPair{Key: key, Value: val}
	}
	stdModules["мат"] = matMod
}
