package interpreter

import (
	"os"
	"strings"
)

// === Модуль "шаблон" ===
//
// Шаблонизатор для генерации HTML/текста.
// Синтаксис:
//   {{выражение}}           — вставка значения
//   {{если условие}}...{{конец}}
//   {{если условие}}...{{иначе}}...{{конец}}
//   {{для элемент в массив}}...{{конец}}
//   {{включить "файл.html"}}
//
// Использование:
//   конст html = шаблон.рендер("<h1>{{имя}}</h1>", {"имя": "Аня"})
//   конст html = шаблон.файл("шаблон.html", {"товары": [...]})

func registerTemplateModule() {
	fns := map[string]func(args ...Object) Object{
		"рендер": func(args ...Object) Object {
			if len(args) < 1 {
				return ErrorWithHint(currentCallToken, "рендер(шаблон, [данные])", "")
			}
			tmpl, ok := args[0].(*String)
			if !ok {
				return ErrorWithHint(currentCallToken, "первый аргумент — строка шаблона", "")
			}
			var data *Hash
			if len(args) >= 2 {
				if h, ok := args[1].(*Hash); ok {
					data = h
				}
			}
			result, err := renderTemplate(tmpl.Value, data, "")
			if err != nil {
				return ErrorWithHint(currentCallToken, err.Error(), "")
			}
			return &String{Value: result}
		},
		"файл": func(args ...Object) Object {
			if len(args) < 1 {
				return ErrorWithHint(currentCallToken, "файл(путь, [данные])", "")
			}
			path, ok := args[0].(*String)
			if !ok {
				return ErrorWithHint(currentCallToken, "первый аргумент — путь к файлу", "")
			}
			content, e := os.ReadFile(path.Value)
			if e != nil {
				return ErrorWithHint(currentCallToken, "не удалось прочитать шаблон: "+e.Error(), "")
			}
			var data *Hash
			if len(args) >= 2 {
				if h, ok := args[1].(*Hash); ok {
					data = h
				}
			}
			baseDir := path.Value
			if idx := strings.LastIndexAny(baseDir, "/\\"); idx >= 0 {
				baseDir = baseDir[:idx]
			} else {
				baseDir = "."
			}
			result, err := renderTemplate(string(content), data, baseDir)
			if err != nil {
				return ErrorWithHint(currentCallToken, err.Error(), "")
			}
			return &String{Value: result}
		},
		"экранировать": func(args ...Object) Object {
			if len(args) < 1 {
				return &String{Value: ""}
			}
			s, ok := args[0].(*String)
			if !ok {
				return &String{Value: args[0].Inspect()}
			}
			return &String{Value: escapeHTML(s.Value)}
		},
	}
	stdModules["шаблон"] = makeHashFromBuiltins(fns)
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

// renderTemplate обрабатывает шаблон с данными.
func renderTemplate(tmpl string, data *Hash, baseDir string) (string, error) {
	var result strings.Builder
	i := 0
	for i < len(tmpl) {
		// Ищем {{
		if i+1 < len(tmpl) && tmpl[i] == '{' && tmpl[i+1] == '{' {
			// Находим закрывающий }}
			end := strings.Index(tmpl[i+2:], "}}")
			if end == -1 {
				result.WriteString(tmpl[i:])
				break
			}
			tag := strings.TrimSpace(tmpl[i+2 : i+2+end])
			i = i + 2 + end + 2

			// Обработка тегов
			if strings.HasPrefix(tag, "если ") {
				cond := strings.TrimPrefix(tag, "если ")
				body, elseBody, newI := extractBlock(tmpl, i)
				i = newI
				if evalCondition(cond, data) {
					rendered, _ := renderTemplate(body, data, baseDir)
					result.WriteString(rendered)
				} else if elseBody != "" {
					rendered, _ := renderTemplate(elseBody, data, baseDir)
					result.WriteString(rendered)
				}
			} else if strings.HasPrefix(tag, "для ") {
				// {{для элемент в массив}}
				parts := strings.SplitN(tag, " в ", 2)
				varName := strings.TrimPrefix(parts[0], "для ")
				varName = strings.TrimSpace(varName)
				arrExpr := ""
				if len(parts) == 2 {
					arrExpr = strings.TrimSpace(parts[1])
				}
				body, _, newI := extractBlock(tmpl, i)
				i = newI
				arr := resolveValue(arrExpr, data)
				if arrObj, ok := arr.(*Array); ok {
					for _, elem := range arrObj.Elements {
						// Создаём копию данных с переменной цикла
						loopData := copyHash(data)
						loopData.Set(&String{Value: varName}, elem)
						rendered, _ := renderTemplate(body, loopData, baseDir)
						result.WriteString(rendered)
					}
				}
			} else if strings.HasPrefix(tag, "включить ") {
				// {{включить "файл.html"}}
				fileName := strings.Trim(strings.TrimPrefix(tag, "включить "), "\"' ")
				path := fileName
				if baseDir != "" {
					path = baseDir + "/" + fileName
				}
				content, e := os.ReadFile(path)
				if e == nil {
					rendered, _ := renderTemplate(string(content), data, baseDir)
					result.WriteString(rendered)
				}
			} else {
				// Выражение — вставка значения
				val := resolveValue(tag, data)
				if val != nil && val != NULL {
					result.WriteString(val.Inspect())
				}
			}
		} else {
			result.WriteByte(tmpl[i])
			i++
		}
	}
	return result.String(), nil
}

// extractBlock извлекает тело блока до {{конец}} или {{иначе}}...{{конец}}.
func extractBlock(tmpl string, start int) (body string, elseBody string, end int) {
	depth := 1
	i := start
	bodyStart := start
	elseStart := -1

	for i < len(tmpl) {
		if i+1 < len(tmpl) && tmpl[i] == '{' && tmpl[i+1] == '{' {
			tagEnd := strings.Index(tmpl[i+2:], "}}")
			if tagEnd == -1 {
				break
			}
			tag := strings.TrimSpace(tmpl[i+2 : i+2+tagEnd])
			if strings.HasPrefix(tag, "если ") || strings.HasPrefix(tag, "для ") {
				depth++
			} else if tag == "конец" {
				depth--
				if depth == 0 {
					if elseStart >= 0 {
						body = tmpl[bodyStart:elseStart]
						elseBody = tmpl[elseStart:i]
					} else {
						body = tmpl[bodyStart:i]
					}
					end = i + 2 + tagEnd + 2
					return
				}
			} else if tag == "иначе" && depth == 1 {
				elseStart = i + 2 + tagEnd + 2
				body = tmpl[bodyStart:i]
			}
			i = i + 2 + tagEnd + 2
		} else {
			i++
		}
	}
	body = tmpl[bodyStart:]
	end = len(tmpl)
	return
}

// resolveValue разрешает выражение вида "имя", "товар.цена", "товар.название".
func resolveValue(expr string, data *Hash) Object {
	if data == nil {
		return NULL
	}
	parts := strings.Split(expr, ".")
	var current Object = data
	for _, part := range parts {
		part = strings.TrimSpace(part)
		switch obj := current.(type) {
		case *Hash:
			key := (&String{Value: part}).HashKey()
			if pair, ok := obj.Pairs[key]; ok {
				current = pair.Value
			} else {
				return NULL
			}
		case *Instance:
			if val, ok := obj.Properties[part]; ok {
				current = val
			} else {
				return NULL
			}
		default:
			return NULL
		}
	}
	return current
}

// evalCondition вычисляет простое условие для шаблона.
// Поддерживает: "переменная", "!переменная", "a == b", "a != b"
func evalCondition(cond string, data *Hash) bool {
	cond = strings.TrimSpace(cond)
	if strings.HasPrefix(cond, "!") {
		val := resolveValue(cond[1:], data)
		return val == NULL || val == FALSE || (val.Type() == "STRING" && val.(*String).Value == "")
	}
	if strings.Contains(cond, " == ") {
		parts := strings.SplitN(cond, " == ", 2)
		left := resolveValue(strings.TrimSpace(parts[0]), data)
		right := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
		return left != nil && left.Inspect() == right
	}
	if strings.Contains(cond, " != ") {
		parts := strings.SplitN(cond, " != ", 2)
		left := resolveValue(strings.TrimSpace(parts[0]), data)
		right := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
		return left == nil || left.Inspect() != right
	}
	val := resolveValue(cond, data)
	return val != nil && val != NULL && val != FALSE
}

// copyHash создаёт поверхностную копию Hash.
func copyHash(h *Hash) *Hash {
	if h == nil {
		return NewHash()
	}
	newH := NewHash()
	for _, k := range h.Keys {
		if pair, ok := h.Pairs[k]; ok {
			newH.Set(pair.Key, pair.Value)
		}
	}
	return newH
}
