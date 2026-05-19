package parser

import (
	"fmt"

	"yasny-lang/ast"
	"yasny-lang/lexer"
)

// === Литералы массивов ===

// parseArrayLiteral поддерживает три формы:
//   []                         — пустой массив
//   [a, b, ...c]               — обычный с поддержкой spread и trailing comma
//   [выражение для x в i если y] — list comprehension
func (p *Parser) parseArrayLiteral() ast.Expression {
	array := &ast.ArrayLiteral{Token: p.curToken}

	if !p.peekTokenIs(lexer.RBRACKET) {
		p.nextToken()

		// Spread в начале: [...arr, ...].
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

		// Comprehension: [выражение для x в коллекция если условие].
		if p.peekTokenIs(lexer.FOR) {
			if _, ok := firstExpr.(*ast.SpreadExpression); ok {
				msg := fmt.Sprintf("❌ Ошибка в строке %d: spread оператор (...) нельзя использовать в list comprehension. Используйте обычное выражение.",
					p.curToken.Line)
				p.errors = append(p.errors, msg)
				return nil
			}

			p.nextToken() // пропускаем 'для'

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

			if p.peekTokenIs(lexer.IF) {
				p.nextToken() // пропускаем 'если'
				p.nextToken()
				comp.Condition = p.parseExpression(LOWEST)
			}

			if !p.expectPeek(lexer.RBRACKET) {
				return nil
			}

			return comp
		}

		// Обычный массив: собираем остальные элементы.
		elements := []ast.Expression{firstExpr}

		for p.peekTokenIs(lexer.COMMA) {
			p.nextToken()

			// Trailing comma: за запятой сразу ].
			if p.peekTokenIs(lexer.RBRACKET) {
				break
			}

			if p.peekTokenIs(lexer.SPREAD) {
				p.nextToken()
				spreadToken := p.curToken
				p.nextToken()
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

	// Пустой массив.
	p.nextToken()
	return array
}

// === Индексирование и срезы ===

// parseIndexExpression обрабатывает четыре формы:
//   x[i]        — обычная индексация
//   x[start:end]— срез
//   x[start:]   — срез от start до конца
//   x[:end]     — срез от начала до end
//   x[:]        — копия (открытый срез)
func (p *Parser) parseIndexExpression(left ast.Expression) ast.Expression {
	tok := p.curToken

	p.nextToken()

	// Срез [:...]: пустое начало.
	if p.curTokenIs(lexer.COLON) {
		slice := &ast.SliceExpression{Token: tok, Left: left, Start: nil}
		if p.peekTokenIs(lexer.RBRACKET) {
			p.nextToken()
			return slice
		}
		p.nextToken()
		slice.End = p.parseExpression(LOWEST)
		if !p.expectPeek(lexer.RBRACKET) {
			return nil
		}
		return slice
	}

	first := p.parseExpression(LOWEST)

	// Срез [start:...].
	if p.peekTokenIs(lexer.COLON) {
		p.nextToken()
		slice := &ast.SliceExpression{Token: tok, Left: left, Start: first}
		if p.peekTokenIs(lexer.RBRACKET) {
			p.nextToken()
			return slice
		}
		p.nextToken()
		slice.End = p.parseExpression(LOWEST)
		if !p.expectPeek(lexer.RBRACKET) {
			return nil
		}
		return slice
	}

	// Обычная индексация x[i].
	exp := &ast.IndexExpression{Token: tok, Left: left, Index: first}
	if !p.expectPeek(lexer.RBRACKET) {
		return nil
	}
	return exp
}

// === Литералы словарей ===

// parseHashLiteral: {"ключ": значение, ...}.
// Поддерживает trailing comma. Сохраняет порядок появления ключей
// в KeyOrder для предсказуемой итерации и вывода.
func (p *Parser) parseHashLiteral() ast.Expression {
	hash := &ast.HashLiteral{Token: p.curToken}
	hash.Pairs = make(map[ast.Expression]ast.Expression)
	hash.KeyOrder = []ast.Expression{}

	for !p.peekTokenIs(lexer.RBRACE) && !p.peekTokenIs(lexer.EOF) {
		p.nextToken()

		// Запятые в начале (например, после trailing comma в предыдущей строке).
		if p.curTokenIs(lexer.COMMA) {
			continue
		}

		key := p.parseExpression(LOWEST)

		if !p.expectPeek(lexer.COLON) {
			return nil
		}

		p.nextToken()
		value := p.parseExpression(LOWEST)

		if _, exists := hash.Pairs[key]; !exists {
			hash.KeyOrder = append(hash.KeyOrder, key)
		}
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

// === Доступ через точку и optional chaining ===

// parseMethodCallExpression: obj.method(args) или obj.field.
// Превращается в IndexExpression (с возможным CallExpression сверху)
// с флагом IsDotAccess=true — чтобы форматер знал, что был синтаксис
// через точку, а не через скобки.
func (p *Parser) parseMethodCallExpression(left ast.Expression) ast.Expression {
	p.nextToken()

	if !p.curTokenIs(lexer.IDENT) {
		return nil
	}

	methodName := p.curToken.Literal
	if p.peekTokenIs(lexer.LPAREN) {
		p.nextToken()

		callToken := p.curToken // токен '(' — содержит Line/Column
		args := p.parseExpressionList(lexer.RPAREN)

		indexExpr := &ast.IndexExpression{
			Token:       callToken,
			Left:        left,
			Index:       &ast.StringLiteral{Value: methodName},
			IsDotAccess: true,
		}

		return &ast.CallExpression{
			Token:     callToken,
			Function:  indexExpr,
			Arguments: args,
		}
	}

	return &ast.IndexExpression{
		Left:        left,
		Index:       &ast.StringLiteral{Value: methodName},
		IsDotAccess: true,
	}
}

// parseOptionalExpression: obj?.field или obj?.method(args).
// Если obj == пусто, всё выражение вычисляется в пусто без ошибки.
func (p *Parser) parseOptionalExpression(left ast.Expression) ast.Expression {
	token := p.curToken
	p.nextToken()

	if !p.curTokenIs(lexer.IDENT) {
		return nil
	}

	right := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if p.peekTokenIs(lexer.LPAREN) {
		p.nextToken()
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

	return &ast.OptionalExpression{
		Token: token,
		Left:  left,
		Right: right,
	}
}
