package lexer

import (
	"unicode"
	"unicode/utf8"
)

// Lexer выполняет лексический анализ исходного кода
type Lexer struct {
	input        string
	position     int  // текущая позиция в байтах
	readPosition int  // следующая позиция
	ch           rune // текущий символ (rune для UTF-8)
	line         int
	column       int
	filename     string

	// Накопленные комментарии и сведения о пустых строках —
	// нужны форматеру для восстановления авторского форматирования.
	// Парсер для исполнения их игнорирует.
	comments       []Token
	blankLineMarks []int // номера строк, перед которыми была пустая строка
}

// New создает новый лексер
func New(input string) *Lexer {
	return NewWithFilename(input, "")
}

// NewWithFilename создает новый лексер с именем файла
func NewWithFilename(input, filename string) *Lexer {
	l := &Lexer{
		input:    input,
		line:     1,
		column:   0,
		filename: filename,
	}
	l.readChar()
	return l
}

// readChar читает следующий символ
func (l *Lexer) readChar() {
	l.position = l.readPosition
	if l.readPosition >= len(l.input) {
		l.ch = 0 // EOF
		l.readPosition++
	} else {
		r, size := utf8.DecodeRuneInString(l.input[l.readPosition:])
		l.ch = r
		l.readPosition += size
	}
	l.column++
	
	if l.ch == '\n' {
		l.line++
		l.column = 0
	}
}

// peekChar смотрит на следующий символ без продвижения
func (l *Lexer) peekChar() rune {
	if l.readPosition >= len(l.input) {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(l.input[l.readPosition:])
	return r
}

// NextToken возвращает следующий токен
func (l *Lexer) NextToken() Token {
	var tok Token

	l.skipWhitespace()

	// Комментарии сохраняются в l.comments через skipComments,
	// а парсер их пропускает. Цикл — на случай нескольких подряд.
	for l.ch == '#' {
		l.skipComments()
	}

	tok.Line = l.line
	tok.Column = l.column
	tok.Filename = l.filename

	switch l.ch {
	case '=':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: EQ, Literal: string(ch) + string(l.ch), Line: tok.Line, Column: tok.Column, Filename: l.filename}
		} else if l.peekChar() == '>' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: ARROW, Literal: string(ch) + string(l.ch), Line: tok.Line, Column: tok.Column, Filename: l.filename}
		} else {
			tok = l.newToken(ASSIGN, l.ch)
		}
	case '+':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: PLUS_ASSIGN, Literal: string(ch) + string(l.ch), Line: tok.Line, Column: tok.Column, Filename: l.filename}
		} else {
			tok = l.newToken(PLUS, l.ch)
		}
	case '-':
		if l.peekChar() == '>' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: RARROW, Literal: string(ch) + string(l.ch)}
		} else if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: MINUS_ASSIGN, Literal: string(ch) + string(l.ch)}
		} else {
			tok = l.newToken(MINUS, l.ch)
		}
	case '*':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: ASTERISK_ASSIGN, Literal: string(ch) + string(l.ch)}
		} else {
			tok = l.newToken(ASTERISK, l.ch)
		}
	case '/':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: SLASH_ASSIGN, Literal: string(ch) + string(l.ch)}
		} else if l.peekChar() == '/' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: INT_DIV, Literal: string(ch) + string(l.ch)}
		} else {
			tok = l.newToken(SLASH, l.ch)
		}
	case '%':
		tok = l.newToken(PERCENT, l.ch)
	case '!':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: NOT_EQ, Literal: string(ch) + string(l.ch)}
		} else {
			tok = l.newToken(BANG, l.ch)
		}
	case '<':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: LTE, Literal: string(ch) + string(l.ch)}
		} else {
			tok = l.newToken(LT, l.ch)
		}
	case '>':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: GTE, Literal: string(ch) + string(l.ch)}
		} else {
			tok = l.newToken(GT, l.ch)
		}
	case ',':
		tok = l.newToken(COMMA, l.ch)
	case '.':
		if l.peekChar() == '.' {
			l.readChar()
			if l.peekChar() == '.' {
				l.readChar()
				tok = Token{Type: SPREAD, Literal: "..."}
			} else {
				tok = Token{Type: DOTDOT, Literal: ".."}
			}
		} else {
			tok = l.newToken(DOT, l.ch)
		}
	case ':':
		tok = l.newToken(COLON, l.ch)
	case ';':
		tok = l.newToken(SEMICOLON, l.ch)
	case '(':
		tok = l.newToken(LPAREN, l.ch)
	case ')':
		tok = l.newToken(RPAREN, l.ch)
	case '{':
		tok = l.newToken(LBRACE, l.ch)
	case '}':
		tok = l.newToken(RBRACE, l.ch)
	case '[':
		tok = l.newToken(LBRACKET, l.ch)
	case ']':
		tok = l.newToken(RBRACKET, l.ch)
	case '?':
		if l.peekChar() == '.' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: OPTIONAL_DOT, Literal: string(ch) + string(l.ch)}
		} else if l.peekChar() == '?' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: NULLISH, Literal: string(ch) + string(l.ch)}
		} else {
			tok = l.newToken(QUESTION, l.ch)
		}
	case '|':
		if l.peekChar() == '>' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: PIPE, Literal: string(ch) + string(l.ch)}
		} else {
			tok = l.newToken(ILLEGAL, l.ch)
		}
	case '@':
		tok = l.newToken(AT, l.ch)
	case '"':
		if l.peekChar() == '"' {
			l.readChar()
			if l.peekChar() == '"' {
				l.readChar()
				tok.Type = STRING
				tok.Literal = l.readMultilineString()
			} else {
				tok.Type = STRING
				tok.Literal = ""
			}
		} else {
			tok.Type = STRING
			tok.Literal = l.readString('"')
		}
	case '\'':
		tok.Type = STRING
		tok.Literal = l.readString('\'')
	case 0:
		tok.Literal = ""
		tok.Type = EOF
	default:
		if isLetter(l.ch) {
			tok.Literal = l.readIdentifier()
			tok.Type = LookupIdent(tok.Literal)
			return tok
		} else if isDigit(l.ch) {
			return l.readNumber()
		} else {
			tok = l.newToken(ILLEGAL, l.ch)
		}
	}

	l.readChar()
	return tok
}

// skipWhitespace пропускает пробелы и переносы строк, попутно
// фиксирует номера строк, перед которыми оказалась пустая строка.
// Эта информация нужна форматеру для восстановления визуальной
// разметки кода.
func (l *Lexer) skipWhitespace() {
	consecutiveNewlines := 0
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		if l.ch == '\n' {
			consecutiveNewlines++
		} else if l.ch != '\r' {
			// Не \n и не \r — это пробел/таб, сбрасывать не надо,
			// потому что пробелы между \n не разрывают серию.
		}
		l.readChar()
	}
	// 2+ подряд '\n' = ≥ 1 пустой строки между токенами.
	if consecutiveNewlines >= 2 {
		l.blankLineMarks = append(l.blankLineMarks, l.line)
	}
}

// skipComments сохраняет комментарий как токен в l.comments и
// продвигается к следующему значимому символу.
func (l *Lexer) skipComments() {
	if l.ch == '#' {
		startLine := l.line
		startCol := l.column
		var sb []rune
		for l.ch != '\n' && l.ch != 0 {
			sb = append(sb, l.ch)
			l.readChar()
		}
		l.comments = append(l.comments, Token{
			Type:     COMMENT,
			Literal:  string(sb),
			Line:     startLine,
			Column:   startCol,
			Filename: l.filename,
		})
		l.skipWhitespace()
	}
}

// Comments возвращает все накопленные комментарии. Используется
// форматером после парсинга для восстановления авторских комментариев.
func (l *Lexer) Comments() []Token {
	return l.comments
}

// BlankLineMarks возвращает номера строк, перед которыми была
// пустая строка. Используется форматером для восстановления
// визуальных разделителей.
func (l *Lexer) BlankLineMarks() []int {
	return l.blankLineMarks
}

// readIdentifier читает идентификатор или ключевое слово.
// Символ ? допускается только в конце идентификатора (простое?),
// но не перед точкой (optional chaining: obj?.поле), не перед
// буквой/цифрой, и не перед пробелом+выражением (тернарник: x ? a : b).
func (l *Lexer) readIdentifier() string {
	position := l.position
	for isLetter(l.ch) || isDigit(l.ch) {
		l.readChar()
	}
	// ? включаем в идентификатор если за ним НЕ следует точка
	// (optional chaining: obj?.поле — ? должен остаться отдельным
	// токеном QUESTION).
	if l.ch == '?' && l.peekChar() != '.' {
		l.readChar()
	}
	return l.input[position:l.position]
}

// readNumber читает число (целое или дробное)
func (l *Lexer) readNumber() Token {
	position := l.position
	tokenType := TokenType(INT)

	for isDigit(l.ch) {
		l.readChar()
	}

	// Проверяем на дробное число
	if l.ch == '.' && isDigit(l.peekChar()) {
		tokenType = FLOAT
		l.readChar() // пропускаем точку
		for isDigit(l.ch) {
			l.readChar()
		}
	}

	return Token{
		Type:    tokenType,
		Literal: l.input[position:l.position],
		Line:    l.line,
		Column:  l.column,
	}
}

// readString читает строковый литерал
func (l *Lexer) readString(quote rune) string {
	var result []rune
	hasInterpolation := false
	braceDepth := 0
	
	// Читаем строку и проверяем интерполяцию
	for {
		l.readChar()
		if l.ch == 0 {
			break
		}
		// Кавычка завершает строку только вне интерполяции {}
		if l.ch == quote && braceDepth == 0 {
			break
		}
		if l.ch == '\\' && braceDepth == 0 {
			// Экранирование работает только вне {...}
			l.readChar()
			switch l.ch {
			case 'n':
				result = append(result, '\n')
			case 't':
				result = append(result, '\t')
			case 'r':
				result = append(result, '\r')
			case '"':
				result = append(result, '"')
			case '\'':
				result = append(result, '\'')
			case '\\':
				result = append(result, '\\')
			default:
				result = append(result, l.ch)
			}
		} else {
			if l.ch == '{' && quote == '"' {
				hasInterpolation = true
				braceDepth++
			} else if l.ch == '}' && quote == '"' && braceDepth > 0 {
				braceDepth--
			}
			result = append(result, l.ch)
		}
	}
	
	// Если есть интерполяция, добавляем маркер в начало
	if hasInterpolation {
		return "\x00" + string(result)
	}
	
	return string(result)
}

// readMultilineString читает многострочный литерал """..."""
func (l *Lexer) readMultilineString() string {
	var result []rune
	hasInterpolation := false
	for {
		l.readChar()
		if l.ch == 0 {
			break
		}
		if l.ch == '"' && l.peekChar() == '"' {
			l.readChar()
			if l.peekChar() == '"' {
				l.readChar()
				break
			}
			result = append(result, '"', '"')
		} else {
			if l.ch == '{' {
				hasInterpolation = true
			}
			result = append(result, l.ch)
		}
	}
	if hasInterpolation {
		return "\x00" + string(result)
	}
	return string(result)
}

// isLetter проверяет, является ли символ буквой
func isLetter(ch rune) bool {
	return unicode.IsLetter(ch) || ch == '_'
}

// isDigit проверяет, является ли символ цифрой
func isDigit(ch rune) bool {
	return unicode.IsDigit(ch)
}

// newToken создает новый токен
func (l *Lexer) newToken(tokenType TokenType, ch rune) Token {
	return Token{Type: tokenType, Literal: string(ch), Line: l.line, Column: l.column, Filename: l.filename}
}
