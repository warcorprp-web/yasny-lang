package interpreter

import (
	"math"
	"sort"
	"time"
)

// stdModules - стандартные модули, доступные глобально без импорта
var stdModules = map[string]*Hash{}

// makeHashFromBuiltins создаёт Hash из набора builtin-функций
func makeHashFromBuiltins(items map[string]func(args ...Object) Object) *Hash {
	h := NewHash()
	// Сортировка имён даёт стабильный порядок ключей в модулях,
	// собранных из карты Go (порядок итерации карты случайный).
	names := make([]string, 0, len(items))
	for name := range items {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		h.Set(&String{Value: name}, &Builtin{Fn: items[name]})
	}
	return h
}

// makeHashFromValues - Hash с константами
func makeHashFromValues(items map[string]Object) *Hash {
	h := NewHash()
	names := make([]string, 0, len(items))
	for name := range items {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		h.Set(&String{Value: name}, items[name])
	}
	return h
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
	matFnNames := make([]string, 0, len(matFns))
	for name := range matFns {
		matFnNames = append(matFnNames, name)
	}
	sort.Strings(matFnNames)
	for _, name := range matFnNames {
		matMod.Set(&String{Value: name}, &Builtin{Fn: matFns[name]})
	}
	stdModules["мат"] = matMod
}
