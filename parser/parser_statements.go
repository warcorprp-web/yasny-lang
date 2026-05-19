package parser

import (
	"yasny-lang/ast"
	"yasny-lang/lexer"
)

// parseExpressionOrAssignment парсит либо обычное выражение-statement,
// либо присваивание (включая составные: +=, -=, *=, /=).
// Левая часть присваивания может быть идентификатором или
// произвольным выражением вида obj.field или arr[i].
func (p *Parser) parseExpressionOrAssignment() ast.Statement {
	expr := p.parseExpression(LOWEST)

	if expr == nil {
		return nil
	}

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

	return &ast.ExpressionStatement{
		Token:      expr.GetToken(),
		Expression: expr,
	}
}

// parseLetStatement: конст имя = значение.
// Также поддерживает деструктуризацию ([a, b] и {x, y}) и
// множественную форму (конст a, b = ... — превращается в
// деструктуризацию по массиву).
func (p *Parser) parseLetStatement() ast.Statement {
	stmt := &ast.LetStatement{Token: p.curToken}

	// Деструктуризация: конст [a, b] = ... или конст {x, y} = ...
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

	// Множественное объявление: конст a, b = ... превращается в
	// деструктуризацию по массиву.
	if p.peekTokenIs(lexer.COMMA) {
		idents := []ast.Expression{stmt.Name}
		for p.peekTokenIs(lexer.COMMA) {
			p.nextToken()
			if !p.expectPeek(lexer.IDENT) {
				return nil
			}
			idents = append(idents, &ast.Identifier{
				Token: p.curToken,
				Value: p.curToken.Literal,
			})
		}

		if !p.expectPeek(lexer.ASSIGN) {
			return nil
		}
		p.nextToken()
		value := p.parseExpression(LOWEST)

		// Если справа тоже несколько значений через запятую —
		// собираем их в массив (тогда оба бока — массивы).
		if p.peekTokenIs(lexer.COMMA) {
			vals := []ast.Expression{value}
			for p.peekTokenIs(lexer.COMMA) {
				p.nextToken()
				p.nextToken()
				vals = append(vals, p.parseExpression(LOWEST))
			}
			value = &ast.ArrayLiteral{Token: stmt.Token, Elements: vals}
		}

		return &ast.DestructuringStatement{
			Token: stmt.Token,
			Pattern: &ast.ArrayLiteral{
				Token:    stmt.Token,
				Elements: idents,
			},
			Value: value,
		}
	}

	if !p.expectPeek(lexer.ASSIGN) {
		return nil
	}

	p.nextToken()

	stmt.Value = p.parseExpression(LOWEST)

	return stmt
}

// parseVarStatement: перем имя = значение. Аналог LetStatement, но
// для изменяемых переменных. Поддерживает множественную форму.
func (p *Parser) parseVarStatement() ast.Statement {
	stmt := &ast.VarStatement{Token: p.curToken}

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}

	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if p.peekTokenIs(lexer.COMMA) {
		idents := []ast.Expression{stmt.Name}
		for p.peekTokenIs(lexer.COMMA) {
			p.nextToken()
			if !p.expectPeek(lexer.IDENT) {
				return nil
			}
			idents = append(idents, &ast.Identifier{
				Token: p.curToken,
				Value: p.curToken.Literal,
			})
		}

		if !p.expectPeek(lexer.ASSIGN) {
			return nil
		}
		p.nextToken()
		value := p.parseExpression(LOWEST)

		if p.peekTokenIs(lexer.COMMA) {
			vals := []ast.Expression{value}
			for p.peekTokenIs(lexer.COMMA) {
				p.nextToken()
				p.nextToken()
				vals = append(vals, p.parseExpression(LOWEST))
			}
			value = &ast.ArrayLiteral{Token: stmt.Token, Elements: vals}
		}

		return &ast.DestructuringStatement{
			Token: stmt.Token,
			Pattern: &ast.ArrayLiteral{
				Token:    stmt.Token,
				Elements: idents,
			},
			Value: value,
		}
	}

	if !p.expectPeek(lexer.ASSIGN) {
		return nil
	}

	p.nextToken()

	stmt.Value = p.parseExpression(LOWEST)

	return stmt
}

// parseAssignmentStatement: имя ОПЕРАТОР значение, где ОПЕРАТОР это
// один из =, +=, -=, *=, /=. Используется только для простых
// идентификаторов; для сложных выражений (obj.field, arr[i]) работает
// parseExpressionOrAssignment.
func (p *Parser) parseAssignmentStatement() *ast.AssignmentStatement {
	stmt := &ast.AssignmentStatement{Token: p.curToken}
	stmt.Left = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

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

// parseYieldStatement: выдать значение (только внутри генераторов).
func (p *Parser) parseYieldStatement() *ast.YieldStatement {
	stmt := &ast.YieldStatement{Token: p.curToken}
	p.nextToken()
	stmt.Value = p.parseExpression(LOWEST)
	return stmt
}

// parseReturnStatement: вернуть значение [, значение ...].
// Несколько значений через запятую автоматически оборачиваются в
// массив для последующей деструктуризации на стороне вызова.
func (p *Parser) parseReturnStatement() *ast.ReturnStatement {
	stmt := &ast.ReturnStatement{Token: p.curToken}

	p.nextToken()

	stmt.ReturnValue = p.parseExpression(LOWEST)

	if p.peekTokenIs(lexer.COMMA) {
		elements := []ast.Expression{stmt.ReturnValue}
		for p.peekTokenIs(lexer.COMMA) {
			p.nextToken()
			p.nextToken()
			elements = append(elements, p.parseExpression(LOWEST))
		}
		stmt.ReturnValue = &ast.ArrayLiteral{
			Token:    stmt.Token,
			Elements: elements,
		}
	}

	return stmt
}

// parseThrowStatement: бросить [значение]. Без значения — re-throw
// текущей ошибки в catch-блоке.
func (p *Parser) parseThrowStatement() *ast.ThrowStatement {
	stmt := &ast.ThrowStatement{Token: p.curToken}

	p.nextToken()

	if !p.curTokenIs(lexer.END) && !p.curTokenIs(lexer.EOF) {
		stmt.Value = p.parseExpression(LOWEST)
	}

	return stmt
}

// parseExpressionStatement оборачивает любое выражение в statement.
func (p *Parser) parseExpressionStatement() *ast.ExpressionStatement {
	stmt := &ast.ExpressionStatement{Token: p.curToken}
	stmt.Expression = p.parseExpression(LOWEST)
	return stmt
}

// === Тесты как языковая конструкция ===

// parseTestStatement: тест "название" <блок> конец.
// Превращается в вызов внутренней функции __тест__("имя", функция-обёртка).
func (p *Parser) parseTestStatement() ast.Statement {
	tok := p.curToken

	if !p.expectPeek(lexer.STRING) {
		return nil
	}
	name := &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}

	body := p.parseBlockStatement()

	fn := &ast.FunctionLiteral{
		Token:      tok,
		Parameters: []*ast.Identifier{},
		Body:       body,
	}

	call := &ast.CallExpression{
		Token:     tok,
		Function:  &ast.Identifier{Token: tok, Value: "__тест__"},
		Arguments: []ast.Expression{name, fn},
	}

	return &ast.ExpressionStatement{Token: tok, Expression: call}
}

// parseAssertStatement: проверить условие.
// Превращается в вызов внутренней функции __проверить__(условие).
func (p *Parser) parseAssertStatement() ast.Statement {
	tok := p.curToken
	p.nextToken()
	cond := p.parseExpression(LOWEST)

	call := &ast.CallExpression{
		Token:     tok,
		Function:  &ast.Identifier{Token: tok, Value: "__проверить__"},
		Arguments: []ast.Expression{cond},
	}

	return &ast.ExpressionStatement{Token: tok, Expression: call}
}
