package interpreter

import (
	"bytes"
	"encoding/csv"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// === Модуль "http" — полный HTTP-клиент ===

// hashToHeaders преобразует Hash в http.Header
func hashToHeaders(h *Hash) http.Header {
	hdrs := http.Header{}
	for _, pair := range h.Pairs {
		k, ok := pair.Key.(*String)
		if !ok {
			continue
		}
		v, ok := pair.Value.(*String)
		if !ok {
			continue
		}
		hdrs.Set(k.Value, v.Value)
	}
	return hdrs
}

// httpRequest универсальная HTTP-функция
func httpRequest(method string, args []Object) Object {
	if len(args) < 1 {
		return ErrorWithHint(currentCallToken,
			"http."+strings.ToLower(method)+"(url, [тело], [заголовки])",
			"")
	}

	urlStr, ok := args[0].(*String)
	if !ok {
		return builtinErrorWrongArgType("http."+strings.ToLower(method), 1, "STRING (строка)", string(args[0].Type()))
	}

	var body io.Reader
	if len(args) >= 2 && args[1] != NULL {
		switch b := args[1].(type) {
		case *String:
			body = bytes.NewBufferString(b.Value)
		case *Hash:
			// Сериализуем в JSON
			jsonStr := hashToJSON(b)
			body = bytes.NewBufferString(jsonStr)
		case *Array:
			jsonStr := arrayToJSON(b)
			body = bytes.NewBufferString(jsonStr)
		}
	}

	req, err := http.NewRequest(method, urlStr.Value, body)
	if err != nil {
		return ErrorWithHint(currentCallToken, "ошибка создания запроса: "+err.Error(), "")
	}

	// Заголовки по умолчанию
	if body != nil {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
	}
	req.Header.Set("User-Agent", "Yasny/0.46")

	// Кастомные заголовки
	if len(args) >= 3 && args[2] != NULL {
		if hashHdr, ok := args[2].(*Hash); ok {
			for k, v := range hashToHeaders(hashHdr) {
				req.Header[k] = v
			}
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ErrorWithHint(currentCallToken, "ошибка запроса: "+err.Error(), "")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ErrorWithHint(currentCallToken, "ошибка чтения ответа: "+err.Error(), "")
	}

	// Возвращаем словарь {статус, тело, заголовки}
	headerHash := make(map[HashKey]HashPair)
	for k, v := range resp.Header {
		key := &String{Value: k}
		val := &String{Value: strings.Join(v, ", ")}
		headerHash[key.HashKey()] = HashPair{Key: key, Value: val}
	}

	result := make(map[HashKey]HashPair)
	statusKey := &String{Value: "статус"}
	result[statusKey.HashKey()] = HashPair{Key: statusKey, Value: &Integer{Value: int64(resp.StatusCode)}}

	bodyKey := &String{Value: "тело"}
	result[bodyKey.HashKey()] = HashPair{Key: bodyKey, Value: &String{Value: string(respBody)}}

	hdrKey := &String{Value: "заголовки"}
	result[hdrKey.HashKey()] = HashPair{Key: hdrKey, Value: &Hash{Pairs: headerHash}}

	urlKey := &String{Value: "url"}
	result[urlKey.HashKey()] = HashPair{Key: urlKey, Value: &String{Value: resp.Request.URL.String()}}

	return &Hash{Pairs: result}
}

// hashToJSON и arrayToJSON — упрощённая сериализация
func hashToJSON(h *Hash) string {
	var sb strings.Builder
	sb.WriteString("{")
	first := true
	for _, p := range h.Pairs {
		if !first {
			sb.WriteString(",")
		}
		first = false
		k, ok := p.Key.(*String)
		if !ok {
			continue
		}
		sb.WriteString("\"")
		sb.WriteString(escapeJSON(k.Value))
		sb.WriteString("\":")
		sb.WriteString(objectToJSON(p.Value))
	}
	sb.WriteString("}")
	return sb.String()
}

func arrayToJSON(a *Array) string {
	var sb strings.Builder
	sb.WriteString("[")
	for i, el := range a.Elements {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(objectToJSON(el))
	}
	sb.WriteString("]")
	return sb.String()
}

func objectToJSON(o Object) string {
	switch v := o.(type) {
	case *String:
		return "\"" + escapeJSON(v.Value) + "\""
	case *Integer:
		return v.Inspect()
	case *Float:
		return v.Inspect()
	case *Boolean:
		if v.Value {
			return "true"
		}
		return "false"
	case *Hash:
		return hashToJSON(v)
	case *Array:
		return arrayToJSON(v)
	default:
		if o == NULL {
			return "null"
		}
		return "\"" + escapeJSON(o.Inspect()) + "\""
	}
}

func escapeJSON(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch r {
		case '"':
			sb.WriteString("\\\"")
		case '\\':
			sb.WriteString("\\\\")
		case '\n':
			sb.WriteString("\\n")
		case '\r':
			sb.WriteString("\\r")
		case '\t':
			sb.WriteString("\\t")
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func registerHttpModule() {
	fns := map[string]func(args ...Object) Object{
		"получить": func(args ...Object) Object {
			return httpRequest("GET", args)
		},
		"пост": func(args ...Object) Object {
			return httpRequest("POST", args)
		},
		"положить": func(args ...Object) Object {
			return httpRequest("PUT", args)
		},
		"удалить": func(args ...Object) Object {
			return httpRequest("DELETE", args)
		},
		"патч": func(args ...Object) Object {
			return httpRequest("PATCH", args)
		},
		"отправить": func(args ...Object) Object {
			// http.отправить(метод, url, [тело], [заголовки])
			if len(args) < 2 {
				return ErrorWithHint(currentCallToken,
					"http.отправить(метод, url, [тело], [заголовки])", "")
			}
			method, ok := args[0].(*String)
			if !ok {
				return builtinErrorWrongArgType("http.отправить", 1, "STRING", string(args[0].Type()))
			}
			return httpRequest(strings.ToUpper(method.Value), args[1:])
		},
	}
	stdModules["http"] = makeHashFromBuiltins(fns)
}

// === Модуль "csv" ===

func registerCsvModule() {
	fns := map[string]func(args ...Object) Object{
		"разобрать": func(args ...Object) Object {
			s, err := getString(args, 0, "csv.разобрать")
			if err != nil {
				return err
			}
			r := csv.NewReader(strings.NewReader(s))
			records, e := r.ReadAll()
			if e != nil {
				return ErrorWithHint(currentCallToken, "ошибка CSV: "+e.Error(), "")
			}
			outer := make([]Object, 0, len(records))
			for _, row := range records {
				inner := make([]Object, 0, len(row))
				for _, cell := range row {
					inner = append(inner, &String{Value: cell})
				}
				outer = append(outer, &Array{Elements: inner})
			}
			return &Array{Elements: outer}
		},
		"с_заголовками": func(args ...Object) Object {
			s, err := getString(args, 0, "csv.с_заголовками")
			if err != nil {
				return err
			}
			r := csv.NewReader(strings.NewReader(s))
			records, e := r.ReadAll()
			if e != nil {
				return ErrorWithHint(currentCallToken, "ошибка CSV: "+e.Error(), "")
			}
			if len(records) == 0 {
				return &Array{Elements: []Object{}}
			}
			headers := records[0]
			result := make([]Object, 0, len(records)-1)
			for _, row := range records[1:] {
				pairs := make(map[HashKey]HashPair)
				for i, cell := range row {
					if i >= len(headers) {
						break
					}
					k := &String{Value: headers[i]}
					v := &String{Value: cell}
					pairs[k.HashKey()] = HashPair{Key: k, Value: v}
				}
				result = append(result, &Hash{Pairs: pairs})
			}
			return &Array{Elements: result}
		},
		"строка": func(args ...Object) Object {
			// csv.строка(массив_массивов) → строка csv
			if len(args) < 1 {
				return builtinErrorWrongArgCount("csv.строка", 1, len(args))
			}
			arr, ok := args[0].(*Array)
			if !ok {
				return builtinErrorWrongArgType("csv.строка", 1, "ARRAY", string(args[0].Type()))
			}
			var buf bytes.Buffer
			w := csv.NewWriter(&buf)
			for _, row := range arr.Elements {
				rowArr, ok := row.(*Array)
				if !ok {
					continue
				}
				cells := make([]string, 0, len(rowArr.Elements))
				for _, c := range rowArr.Elements {
					cells = append(cells, c.Inspect())
				}
				w.Write(cells)
			}
			w.Flush()
			return &String{Value: buf.String()}
		},
	}
	stdModules["csv"] = makeHashFromBuiltins(fns)
}

// === Модуль "случайное" ===

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

func registerRandomModule() {
	fns := map[string]func(args ...Object) Object{
		"число": func(args ...Object) Object {
			// случайное.число() → 0.0..1.0
			return &Float{Value: rng.Float64()}
		},
		"целое": func(args ...Object) Object {
			// случайное.целое(min, max) → целое min..max включительно
			if len(args) != 2 {
				return builtinErrorWrongArgCount("случайное.целое", 2, len(args))
			}
			lo, ok := args[0].(*Integer)
			if !ok {
				return builtinErrorWrongArgType("случайное.целое", 1, "INTEGER", string(args[0].Type()))
			}
			hi, ok := args[1].(*Integer)
			if !ok {
				return builtinErrorWrongArgType("случайное.целое", 2, "INTEGER", string(args[1].Type()))
			}
			if hi.Value < lo.Value {
				return ErrorWithHint(currentCallToken, "max должен быть >= min", "")
			}
			return &Integer{Value: lo.Value + rng.Int63n(hi.Value-lo.Value+1)}
		},
		"элемент": func(args ...Object) Object {
			// случайное.элемент(массив)
			if len(args) != 1 {
				return builtinErrorWrongArgCount("случайное.элемент", 1, len(args))
			}
			arr, ok := args[0].(*Array)
			if !ok {
				return builtinErrorWrongArgType("случайное.элемент", 1, "ARRAY", string(args[0].Type()))
			}
			if len(arr.Elements) == 0 {
				return ErrorWithHint(currentCallToken, "пустой массив", "")
			}
			return arr.Elements[rng.Intn(len(arr.Elements))]
		},
		"перемешать": func(args ...Object) Object {
			// случайное.перемешать(массив) → новый массив
			if len(args) != 1 {
				return builtinErrorWrongArgCount("случайное.перемешать", 1, len(args))
			}
			arr, ok := args[0].(*Array)
			if !ok {
				return builtinErrorWrongArgType("случайное.перемешать", 1, "ARRAY", string(args[0].Type()))
			}
			result := make([]Object, len(arr.Elements))
			copy(result, arr.Elements)
			rng.Shuffle(len(result), func(i, j int) {
				result[i], result[j] = result[j], result[i]
			})
			return &Array{Elements: result}
		},
		"семя": func(args ...Object) Object {
			// случайное.семя(число) - установить seed для воспроизводимости
			if len(args) != 1 {
				return builtinErrorWrongArgCount("случайное.семя", 1, len(args))
			}
			i, ok := args[0].(*Integer)
			if !ok {
				return builtinErrorWrongArgType("случайное.семя", 1, "INTEGER", string(args[0].Type()))
			}
			rng = rand.New(rand.NewSource(i.Value))
			return NULL
		},
		"буква": func(args ...Object) Object {
			// случайная заглавная латинская
			return &String{Value: string('A' + rune(rng.Intn(26)))}
		},
	}
	stdModules["случайное"] = makeHashFromBuiltins(fns)
}

// === Расширение модуля "ос" — реальный запуск процессов ===

func extendOsModule() {
	mod, ok := stdModules["ос"]
	if !ok {
		return
	}

	addFn := func(name string, fn func(args ...Object) Object) {
		key := &String{Value: name}
		mod.Pairs[key.HashKey()] = HashPair{Key: key, Value: &Builtin{Fn: fn}}
	}

	addFn("запустить", func(args ...Object) Object {
		// ос.запустить(команда, ...аргументы) → строка stdout
		if len(args) < 1 {
			return builtinErrorWrongArgCount("ос.запустить", 1, len(args))
		}
		cmd, ok := args[0].(*String)
		if !ok {
			return builtinErrorWrongArgType("ос.запустить", 1, "STRING", string(args[0].Type()))
		}
		cmdArgs := make([]string, 0, len(args)-1)
		for i, a := range args[1:] {
			s, ok := a.(*String)
			if !ok {
				return builtinErrorWrongArgType("ос.запустить", i+2, "STRING", string(a.Type()))
			}
			cmdArgs = append(cmdArgs, s.Value)
		}
		out, err := exec.Command(cmd.Value, cmdArgs...).Output()
		if err != nil {
			return ErrorWithHint(currentCallToken, "ошибка запуска: "+err.Error(), "")
		}
		return &String{Value: string(out)}
	})

	addFn("выполнить", func(args ...Object) Object {
		// ос.выполнить(команда, ...аргументы) → {статус, вывод, ошибка}
		if len(args) < 1 {
			return builtinErrorWrongArgCount("ос.выполнить", 1, len(args))
		}
		cmd, ok := args[0].(*String)
		if !ok {
			return builtinErrorWrongArgType("ос.выполнить", 1, "STRING", string(args[0].Type()))
		}
		cmdArgs := make([]string, 0, len(args)-1)
		for _, a := range args[1:] {
			s, _ := a.(*String)
			if s != nil {
				cmdArgs = append(cmdArgs, s.Value)
			}
		}
		c := exec.Command(cmd.Value, cmdArgs...)
		var stdout, stderr bytes.Buffer
		c.Stdout = &stdout
		c.Stderr = &stderr
		err := c.Run()
		exitCode := 0
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = -1
			}
		}
		result := make(map[HashKey]HashPair)
		k1 := &String{Value: "статус"}
		result[k1.HashKey()] = HashPair{Key: k1, Value: &Integer{Value: int64(exitCode)}}
		k2 := &String{Value: "вывод"}
		result[k2.HashKey()] = HashPair{Key: k2, Value: &String{Value: stdout.String()}}
		k3 := &String{Value: "ошибки"}
		result[k3.HashKey()] = HashPair{Key: k3, Value: &String{Value: stderr.String()}}
		k4 := &String{Value: "сообщение"}
		result[k4.HashKey()] = HashPair{Key: k4, Value: &String{Value: errMsg}}
		return &Hash{Pairs: result}
	})
}

// === Модуль "cli" — парсер аргументов командной строки ===

func registerCliModule() {
	fns := map[string]func(args ...Object) Object{
		"флаг": func(args ...Object) Object {
			// cli.флаг("--имя") → значение или нет
			// Поддерживает --имя=значение и --имя значение
			name, err := getString(args, 0, "cli.флаг")
			if err != nil {
				return err
			}
			osArgs := getOsArgs()
			for i, a := range osArgs {
				if strings.HasPrefix(a, name+"=") {
					return &String{Value: strings.TrimPrefix(a, name+"=")}
				}
				if a == name && i+1 < len(osArgs) && !strings.HasPrefix(osArgs[i+1], "-") {
					return &String{Value: osArgs[i+1]}
				}
			}
			return NULL
		},
		"есть_флаг": func(args ...Object) Object {
			// cli.есть_флаг("--помощь") → да/нет
			name, err := getString(args, 0, "cli.есть_флаг")
			if err != nil {
				return err
			}
			osArgs := getOsArgs()
			for _, a := range osArgs {
				if a == name || strings.HasPrefix(a, name+"=") {
					return TRUE
				}
			}
			return FALSE
		},
		"позиционные": func(args ...Object) Object {
			// cli.позиционные() → массив всех аргументов, не начинающихся с '-'
			// Соглашение: значения флагов всегда через '=' (--имя=Анна).
			// Тогда позиционные = всё, что не начинается с '-'.
			osArgs := getOsArgs()
			result := make([]Object, 0)
			for _, a := range osArgs {
				if strings.HasPrefix(a, "-") {
					continue
				}
				result = append(result, &String{Value: a})
			}
			return &Array{Elements: result}
		},
		"позиционный": func(args ...Object) Object {
			if len(args) != 1 {
				return builtinErrorWrongArgCount("cli.позиционный", 1, len(args))
			}
			i, ok := args[0].(*Integer)
			if !ok {
				return builtinErrorWrongArgType("cli.позиционный", 1, "INTEGER", string(args[0].Type()))
			}
			osArgs := getOsArgs()
			pos := make([]string, 0)
			for _, a := range osArgs {
				if strings.HasPrefix(a, "-") {
					continue
				}
				pos = append(pos, a)
			}
			if int(i.Value) < 0 || int(i.Value) >= len(pos) {
				return NULL
			}
			return &String{Value: pos[i.Value]}
		},
		"все_флаги": func(args ...Object) Object {
			// cli.все_флаги() → словарь {флаг: значение или да}
			osArgs := getOsArgs()
			result := make(map[HashKey]HashPair)
			for _, a := range osArgs {
				if !strings.HasPrefix(a, "-") {
					continue
				}
				if eq := strings.Index(a, "="); eq != -1 {
					k := &String{Value: a[:eq]}
					v := &String{Value: a[eq+1:]}
					result[k.HashKey()] = HashPair{Key: k, Value: v}
				} else {
					k := &String{Value: a}
					result[k.HashKey()] = HashPair{Key: k, Value: TRUE}
				}
			}
			return &Hash{Pairs: result}
		},
	}
	stdModules["cli"] = makeHashFromBuiltins(fns)
}

// getOsArgs возвращает аргументы программы без бинарника и имени скрипта
// os.Args = ["yasny", "script.ya", "--flag", "value", "pos1"]
// возвращаем: ["--flag", "value", "pos1"]
func getOsArgs() []string {
	all := os.Args
	if len(all) <= 2 {
		return []string{}
	}
	return all[2:]
}

func init() {
	registerHttpModule()
	registerCsvModule()
	registerRandomModule()
	extendOsModule()
	registerCliModule()
}
