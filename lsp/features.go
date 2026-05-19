package lsp

import (
	"encoding/json"
	"fmt"
	"strings"

	"yasny-lang/formatter"

	"github.com/sourcegraph/jsonrpc2"
)

// Встроенные функции и модули для автодополнения.
var builtinFunctions = []struct {
	Name   string
	Detail string
	Doc    string
}{
	{"вывод", "функция(значение)", "Выводит значение на экран"},
	{"ввод", "функция(подсказка)", "Читает строку с клавиатуры"},
	{"длина", "функция(объект)", "Возвращает длину строки/массива/объекта"},
	{"тип", "функция(значение)", "Возвращает тип значения как строку"},
	{"строка", "функция(значение)", "Преобразует значение в строку"},
	{"число", "функция(строка)", "Преобразует строку в число"},
	{"округл", "функция(число)", "Округляет число"},
	{"добавить", "функция(массив, элемент)", "Добавляет элемент в конец массива"},
	{"удалить", "функция(массив, индекс)", "Удаляет элемент по индексу"},
	{"фильтр", "функция(массив, условие)", "Фильтрует массив по условию"},
	{"преобразовать", "функция(массив, функция)", "Преобразует каждый элемент"},
	{"свернуть", "функция(массив, функция, начальное)", "Сворачивает массив в одно значение"},
	{"сортировать", "функция(массив)", "Сортирует массив"},
	{"сумма", "функция(массив)", "Сумма элементов массива"},
	{"диапазон", "функция(от, до)", "Создаёт массив чисел от..до"},
	{"ключи", "функция(объект)", "Массив ключей объекта"},
	{"значения", "функция(объект)", "Массив значений объекта"},
	{"содержит", "функция(строка, подстрока)", "Проверяет наличие подстроки"},
	{"разделить", "функция(строка, разделитель)", "Разбивает строку на массив"},
	{"соединить", "функция(массив, разделитель)", "Соединяет массив в строку"},
	{"заменить", "функция(строка, что, чем)", "Заменяет подстроку"},
	{"повторить", "функция(строка, раз)", "Повторяет строку N раз"},
	{"корень", "функция(число)", "Квадратный корень"},
	{"абс", "функция(число)", "Модуль числа"},
	{"мин", "функция(a, b)", "Минимум из двух"},
	{"макс", "функция(a, b)", "Максимум из двух"},
}

var stdModules = map[string][]struct {
	Name   string
	Detail string
}{
	"мат":       {{"пи", "3.14159..."}, {"е", "2.71828..."}, {"sin", "функция(x)"}, {"cos", "функция(x)"}, {"лог", "функция(x)"}, {"пол", "функция(x)"}, {"потолок", "функция(x)"}},
	"время":     {{"сейчас", "функция()"}, {"строка", "функция()"}, {"год", "функция()"}, {"месяц", "функция()"}, {"день", "функция()"}, {"час", "функция()"}, {"спать", "функция(мс)"}, {"метка", "функция()"}},
	"json":      {{"создать", "функция(объект)"}, {"разобрать", "функция(строка)"}},
	"регвыр":    {{"найти", "функция(текст, шаблон)"}, {"заменить", "функция(текст, шаблон, замена)"}, {"совпадает", "функция(текст, шаблон)"}},
	"http":      {{"получить", "функция(url)"}, {"пост", "функция(url, тело)"}, {"положить", "функция(url, тело)"}, {"удалить", "функция(url)"}, {"приложение", "функция()"}},
	"крипто":    {{"sha256", "функция(строка)"}, {"sha1", "функция(строка)"}, {"md5", "функция(строка)"}, {"hmac_sha256", "функция(сообщение, ключ)"}, {"aes_зашифровать", "функция(текст, ключ)"}, {"aes_расшифровать", "функция(шифр, ключ)"}, {"jwt_создать", "функция(данные, секрет)"}, {"jwt_проверить", "функция(токен, секрет)"}, {"base64_кодировать", "функция(строка)"}, {"base64_декодировать", "функция(строка)"}},
	"csv":       {{"разобрать", "функция(текст)"}, {"с_заголовками", "функция(текст)"}, {"строка", "функция(массив)"}},
	"случайное": {{"число", "функция()"}, {"целое", "функция(мин, макс)"}, {"элемент", "функция(массив)"}},
	"файлы":     {{"читать", "функция(путь)"}, {"записать", "функция(путь, содержимое)"}, {"существует", "функция(путь)"}},
	"путь":      {{"соединить", "функция(части...)"}, {"имя_файла", "функция(путь)"}, {"расширение", "функция(путь)"}, {"абсолютный", "функция(путь)"}},
	"ос":        {{"переменная_среды", "функция(имя)"}, {"аргументы", "функция()"}, {"выйти", "функция(код)"}},
	"бд":        {{"открыть", "функция(путь) — SQLite"}, {"подключить", "функция(строка) — PostgreSQL"}},
	"вс":        {{"подключить", "функция(url)"}, {"сервер", "функция(порт, обработчик)"}},
	"шаблон":    {{"рендер", "функция(строка, данные)"}, {"файл", "функция(путь, данные)"}, {"экранировать", "функция(html)"}},
	"cli":       {{"аргумент", "функция(имя, по_умолчанию)"}, {"помощь", "функция()"}},
}

// === Completion ===

func (s *Server) completion(req *jsonrpc2.Request) (interface{}, error) {
	var params struct {
		TextDocument struct{ URI string } `json:"textDocument"`
		Position     struct {
			Line      int `json:"line"`
			Character int `json:"character"`
		} `json:"position"`
	}
	json.Unmarshal(*req.Params, &params)

	s.mu.Lock()
	doc, ok := s.docs[params.TextDocument.URI]
	s.mu.Unlock()
	if !ok {
		return nil, nil
	}

	line := getLine(doc.Content, params.Position.Line)
	col := params.Position.Character
	if col > len([]rune(line)) {
		col = len([]rune(line))
	}

	// Проверяем — это дополнение после точки (модуль.)?
	prefix := string([]rune(line)[:col])
	if dotIdx := strings.LastIndex(prefix, "."); dotIdx >= 0 {
		moduleName := strings.TrimSpace(prefix[:dotIdx])
		// Убираем всё до последнего пробела/скобки
		for _, sep := range []string{" ", "(", ",", "="} {
			if idx := strings.LastIndex(moduleName, sep); idx >= 0 {
				moduleName = moduleName[idx+1:]
			}
		}
		if methods, ok := stdModules[moduleName]; ok {
			items := []interface{}{}
			for _, m := range methods {
				items = append(items, map[string]interface{}{
					"label":  m.Name,
					"kind":   2, // Method
					"detail": m.Detail,
				})
			}
			return map[string]interface{}{"items": items}, nil
		}
	}

	// Общее дополнение: символы документа + встроенные + модули
	items := []interface{}{}

	// Символы из текущего документа
	for _, sym := range doc.Symbols {
		items = append(items, map[string]interface{}{
			"label":  sym.Name,
			"kind":   symbolKindToCompletionKind(sym.Kind),
			"detail": sym.Detail,
		})
	}

	// Встроенные функции
	for _, f := range builtinFunctions {
		items = append(items, map[string]interface{}{
			"label":         f.Name,
			"kind":          3, // Function
			"detail":        f.Detail,
			"documentation": f.Doc,
		})
	}

	// Модули
	for name := range stdModules {
		items = append(items, map[string]interface{}{
			"label":  name,
			"kind":   9, // Module
			"detail": "модуль",
		})
	}

	// Ключевые слова
	keywords := []string{"функция", "конст", "перем", "если", "иначеесли", "иначе", "для", "пока", "вернуть", "класс", "конец", "импорт", "экспорт", "из", "попытка", "поймать", "бросить", "прервать", "продолжить", "совпадение", "когда", "да", "нет", "ничего", "и", "или", "не", "это", "новый"}
	for _, kw := range keywords {
		items = append(items, map[string]interface{}{
			"label": kw,
			"kind":  14, // Keyword
		})
	}

	return map[string]interface{}{"items": items}, nil
}

// === Hover ===

func (s *Server) hover(req *jsonrpc2.Request) (interface{}, error) {
	var params struct {
		TextDocument struct{ URI string } `json:"textDocument"`
		Position     struct {
			Line      int `json:"line"`
			Character int `json:"character"`
		} `json:"position"`
	}
	json.Unmarshal(*req.Params, &params)

	s.mu.Lock()
	doc, ok := s.docs[params.TextDocument.URI]
	s.mu.Unlock()
	if !ok {
		return nil, nil
	}

	word := getWordAt(doc.Content, params.Position.Line, params.Position.Character)
	if word == "" {
		return nil, nil
	}

	// Проверяем встроенные
	for _, f := range builtinFunctions {
		if f.Name == word {
			return map[string]interface{}{
				"contents": map[string]interface{}{
					"kind":  "markdown",
					"value": "**" + f.Name + "** — " + f.Detail + "\n\n" + f.Doc,
				},
			}, nil
		}
	}

	// Проверяем модули
	if methods, ok := stdModules[word]; ok {
		var sb strings.Builder
		sb.WriteString("**модуль " + word + "**\n\n")
		for _, m := range methods {
			sb.WriteString("- `" + word + "." + m.Name + "` — " + m.Detail + "\n")
		}
		return map[string]interface{}{
			"contents": map[string]interface{}{"kind": "markdown", "value": sb.String()},
		}, nil
	}

	// Проверяем символы документа
	for _, sym := range doc.Symbols {
		if sym.Name == word {
			return map[string]interface{}{
				"contents": map[string]interface{}{
					"kind":  "markdown",
					"value": "**" + sym.Name + "** — " + sym.Detail + " (строка " + itoa(sym.Line+1) + ")",
				},
			}, nil
		}
	}

	return nil, nil
}

// === Definition ===

func (s *Server) definition(req *jsonrpc2.Request) (interface{}, error) {
	var params struct {
		TextDocument struct{ URI string } `json:"textDocument"`
		Position     struct {
			Line      int `json:"line"`
			Character int `json:"character"`
		} `json:"position"`
	}
	json.Unmarshal(*req.Params, &params)

	s.mu.Lock()
	doc, ok := s.docs[params.TextDocument.URI]
	s.mu.Unlock()
	if !ok {
		return nil, nil
	}

	word := getWordAt(doc.Content, params.Position.Line, params.Position.Character)
	for _, sym := range doc.Symbols {
		if sym.Name == word {
			return map[string]interface{}{
				"uri":   params.TextDocument.URI,
				"range": makeRange(sym.Line, 0, sym.Line, len(sym.Name)),
			}, nil
		}
	}
	return nil, nil
}

// === References ===

func (s *Server) references(req *jsonrpc2.Request) (interface{}, error) {
	var params struct {
		TextDocument struct{ URI string } `json:"textDocument"`
		Position     struct {
			Line      int `json:"line"`
			Character int `json:"character"`
		} `json:"position"`
	}
	json.Unmarshal(*req.Params, &params)

	s.mu.Lock()
	doc, ok := s.docs[params.TextDocument.URI]
	s.mu.Unlock()
	if !ok {
		return nil, nil
	}

	word := getWordAt(doc.Content, params.Position.Line, params.Position.Character)
	if word == "" {
		return nil, nil
	}

	// Ищем все вхождения слова в документе
	refs := []interface{}{}
	lines := strings.Split(doc.Content, "\n")
	for i, line := range lines {
		runes := []rune(line)
		for j := 0; j <= len(runes)-len([]rune(word)); j++ {
			if string(runes[j:j+len([]rune(word))]) == word {
				// Проверяем границы слова
				before := j == 0 || !isIdentRune(runes[j-1])
				after := j+len([]rune(word)) >= len(runes) || !isIdentRune(runes[j+len([]rune(word))])
				if before && after {
					refs = append(refs, map[string]interface{}{
						"uri":   params.TextDocument.URI,
						"range": makeRange(i, j, i, j+len([]rune(word))),
					})
				}
			}
		}
	}
	return refs, nil
}

// === Rename ===

func (s *Server) rename(req *jsonrpc2.Request) (interface{}, error) {
	var params struct {
		TextDocument struct{ URI string } `json:"textDocument"`
		Position     struct {
			Line      int `json:"line"`
			Character int `json:"character"`
		} `json:"position"`
		NewName string `json:"newName"`
	}
	json.Unmarshal(*req.Params, &params)

	s.mu.Lock()
	doc, ok := s.docs[params.TextDocument.URI]
	s.mu.Unlock()
	if !ok {
		return nil, nil
	}

	word := getWordAt(doc.Content, params.Position.Line, params.Position.Character)
	if word == "" {
		return nil, nil
	}

	// Собираем все вхождения
	edits := []interface{}{}
	lines := strings.Split(doc.Content, "\n")
	for i, line := range lines {
		runes := []rune(line)
		for j := 0; j <= len(runes)-len([]rune(word)); j++ {
			if string(runes[j:j+len([]rune(word))]) == word {
				before := j == 0 || !isIdentRune(runes[j-1])
				after := j+len([]rune(word)) >= len(runes) || !isIdentRune(runes[j+len([]rune(word))])
				if before && after {
					edits = append(edits, map[string]interface{}{
						"range":   makeRange(i, j, i, j+len([]rune(word))),
						"newText": params.NewName,
					})
				}
			}
		}
	}

	return map[string]interface{}{
		"changes": map[string]interface{}{
			params.TextDocument.URI: edits,
		},
	}, nil
}

// === Document Symbols ===

func (s *Server) documentSymbol(req *jsonrpc2.Request) (interface{}, error) {
	var params struct {
		TextDocument struct{ URI string } `json:"textDocument"`
	}
	json.Unmarshal(*req.Params, &params)

	s.mu.Lock()
	doc, ok := s.docs[params.TextDocument.URI]
	s.mu.Unlock()
	if !ok {
		return nil, nil
	}

	symbols := []interface{}{}
	for _, sym := range doc.Symbols {
		symbols = append(symbols, map[string]interface{}{
			"name":           sym.Name,
			"kind":           sym.Kind,
			"range":          makeRange(sym.Line, 0, sym.Line, 100),
			"selectionRange": makeRange(sym.Line, 0, sym.Line, len([]rune(sym.Name))),
		})
	}
	return symbols, nil
}

// === Formatting ===

func (s *Server) formatting(req *jsonrpc2.Request) (interface{}, error) {
	var params struct {
		TextDocument struct{ URI string } `json:"textDocument"`
	}
	json.Unmarshal(*req.Params, &params)

	s.mu.Lock()
	doc, ok := s.docs[params.TextDocument.URI]
	s.mu.Unlock()
	if !ok {
		return nil, nil
	}

	formatted, err := formatter.Format(doc.Content)
	if err != nil {
		return nil, nil
	}
	if formatted == doc.Content {
		return []interface{}{}, nil
	}

	lines := strings.Split(doc.Content, "\n")
	return []interface{}{
		map[string]interface{}{
			"range":   makeRange(0, 0, len(lines), 0),
			"newText": formatted,
		},
	}, nil
}

// === Signature Help ===

func (s *Server) signatureHelp(req *jsonrpc2.Request) (interface{}, error) {
	var params struct {
		TextDocument struct{ URI string } `json:"textDocument"`
		Position     struct {
			Line      int `json:"line"`
			Character int `json:"character"`
		} `json:"position"`
	}
	json.Unmarshal(*req.Params, &params)

	s.mu.Lock()
	doc, ok := s.docs[params.TextDocument.URI]
	s.mu.Unlock()
	if !ok {
		return nil, nil
	}

	line := getLine(doc.Content, params.Position.Line)
	col := params.Position.Character
	prefix := string([]rune(line)[:col])

	// Ищем имя функции перед (
	funcName := ""
	if idx := strings.LastIndex(prefix, "("); idx >= 0 {
		before := strings.TrimSpace(prefix[:idx])
		parts := strings.FieldsFunc(before, func(r rune) bool {
			return r == ' ' || r == ',' || r == '=' || r == '('
		})
		if len(parts) > 0 {
			funcName = parts[len(parts)-1]
			if dotIdx := strings.LastIndex(funcName, "."); dotIdx >= 0 {
				funcName = funcName[dotIdx+1:]
			}
		}
	}

	if funcName == "" {
		return nil, nil
	}

	for _, f := range builtinFunctions {
		if f.Name == funcName {
			return map[string]interface{}{
				"signatures": []interface{}{
					map[string]interface{}{
						"label":         f.Name + "(" + extractParams(f.Detail) + ")",
						"documentation": f.Doc,
					},
				},
				"activeSignature": 0,
			}, nil
		}
	}

	// Ищем в символах документа
	for _, sym := range doc.Symbols {
		if sym.Name == funcName && sym.Kind == SymbolFunction {
			return map[string]interface{}{
				"signatures": []interface{}{
					map[string]interface{}{
						"label": funcName + "(...)",
					},
				},
			}, nil
		}
	}

	return nil, nil
}

// === Utilities ===

func getLine(content string, line int) string {
	lines := strings.Split(content, "\n")
	if line < 0 || line >= len(lines) {
		return ""
	}
	return lines[line]
}

func getWordAt(content string, line, col int) string {
	l := getLine(content, line)
	runes := []rune(l)
	if col > len(runes) {
		col = len(runes)
	}
	start := col
	for start > 0 && isIdentRune(runes[start-1]) {
		start--
	}
	end := col
	for end < len(runes) && isIdentRune(runes[end]) {
		end++
	}
	if start == end {
		return ""
	}
	return string(runes[start:end])
}

func isIdentRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= 'а' && r <= 'я') || (r >= 'А' && r <= 'Я') ||
		r == 'ё' || r == 'Ё' || r == '_' || r == '?' ||
		(r >= '0' && r <= '9')
}

func symbolKindToCompletionKind(kind int) int {
	switch kind {
	case SymbolFunction:
		return 3
	case SymbolClass:
		return 7
	case SymbolVariable:
		return 6
	case SymbolConstant:
		return 21
	case SymbolModule:
		return 9
	}
	return 6
}

func extractParams(detail string) string {
	if idx := strings.Index(detail, "("); idx >= 0 {
		if end := strings.Index(detail[idx:], ")"); end >= 0 {
			return detail[idx+1 : idx+end]
		}
	}
	return ""
}

func itoa(n int) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%d", n), "0"), ".")
}

// Нужен fmt для itoa
