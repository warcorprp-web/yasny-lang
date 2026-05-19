package parser

import (
	"fmt"
	"yasny-lang/ast"
	"yasny-lang/lexer"
)

// Приоритеты операторов
const (
	_ int = iota
	LOWEST
	PIPE        // |>
	TERNARY     // ? :
	OR          // или
	AND         // и
	EQUALS      // ==
	LESSGREATER // > или <
	SUM         // +
	PRODUCT     // *
	PREFIX      // -X или не X
	CALL        // функция(X)
	INDEX       // массив[индекс]
)

var precedences = map[lexer.TokenType]int{
	lexer.ARROW:        LOWEST + 1, // Лямбды имеют низкий приоритет
	lexer.PIPE:         PIPE,
	lexer.NULLISH:      OR,
	lexer.DOTDOT:       SUM,        // Диапазоны между сложением и умножением
	lexer.QUESTION:     TERNARY,
	lexer.OR:           OR,
	lexer.AND:          AND,
	lexer.EQ:           EQUALS,
	lexer.NOT_EQ:       EQUALS,
	lexer.LT:           LESSGREATER,
	lexer.GT:           LESSGREATER,
	lexer.LTE:          LESSGREATER,
	lexer.GTE:          LESSGREATER,
	lexer.PLUS:         SUM,
	lexer.MINUS:        SUM,
	lexer.SLASH:        PRODUCT,
	lexer.INT_DIV:      PRODUCT,
	lexer.ASTERISK:     PRODUCT,
	lexer.PERCENT:      PRODUCT,
	lexer.LPAREN:       CALL,
	lexer.LBRACKET:     INDEX,
	lexer.DOT:          CALL,
	lexer.OPTIONAL_DOT: CALL,
}

type (
	prefixParseFn func() ast.Expression
	infixParseFn  func(ast.Expression) ast.Expression
)

// Parser парсит токены в AST
type Parser struct {
	l      *lexer.Lexer
	errors []string

	curToken  lexer.Token
	peekToken lexer.Token

	prefixParseFns map[lexer.TokenType]prefixParseFn
	infixParseFns  map[lexer.TokenType]infixParseFn
}

// New создает новый парсер
func New(l *lexer.Lexer) *Parser {
	p := &Parser{
		l:      l,
		errors: []string{},
	}

	p.prefixParseFns = make(map[lexer.TokenType]prefixParseFn)
	p.registerPrefix(lexer.IDENT, p.parseIdentifier)
	p.registerPrefix(lexer.THIS, p.parseThis)
	p.registerPrefix(lexer.PARENT, p.parseParent)
	p.registerPrefix(lexer.INT, p.parseIntegerLiteral)
	p.registerPrefix(lexer.FLOAT, p.parseFloatLiteral)
	p.registerPrefix(lexer.STRING, p.parseStringLiteral)
	p.registerPrefix(lexer.TRUE, p.parseBoolean)
	p.registerPrefix(lexer.FALSE, p.parseBoolean)
	p.registerPrefix(lexer.NULL_LIT, p.parseNullLiteral)
	p.registerPrefix(lexer.MINUS, p.parsePrefixExpression)
	p.registerPrefix(lexer.NOT, p.parsePrefixExpression)
	p.registerPrefix(lexer.BANG, p.parsePrefixExpression)
	p.registerPrefix(lexer.LPAREN, p.parseGroupedExpression)
	p.registerPrefix(lexer.IF, p.parseIfExpression)
	p.registerPrefix(lexer.MATCH, p.parseMatchExpression)
	p.registerPrefix(lexer.FUNCTION, p.parseFunctionLiteral)
	p.registerPrefix(lexer.LBRACKET, p.parseArrayLiteral)
	p.registerPrefix(lexer.LBRACE, p.parseHashLiteral)
	p.registerPrefix(lexer.FOR, p.parseForExpression)
	p.registerPrefix(lexer.WHILE, p.parseWhileExpression)
	p.registerPrefix(lexer.TRY, p.parseTryExpression)
	p.registerPrefix(lexer.NEW, p.parseNewExpression)
	p.registerPrefix(lexer.ASYNC, p.parseAsyncExpression)
	p.registerPrefix(lexer.AWAIT, p.parseAwaitExpression)

	p.infixParseFns = make(map[lexer.TokenType]infixParseFn)
	p.registerInfix(lexer.PLUS, p.parseInfixExpression)
	p.registerInfix(lexer.MINUS, p.parseInfixExpression)
	p.registerInfix(lexer.SLASH, p.parseInfixExpression)
	p.registerInfix(lexer.INT_DIV, p.parseInfixExpression)
	p.registerInfix(lexer.ASTERISK, p.parseInfixExpression)
	p.registerInfix(lexer.PERCENT, p.parseInfixExpression)
	p.registerInfix(lexer.EQ, p.parseInfixExpression)
	p.registerInfix(lexer.NOT_EQ, p.parseInfixExpression)
	p.registerInfix(lexer.LT, p.parseInfixExpression)
	p.registerInfix(lexer.GT, p.parseInfixExpression)
	p.registerInfix(lexer.LTE, p.parseInfixExpression)
	p.registerInfix(lexer.GTE, p.parseInfixExpression)
	p.registerInfix(lexer.AND, p.parseInfixExpression)
	p.registerInfix(lexer.OR, p.parseInfixExpression)
	p.registerInfix(lexer.LPAREN, p.parseCallExpression)
	p.registerInfix(lexer.LBRACKET, p.parseIndexExpression)
	p.registerInfix(lexer.DOT, p.parseMethodCallExpression)
	p.registerInfix(lexer.OPTIONAL_DOT, p.parseOptionalExpression)
	p.registerInfix(lexer.DOTDOT, p.parseRangeExpression)
	p.registerInfix(lexer.QUESTION, p.parseTernaryExpression)
	p.registerInfix(lexer.ARROW, p.parseLambdaExpression)
	p.registerInfix(lexer.PIPE, p.parsePipeExpression)
	p.registerInfix(lexer.NULLISH, p.parseInfixExpression)

	// Читаем два токена для инициализации
	p.nextToken()
	p.nextToken()

	return p
}

func (p *Parser) Errors() []string {
	return p.errors
}

func (p *Parser) peekError(t lexer.TokenType) {
	hint := ""
	switch t {
	case lexer.ASSIGN:
		hint = " Возможно, вы забыли знак '=' после объявления переменной?"
	case lexer.LPAREN:
		hint = " Возможно, вы забыли открывающую скобку '('?"
	case lexer.RPAREN:
		hint = " Возможно, вы забыли закрывающую скобку ')'?"
	case lexer.LBRACE:
		hint = " Возможно, вы забыли открывающую фигурную скобку '{'?"
	case lexer.RBRACE:
		hint = " Возможно, вы забыли закрывающую фигурную скобку '}'?"
	case lexer.RBRACKET:
		hint = " Возможно, вы забыли закрывающую квадратную скобку ']'?"
	case lexer.SEMICOLON:
		hint = " Возможно, нужна точка с запятой?"
	case lexer.COMMA:
		hint = " Возможно, нужна запятая между элементами?"
	}
	
	msg := fmt.Sprintf("❌ Ошибка парсинга в строке %d: ожидался %s, получен %s%s", 
		p.peekToken.Line, translateTokenType(t), translateTokenType(p.peekToken.Type), hint)
	p.errors = append(p.errors, msg)
}

func translateTokenType(t lexer.TokenType) string {
	translations := map[lexer.TokenType]string{
		lexer.ASSIGN:    "'=' (присваивание)",
		lexer.LPAREN:    "'(' (открывающая скобка)",
		lexer.RPAREN:    "')' (закрывающая скобка)",
		lexer.LBRACE:    "'{' (открывающая фигурная скобка)",
		lexer.RBRACE:    "'}' (закрывающая фигурная скобка)",
		lexer.LBRACKET:  "'[' (открывающая квадратная скобка)",
		lexer.RBRACKET:  "']' (закрывающая квадратная скобка)",
		lexer.COMMA:     "',' (запятая)",
		lexer.SEMICOLON: "';' (точка с запятой)",
		lexer.COLON:     "':' (двоеточие)",
		lexer.IDENT:     "идентификатор (имя переменной/функции)",
		lexer.INT:       "целое число",
		lexer.FLOAT:     "дробное число",
		lexer.STRING:    "строка",
	}
	
	if trans, ok := translations[t]; ok {
		return trans
	}
	return string(t)
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) curTokenIs(t lexer.TokenType) bool {
	return p.curToken.Type == t
}

func (p *Parser) peekTokenIs(t lexer.TokenType) bool {
	return p.peekToken.Type == t
}

func (p *Parser) expectPeek(t lexer.TokenType) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	}
	p.peekError(t)
	return false
}

func (p *Parser) peekPrecedence() int {
	if p, ok := precedences[p.peekToken.Type]; ok {
		return p
	}
	return LOWEST
}

func (p *Parser) curPrecedence() int {
	if p, ok := precedences[p.curToken.Type]; ok {
		return p
	}
	return LOWEST
}

func (p *Parser) registerPrefix(tokenType lexer.TokenType, fn prefixParseFn) {
	p.prefixParseFns[tokenType] = fn
}

func (p *Parser) registerInfix(tokenType lexer.TokenType, fn infixParseFn) {
	p.infixParseFns[tokenType] = fn
}

// ParseProgram парсит всю программу
func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{}
	program.Statements = []ast.Statement{}

	for !p.curTokenIs(lexer.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}
		p.nextToken()
	}

	return program
}

func (p *Parser) parseStatement() ast.Statement {
	switch p.curToken.Type {
	case lexer.LET:
		return p.parseLetStatement()
	case lexer.VAR:
		return p.parseVarStatement()
	case lexer.RETURN:
		return p.parseReturnStatement()
	case lexer.YIELD:
		return p.parseYieldStatement()
	case lexer.THROW:
		return p.parseThrowStatement()
	case lexer.BREAK:
		return &ast.BreakStatement{Token: p.curToken}
	case lexer.CONTINUE:
		return &ast.ContinueStatement{Token: p.curToken}
	case lexer.CLASS:
		return p.parseClassStatement()
	case lexer.ENUM:
		return p.parseEnumStatement()
	case lexer.TEST:
		return p.parseTestStatement()
	case lexer.ASSERT:
		return p.parseAssertStatement()
	case lexer.AT:
		return p.parseDecoratedFunction()
	case lexer.IMPORT:
		return p.parseImportStatement()
	case lexer.EXPORT:
		return p.parseExportStatement()
	default:
		// Парсим выражение, может быть присваивание
		return p.parseExpressionOrAssignment()
	}
}

func (p *Parser) parseExpression(precedence int) ast.Expression {
	prefix := p.prefixParseFns[p.curToken.Type]
	if prefix == nil {
		p.noPrefixParseFnError(p.curToken.Type)
		return nil
	}
	leftExp := prefix()

	for !p.peekTokenIs(lexer.EOF) && precedence < p.peekPrecedence() {
		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil {
			return leftExp
		}

		p.nextToken()

		leftExp = infix(leftExp)
	}

	return leftExp
}

func (p *Parser) noPrefixParseFnError(t lexer.TokenType) {
	hint := ""
	switch t {
	case lexer.RPAREN, lexer.RBRACE, lexer.RBRACKET:
		hint = " Возможно, лишняя закрывающая скобка или неправильная вложенность?"
	case lexer.COMMA:
		hint = " Запятая не может начинать выражение."
	case lexer.SEMICOLON:
		hint = " Точка с запятой не может начинать выражение."
	default:
		hint = " Проверьте синтаксис выражения."
	}
	
	msg := fmt.Sprintf("❌ Ошибка парсинга в строке %d: неожиданный токен %s.%s", 
		p.curToken.Line, translateTokenType(t), hint)
	p.errors = append(p.errors, msg)
}

func (p *Parser) parseBlockStatement() *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.curToken}
	block.Statements = []ast.Statement{}

	p.nextToken()

	for !p.curTokenIs(lexer.END) && !p.curTokenIs(lexer.ELSE) && !p.curTokenIs(lexer.ELSE_IF) && !p.curTokenIs(lexer.CATCH) && !p.curTokenIs(lexer.FINALLY) && !p.curTokenIs(lexer.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken()
	}

	return block
}

// parseInlineOrBlock — если после условия идёт ':', парсит одно
// выражение (короткая форма "если cond: выражение"), иначе парсит
// полный блок до 'конец'. Возвращает блок и флаг — была ли inline-форма.
func (p *Parser) parseInlineOrBlock() (*ast.BlockStatement, bool) {
	return p.parseInlineOrBlockMode(false)
}

// parseInlineOrBlockImplicitReturn — как parseInlineOrBlock, но для
// inline-формы последнее выражение оборачивается в ReturnStatement
// (для функций).
func (p *Parser) parseInlineOrBlockImplicitReturn() (*ast.BlockStatement, bool) {
	return p.parseInlineOrBlockMode(true)
}

func (p *Parser) parseInlineOrBlockMode(implicitReturn bool) (*ast.BlockStatement, bool) {
	if p.peekTokenIs(lexer.COLON) {
		p.nextToken() // curToken = ':'
		p.nextToken() // curToken = первый токен statement
		block := &ast.BlockStatement{Token: p.curToken}
		block.Statements = []ast.Statement{}
		stmt := p.parseStatement()
		if stmt != nil {
			if implicitReturn {
				if expStmt, ok := stmt.(*ast.ExpressionStatement); ok {
					stmt = &ast.ReturnStatement{
						Token:       expStmt.Token,
						ReturnValue: expStmt.Expression,
					}
				}
			}
			block.Statements = append(block.Statements, stmt)
		}
		if p.peekTokenIs(lexer.ELSE) || p.peekTokenIs(lexer.ELSE_IF) {
			p.nextToken()
		}
		return block, true
	}
	return p.parseBlockStatement(), false
}

