package interpreter

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Хелперы для извлечения аргументов

func getString(args []Object, idx int, fnName string) (string, *Error) {
	if idx >= len(args) {
		return "", builtinErrorWrongArgCount(fnName, idx+1, len(args))
	}
	s, ok := args[idx].(*String)
	if !ok {
		return "", builtinErrorWrongArgType(fnName, idx+1, "STRING (строка)", string(args[idx].Type()))
	}
	return s.Value, nil
}

func nullObj() Object { return NULL }

// === Модуль "файлы" ===

func registerFilesModule() {
	fns := map[string]func(args ...Object) Object{
		"читать": func(args ...Object) Object {
			path, err := getString(args, 0, "файлы.читать")
			if err != nil {
				return err
			}
			data, e := os.ReadFile(path)
			if e != nil {
				return ErrorWithHint(currentCallToken, "не удалось прочитать файл: "+e.Error(), "Проверьте путь и права доступа.")
			}
			return &String{Value: string(data)}
		},
		"записать": func(args ...Object) Object {
			path, err := getString(args, 0, "файлы.записать")
			if err != nil {
				return err
			}
			content, err := getString(args, 1, "файлы.записать")
			if err != nil {
				return err
			}
			if e := os.WriteFile(path, []byte(content), 0644); e != nil {
				return ErrorWithHint(currentCallToken, "не удалось записать файл: "+e.Error(), "")
			}
			return NULL
		},
		"добавить": func(args ...Object) Object {
			path, err := getString(args, 0, "файлы.добавить")
			if err != nil {
				return err
			}
			content, err := getString(args, 1, "файлы.добавить")
			if err != nil {
				return err
			}
			f, e := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if e != nil {
				return ErrorWithHint(currentCallToken, "не удалось открыть файл: "+e.Error(), "")
			}
			defer f.Close()
			if _, e := f.WriteString(content); e != nil {
				return ErrorWithHint(currentCallToken, "не удалось записать: "+e.Error(), "")
			}
			return NULL
		},
		"существует": func(args ...Object) Object {
			path, err := getString(args, 0, "файлы.существует")
			if err != nil {
				return err
			}
			_, e := os.Stat(path)
			if e == nil {
				return TRUE
			}
			if os.IsNotExist(e) {
				return FALSE
			}
			return FALSE
		},
		"удалить": func(args ...Object) Object {
			path, err := getString(args, 0, "файлы.удалить")
			if err != nil {
				return err
			}
			if e := os.Remove(path); e != nil {
				return ErrorWithHint(currentCallToken, "не удалось удалить: "+e.Error(), "")
			}
			return NULL
		},
		"переименовать": func(args ...Object) Object {
			oldP, err := getString(args, 0, "файлы.переименовать")
			if err != nil {
				return err
			}
			newP, err := getString(args, 1, "файлы.переименовать")
			if err != nil {
				return err
			}
			if e := os.Rename(oldP, newP); e != nil {
				return ErrorWithHint(currentCallToken, "не удалось переименовать: "+e.Error(), "")
			}
			return NULL
		},
		"список": func(args ...Object) Object {
			path, err := getString(args, 0, "файлы.список")
			if err != nil {
				return err
			}
			entries, e := os.ReadDir(path)
			if e != nil {
				return ErrorWithHint(currentCallToken, "не удалось прочитать директорию: "+e.Error(), "")
			}
			result := make([]Object, 0, len(entries))
			for _, en := range entries {
				result = append(result, &String{Value: en.Name()})
			}
			return &Array{Elements: result}
		},
		"создать_дир": func(args ...Object) Object {
			path, err := getString(args, 0, "файлы.создать_дир")
			if err != nil {
				return err
			}
			if e := os.MkdirAll(path, 0755); e != nil {
				return ErrorWithHint(currentCallToken, "не удалось создать директорию: "+e.Error(), "")
			}
			return NULL
		},
		"удалить_дир": func(args ...Object) Object {
			path, err := getString(args, 0, "файлы.удалить_дир")
			if err != nil {
				return err
			}
			if e := os.RemoveAll(path); e != nil {
				return ErrorWithHint(currentCallToken, "не удалось удалить директорию: "+e.Error(), "")
			}
			return NULL
		},
		"размер": func(args ...Object) Object {
			path, err := getString(args, 0, "файлы.размер")
			if err != nil {
				return err
			}
			info, e := os.Stat(path)
			if e != nil {
				return ErrorWithHint(currentCallToken, "не удалось получить размер: "+e.Error(), "")
			}
			return &Integer{Value: info.Size()}
		},
		"это_файл": func(args ...Object) Object {
			path, err := getString(args, 0, "файлы.это_файл")
			if err != nil {
				return err
			}
			info, e := os.Stat(path)
			if e != nil {
				return FALSE
			}
			if info.Mode().IsRegular() {
				return TRUE
			}
			return FALSE
		},
		"это_дир": func(args ...Object) Object {
			path, err := getString(args, 0, "файлы.это_дир")
			if err != nil {
				return err
			}
			info, e := os.Stat(path)
			if e != nil {
				return FALSE
			}
			if info.IsDir() {
				return TRUE
			}
			return FALSE
		},
	}
	stdModules["файлы"] = makeHashFromBuiltins(fns)
}

// === Модуль "путь" ===

func registerPathModule() {
	fns := map[string]func(args ...Object) Object{
		"соединить": func(args ...Object) Object {
			parts := make([]string, 0, len(args))
			for i, a := range args {
				s, ok := a.(*String)
				if !ok {
					return builtinErrorWrongArgType("путь.соединить", i+1, "STRING", string(a.Type()))
				}
				parts = append(parts, s.Value)
			}
			return &String{Value: filepath.Join(parts...)}
		},
		"расширение": func(args ...Object) Object {
			s, err := getString(args, 0, "путь.расширение")
			if err != nil {
				return err
			}
			return &String{Value: filepath.Ext(s)}
		},
		"имя": func(args ...Object) Object {
			s, err := getString(args, 0, "путь.имя")
			if err != nil {
				return err
			}
			return &String{Value: filepath.Base(s)}
		},
		"директория": func(args ...Object) Object {
			s, err := getString(args, 0, "путь.директория")
			if err != nil {
				return err
			}
			return &String{Value: filepath.Dir(s)}
		},
		"абсолютный": func(args ...Object) Object {
			s, err := getString(args, 0, "путь.абсолютный")
			if err != nil {
				return err
			}
			abs, e := filepath.Abs(s)
			if e != nil {
				return ErrorWithHint(currentCallToken, "не удалось вычислить путь: "+e.Error(), "")
			}
			return &String{Value: abs}
		},
		"относительный": func(args ...Object) Object {
			base, err := getString(args, 0, "путь.относительный")
			if err != nil {
				return err
			}
			target, err := getString(args, 1, "путь.относительный")
			if err != nil {
				return err
			}
			rel, e := filepath.Rel(base, target)
			if e != nil {
				return ErrorWithHint(currentCallToken, "не удалось вычислить относительный путь: "+e.Error(), "")
			}
			return &String{Value: rel}
		},
		"очистить": func(args ...Object) Object {
			s, err := getString(args, 0, "путь.очистить")
			if err != nil {
				return err
			}
			return &String{Value: filepath.Clean(s)}
		},
	}
	stdModules["путь"] = makeHashFromBuiltins(fns)
}

// === Модуль "ос" ===

func registerOsModule() {
	fns := map[string]func(args ...Object) Object{
		"переменная": func(args ...Object) Object {
			name, err := getString(args, 0, "ос.переменная")
			if err != nil {
				return err
			}
			val, ok := os.LookupEnv(name)
			if !ok {
				return NULL
			}
			return &String{Value: val}
		},
		"установить_переменную": func(args ...Object) Object {
			name, err := getString(args, 0, "ос.установить_переменную")
			if err != nil {
				return err
			}
			val, err := getString(args, 1, "ос.установить_переменную")
			if err != nil {
				return err
			}
			if e := os.Setenv(name, val); e != nil {
				return ErrorWithHint(currentCallToken, "не удалось установить переменную: "+e.Error(), "")
			}
			return NULL
		},
		"аргументы": func(args ...Object) Object {
			result := make([]Object, 0, len(os.Args))
			for _, a := range os.Args {
				result = append(result, &String{Value: a})
			}
			return &Array{Elements: result}
		},
		"выход": func(args ...Object) Object {
			code := 0
			if len(args) >= 1 {
				if i, ok := args[0].(*Integer); ok {
					code = int(i.Value)
				}
			}
			os.Exit(code)
			return NULL
		},
		"текущая_директория": func(args ...Object) Object {
			d, e := os.Getwd()
			if e != nil {
				return ErrorWithHint(currentCallToken, "не удалось получить директорию: "+e.Error(), "")
			}
			return &String{Value: d}
		},
		"сменить_директорию": func(args ...Object) Object {
			path, err := getString(args, 0, "ос.сменить_директорию")
			if err != nil {
				return err
			}
			if e := os.Chdir(path); e != nil {
				return ErrorWithHint(currentCallToken, "не удалось сменить директорию: "+e.Error(), "")
			}
			return NULL
		},
		"домашняя_директория": func(args ...Object) Object {
			d, e := os.UserHomeDir()
			if e != nil {
				return ErrorWithHint(currentCallToken, "не удалось получить домашнюю директорию: "+e.Error(), "")
			}
			return &String{Value: d}
		},
		"временная_директория": func(args ...Object) Object {
			return &String{Value: os.TempDir()}
		},
		"запустить": func(args ...Object) Object {
			// ос.запустить(команда, ...аргументы) → строка с stdout
			// Запрещено в WASM, но в нативе работает
			return ErrorWithHint(currentCallToken,
				"ос.запустить пока не реализовано",
				"Используйте читать_файл/записать_файл для работы с данными.")
		},
	}

	values := map[string]Object{
		"имя": &String{Value: runtime.GOOS},
		// "linux", "darwin", "windows", "js" (для wasm)
		"архитектура": &String{Value: runtime.GOARCH},
		// "amd64", "arm64", "wasm"
		"разделитель_путей": &String{Value: string(os.PathSeparator)},
	}
	mod := makeHashFromValues(values)
	for name, fn := range fns {
		key := &String{Value: name}
		mod.Pairs[key.HashKey()] = HashPair{Key: key, Value: &Builtin{Fn: fn}}
	}
	stdModules["ос"] = mod
}

// === Модуль "крипто" ===

func registerCryptoModule() {
	fns := map[string]func(args ...Object) Object{
		"sha256": func(args ...Object) Object {
			s, err := getString(args, 0, "крипто.sha256")
			if err != nil {
				return err
			}
			h := sha256.Sum256([]byte(s))
			return &String{Value: hex.EncodeToString(h[:])}
		},
		"sha1": func(args ...Object) Object {
			s, err := getString(args, 0, "крипто.sha1")
			if err != nil {
				return err
			}
			h := sha1.Sum([]byte(s))
			return &String{Value: hex.EncodeToString(h[:])}
		},
		"md5": func(args ...Object) Object {
			s, err := getString(args, 0, "крипто.md5")
			if err != nil {
				return err
			}
			h := md5.Sum([]byte(s))
			return &String{Value: hex.EncodeToString(h[:])}
		},
		"base64_кодировать": func(args ...Object) Object {
			s, err := getString(args, 0, "крипто.base64_кодировать")
			if err != nil {
				return err
			}
			return &String{Value: base64.StdEncoding.EncodeToString([]byte(s))}
		},
		"base64_декодировать": func(args ...Object) Object {
			s, err := getString(args, 0, "крипто.base64_декодировать")
			if err != nil {
				return err
			}
			data, e := base64.StdEncoding.DecodeString(s)
			if e != nil {
				return ErrorWithHint(currentCallToken, "не удалось декодировать base64: "+e.Error(), "")
			}
			return &String{Value: string(data)}
		},
		"hex_кодировать": func(args ...Object) Object {
			s, err := getString(args, 0, "крипто.hex_кодировать")
			if err != nil {
				return err
			}
			return &String{Value: hex.EncodeToString([]byte(s))}
		},
		"hex_декодировать": func(args ...Object) Object {
			s, err := getString(args, 0, "крипто.hex_декодировать")
			if err != nil {
				return err
			}
			data, e := hex.DecodeString(s)
			if e != nil {
				return ErrorWithHint(currentCallToken, "не удалось декодировать hex: "+e.Error(), "")
			}
			return &String{Value: string(data)}
		},
	}
	stdModules["крипто"] = makeHashFromBuiltins(fns)
}

// === Дополнения к "время" ===

func extendTimeModule() {
	mod, ok := stdModules["время"]
	if !ok {
		return
	}

	addFn := func(name string, fn func(args ...Object) Object) {
		key := &String{Value: name}
		mod.Pairs[key.HashKey()] = HashPair{Key: key, Value: &Builtin{Fn: fn}}
	}

	addFn("разобрать", func(args ...Object) Object {
		s, err := getString(args, 0, "время.разобрать")
		if err != nil {
			return err
		}
		format := "2006-01-02 15:04:05"
		if len(args) >= 2 {
			f, err := getString(args, 1, "время.разобрать")
			if err != nil {
				return err
			}
			format = f
		}
		t, e := time.Parse(format, s)
		if e != nil {
			return ErrorWithHint(currentCallToken, "не удалось разобрать время: "+e.Error(),
				"Формат должен совпадать со строкой. Пример формата: 2006-01-02 15:04:05")
		}
		return &Integer{Value: t.UnixMilli()}
	})

	addFn("формат", func(args ...Object) Object {
		// время.формат(метка_мс, формат)
		if len(args) < 2 {
			return builtinErrorWrongArgCount("время.формат", 2, len(args))
		}
		ms, ok := args[0].(*Integer)
		if !ok {
			return builtinErrorWrongArgType("время.формат", 1, "INTEGER (метка)", string(args[0].Type()))
		}
		format, err := getString(args, 1, "время.формат")
		if err != nil {
			return err
		}
		t := time.Unix(0, ms.Value*int64(time.Millisecond))
		return &String{Value: t.Format(format)}
	})

	addFn("разница", func(args ...Object) Object {
		// время.разница(метка1, метка2) → миллисекунды
		if len(args) < 2 {
			return builtinErrorWrongArgCount("время.разница", 2, len(args))
		}
		a, ok := args[0].(*Integer)
		if !ok {
			return builtinErrorWrongArgType("время.разница", 1, "INTEGER", string(args[0].Type()))
		}
		b, ok := args[1].(*Integer)
		if !ok {
			return builtinErrorWrongArgType("время.разница", 2, "INTEGER", string(args[1].Type()))
		}
		diff := b.Value - a.Value
		if diff < 0 {
			diff = -diff
		}
		return &Integer{Value: diff}
	})
}

// === Регистрация всех новых модулей ===

func init() {
	registerFilesModule()
	registerPathModule()
	registerOsModule()
	registerCryptoModule()
	extendTimeModule()

	// Заглушка: подавим "unused" предупреждения
	_ = strings.ToLower
	_ = nullObj
}
