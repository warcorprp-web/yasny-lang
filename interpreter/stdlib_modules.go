package interpreter

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// === Модуль "json" ===

// jsonParseString парсит JSON-строку в объект Ясного.
func jsonParseString(s string) Object {
	var data interface{}
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return ErrorInvalidJSON(currentCallToken)
	}
	return nativeToObject(data)
}

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
			return jsonParseString(s.Value)
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

// extendHttpModule добавляет серверные функции в модуль http.
// Новый API:
//   http.сервер(порт, обработчик) — простой сервер (обратная совместимость)
//   http.приложение() → app с методами маршрут/получить/пост/запустить
func extendHttpModule() {
	mod, ok := stdModules["http"]
	if !ok {
		return
	}

	// Простой сервер (обратная совместимость)
	mod.Set(&String{Value: "сервер"}, &Builtin{Fn: func(args ...Object) Object {
		if len(args) != 2 {
			return builtinErrorWrongArgCount("http.сервер", 2, len(args))
		}
		port, ok := args[0].(*Integer)
		if !ok {
			return ErrorWithHint(currentCallToken, "порт должен быть числом", "http.сервер(8080, обработчик)")
		}
		handler := args[1]
		if handler.Type() != "FUNCTION" {
			return ErrorWithHint(currentCallToken, "второй аргумент — функция", "")
		}

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			req := buildRequestHash(r)
			result := ApplyFunctionCallback(handler, []Object{req})
			writeResponse(w, result)
		})

		addr := fmt.Sprintf(":%d", port.Value)
		fmt.Printf("Сервер запущен на http://localhost:%d\n", port.Value)
		if err := http.ListenAndServe(addr, mux); err != nil {
			return ErrorWithHint(currentCallToken, "ошибка сервера: "+err.Error(), "")
		}
		return NULL
	}})

	// Приложение с routing
	mod.Set(&String{Value: "приложение"}, &Builtin{Fn: func(args ...Object) Object {
		return newHTTPApp()
	}})
}

// buildRequestHash создаёт полный объект запроса из http.Request.
func buildRequestHash(r *http.Request) *Hash {
	req := NewHash()
	req.Set(&String{Value: "метод"}, &String{Value: r.Method})
	req.Set(&String{Value: "путь"}, &String{Value: r.URL.Path})
	req.Set(&String{Value: "url"}, &String{Value: r.URL.String()})

	// Query params
	params := NewHash()
	for k, v := range r.URL.Query() {
		if len(v) == 1 {
			params.Set(&String{Value: k}, &String{Value: v[0]})
		} else {
			elems := make([]Object, len(v))
			for i, s := range v {
				elems[i] = &String{Value: s}
			}
			params.Set(&String{Value: k}, &Array{Elements: elems})
		}
	}
	req.Set(&String{Value: "параметры"}, params)

	// Headers
	hdrs := NewHash()
	for k, v := range r.Header {
		hdrs.Set(&String{Value: strings.ToLower(k)}, &String{Value: strings.Join(v, ", ")})
	}
	req.Set(&String{Value: "заголовки"}, hdrs)

	// Cookies
	cookies := NewHash()
	for _, c := range r.Cookies() {
		cookies.Set(&String{Value: c.Name}, &String{Value: c.Value})
	}
	req.Set(&String{Value: "куки"}, cookies)

	// Body
	if r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		if err == nil && len(bodyBytes) > 0 {
			req.Set(&String{Value: "тело"}, &String{Value: string(bodyBytes)})
			// Автопарсинг JSON
			ct := r.Header.Get("Content-Type")
			if strings.Contains(ct, "application/json") {
				parsed := jsonParseString(string(bodyBytes))
				if parsed != nil && !isError(parsed) {
					req.Set(&String{Value: "json"}, parsed)
				}
			}
			// Автопарсинг form data
			if strings.Contains(ct, "application/x-www-form-urlencoded") {
				formData := NewHash()
				pairs := strings.Split(string(bodyBytes), "&")
				for _, pair := range pairs {
					kv := strings.SplitN(pair, "=", 2)
					if len(kv) == 2 {
						key, _ := url.QueryUnescape(kv[0])
						val, _ := url.QueryUnescape(kv[1])
						formData.Set(&String{Value: key}, &String{Value: val})
					}
				}
				req.Set(&String{Value: "форма"}, formData)
			}
		} else {
			req.Set(&String{Value: "тело"}, &String{Value: ""})
		}
	}

	return req
}

// writeResponse записывает ответ из объекта Ясного в http.ResponseWriter.
// Поддерживает:
//   - строка → text/html
//   - hash с полями {статус, тело, заголовки, тип} → полный контроль
//   - hash/array без поля "статус" → JSON 200
func writeResponse(w http.ResponseWriter, result Object) {
	if result == nil || result == NULL {
		w.WriteHeader(204)
		return
	}

	switch r := result.(type) {
	case *String:
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(r.Value))
	case *Hash:
		// Проверяем — это объект ответа или данные?
		if statusPair, ok := r.Pairs[(&String{Value: "статус"}).HashKey()]; ok {
			// Объект ответа: {статус: 200, тело: "...", заголовки: {...}, тип: "..."}
			status := 200
			if s, ok := statusPair.Value.(*Integer); ok {
				status = int(s.Value)
			}

			// Заголовки
			if hdrPair, ok := r.Pairs[(&String{Value: "заголовки"}).HashKey()]; ok {
				if hdrHash, ok := hdrPair.Value.(*Hash); ok {
					for _, p := range hdrHash.orderedPairs() {
						if k, ok := p.Key.(*String); ok {
							if v, ok := p.Value.(*String); ok {
								w.Header().Set(k.Value, v.Value)
							}
						}
					}
				}
			}

			// Content-Type
			if typePair, ok := r.Pairs[(&String{Value: "тип"}).HashKey()]; ok {
				if t, ok := typePair.Value.(*String); ok {
					w.Header().Set("Content-Type", t.Value)
				}
			}
			if w.Header().Get("Content-Type") == "" {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
			}

			// Cookies
			if cookiePair, ok := r.Pairs[(&String{Value: "куки"}).HashKey()]; ok {
				if cookieHash, ok := cookiePair.Value.(*Hash); ok {
					for _, p := range cookieHash.orderedPairs() {
						if k, ok := p.Key.(*String); ok {
							cookie := &http.Cookie{Name: k.Value, Path: "/"}
							if v, ok := p.Value.(*String); ok {
								cookie.Value = v.Value
							} else if p.Value == NULL {
								cookie.MaxAge = -1 // удалить
							} else if h, ok := p.Value.(*Hash); ok {
								// {значение: "x", путь: "/", макс_возраст: 3600, http_only: да}
								if vp, ok := h.Pairs[(&String{Value: "значение"}).HashKey()]; ok {
									if vs, ok := vp.Value.(*String); ok {
										cookie.Value = vs.Value
									}
								}
								if pp, ok := h.Pairs[(&String{Value: "путь"}).HashKey()]; ok {
									if ps, ok := pp.Value.(*String); ok {
										cookie.Path = ps.Value
									}
								}
								if mp, ok := h.Pairs[(&String{Value: "макс_возраст"}).HashKey()]; ok {
									if mi, ok := mp.Value.(*Integer); ok {
										cookie.MaxAge = int(mi.Value)
									}
								}
								if hp, ok := h.Pairs[(&String{Value: "http_only"}).HashKey()]; ok {
									cookie.HttpOnly = hp.Value == TRUE
								}
							}
							http.SetCookie(w, cookie)
						}
					}
				}
			}

			w.WriteHeader(status)

			// Тело
			if bodyPair, ok := r.Pairs[(&String{Value: "тело"}).HashKey()]; ok {
				if s, ok := bodyPair.Value.(*String); ok {
					w.Write([]byte(s.Value))
				} else {
					w.Write([]byte(objectToJSON(bodyPair.Value)))
				}
			}
		} else {
			// Просто данные → JSON
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Write([]byte(objectToJSON(r)))
		}
	case *Array:
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write([]byte(objectToJSON(r)))
	default:
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(result.Inspect()))
	}
}

// === HTTP-приложение с routing ===

type httpRoute struct {
	method  string
	path    string
	handler Object
}

func newHTTPApp() *Hash {
	routes := &[]httpRoute{}
	middlewares := &[]Object{}

	app := NewHash()

	// Регистрация маршрутов
	addRoute := func(method string) *Builtin {
		return &Builtin{Fn: func(args ...Object) Object {
			if len(args) < 2 {
				return ErrorWithHint(currentCallToken, method+"(путь, обработчик)", "")
			}
			path, ok := args[0].(*String)
			if !ok {
				return ErrorWithHint(currentCallToken, "путь должен быть строкой", "")
			}
			*routes = append(*routes, httpRoute{method: method, path: path.Value, handler: args[1]})
			return app
		}}
	}

	app.Set(&String{Value: "получить"}, addRoute("GET"))
	app.Set(&String{Value: "пост"}, addRoute("POST"))
	app.Set(&String{Value: "положить"}, addRoute("PUT"))
	app.Set(&String{Value: "удалить"}, addRoute("DELETE"))
	app.Set(&String{Value: "патч"}, addRoute("PATCH"))
	app.Set(&String{Value: "маршрут"}, &Builtin{Fn: func(args ...Object) Object {
		if len(args) < 3 {
			return ErrorWithHint(currentCallToken, "маршрут(метод, путь, обработчик)", "")
		}
		m, ok := args[0].(*String)
		if !ok {
			return ErrorWithHint(currentCallToken, "метод должен быть строкой", "")
		}
		p, ok := args[1].(*String)
		if !ok {
			return ErrorWithHint(currentCallToken, "путь должен быть строкой", "")
		}
		*routes = append(*routes, httpRoute{method: strings.ToUpper(m.Value), path: p.Value, handler: args[2]})
		return app
	}})

	// Middleware
	app.Set(&String{Value: "использовать"}, &Builtin{Fn: func(args ...Object) Object {
		for _, a := range args {
			if a.Type() == "FUNCTION" || a.Type() == "BUILTIN" {
				*middlewares = append(*middlewares, a)
			}
		}
		return app
	}})

	// Статические файлы
	app.Set(&String{Value: "статика"}, &Builtin{Fn: func(args ...Object) Object {
		if len(args) < 2 {
			return ErrorWithHint(currentCallToken, "статика(url_путь, папка)", "статика(\"/static\", \"./public\")")
		}
		urlPath, _ := args[0].(*String)
		dirPath, _ := args[1].(*String)
		if urlPath == nil || dirPath == nil {
			return ErrorWithHint(currentCallToken, "аргументы должны быть строками", "")
		}
		*routes = append(*routes, httpRoute{method: "__STATIC__", path: urlPath.Value, handler: &String{Value: dirPath.Value}})
		return app
	}})

	// Запуск
	app.Set(&String{Value: "запустить"}, &Builtin{Fn: func(args ...Object) Object {
		if len(args) < 1 {
			return ErrorWithHint(currentCallToken, "запустить(порт)", "")
		}
		port, ok := args[0].(*Integer)
		if !ok {
			return ErrorWithHint(currentCallToken, "порт должен быть числом", "")
		}

		mux := http.NewServeMux()

		// Группируем маршруты по пути (один HandleFunc на путь)
		pathHandlers := map[string][]httpRoute{}
		for _, route := range *routes {
			if route.method == "__STATIC__" {
				dir := route.handler.(*String).Value
				fs := http.FileServer(http.Dir(dir))
				prefix := route.path
				mux.Handle(prefix, http.StripPrefix(prefix, fs))
				continue
			}
			pathHandlers[route.path] = append(pathHandlers[route.path], route)
		}

		for path, handlers := range pathHandlers {
			localHandlers := handlers
			mux.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
				// Ищем обработчик для метода
				var matched *httpRoute
				for i := range localHandlers {
					if localHandlers[i].method == "" || localHandlers[i].method == req.Method {
						matched = &localHandlers[i]
						break
					}
				}
				if matched == nil {
					w.WriteHeader(405)
					w.Write([]byte("Метод не разрешён"))
					return
				}

				reqHash := buildRequestHash(req)

				// Middleware
				for _, mw := range *middlewares {
					res := ApplyFunctionCallback(mw, []Object{reqHash})
					if isError(res) || res == FALSE {
						w.WriteHeader(403)
						return
					}
				}

				result := ApplyFunctionCallback(matched.handler, []Object{reqHash})
				writeResponse(w, result)
			})
		}

		addr := fmt.Sprintf(":%d", port.Value)
		fmt.Printf("HTTP-сервер запущен на http://localhost:%d\n", port.Value)
		if err := http.ListenAndServe(addr, mux); err != nil {
			return ErrorWithHint(currentCallToken, "ошибка: "+err.Error(), "")
		}
		return NULL
	}})

	return app
}

func init() {
	registerJsonModule()
	registerRegexModule()
	registerDBModule()
	registerPostgresInDB()
	registerWebSocketModule()
	registerTemplateModule()
}
