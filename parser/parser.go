package parser

import (
	"fmt"
	"yasny-lang/ast"
	"yasny-lang/lexer"
	"strconv"
)

// Приоритеты операторов
const (
	_ int = iota
	LOWEST
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

	p.infixParseFns = make(map[lexer.TokenType]infixParseFn)
	p.registerInfix(lexer.PLUS, p.parseInfixExpression)
	p.registerInfix(lexer.MINUS, p.parseInfixExpression)
	p.registerInfix(lexer.SLASH, p.parseInfixExpression)
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
	case lexer.THROW:
		return p.parseThrowStatement()
	case lexer.BREAK:
		return &ast.BreakStatement{Token: p.curToken}
	case lexer.CONTINUE:
		return &ast.ContinueStatement{Token: p.curToken}
	case lexer.CLASS:
		return p.parseClassStatement()
	case lexer.IMPORT:
		return p.parseImportStatement()
	case lexer.EXPORT:
		return p.parseExportStatement()
	default:
		// Парсим выражение, может быть присваивание
		return p.parseExpressionOrAssignment()
	}
}

func (p *Parser) parseExpressionOrAssignment() ast.Statement {
	expr := p.parseExpression(LOWEST)
	
	if expr == nil {
		return nil
	}
	
	// Если после выражения идет =, +=, -=, *=, /= это присваивание
	var operator string
	if p.peekTokenIs(lexer.ASSIGN) {
		operator = "="
		p.nextToken()
	} else if p.peekTokenIs(lexer.PLUS_ASSIGN) {
		operator = "+="
		p.nextToken()
	} else if p.peekTokenIs(lexer.MINUS_ASSIGN) {
		operator = "-="
		p.nextToken()
	} else if p.peekTokenIs(lexer.ASTERISK_ASSIGN) {
		operator = "*="
		p.nextToken()
	} else if p.peekTokenIs(lexer.SLASH_ASSIGN) {
		operator = "/="
		p.nextToken()
	}
	
	if operator != "" {
		p.nextToken()
		value := p.parseExpression(LOWEST)
		
		return &ast.AssignmentStatement{
			Token:    expr.GetToken(),
			Left:     expr,
			Operator: operator,
			Value:    value,
		}
	}
	
	// Обычное выражение
	return &ast.ExpressionStatement{
		Token:      expr.GetToken(),
		Expression: expr,
	}
}

func (p *Parser) parseLetStatement() ast.Statement {
	stmt := &ast.LetStatement{Token: p.curToken}

	// Проверяем на деструктуризацию: конст [a, b] = ... или конст {x, y} = ...
	if p.peekTokenIs(lexer.LBRACKET) || p.peekTokenIs(lexer.LBRACE) {
		p.nextToken()
		pattern := p.parseExpression(LOWEST)
		
		if !p.expectPeek(lexer.ASSIGN) {
			return nil
		}
		
		p.nextToken()
		value := p.parseExpression(LOWEST)
		
		return &ast.DestructuringStatement{
			Token:   stmt.Token,
			Pattern: pattern,
			Value:   value,
		}
	}

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}

	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(lexer.ASSIGN) {
		return nil
	}

	p.nextToken()

	stmt.Value = p.parseExpression(LOWEST)

	return stmt
}

func (p *Parser) parseClassStatement() ast.Statement {
	className := p.curToken
	
	if !p.expectPeek(lexer.IDENT) {
		return nil
	}
	
	name := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	
	// Проверяем наследование
	var parentName *ast.Identifier
	if p.peekTokenIs(lexer.EXTENDS) {
		p.nextToken() // съедаем EXTENDS
		if !p.expectPeek(lexer.IDENT) {
			return nil
		}
		parentName = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	}
	
	methods := make(map[string]*ast.FunctionLiteral)
	
	p.nextToken()
	
	for !p.curTokenIs(lexer.END) && !p.curTokenIs(lexer.EOF) {
		if p.curTokenIs(lexer.FUNCTION) {
			p.nextToken()
			if !p.curTokenIs(lexer.IDENT) {
				p.nextToken()
				continue
			}
			methodName := p.curToken.Literal
			
			if !p.expectPeek(lexer.LPAREN) {
				continue
			}
			
			lit := &ast.FunctionLiteral{Token: className}
			lit.Parameters = p.parseFunctionParameters()
			
			if !p.expectPeek(lexer.RPAREN) {
				return nil
			}
			
			lit.Body = p.parseInlineOrBlock()
			methods[methodName] = lit
		}
		p.nextToken()
	}
	
	pairs := []ast.Expression{}
	for methodName, fn := range methods {
		pairs = append(pairs, &ast.StringLiteral{Value: methodName})
		pairs = append(pairs, fn)
	}
	
	// Добавляем родителя если есть
	if parentName != nil {
		pairs = append(pairs, &ast.StringLiteral{Value: "__parent_name__"})
		pairs = append(pairs, &ast.StringLiteral{Value: parentName.Value})
	}
	
	hashLit := &ast.HashLiteral{Pairs: make(map[ast.Expression]ast.Expression)}
	for i := 0; i < len(pairs); i += 2 {
		hashLit.Pairs[pairs[i]] = pairs[i+1]
	}
	
	return &ast.LetStatement{
		Token: className,
		Name:  name,
		Value: hashLit,
	}
}

func (p *Parser) parseVarStatement() *ast.VarStatement {
	stmt := &ast.VarStatement{Token: p.curToken}

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}

	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(lexer.ASSIGN) {
		return nil
	}

	p.nextToken()

	stmt.Value = p.parseExpression(LOWEST)

	return stmt
}

func (p *Parser) parseAssignmentStatement() *ast.AssignmentStatement {
	stmt := &ast.AssignmentStatement{Token: p.curToken}
	stmt.Left = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	// Проверяем какой оператор присваивания
	if p.peekTokenIs(lexer.ASSIGN) {
		p.nextToken()
		stmt.Operator = "="
	} else if p.peekTokenIs(lexer.PLUS_ASSIGN) {
		p.nextToken()
		stmt.Operator = "+="
	} else if p.peekTokenIs(lexer.MINUS_ASSIGN) {
		p.nextToken()
		stmt.Operator = "-="
	} else if p.peekTokenIs(lexer.ASTERISK_ASSIGN) {
		p.nextToken()
		stmt.Operator = "*="
	} else if p.peekTokenIs(lexer.SLASH_ASSIGN) {
		p.nextToken()
		stmt.Operator = "/="
	} else {
		return nil
	}

	p.nextToken()

	stmt.Value = p.parseExpression(LOWEST)

	return stmt
}

func (p *Parser) parseReturnStatement() *ast.ReturnStatement {
	stmt := &ast.ReturnStatement{Token: p.curToken}

	p.nextToken()

	stmt.ReturnValue = p.parseExpression(LOWEST)

	return stmt
}

func (p *Parser) parseThrowStatement() *ast.ThrowStatement {
	stmt := &ast.ThrowStatement{Token: p.curToken}

	p.nextToken()

	// Если нет значения (re-throw), Value будет nil
	if !p.curTokenIs(lexer.END) && !p.curTokenIs(lexer.EOF) {
		stmt.Value = p.parseExpression(LOWEST)
	}

	return stmt
}

func (p *Parser) parseExpressionStatement() *ast.ExpressionStatement {
	stmt := &ast.ExpressionStatement{Token: p.curToken}
	stmt.Expression = p.parseExpression(LOWEST)
	return stmt
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

func (p *Parser) parseIdentifier() ast.Expression {
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseThis() ast.Expression {
	return &ast.Identifier{Token: p.curToken, Value: "это"}
}

func (p *Parser) parseParent() ast.Expression {
	return &ast.Identifier{Token: p.curToken, Value: "родитель"}
}

func (p *Parser) parseIntegerLiteral() ast.Expression {
	lit := &ast.IntegerLiteral{Token: p.curToken}

	value, err := strconv.ParseInt(p.curToken.Literal, 0, 64)
	if err != nil {
		msg := fmt.Sprintf("❌ Ошибка в строке %d: не удалось распарсить '%s' как целое число. Проверьте формат числа.", 
			p.curToken.Line, p.curToken.Literal)
		p.errors = append(p.errors, msg)
		return nil
	}

	lit.Value = value
	return lit
}

func (p *Parser) parseFloatLiteral() ast.Expression {
	lit := &ast.FloatLiteral{Token: p.curToken}

	value, err := strconv.ParseFloat(p.curToken.Literal, 64)
	if err != nil {
		msg := fmt.Sprintf("❌ Ошибка в строке %d: не удалось распарсить '%s' как дробное число. Проверьте формат числа.", 
			p.curToken.Line, p.curToken.Literal)
		p.errors = append(p.errors, msg)
		return nil
	}

	lit.Value = value
	return lit
}

func (p *Parser) parseStringLiteral() ast.Expression {
	return &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseBoolean() ast.Expression {
	return &ast.Boolean{Token: p.curToken, Value: p.curTokenIs(lexer.TRUE)}
}

func (p *Parser) parsePrefixExpression() ast.Expression {
	expression := &ast.PrefixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
	}

	p.nextToken()

	expression.Right = p.parseExpression(PREFIX)

	return expression
}

func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	expression := &ast.InfixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
		Left:     left,
	}

	precedence := p.curPrecedence()
	p.nextToken()
	expression.Right = p.parseExpression(precedence)

	return expression
}

func (p *Parser) parseGroupedExpression() ast.Expression {
	p.nextToken()

	exp := p.parseExpression(LOWEST)

	if !p.expectPeek(lexer.RPAREN) {
		return nil
	}

	return exp
}

func (p *Parser) parseIfExpression() ast.Expression {
	expression := &ast.IfExpression{Token: p.curToken}

	p.nextToken()
	expression.Condition = p.parseExpression(LOWEST)

	expression.Consequence = p.parseInlineOrBlock()

	if p.curTokenIs(lexer.ELSE_IF) {
		// иначеесли - создаем вложенный if как alternative
		elseIfExpr := &ast.IfExpression{Token: p.curToken}
		
		p.nextToken()
		elseIfExpr.Condition = p.parseExpression(LOWEST)
		elseIfExpr.Consequence = p.parseInlineOrBlock()
		
		// Рекурсивно обрабатываем следующие иначеесли/иначе
		if p.curTokenIs(lexer.ELSE_IF) || p.curTokenIs(lexer.ELSE) {
			if p.curTokenIs(lexer.ELSE_IF) {
				elseIfExpr.Alternative = &ast.BlockStatement{
					Statements: []ast.Statement{
						&ast.ExpressionStatement{
							Expression: p.parseIfExpression(),
						},
					},
				}
			} else {
				elseIfExpr.Alternative = p.parseInlineOrBlock()
			}
		}
		
		expression.Alternative = &ast.BlockStatement{
			Statements: []ast.Statement{
				&ast.ExpressionStatement{
					Expression: elseIfExpr,
				},
			},
		}
	} else if p.curTokenIs(lexer.ELSE) {
		expression.Alternative = p.parseInlineOrBlock()
	}

	return expression
}

func (p *Parser) parseMatchExpression() ast.Expression {
	expr := &ast.MatchExpression{Token: p.curToken}
	
	p.nextToken()
	expr.Value = p.parseExpression(LOWEST)
	
	expr.Cases = []*ast.MatchCase{}
	
	// Парсим случаи: когда паттерн: результат
	for !p.curTokenIs(lexer.END) && !p.curTokenIs(lexer.EOF) {
		if p.curTokenIs(lexer.WHEN) {
			matchCase := &ast.MatchCase{Token: p.curToken}
			
			p.nextToken()
			matchCase.Pattern = p.parseExpression(LOWEST)
			
			if !p.expectPeek(lexer.COLON) {
				return nil
			}
			
			p.nextToken()
			matchCase.Result = p.parseExpression(LOWEST)
			
			expr.Cases = append(expr.Cases, matchCase)
			p.nextToken()
		} else if p.curTokenIs(lexer.ELSE) {
			matchCase := &ast.MatchCase{Token: p.curToken}
			matchCase.Pattern = nil // nil означает default
			
			if !p.expectPeek(lexer.COLON) {
				return nil
			}
			
			p.nextToken()
			matchCase.Result = p.parseExpression(LOWEST)
			
			expr.Cases = append(expr.Cases, matchCase)
			p.nextToken()
		} else {
			p.nextToken()
		}
	}
	
	return expr
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

// parseInlineOrBlock - если после условия идёт ':', парсит одно выражение
// (короткая форма "если cond: выражение"), иначе парсит полный блок до КОНЕЦ.
// Используется для если/для/пока/функция чтобы поддерживать обе формы.
func (p *Parser) parseInlineOrBlock() *ast.BlockStatement {
	if p.peekTokenIs(lexer.COLON) {
		p.nextToken() // curToken = ':'
		p.nextToken() // curToken = первый токен statement
		block := &ast.BlockStatement{Token: p.curToken}
		block.Statements = []ast.Statement{}
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		// Если дальше идёт ИНАЧЕ/ИНАЧЕЕСЛИ - продвинемся,
		// чтобы родительский парсер (parseIfExpression) увидел их
		if p.peekTokenIs(lexer.ELSE) || p.peekTokenIs(lexer.ELSE_IF) {
			p.nextToken()
		}
		return block
	}
	return p.parseBlockStatement()
}

func (p *Parser) parseFunctionLiteral() ast.Expression {
	lit := &ast.FunctionLiteral{Token: p.curToken}

	// Имя функции (опционально)
	if p.peekTokenIs(lexer.IDENT) {
		p.nextToken()
		lit.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	}

	if !p.expectPeek(lexer.LPAREN) {
		return nil
	}

	lit.Parameters = p.parseFunctionParameters()

	if !p.expectPeek(lexer.RPAREN) {
		return nil
	}

	lit.Body = p.parseInlineOrBlock()

	return lit
}

func (p *Parser) parseFunctionParameters() []*ast.Identifier {
	identifiers := []*ast.Identifier{}

	if p.peekTokenIs(lexer.RPAREN) {
		return identifiers
	}

	p.nextToken()

	ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	identifiers = append(identifiers, ident)

	for p.peekTokenIs(lexer.COMMA) {
		p.nextToken()
		p.nextToken()
		ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		identifiers = append(identifiers, ident)
	}

	return identifiers
}

func (p *Parser) parseCallExpression(function ast.Expression) ast.Expression {
	exp := &ast.CallExpression{Token: p.curToken, Function: function}
	exp.Arguments = p.parseExpressionList(lexer.RPAREN)
	return exp
}

func (p *Parser) parseExpressionList(end lexer.TokenType) []ast.Expression {
	list := []ast.Expression{}

	if p.peekTokenIs(end) {
		p.nextToken()
		return list
	}

	p.nextToken()
	
	// Проверяем на spread
	if p.curTokenIs(lexer.SPREAD) {
		spreadToken := p.curToken
		p.nextToken()
		list = append(list, &ast.SpreadExpression{
			Token: spreadToken,
			Value: p.parseExpression(LOWEST),
		})
	} else {
		list = append(list, p.parseExpression(LOWEST))
	}

	for p.peekTokenIs(lexer.COMMA) {
		p.nextToken()
		p.nextToken()
		
		// Проверяем на spread
		if p.curTokenIs(lexer.SPREAD) {
			spreadToken := p.curToken
			p.nextToken()
			list = append(list, &ast.SpreadExpression{
				Token: spreadToken,
				Value: p.parseExpression(LOWEST),
			})
		} else {
			list = append(list, p.parseExpression(LOWEST))
		}
	}

	if !p.expectPeek(end) {
		return nil
	}

	return list
}

func (p *Parser) parseArrayLiteral() ast.Expression {
	array := &ast.ArrayLiteral{Token: p.curToken}
	
	// Проверяем на comprehension: [выражение для переменная в итератор]
	if !p.peekTokenIs(lexer.RBRACKET) {
		p.nextToken()
		
		// Проверяем на spread в начале
		var firstExpr ast.Expression
		if p.curTokenIs(lexer.SPREAD) {
			spreadToken := p.curToken
			p.nextToken()
			firstExpr = &ast.SpreadExpression{
				Token: spreadToken,
				Value: p.parseExpression(LOWEST),
			}
		} else {
			firstExpr = p.parseExpression(LOWEST)
		}
		
		// Если после выражения идет "для" - это comprehension
		if p.peekTokenIs(lexer.FOR) {
			// Spread не может быть в comprehension
			if _, ok := firstExpr.(*ast.SpreadExpression); ok {
				msg := fmt.Sprintf("❌ Ошибка в строке %d: spread оператор (...) нельзя использовать в list comprehension. Используйте обычное выражение.", 
					p.curToken.Line)
				p.errors = append(p.errors, msg)
				return nil
			}
			
			p.nextToken() // пропускаем "для"
			
			if !p.expectPeek(lexer.IDENT) {
				return nil
			}
			
			variable := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
			
			if !p.expectPeek(lexer.IN) {
				return nil
			}
			
			p.nextToken()
			iterable := p.parseExpression(LOWEST)
			
			comp := &ast.ArrayComprehension{
				Token:    array.Token,
				Element:  firstExpr,
				Variable: variable,
				Iterable: iterable,
			}
			
			// Проверяем опциональное "если"
			if p.peekTokenIs(lexer.IF) {
				p.nextToken() // пропускаем "если"
				p.nextToken()
				comp.Condition = p.parseExpression(LOWEST)
			}
			
			if !p.expectPeek(lexer.RBRACKET) {
				return nil
			}
			
			return comp
		}
		
		// Обычный массив - собираем остальные элементы
		elements := []ast.Expression{firstExpr}
		
		for p.peekTokenIs(lexer.COMMA) {
			p.nextToken() // пропускаем запятую
			
			// Проверяем, не закрывающая ли скобка (trailing comma)
			if p.peekTokenIs(lexer.RBRACKET) {
				break
			}
			
			// Проверяем на spread оператор
			if p.peekTokenIs(lexer.SPREAD) {
				p.nextToken() // переходим на ...
				spreadToken := p.curToken
				p.nextToken() // переходим на выражение
				spreadExpr := &ast.SpreadExpression{
					Token: spreadToken,
					Value: p.parseExpression(LOWEST),
				}
				elements = append(elements, spreadExpr)
			} else {
				p.nextToken()
				elements = append(elements, p.parseExpression(LOWEST))
			}
		}
		
		if !p.expectPeek(lexer.RBRACKET) {
			return nil
		}
		
		array.Elements = elements
		return array
	}
	
	// Пустой массив
	p.nextToken()
	return array
}

func (p *Parser) parseIndexExpression(left ast.Expression) ast.Expression {
	exp := &ast.IndexExpression{Token: p.curToken, Left: left}

	p.nextToken()
	exp.Index = p.parseExpression(LOWEST)

	if !p.expectPeek(lexer.RBRACKET) {
		return nil
	}

	return exp
}

func (p *Parser) parseForExpression() ast.Expression {
	expression := &ast.ForExpression{Token: p.curToken}

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}

	variable := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	// Проверяем: "для i, элемент в массив" (с индексом)
	if p.peekTokenIs(lexer.COMMA) {
		p.nextToken() // пропускаем запятую
		
		if !p.expectPeek(lexer.IDENT) {
			return nil
		}
		
		valueVar := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		
		if !p.expectPeek(lexer.IN) {
			return nil
		}
		
		p.nextToken()
		
		forIn := &ast.ForInExpression{
			Token:    expression.Token,
			Index:    variable,      // первая переменная - индекс
			Variable: valueVar,      // вторая - значение
			Iterable: p.parseExpression(LOWEST),
		}
		forIn.Body = p.parseInlineOrBlock()
		return forIn
	}

	// Проверяем: "для элемент в массив" (без индекса)
	if p.peekTokenIs(lexer.IN) {
		p.nextToken() // пропускаем "в"
		p.nextToken()
		
		forIn := &ast.ForInExpression{
			Token:    expression.Token,
			Variable: variable,
			Iterable: p.parseExpression(LOWEST),
		}
		forIn.Body = p.parseInlineOrBlock()
		return forIn
	}

	// для i от ... до ...
	expression.Variable = variable

	if !p.expectPeek(lexer.FROM) {
		return nil
	}

	p.nextToken()
	expression.From = p.parseExpression(LOWEST)

	if !p.expectPeek(lexer.TO) {
		return nil
	}

	p.nextToken()
	expression.To = p.parseExpression(LOWEST)

	expression.Body = p.parseInlineOrBlock()

	return expression
}

func (p *Parser) parseWhileExpression() ast.Expression {
	expression := &ast.WhileExpression{Token: p.curToken}

	p.nextToken()
	expression.Condition = p.parseExpression(LOWEST)

	expression.Body = p.parseInlineOrBlock()

	return expression
}

func (p *Parser) parseHashLiteral() ast.Expression {
	hash := &ast.HashLiteral{Token: p.curToken}
	hash.Pairs = make(map[ast.Expression]ast.Expression)

	for !p.peekTokenIs(lexer.RBRACE) && !p.peekTokenIs(lexer.EOF) {
		p.nextToken()
		
		// Пропускаем запятые в начале
		if p.curTokenIs(lexer.COMMA) {
			continue
		}
		
		key := p.parseExpression(LOWEST)

		if !p.expectPeek(lexer.COLON) {
			return nil
		}

		p.nextToken()
		value := p.parseExpression(LOWEST)

		hash.Pairs[key] = value

		if !p.peekTokenIs(lexer.RBRACE) && !p.expectPeek(lexer.COMMA) {
			return nil
		}
	}

	if !p.expectPeek(lexer.RBRACE) {
		return nil
	}

	return hash
}

func (p *Parser) parseTryExpression() ast.Expression {
	expression := &ast.TryExpression{Token: p.curToken}

	// Парсим тело попытки (до CATCH, FINALLY или END)
	expression.Body = p.parseTryBlockStatement()

	// Если есть CATCH
	if p.curTokenIs(lexer.CATCH) {
		catchToken := p.curToken
		
		// Проверяем переменную для ошибки
		// Переменная должна быть на той же строке что и CATCH
		if p.peekTokenIs(lexer.IDENT) && p.peekToken.Line == catchToken.Line {
			p.nextToken()
			expression.CatchVar = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		}
		
		// Парсим тело catch
		expression.CatchBody = p.parseCatchBlockStatement()
	}

	// Если есть FINALLY
	if p.curTokenIs(lexer.FINALLY) {
		expression.FinallyBody = p.parseBlockStatement()
	}

	return expression
}

func (p *Parser) parseTryBlockStatement() *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.curToken}
	block.Statements = []ast.Statement{}

	p.nextToken()

	// Останавливаемся на CATCH, FINALLY или END
	for !p.curTokenIs(lexer.CATCH) && !p.curTokenIs(lexer.FINALLY) && !p.curTokenIs(lexer.END) && !p.curTokenIs(lexer.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken()
	}

	return block
}

func (p *Parser) parseCatchBlockStatement() *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.curToken}
	block.Statements = []ast.Statement{}

	p.nextToken()

	// Останавливаемся на FINALLY или END
	for !p.curTokenIs(lexer.FINALLY) && !p.curTokenIs(lexer.END) && !p.curTokenIs(lexer.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken()
	}

	return block
}

func (p *Parser) parseNewExpression() ast.Expression {
	token := p.curToken
	p.nextToken()
	
	if !p.curTokenIs(lexer.IDENT) {
		return nil
	}
	
	className := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	
	if !p.expectPeek(lexer.LPAREN) {
		return nil
	}
	
	args := p.parseExpressionList(lexer.RPAREN)
	
	return &ast.NewExpression{
		Token:     token,
		ClassName: className,
		Arguments: args,
	}
}

// parseImportStatement парсит импорт модуль из "файл.pr"
func (p *Parser) parseImportStatement() *ast.ImportStatement {
	stmt := &ast.ImportStatement{Token: p.curToken}
	
	if !p.expectPeek(lexer.IDENT) {
		return nil
	}
	
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	
	if !p.expectPeek(lexer.FROM_KW) {
		return nil
	}
	
	if !p.expectPeek(lexer.STRING) {
		return nil
	}
	
	stmt.Path = p.curToken.Literal
	
	// Опционально: как алиас
	if p.peekTokenIs(lexer.AS) {
		p.nextToken()
		if !p.expectPeek(lexer.IDENT) {
			return nil
		}
		stmt.Alias = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	}
	
	return stmt
}

// parseExportStatement парсит экспорт функция/переменная
func (p *Parser) parseExportStatement() *ast.ExportStatement {
	stmt := &ast.ExportStatement{Token: p.curToken}
	
	p.nextToken()
	stmt.Statement = p.parseStatement()
	
	if stmt.Statement == nil {
		return nil
	}
	
	return stmt
}

func (p *Parser) parseMethodCallExpression(left ast.Expression) ast.Expression {
	p.nextToken()
	
	if !p.curTokenIs(lexer.IDENT) {
		return nil
	}
	
	methodName := p.curToken.Literal
	
	// Если после идентификатора идет (, это вызов метода
	if p.peekTokenIs(lexer.LPAREN) {
		p.nextToken() // пропускаем (
		
		args := p.parseExpressionList(lexer.RPAREN)
		
		indexExpr := &ast.IndexExpression{
			Left:  left,
			Index: &ast.StringLiteral{Value: methodName},
		}
		
		return &ast.CallExpression{
			Function:  indexExpr,
			Arguments: args,
		}
	}
	
	// Иначе это просто доступ к полю: obj.field
	return &ast.IndexExpression{
		Left:  left,
		Index: &ast.StringLiteral{Value: methodName},
	}
}

func (p *Parser) parseOptionalExpression(left ast.Expression) ast.Expression {
	token := p.curToken
	p.nextToken()
	
	if !p.curTokenIs(lexer.IDENT) {
		return nil
	}
	
	right := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	
	// Если после идентификатора идет (, это вызов метода
	if p.peekTokenIs(lexer.LPAREN) {
		p.nextToken() // пропускаем (
		args := p.parseExpressionList(lexer.RPAREN)
		
		return &ast.OptionalExpression{
			Token: token,
			Left:  left,
			Right: &ast.CallExpression{
				Function:  right,
				Arguments: args,
			},
		}
	}
	
	// Иначе это просто доступ к полю: obj?.field
	return &ast.OptionalExpression{
		Token: token,
		Left:  left,
		Right: right,
	}
}

func (p *Parser) parseTernaryExpression(condition ast.Expression) ast.Expression {
	exp := &ast.TernaryExpression{
		Token:     p.curToken,
		Condition: condition,
	}
	
	p.nextToken()
	exp.Consequence = p.parseExpression(LOWEST)
	
	if !p.expectPeek(lexer.COLON) {
		return nil
	}
	
	p.nextToken()
	exp.Alternative = p.parseExpression(LOWEST)
	
	return exp
}


func (p *Parser) parseLambdaExpression(left ast.Expression) ast.Expression {
	lambda := &ast.FunctionLiteral{Token: p.curToken}
	
	// Параметры из left
	// Может быть: х => ... или (а, б) => ...
	switch node := left.(type) {
	case *ast.Identifier:
		// Один параметр: х => ...
		lambda.Parameters = []*ast.Identifier{node}
	case *ast.CallExpression:
		// Несколько параметров в скобках: (а, б) => ...
		// Парсер видит это как вызов функции, извлекаем аргументы
		if ident, ok := node.Function.(*ast.Identifier); ok {
			lambda.Parameters = []*ast.Identifier{ident}
			for _, arg := range node.Arguments {
				if id, ok := arg.(*ast.Identifier); ok {
					lambda.Parameters = append(lambda.Parameters, id)
				}
			}
		}
	default:
		msg := fmt.Sprintf("❌ Ошибка в строке %d: неверный синтаксис лямбда-функции. Используйте: x => выражение или (x, y) => выражение", 
			p.curToken.Line)
		p.errors = append(p.errors, msg)
		return nil
	}
	
	p.nextToken()
	
	// Тело - одно выражение, оборачиваем в return
	expr := p.parseExpression(LOWEST)
	lambda.Body = &ast.BlockStatement{
		Statements: []ast.Statement{
			&ast.ReturnStatement{
				Token:       p.curToken,
				ReturnValue: expr,
			},
		},
	}
	
	return lambda
}

func (p *Parser) parseRangeExpression(left ast.Expression) ast.Expression {
	exp := &ast.RangeExpression{
		Token: p.curToken,
		Start: left,
	}
	
	precedence := p.curPrecedence()
	p.nextToken()
	exp.End = p.parseExpression(precedence)
	
	return exp
}
