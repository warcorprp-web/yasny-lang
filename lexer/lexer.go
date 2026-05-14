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
	
	for l.ch == '#' || (l.ch == '/' && l.peekChar() == '/') {
		for l.ch != '\n' && l.ch != 0 {
			l.readChar()
		}
		l.skipWhitespace()
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

// skipWhitespace пропускает пробелы и переносы строк
func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		l.readChar()
	}
}

// skipComments пропускает комментарии
func (l *Lexer) skipComments() {
	if l.ch == '#' || (l.ch == '/' && l.peekChar() == '/') {
		for l.ch != '\n' && l.ch != 0 {
			l.readChar()
		}
		l.skipWhitespace()
	}
}

// readIdentifier читает идентификатор или ключевое слово
func (l *Lexer) readIdentifier() string {
	position := l.position
	for isLetter(l.ch) || isDigit(l.ch) {
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
	
	// Читаем строку и проверяем интерполяцию
	for {
		l.readChar()
		if l.ch == quote || l.ch == 0 {
			break
		}
		if l.ch == '\\' {
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
				// Интерполяция только в двойных кавычках
				hasInterpolation = true
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
