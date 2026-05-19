package interpreter

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
)

// === Модуль "json" ===

func registerJsonModule() {
	fns := map[string]func(args ...Object) Object{
		"разобрать": func(args ...Object) Object {
			if len(args) != 1 {
				return builtinErrorWrongArgCount("json.разобрать", 1, len(args))
			}
			s, ok := args[0].(*String)
			if !ok {
				return builtinErrorWrongArgType("json.разобрать", 1, "STRING (строка)", string(args[0].Type()))
			}
			var data interface{}
			if err := json.Unmarshal([]byte(s.Value), &data); err != nil {
				return ErrorInvalidJSON(currentCallToken)
			}
			return nativeToObject(data)
		},
		"создать": func(args ...Object) Object {
			if len(args) != 1 {
				return builtinErrorWrongArgCount("json.создать", 1, len(args))
			}
			return &String{Value: objectToJSON(args[0])}
		},
	}
	stdModules["json"] = makeHashFromBuiltins(fns)
}

// === Модуль "регвыр" — регулярные выражения ===

func registerRegexModule() {
	fns := map[string]func(args ...Object) Object{
		"найти": func(args ...Object) Object {
			if len(args) != 2 {
				return builtinErrorWrongArgCount("регвыр.найти", 2, len(args))
			}
			text, ok1 := args[0].(*String)
			pat, ok2 := args[1].(*String)
			if !ok1 || !ok2 {
				return ErrorWithHint(currentCallToken,
					"регвыр.найти(строка, шаблон)",
					"Оба аргумента должны быть строками.")
			}
			re, err := regexp.Compile(pat.Value)
			if err != nil {
				return ErrorWithHint(currentCallToken,
					fmt.Sprintf("неверное регулярное выражение: %s", err.Error()),
					"Проверьте синтаксис шаблона.")
			}
			matches := re.FindAllString(text.Value, -1)
			out := make([]Object, len(matches))
			for i, m := range matches {
				out[i] = &String{Value: m}
			}
			return &Array{Elements: out}
		},
		"заменить": func(args ...Object) Object {
			if len(args) != 3 {
				return builtinErrorWrongArgCount("регвыр.заменить", 3, len(args))
			}
			text, ok1 := args[0].(*String)
			pat, ok2 := args[1].(*String)
			rep, ok3 := args[2].(*String)
			if !ok1 || !ok2 || !ok3 {
				return ErrorWithHint(currentCallToken,
					"регвыр.заменить(строка, шаблон, замена)",
					"Все аргументы должны быть строками.")
			}
			re, err := regexp.Compile(pat.Value)
			if err != nil {
				return ErrorWithHint(currentCallToken,
					fmt.Sprintf("неверное регулярное выражение: %s", err.Error()),
					"Проверьте синтаксис шаблона.")
			}
			return &String{Value: re.ReplaceAllString(text.Value, rep.Value)}
		},
		"совпадает": func(args ...Object) Object {
			if len(args) != 2 {
				return builtinErrorWrongArgCount("регвыр.совпадает", 2, len(args))
			}
			text, ok1 := args[0].(*String)
			pat, ok2 := args[1].(*String)
			if !ok1 || !ok2 {
				return ErrorWithHint(currentCallToken,
					"регвыр.совпадает(строка, шаблон)",
					"Оба аргумента должны быть строками.")
			}
			re, err := regexp.Compile(pat.Value)
			if err != nil {
				return ErrorWithHint(currentCallToken,
					fmt.Sprintf("неверное регулярное выражение: %s", err.Error()),
					"Проверьте синтаксис шаблона.")
			}
			if re.MatchString(text.Value) {
				return TRUE
			}
			return FALSE
		},
	}
	stdModules["регвыр"] = makeHashFromBuiltins(fns)
}

// extendHttpModule добавляет функцию сервер в существующий модуль http
func extendHttpModule() {
	mod, ok := stdModules["http"]
	if !ok {
		return
	}
	server := &Builtin{Fn: func(args ...Object) Object {
		if len(args) != 2 {
			return builtinErrorWrongArgCount("http.сервер", 2, len(args))
		}
		port, ok := args[0].(*Integer)
		if !ok {
			return ErrorWithHint(currentCallToken,
				"первый аргумент http.сервер должен быть целым числом (порт)",
				"Используйте: http.сервер(8080, обработчик)")
		}
		handler := args[1]
		if handler.Type() != "FUNCTION" {
			return ErrorWithHint(currentCallToken,
				"второй аргумент http.сервер должен быть функцией",
				"Передайте функцию-обработчик: http.сервер(8080, обработать)")
		}

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			request := NewHash()
			request.Set(&String{Value: "путь"}, &String{Value: r.URL.Path})
			request.Set(&String{Value: "метод"}, &String{Value: r.Method})
			if ApplyFunctionCallback != nil {
				result := ApplyFunctionCallback(handler, []Object{request})
				if s, ok := result.(*String); ok {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					fmt.Fprint(w, s.Value)
				} else {
					w.Header().Set("Content-Type", "text/plain; charset=utf-8")
					fmt.Fprint(w, result.Inspect())
				}
			}
		})

		addr := fmt.Sprintf(":%d", port.Value)
		fmt.Printf("Сервер запущен на http://localhost:%d\n", port.Value)
		if err := http.ListenAndServe(addr, mux); err != nil {
			return ErrorWithHint(currentCallToken,
				fmt.Sprintf("не удалось запустить сервер: %s", err.Error()),
				"Проверьте, что порт свободен и доступен.")
		}
		return NULL
	}}
	mod.Set(&String{Value: "сервер"}, server)
}

func init() {
	registerJsonModule()
	registerRegexModule()
	extendHttpModule()
	registerDBModule()
	registerPostgresInDB()
	registerWebSocketModule()
}
