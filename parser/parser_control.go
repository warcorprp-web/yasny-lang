package parser

import (
	"yasny-lang/ast"
	"yasny-lang/lexer"
)

// === Условные выражения ===

// parseIfExpression: если условие <блок> [иначеесли ...] [иначе <блок>] конец.
// Цепочка иначеесли разворачивается в дерево вложенных IfExpression.
func (p *Parser) parseIfExpression() ast.Expression {
	expression := &ast.IfExpression{Token: p.curToken}

	p.nextToken()
	expression.Condition = p.parseExpression(LOWEST)

	expression.Consequence = p.parseInlineOrBlock()

	if p.curTokenIs(lexer.ELSE_IF) {
		// иначеесли — создаём вложенный if в качестве alternative.
		elseIfExpr := &ast.IfExpression{Token: p.curToken}

		p.nextToken()
		elseIfExpr.Condition = p.parseExpression(LOWEST)
		elseIfExpr.Consequence = p.parseInlineOrBlock()

		// Рекурсивно обрабатываем следующие иначеесли/иначе.
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

// parseMatchExpression: совпадение значение когда P: R ... иначе: R конец.
func (p *Parser) parseMatchExpression() ast.Expression {
	expr := &ast.MatchExpression{Token: p.curToken}

	p.nextToken()
	expr.Value = p.parseExpression(LOWEST)

	expr.Cases = []*ast.MatchCase{}

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
			matchCase.Pattern = nil // nil — это default-ветка

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

// === Циклы ===

// parseForExpression обрабатывает три формы:
//   для i от A до B [по N]   — со счётчиком
//   для x в коллекция        — итерация по коллекции
//   для i, x в коллекция     — итерация с индексом
func (p *Parser) parseForExpression() ast.Expression {
	expression := &ast.ForExpression{Token: p.curToken}

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}

	variable := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	// для i, x в коллекция
	if p.peekTokenIs(lexer.COMMA) {
		p.nextToken()

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
			Index:    variable,
			Variable: valueVar,
			Iterable: p.parseExpression(LOWEST),
		}
		forIn.Body = p.parseInlineOrBlock()
		return forIn
	}

	// для x в коллекция
	if p.peekTokenIs(lexer.IN) {
		p.nextToken()
		p.nextToken()

		forIn := &ast.ForInExpression{
			Token:    expression.Token,
			Variable: variable,
			Iterable: p.parseExpression(LOWEST),
		}
		forIn.Body = p.parseInlineOrBlock()
		return forIn
	}

	// для i от A до B [по N]
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

	if p.peekTokenIs(lexer.STEP) {
		p.nextToken() // на 'по'
		p.nextToken() // на выражение
		expression.Step = p.parseExpression(LOWEST)
	}

	expression.Body = p.parseInlineOrBlock()

	return expression
}

// parseWhileExpression: пока условие <блок> конец.
func (p *Parser) parseWhileExpression() ast.Expression {
	expression := &ast.WhileExpression{Token: p.curToken}

	p.nextToken()
	expression.Condition = p.parseExpression(LOWEST)

	expression.Body = p.parseInlineOrBlock()

	return expression
}

// === Обработка ошибок ===

// parseTryExpression: попытка <блок> [поймать [v] <блок>] [всегда <блок>] конец.
func (p *Parser) parseTryExpression() ast.Expression {
	expression := &ast.TryExpression{Token: p.curToken}

	expression.Body = p.parseTryBlockStatement()

	if p.curTokenIs(lexer.CATCH) {
		catchToken := p.curToken

		// Переменная для ошибки должна быть на той же строке, что
		// и слово 'поймать', чтобы отделить её от кода блока.
		if p.peekTokenIs(lexer.IDENT) && p.peekToken.Line == catchToken.Line {
			p.nextToken()
			expression.CatchVar = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		}

		expression.CatchBody = p.parseCatchBlockStatement()
	}

	if p.curTokenIs(lexer.FINALLY) {
		expression.FinallyBody = p.parseBlockStatement()
	}

	return expression
}

// parseTryBlockStatement читает блок до CATCH/FINALLY/END.
func (p *Parser) parseTryBlockStatement() *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.curToken}
	block.Statements = []ast.Statement{}

	p.nextToken()

	for !p.curTokenIs(lexer.CATCH) && !p.curTokenIs(lexer.FINALLY) && !p.curTokenIs(lexer.END) && !p.curTokenIs(lexer.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken()
	}

	return block
}

// parseCatchBlockStatement читает блок до FINALLY/END.
func (p *Parser) parseCatchBlockStatement() *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.curToken}
	block.Statements = []ast.Statement{}

	p.nextToken()

	for !p.curTokenIs(lexer.FINALLY) && !p.curTokenIs(lexer.END) && !p.curTokenIs(lexer.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken()
	}

	return block
}
