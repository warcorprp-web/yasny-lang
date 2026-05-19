package lexer

// TokenType представляет тип токена
type TokenType string

// Token представляет один токен в исходном коде
type Token struct {
	Type     TokenType
	Literal  string
	Line     int
	Column   int
	Filename string
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
	
	PLUS_ASSIGN     = "+="
	MINUS_ASSIGN    = "-="
	ASTERISK_ASSIGN = "*="
	SLASH_ASSIGN    = "/="
	
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
	PIPE         = "|>"
	AT           = "@"
	QUESTION     = "?"
	NULLISH      = "??"

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
	STEP     = "ПО"
	IN       = "В"
	WHILE    = "ПОКА"
	REPEAT   = "ПОВТОРИ"
	TIMES    = "РАЗ"
	CLASS    = "КЛАСС"
	EXTENDS  = "НАСЛЕДУЕТ"
	ENUM     = "ПЕРЕЧИСЛЕНИЕ"
	TEST     = "ТЕСТ"
	ASSERT   = "ПРОВЕРИТЬ"
	STATIC   = "СТАТИЧНАЯ"
	YIELD    = "ВЫДАТЬ"
	ASYNC    = "АСИНХ"
	AWAIT    = "ЖДАТЬ"
	NEW      = "НОВЫЙ"
	PARENT   = "РОДИТЕЛЬ"
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

// keywords содержит все ключевые слова языка.
// Список канонический — у каждого понятия одно слово.
var keywords = map[string]TokenType{
	"конст":        LET,
	"перем":        VAR,
	"функция":      FUNCTION,
	"вернуть":      RETURN,
	"если":         IF,
	"иначеесли":    ELSE_IF,
	"иначе":        ELSE,
	"для":          FOR,
	"от":           FROM,
	"до":           TO,
	"по":           STEP,
	"в":            IN,
	"пока":         WHILE,
	"повтори":      REPEAT,
	"раз":          TIMES,
	"класс":        CLASS,
	"перечисление": ENUM,
	"тест":         TEST,
	"проверить":    ASSERT,
	"статичная":    STATIC,
	"выдать":       YIELD,
	"асинх":        ASYNC,
	"ждать":        AWAIT,
	"наследует":    EXTENDS,
	"родитель":     PARENT,
	"новый":        NEW,
	"конец":        END,
	"да":           TRUE,
	"нет":          FALSE,
	"и":            AND,
	"или":          OR,
	"не":           NOT,
	"это":          THIS,
	"попытка":      TRY,
	"поймать":      CATCH,
	"всегда":       FINALLY,
	"бросить":      THROW,
	"прервать":     BREAK,
	"продолжить":   CONTINUE,
	"совпадение":   MATCH,
	"когда":        WHEN,
	"импорт":       IMPORT,
	"экспорт":      EXPORT,
	"из":           FROM_KW,
	"как":          AS,
}

// LookupIdent проверяет, является ли идентификатор ключевым словом
func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}
