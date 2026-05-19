// Package mcp реализует Model Context Protocol сервер для языка Ясный.
// MCP — это JSON-RPC 2.0 поверх stdin/stdout, позволяющий AI-ассистентам
// (Claude, Cursor, Continue) работать с инструментами языка.
//
// Запуск: yasny mcp
//
// Подключение в Claude Desktop (~/.config/Claude/config.json):
//
//   {
//     "mcpServers": {
//       "yasny": { "command": "yasny", "args": ["mcp"] }
//     }
//   }
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"yasny-lang/formatter"
	"yasny-lang/interpreter"
	"yasny-lang/lexer"
	"yasny-lang/linter"
	"yasny-lang/parser"
)

// MCP-протокол: версия 2024-11-05.
const protocolVersion = "2024-11-05"

// Run запускает MCP-сервер на stdin/stdout.
func Run() {
	srv := &server{}
	srv.serve(os.Stdin, os.Stdout)
}

type server struct {
	mu sync.Mutex
}

// === JSON-RPC структуры ===

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *errObj         `json:"error,omitempty"`
}

type errObj struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// serve читает запросы из in, обрабатывает, пишет ответы в out.
// MCP использует line-delimited JSON-RPC (по строке на сообщение).
func (s *server) serve(in io.Reader, out io.Writer) {
	reader := bufio.NewReader(in)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}
		resp := s.handle(&req)
		if resp == nil {
			continue // notification — не отвечаем
		}
		data, _ := json.Marshal(resp)
		out.Write(data)
		out.Write([]byte("\n"))
	}
}

// handle обрабатывает один запрос. Возвращает nil для нотификаций.
func (s *server) handle(req *request) *response {
	// Нотификации (без id) не требуют ответа.
	isNotification := len(req.ID) == 0 || string(req.ID) == "null"

	switch req.Method {
	case "initialize":
		return s.success(req, map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "yasny",
				"version": "0.60.0",
			},
		})
	case "notifications/initialized", "initialized":
		return nil
	case "tools/list":
		return s.success(req, map[string]any{"tools": toolList()})
	case "tools/call":
		return s.callTool(req)
	case "ping":
		return s.success(req, map[string]any{})
	}

	if isNotification {
		return nil
	}
	return &response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Error:   &errObj{Code: -32601, Message: "method not found: " + req.Method},
	}
}

func (s *server) success(req *request, result any) *response {
	if len(req.ID) == 0 {
		return nil
	}
	return &response{JSONRPC: "2.0", ID: req.ID, Result: result}
}

func (s *server) errorResp(req *request, code int, msg string) *response {
	if len(req.ID) == 0 {
		return nil
	}
	return &response{JSONRPC: "2.0", ID: req.ID, Error: &errObj{Code: code, Message: msg}}
}

// === Список инструментов ===

func toolList() []map[string]any {
	codeProp := map[string]any{
		"type":        "string",
		"description": "Исходный код на языке Ясный",
	}
	codeRequired := []string{"code"}

	return []map[string]any{
		{
			"name":        "run",
			"description": "Выполнить код на языке Ясный и вернуть вывод. Таймаут 10 секунд.",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{"code": codeProp},
				"required":   codeRequired,
			},
		},
		{
			"name":        "format",
			"description": "Отформатировать код по канону СТИЛЬ.md (как gofmt/prettier).",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{"code": codeProp},
				"required":   codeRequired,
			},
		},
		{
			"name":        "lint",
			"description": "Проверить код на потенциальные проблемы (неиспользуемые переменные, недостижимый код и т.д.).",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{"code": codeProp},
				"required":   codeRequired,
			},
		},
		{
			"name":        "syntax_check",
			"description": "Проверить синтаксис без выполнения. Возвращает список ошибок парсера.",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{"code": codeProp},
				"required":   codeRequired,
			},
		},
		{
			"name":        "list_modules",
			"description": "Список встроенных модулей stdlib с кратким описанием.",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			"name":        "module_info",
			"description": "Подробная информация о встроенном модуле: список функций и примеры.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Имя модуля: мат, время, json, http, бд, вс, крипто, и т.д.",
					},
				},
				"required": []string{"name"},
			},
		},
		{
			"name":        "search_packages",
			"description": "Поиск пакетов в реестре yasny-registry по подстроке.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Подстрока для поиска (по имени и описанию). Пусто = все пакеты.",
					},
				},
			},
		},
		{
			"name":        "language_reference",
			"description": "Справка по языку: ключевые слова, операторы, конструкции. Возвращает шпаргалку.",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}
}

// === Вызов инструментов ===

func (s *server) callTool(req *request) *response {
	var p struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &p); err != nil {
		return s.errorResp(req, -32602, "неверные параметры")
	}

	var content []map[string]any
	var isError bool

	switch p.Name {
	case "run":
		content, isError = toolRun(getString(p.Arguments, "code"))
	case "format":
		content, isError = toolFormat(getString(p.Arguments, "code"))
	case "lint":
		content, isError = toolLint(getString(p.Arguments, "code"))
	case "syntax_check":
		content, isError = toolSyntaxCheck(getString(p.Arguments, "code"))
	case "list_modules":
		content, isError = toolListModules()
	case "module_info":
		content, isError = toolModuleInfo(getString(p.Arguments, "name"))
	case "search_packages":
		content, isError = toolSearchPackages(getString(p.Arguments, "query"))
	case "language_reference":
		content, isError = toolLanguageReference()
	default:
		return s.errorResp(req, -32602, "неизвестный инструмент: "+p.Name)
	}

	return s.success(req, map[string]any{
		"content": content,
		"isError": isError,
	})
}

func textContent(s string) []map[string]any {
	return []map[string]any{{"type": "text", "text": s}}
}

func getString(args map[string]any, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

// === Реализация инструментов ===

// toolRun выполняет код в подпроцессе с таймаутом.
// Используется отдельный процесс чтобы изолировать состояние и ловить
// бесконечные циклы. interpreter.OutputWriter перехватывает вывод.
func toolRun(code string) ([]map[string]any, bool) {
	if code == "" {
		return textContent("ошибка: пустой код"), true
	}

	// Запускаем себя с -e флагом или через временный файл.
	// Самый простой способ — через временный файл и yasny.
	tmpFile, err := os.CreateTemp("", "yasny-mcp-*.ya")
	if err != nil {
		return textContent("не удалось создать временный файл: " + err.Error()), true
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString(code)
	tmpFile.Close()

	exe, err := os.Executable()
	if err != nil {
		exe = "yasny"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, exe, tmpFile.Name())
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	output := stdout.String()
	if errOut := stderr.String(); errOut != "" {
		if output != "" {
			output += "\n"
		}
		output += "STDERR:\n" + errOut
	}

	if ctx.Err() == context.DeadlineExceeded {
		return textContent(output + "\n\nПрервано: превышен таймаут 10 секунд"), true
	}
	if err != nil {
		return textContent(output + "\n\nОшибка выполнения: " + err.Error()), true
	}

	if output == "" {
		output = "(программа завершилась без вывода)"
	}
	return textContent(output), false
}

func toolFormat(code string) ([]map[string]any, bool) {
	if code == "" {
		return textContent("ошибка: пустой код"), true
	}
	formatted, err := formatter.Format(code)
	if err != nil {
		return textContent("ошибка форматирования: " + err.Error()), true
	}
	return textContent(formatted), false
}

func toolLint(code string) ([]map[string]any, bool) {
	if code == "" {
		return textContent("ошибка: пустой код"), true
	}
	issues := linter.Lint(code, "<inline>")
	if len(issues) == 0 {
		return textContent("✓ Проблем не найдено"), false
	}
	var sb strings.Builder
	for _, issue := range issues {
		sb.WriteString(issue.String())
		sb.WriteString("\n")
	}
	sb.WriteString(fmt.Sprintf("\nНайдено проблем: %d", len(issues)))
	return textContent(sb.String()), false
}

func toolSyntaxCheck(code string) ([]map[string]any, bool) {
	if code == "" {
		return textContent("ошибка: пустой код"), true
	}
	l := lexer.NewWithFilename(code, "<inline>")
	p := parser.New(l)
	p.ParseProgram()
	if len(p.Errors()) == 0 {
		return textContent("✓ Синтаксис корректен"), false
	}
	var sb strings.Builder
	for _, e := range p.Errors() {
		sb.WriteString(e)
		sb.WriteString("\n")
	}
	return textContent(sb.String()), true
}

func toolListModules() ([]map[string]any, bool) {
	modules := []struct {
		name, desc string
	}{
		{"мат", "Математика: пи, е, sin, cos, лог, корень, абс, мин, макс"},
		{"время", "Время и даты: сейчас, строка, год, месяц, день, спать, разобрать"},
		{"json", "JSON: разобрать(строка), создать(объект)"},
		{"регвыр", "Регулярные выражения: найти, заменить, совпадает"},
		{"http", "HTTP клиент и сервер: получить, пост, приложение() с routing"},
		{"бд", "Базы данных: SQLite (бд.открыть) и PostgreSQL (бд.подключить)"},
		{"вс", "WebSocket: клиент (вс.подключить) и сервер (вс.сервер)"},
		{"крипто", "Криптография: SHA, MD5, HMAC, AES, JWT, base64, hex"},
		{"шаблон", "HTML-шаблонизатор: рендер, файл, экранировать"},
		{"csv", "CSV: разобрать, с_заголовками, строка"},
		{"случайное", "Рандом: число, целое, элемент"},
		{"файлы", "Файлы: читать, записать, существует"},
		{"путь", "Пути: соединить, имя_файла, расширение, абсолютный"},
		{"ос", "ОС: переменная_среды, аргументы, выйти"},
		{"cli", "CLI: парсинг аргументов командной строки"},
	}
	var sb strings.Builder
	sb.WriteString("Встроенные модули stdlib (доступны через `импорт ИМЯ из \"имя\"`):\n\n")
	for _, m := range modules {
		sb.WriteString(fmt.Sprintf("- **%s** — %s\n", m.name, m.desc))
	}
	return textContent(sb.String()), false
}

func toolModuleInfo(name string) ([]map[string]any, bool) {
	info, ok := moduleDocs[name]
	if !ok {
		available := make([]string, 0, len(moduleDocs))
		for k := range moduleDocs {
			available = append(available, k)
		}
		return textContent("модуль '" + name + "' не найден. Доступны: " + strings.Join(available, ", ")), true
	}
	return textContent(info), false
}

func toolSearchPackages(query string) ([]map[string]any, bool) {
	// Загружаем реестр напрямую
	registryURL := "https://raw.githubusercontent.com/warcorprp-web/yasny-registry/main/registry.json"
	cmd := exec.Command("curl", "-sf", "-H", "Cache-Control: no-cache", registryURL)
	out, err := cmd.Output()
	if err != nil {
		return textContent("не удалось загрузить реестр: " + err.Error()), true
	}
	var registry struct {
		Packages map[string]struct {
			URL  string `json:"url"`
			Desc string `json:"описание"`
		} `json:"пакеты"`
	}
	if err := json.Unmarshal(out, &registry); err != nil {
		return textContent("ошибка разбора реестра: " + err.Error()), true
	}

	var sb strings.Builder
	q := strings.ToLower(query)
	count := 0
	for name, pkg := range registry.Packages {
		if q != "" && !strings.Contains(strings.ToLower(name), q) && !strings.Contains(strings.ToLower(pkg.Desc), q) {
			continue
		}
		sb.WriteString(fmt.Sprintf("**%s** — %s\n  Установить: yasny подключить %s\n  Репо: %s\n\n", name, pkg.Desc, name, pkg.URL))
		count++
	}
	if count == 0 {
		return textContent("ничего не найдено по запросу: " + query), false
	}
	return textContent(fmt.Sprintf("Найдено: %d\n\n%s", count, sb.String())), false
}

func toolLanguageReference() ([]map[string]any, bool) {
	return textContent(languageReference), false
}

// === Подавление неиспользуемых импортов ===
var _ = interpreter.OutputWriter
