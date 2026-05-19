package interpreter

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // pure Go SQLite — без CGo
)

// === Модуль "бд" ===
//
// Работа с реляционными базами данных. Поддерживается SQLite (драйвер
// встроен, файлы или :memory:). Архитектура такова, что добавить
// PostgreSQL/MySQL — это импорт нового драйвера и одна строка в
// "открыть".
//
// Использование:
//
//   импорт бд из "бд"
//   конст соед = бд.открыть("данные.db")
//   соед.выполнить("CREATE TABLE люди (имя TEXT, возраст INTEGER)")
//   соед.выполнить("INSERT INTO люди VALUES (?, ?)", "Анна", 25)
//   конст ряды = соед.запрос("SELECT * FROM люди WHERE возраст > ?", 18)
//   для ряд в ряды: вывод(ряд["имя"])
//
// Безопасность: ВСЕГДА используйте параметризованные запросы (?),
// никогда не вставляйте значения через конкатенацию строк — это
// SQL-инъекция.

// dbExecutor — общий интерфейс для DB и Tx. Позволяет переиспользовать
// функции запроса/выполнения между обычным соединением и транзакцией.
type dbExecutor interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
}

// objectsToArgs преобразует объекты Ясного в значения для драйвера БД.
func objectsToArgs(args []Object) ([]any, *Error) {
	result := make([]any, len(args))
	for i, a := range args {
		switch v := a.(type) {
		case *Integer:
			result[i] = v.Value
		case *Float:
			result[i] = v.Value
		case *String:
			result[i] = v.Value
		case *Boolean:
			result[i] = v.Value
		case *Null:
			result[i] = nil
		default:
			return nil, ErrorWithHint(
				currentCallToken,
				fmt.Sprintf("параметр %d имеет тип %s — поддерживаются только целое, дробное, строка, булево, ничего", i+1, translateType(string(a.Type()))),
				"Преобразуйте значение в один из поддерживаемых типов.",
			)
		}
	}
	return result, nil
}

// goValueToObject преобразует значение из БД в объект Ясного.
func goValueToObject(v any) Object {
	if v == nil {
		return NULL
	}
	switch val := v.(type) {
	case int64:
		return NewInteger(val)
	case int:
		return NewInteger(int64(val))
	case float64:
		return &Float{Value: val}
	case string:
		return &String{Value: val}
	case []byte:
		return &String{Value: string(val)}
	case bool:
		if val {
			return TRUE
		}
		return FALSE
	default:
		return &String{Value: fmt.Sprintf("%v", val)}
	}
}

// dbDoExec — общая реализация INSERT/UPDATE/DELETE/CREATE.
func dbDoExec(exec dbExecutor, args []Object) Object {
	if len(args) < 1 {
		return ErrorWithHint(currentCallToken, "выполнить требует SQL-запрос", "выполнить(\"INSERT ...\", знач1, знач2)")
	}
	sqlStr, ok := args[0].(*String)
	if !ok {
		return ErrorWithHint(currentCallToken, "первый аргумент должен быть строкой SQL", "")
	}
	params, errArg := objectsToArgs(args[1:])
	if errArg != nil {
		return errArg
	}
	res, err := exec.Exec(sqlStr.Value, params...)
	if err != nil {
		return ErrorWithHint(currentCallToken, "ошибка SQL: "+err.Error(), "Проверьте синтаксис запроса и параметры.")
	}
	insertID, _ := res.LastInsertId()
	affected, _ := res.RowsAffected()
	h := NewHash()
	h.Set(&String{Value: "вставлено_id"}, NewInteger(insertID))
	h.Set(&String{Value: "затронуто"}, NewInteger(affected))
	return h
}

// dbDoQuery — общая реализация SELECT, возвращает массив hash-ей.
func dbDoQuery(exec dbExecutor, args []Object) Object {
	if len(args) < 1 {
		return ErrorWithHint(currentCallToken, "запрос требует SQL", "запрос(\"SELECT ...\", знач1)")
	}
	sqlStr, ok := args[0].(*String)
	if !ok {
		return ErrorWithHint(currentCallToken, "первый аргумент должен быть строкой SQL", "")
	}
	params, errArg := objectsToArgs(args[1:])
	if errArg != nil {
		return errArg
	}
	rows, err := exec.Query(sqlStr.Value, params...)
	if err != nil {
		return ErrorWithHint(currentCallToken, "ошибка SQL: "+err.Error(), "")
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return ErrorWithHint(currentCallToken, "ошибка чтения колонок: "+err.Error(), "")
	}

	result := []Object{}
	for rows.Next() {
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return ErrorWithHint(currentCallToken, "ошибка чтения строки: "+err.Error(), "")
		}
		row := NewHash()
		for i, col := range cols {
			row.Set(&String{Value: col}, goValueToObject(values[i]))
		}
		result = append(result, row)
	}
	return &Array{Elements: result}
}

// dbDoQueryRow возвращает первую строку или NULL.
func dbDoQueryRow(exec dbExecutor, args []Object) Object {
	res := dbDoQuery(exec, args)
	if isError(res) {
		return res
	}
	arr := res.(*Array)
	if len(arr.Elements) == 0 {
		return NULL
	}
	return arr.Elements[0]
}

// dbDoScalar возвращает первое значение первой строки или NULL.
func dbDoScalar(exec dbExecutor, args []Object) Object {
	row := dbDoQueryRow(exec, args)
	if isError(row) {
		return row
	}
	if row == NULL {
		return NULL
	}
	hash := row.(*Hash)
	for _, k := range hash.Keys {
		if pair, ok := hash.Pairs[k]; ok {
			return pair.Value
		}
	}
	return NULL
}

// makeExecutorMethods создаёт hash с методами выполнить/запрос/строка/значение
// для любого dbExecutor (DB или Tx).
func makeExecutorMethods(exec dbExecutor) *Hash {
	h := NewHash()
	h.Set(&String{Value: "выполнить"}, &Builtin{Fn: func(args ...Object) Object {
		return dbDoExec(exec, args)
	}})
	h.Set(&String{Value: "запрос"}, &Builtin{Fn: func(args ...Object) Object {
		return dbDoQuery(exec, args)
	}})
	h.Set(&String{Value: "строка"}, &Builtin{Fn: func(args ...Object) Object {
		return dbDoQueryRow(exec, args)
	}})
	h.Set(&String{Value: "значение"}, &Builtin{Fn: func(args ...Object) Object {
		return dbDoScalar(exec, args)
	}})
	return h
}

// newDBConnection создаёт hash с методами для работы с открытой БД.
// Включает методы dbExecutor + закрыть/транзакция.
func newDBConnection(db *sql.DB, path string) *Hash {
	h := makeExecutorMethods(db)
	h.Set(&String{Value: "__путь__"}, &String{Value: path})

	closed := false

	h.Set(&String{Value: "закрыть"}, &Builtin{Fn: func(args ...Object) Object {
		if !closed {
			db.Close()
			closed = true
		}
		return NULL
	}})

	// транзакция(функция(тx)): автоматический Begin → выполнение
	// функции → Commit. Если функция бросает ошибку или возвращает
	// ошибку — Rollback.
	h.Set(&String{Value: "транзакция"}, &Builtin{Fn: func(args ...Object) Object {
		if len(args) != 1 {
			return ErrorWithHint(currentCallToken, "транзакция требует одну функцию", "транзакция((tx) => ...)")
		}
		fn := args[0]
		if fn.Type() != "FUNCTION" && fn.Type() != "BUILTIN" {
			return ErrorWithHint(currentCallToken, "аргумент должен быть функцией", "")
		}
		tx, err := db.Begin()
		if err != nil {
			return ErrorWithHint(currentCallToken, "не удалось начать транзакцию: "+err.Error(), "")
		}
		txHash := makeExecutorMethods(tx)
		result := ApplyFunctionCallback(fn, []Object{txHash})
		if isError(result) {
			tx.Rollback()
			return result
		}
		if err := tx.Commit(); err != nil {
			return ErrorWithHint(currentCallToken, "не удалось зафиксировать: "+err.Error(), "")
		}
		return result
	}})

	return h
}

func registerDBModule() {
	fns := map[string]func(args ...Object) Object{
		"открыть": func(args ...Object) Object {
			if len(args) != 1 {
				return ErrorWithHint(currentCallToken, "открыть требует один аргумент — путь к файлу или ':memory:'", "бд.открыть(\"данные.db\")")
			}
			path, ok := args[0].(*String)
			if !ok {
				return ErrorWithHint(currentCallToken, "путь должен быть строкой", "")
			}
			db, err := sql.Open("sqlite", path.Value)
			if err != nil {
				return ErrorWithHint(currentCallToken, "не удалось открыть БД: "+err.Error(), "Проверьте путь и права на запись.")
			}
			if err := db.Ping(); err != nil {
				db.Close()
				return ErrorWithHint(currentCallToken, "не удалось подключиться: "+err.Error(), "")
			}
			return newDBConnection(db, path.Value)
		},
	}
	stdModules["бд"] = makeHashFromBuiltins(fns)
}
