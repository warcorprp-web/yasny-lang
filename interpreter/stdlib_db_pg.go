//go:build !js

package interpreter

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/lib/pq" // PostgreSQL драйвер
)

// registerPostgresInDB добавляет метод "подключить" в модуль бд.
// Вызывается из init() в stdlib_modules.go.
// Не компилируется для WASM (build tag !js).
func registerPostgresInDB() {
	mod, ok := stdModules["бд"]
	if !ok {
		return
	}

	mod.Set(&String{Value: "подключить"}, &Builtin{Fn: func(args ...Object) Object {
		if len(args) != 1 {
			return ErrorWithHint(currentCallToken,
				"подключить требует строку подключения",
				"бд.подключить(\"postgres://user:pass@localhost:5432/dbname?sslmode=disable\")")
		}
		connStr, ok := args[0].(*String)
		if !ok {
			return ErrorWithHint(currentCallToken, "аргумент должен быть строкой", "")
		}

		// Поддерживаем оба формата:
		// 1. URL: postgres://user:pass@host:port/db?sslmode=disable
		// 2. DSN: host=localhost port=5432 user=... password=... dbname=... sslmode=disable
		driverName := "postgres"
		dsn := connStr.Value

		db, err := sql.Open(driverName, dsn)
		if err != nil {
			return ErrorWithHint(currentCallToken, "не удалось открыть PostgreSQL: "+err.Error(), "Проверьте строку подключения.")
		}
		if err := db.Ping(); err != nil {
			db.Close()
			return ErrorWithHint(currentCallToken, "не удалось подключиться к PostgreSQL: "+err.Error(),
				"Убедитесь что сервер запущен и данные верны.")
		}

		return newPgConnection(db, connStr.Value)
	}})
}

// newPgConnection создаёт hash с методами для PostgreSQL.
// Отличия от SQLite:
// - Параметры через $1, $2 (но мы автоматически конвертируем ? → $N)
// - Интроспекция через information_schema
// - Миграции через ту же таблицу __миграции__
func newPgConnection(db *sql.DB, connStr string) *Hash {
	// Обёртка exec которая конвертирует ? в $1, $2, ...
	pgExec := &pgExecutor{db: db}

	h := makeExecutorMethods(pgExec)
	h.Set(&String{Value: "__тип__"}, &String{Value: "postgresql"})

	closed := false
	h.Set(&String{Value: "закрыть"}, &Builtin{Fn: func(args ...Object) Object {
		if !closed {
			db.Close()
			closed = true
		}
		return NULL
	}})

	// транзакция
	h.Set(&String{Value: "транзакция"}, &Builtin{Fn: func(args ...Object) Object {
		if len(args) != 1 || (args[0].Type() != "FUNCTION" && args[0].Type() != "BUILTIN") {
			return ErrorWithHint(currentCallToken, "транзакция требует одну функцию", "")
		}
		tx, err := db.Begin()
		if err != nil {
			return ErrorWithHint(currentCallToken, "не удалось начать транзакцию: "+err.Error(), "")
		}
		txHash := makeExecutorMethods(&pgTxExecutor{tx: tx})
		result := ApplyFunctionCallback(args[0], []Object{txHash})
		if isError(result) {
			tx.Rollback()
			return result
		}
		if err := tx.Commit(); err != nil {
			return ErrorWithHint(currentCallToken, "не удалось зафиксировать: "+err.Error(), "")
		}
		return result
	}})

	// подготовить
	h.Set(&String{Value: "подготовить"}, &Builtin{Fn: func(args ...Object) Object {
		if len(args) != 1 {
			return ErrorWithHint(currentCallToken, "подготовить(sql)", "")
		}
		s, ok := args[0].(*String)
		if !ok {
			return ErrorWithHint(currentCallToken, "аргумент должен быть строкой SQL", "")
		}
		converted := convertPlaceholders(s.Value)
		return newPreparedStatement(db, converted)
	}})

	// таблицы
	h.Set(&String{Value: "таблицы"}, &Builtin{Fn: func(args ...Object) Object {
		rows, err := db.Query("SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name")
		if err != nil {
			return ErrorWithHint(currentCallToken, "ошибка: "+err.Error(), "")
		}
		defer rows.Close()
		result := []Object{}
		for rows.Next() {
			var name string
			rows.Scan(&name)
			result = append(result, &String{Value: name})
		}
		return &Array{Elements: result}
	}})

	// колонки
	h.Set(&String{Value: "колонки"}, &Builtin{Fn: func(args ...Object) Object {
		if len(args) != 1 {
			return ErrorWithHint(currentCallToken, "колонки(таблица)", "")
		}
		tableName, ok := args[0].(*String)
		if !ok {
			return ErrorWithHint(currentCallToken, "аргумент должен быть строкой", "")
		}
		rows, err := db.Query(`
			SELECT column_name, data_type, is_nullable, column_default,
			       (SELECT COUNT(*) FROM information_schema.key_column_usage k
			        JOIN information_schema.table_constraints c ON k.constraint_name = c.constraint_name
			        WHERE c.constraint_type = 'PRIMARY KEY' AND k.table_name = $1 AND k.column_name = columns.column_name) as is_pk
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1
			ORDER BY ordinal_position`, tableName.Value)
		if err != nil {
			return ErrorWithHint(currentCallToken, "ошибка: "+err.Error(), "")
		}
		defer rows.Close()
		result := []Object{}
		for rows.Next() {
			var name, colType, nullable string
			var dflt sql.NullString
			var isPK int
			rows.Scan(&name, &colType, &nullable, &dflt, &isPK)
			col := NewHash()
			col.Set(&String{Value: "имя"}, &String{Value: name})
			col.Set(&String{Value: "тип"}, &String{Value: colType})
			col.Set(&String{Value: "обязательное"}, nativeBoolToBoolean(nullable == "NO"))
			col.Set(&String{Value: "первичный_ключ"}, nativeBoolToBoolean(isPK > 0))
			if dflt.Valid {
				col.Set(&String{Value: "по_умолчанию"}, &String{Value: dflt.String})
			} else {
				col.Set(&String{Value: "по_умолчанию"}, NULL)
			}
			result = append(result, col)
		}
		return &Array{Elements: result}
	}})

	// мигрировать
	h.Set(&String{Value: "мигрировать"}, &Builtin{Fn: func(args ...Object) Object {
		db.Exec("CREATE TABLE IF NOT EXISTS __миграции__ (версия INTEGER PRIMARY KEY, применена TIMESTAMP DEFAULT NOW())")
		if len(args) != 2 {
			return ErrorWithHint(currentCallToken, "мигрировать(версия, sql)", "")
		}
		versionObj, ok := args[0].(*Integer)
		if !ok {
			return ErrorWithHint(currentCallToken, "версия должна быть целым числом", "")
		}
		sqlStr, ok := args[1].(*String)
		if !ok {
			return ErrorWithHint(currentCallToken, "второй аргумент — SQL", "")
		}
		var count int
		db.QueryRow("SELECT COUNT(*) FROM __миграции__ WHERE версия = $1", versionObj.Value).Scan(&count)
		if count > 0 {
			return FALSE
		}
		_, err := db.Exec(sqlStr.Value)
		if err != nil {
			return ErrorWithHint(currentCallToken, fmt.Sprintf("ошибка миграции %d: %s", versionObj.Value, err.Error()), "")
		}
		db.Exec("INSERT INTO __миграции__ (версия) VALUES ($1)", versionObj.Value)
		return TRUE
	}})

	// версия
	h.Set(&String{Value: "версия"}, &Builtin{Fn: func(args ...Object) Object {
		db.Exec("CREATE TABLE IF NOT EXISTS __миграции__ (версия INTEGER PRIMARY KEY, применена TIMESTAMP DEFAULT NOW())")
		var version sql.NullInt64
		db.QueryRow("SELECT MAX(версия) FROM __миграции__").Scan(&version)
		if !version.Valid {
			return NewInteger(0)
		}
		return NewInteger(version.Int64)
	}})

	return h
}

// convertPlaceholders заменяет ? на $1, $2, $3... для PostgreSQL.
// Пропускает ? внутри строковых литералов.
func convertPlaceholders(sql string) string {
	var result strings.Builder
	n := 1
	inString := false
	for i := 0; i < len(sql); i++ {
		ch := sql[i]
		if ch == '\'' {
			inString = !inString
		}
		if ch == '?' && !inString {
			result.WriteString(fmt.Sprintf("$%d", n))
			n++
		} else {
			result.WriteByte(ch)
		}
	}
	return result.String()
}

// pgExecutor оборачивает *sql.DB и конвертирует ? → $N.
type pgExecutor struct {
	db *sql.DB
}

func (e *pgExecutor) Exec(query string, args ...any) (sql.Result, error) {
	return e.db.Exec(convertPlaceholders(query), args...)
}

func (e *pgExecutor) Query(query string, args ...any) (*sql.Rows, error) {
	return e.db.Query(convertPlaceholders(query), args...)
}

// pgTxExecutor — то же для транзакций.
type pgTxExecutor struct {
	tx *sql.Tx
}

func (e *pgTxExecutor) Exec(query string, args ...any) (sql.Result, error) {
	return e.tx.Exec(convertPlaceholders(query), args...)
}

func (e *pgTxExecutor) Query(query string, args ...any) (*sql.Rows, error) {
	return e.tx.Query(convertPlaceholders(query), args...)
}
