package lexer

// TokenType представляет тип токена
type TokenType string

// Token представляет один токен в исходном коде
type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Column  int
}

// Типы токенов
const (
	// Специальные
	ILLEGAL = "ILLEGAL"
	EOF     = "EOF"

	// Идентификаторы и литералы
	IDENT  = "IDENT"  // имена переменных, функций
	INT    = "INT"    // 123
	FLOAT  = "FLOAT"  // 123.45
	STRING = "STRING" // "hello"

	// Операторы
	ASSIGN   = "="
	PLUS     = "+"
	MINUS    = "-"
	BANG     = "!"
	ASTERISK = "*"
	SLASH    = "/"
	PERCENT  = "%"
	
	EQ     = "=="
	NOT_EQ = "!="
	LT     = "<"
	GT     = ">"
	LTE    = "<="
	GTE    = ">="

	// Разделители
	COMMA        = ","
	COLON        = ":"
	SEMICOLON    = ";"
	DOT          = "."
	DOTDOT       = ".."
	SPREAD       = "..."
	OPTIONAL_DOT = "?."
	LPAREN       = "("
	RPAREN       = ")"
	LBRACE       = "{"
	RBRACE       = "}"
	LBRACKET     = "["
	RBRACKET     = "]"
	ARROW        = "=>"
	RARROW       = "->"
	QUESTION     = "?"

	// Ключевые слова
	LET      = "ПУСТЬ"
	VAR      = "VAR"
	FUNCTION = "ФУНКЦИЯ"
	RETURN   = "ВЕРНУТЬ"
	IF       = "ЕСЛИ"
	ELSE_IF  = "ИНАЧЕЕСЛИ"
	ELSE     = "ИНАЧЕ"
	FOR      = "ДЛЯ"
	FROM     = "ОТ"
	TO       = "ДО"
	IN       = "В"
	WHILE    = "ПОКА"
	REPEAT   = "ПОВТОРИ"
	TIMES    = "РАЗ"
	CLASS    = "КЛАСС"
	NEW      = "НОВЫЙ"
	END      = "КОНЕЦ"
	TRUE     = "ДА"
	FALSE    = "НЕТ"
	AND      = "И"
	OR       = "ИЛИ"
	NOT      = "НЕ"
	THIS     = "ЭТО"
	TRY      = "ПОПЫТКА"
	CATCH    = "ПОЙМАТЬ"
	FINALLY  = "ВСЕГДА"
	THROW    = "БРОСИТЬ"
	BREAK    = "ПРЕРВАТЬ"
	CONTINUE = "ПРОДОЛЖИТЬ"
	MATCH    = "СОВПАДЕНИЕ"
	WHEN     = "КОГДА"
	IMPORT   = "ИМПОРТ"
	EXPORT   = "ЭКСПОРТ"
	FROM_KW  = "ИЗ"
	AS       = "КАК"
)

// keywords содержит все ключевые слова языка
var keywords = map[string]TokenType{
	"конст":      LET,
	"пусть":      LET, // алиас для совместимости
	"перем":      VAR,
	"var":        VAR, // алиас
	"функция":    FUNCTION,
	"процедура":  FUNCTION,
	"вернуть":    RETURN,
	"возврат":    RETURN,
	"если":       IF,
	"иначеесли":  ELSE_IF,
	"иначе":      ELSE,
	"для":        FOR,
	"от":         FROM,
	"до":         TO,
	"по":         TO,
	"в":          IN,
	"пока":       WHILE,
	"повтори":    REPEAT,
	"раз":        TIMES,
	"класс":      CLASS,
	"новый":      NEW,
	"конец":      END,
	"да":         TRUE,
	"истина":     TRUE,
	"нет":        FALSE,
	"ложь":       FALSE,
	"и":          AND,
	"или":        OR,
	"не":         NOT,
	"это":        THIS,
	"попытка":    TRY,
	"поймать":    CATCH,
	"всегда":     FINALLY,
	"бросить":    THROW,
	"прервать":   BREAK,
	"продолжить": CONTINUE,
	"совпадение": MATCH,
	"когда":      WHEN,
	"импорт":     IMPORT,
	"экспорт":    EXPORT,
	"из":         FROM_KW,
	"как":        AS,
}

// LookupIdent проверяет, является ли идентификатор ключевым словом
func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}
