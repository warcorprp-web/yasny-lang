package parser

import (
	"fmt"

	"yasny-lang/ast"
	"yasny-lang/lexer"
)

// === Литералы и определения функций ===

// parseFunctionLiteral: функция [имя](параметры) <блок> конец.
// Имя необязательно; если есть, литерал может использоваться как
// именованная функция в LetStatement.
func (p *Parser) parseFunctionLiteral() ast.Expression {
	lit := &ast.FunctionLiteral{Token: p.curToken}

	if p.peekTokenIs(lexer.IDENT) {
		p.nextToken()
		lit.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	}

	if !p.expectPeek(lexer.LPAREN) {
		return nil
	}

	lit.Parameters, lit.Defaults, lit.HasRest = p.parseFunctionParametersFull()

	if !p.expectPeek(lexer.RPAREN) {
		return nil
	}

	lit.Body = p.parseInlineOrBlockImplicitReturn()

	return lit
}

// parseFunctionParametersFull парсит параметры с поддержкой
// default-значений (param = выражение) и rest-параметра (...args).
// Возвращает: имена, дефолты по индексу (nil если нет), флаг
// присутствия rest. Rest должен идти последним.
func (p *Parser) parseFunctionParametersFull() ([]*ast.Identifier, []ast.Expression, bool) {
	identifiers := []*ast.Identifier{}
	defaults := []ast.Expression{}
	hasRest := false

	if p.peekTokenIs(lexer.RPAREN) {
		return identifiers, defaults, hasRest
	}

	parseOne := func() bool {
		if p.curTokenIs(lexer.SPREAD) {
			hasRest = true
			p.nextToken()
		}
		if !p.curTokenIs(lexer.IDENT) {
			return false
		}
		ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		identifiers = append(identifiers, ident)

		if p.peekTokenIs(lexer.ASSIGN) {
			p.nextToken() // на '='
			p.nextToken() // на выражение
			defaults = append(defaults, p.parseExpression(LOWEST))
		} else {
			defaults = append(defaults, nil)
		}
		return true
	}

	p.nextToken()
	if !parseOne() {
		return identifiers, defaults, hasRest
	}

	for p.peekTokenIs(lexer.COMMA) {
		if hasRest {
			p.errors = append(p.errors, "rest-параметр (...args) должен быть последним")
			return identifiers, defaults, hasRest
		}
		p.nextToken() // на ','
		p.nextToken() // на следующий параметр
		if !parseOne() {
			break
		}
	}

	return identifiers, defaults, hasRest
}

// parseFunctionParameters — упрощённая версия, возвращает только имена.
// Используется там, где default/rest не нужны.
func (p *Parser) parseFunctionParameters() []*ast.Identifier {
	idents, _, _ := p.parseFunctionParametersFull()
	return idents
}

// === Вызовы и списки аргументов ===

// parseCallExpression: foo(arg1, arg2, ...). Левая часть — функция.
func (p *Parser) parseCallExpression(function ast.Expression) ast.Expression {
	exp := &ast.CallExpression{Token: p.curToken, Function: function}
	exp.Arguments = p.parseExpressionList(lexer.RPAREN)
	return exp
}

// parseExpressionList читает разделённый запятыми список выражений
// до указанного завершающего токена. Поддерживает spread (...x).
func (p *Parser) parseExpressionList(end lexer.TokenType) []ast.Expression {
	list := []ast.Expression{}

	if p.peekTokenIs(end) {
		p.nextToken()
		return list
	}

	p.nextToken()

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

// === Async/await ===

// parseAsyncExpression: асинх <выражение> — запускает выражение в
// горутине, возвращает Future.
func (p *Parser) parseAsyncExpression() ast.Expression {
	tok := p.curToken
	p.nextToken()
	body := p.parseExpression(PREFIX)
	return &ast.AsyncExpression{Token: tok, Body: body}
}

// parseAwaitExpression: ждать <выражение> — ждёт результат Future.
func (p *Parser) parseAwaitExpression() ast.Expression {
	tok := p.curToken
	p.nextToken()
	body := p.parseExpression(PREFIX)
	return &ast.AwaitExpression{Token: tok, Body: body}
}

// === Декораторы ===

// parseDecoratedFunction: @декоратор1 [@декоратор2 ...] функция имя().
// Применяет декораторы в обратном порядке: @a @b f становится a(b(f)).
func (p *Parser) parseDecoratedFunction() ast.Statement {
	tok := p.curToken
	decorators := []ast.Expression{}

	for p.curTokenIs(lexer.AT) {
		p.nextToken()
		dec := p.parseExpression(LOWEST)
		decorators = append(decorators, dec)
		p.nextToken()
	}

	if !p.curTokenIs(lexer.FUNCTION) {
		msg := fmt.Sprintf("после декораторов ожидается ключевое слово 'функция', получено '%s'", p.curToken.Literal)
		p.errors = append(p.errors, msg)
		return nil
	}

	fnExpr := p.parseFunctionLiteral()
	fn, ok := fnExpr.(*ast.FunctionLiteral)
	if !ok || fn.Name == nil {
		p.errors = append(p.errors, "@-декоратор требует именованной функции")
		return nil
	}

	// @a @b f → a(b(f))
	var value ast.Expression = fn
	for i := len(decorators) - 1; i >= 0; i-- {
		value = &ast.CallExpression{
			Token:     tok,
			Function:  decorators[i],
			Arguments: []ast.Expression{value},
		}
	}

	return &ast.LetStatement{
		Token: tok,
		Name:  fn.Name,
		Value: value,
	}
}
