package parser

import (
	"fmt"
	"strconv"

	"yasny-lang/ast"
	"yasny-lang/lexer"
)

// === Литералы и простые выражения ===

func (p *Parser) parseIdentifier() ast.Expression {
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

// 'это' внутри метода ссылается на текущий экземпляр.
func (p *Parser) parseThis() ast.Expression {
	return &ast.Identifier{Token: p.curToken, Value: "это"}
}

// 'родитель' внутри метода-наследника ссылается на родительский класс.
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

func (p *Parser) parseNullLiteral() ast.Expression {
	return &ast.NullLiteral{Token: p.curToken}
}

// === Операторы ===

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

// parseGroupedExpression обрабатывает три случая для скобок:
//   () => выражение         — лямбда без параметров
//   (a, b, c) => выражение  — лямбда с несколькими параметрами
//   (выражение)             — обычная группировка
func (p *Parser) parseGroupedExpression() ast.Expression {
	tok := p.curToken

	// () может быть только в начале лямбды.
	if p.peekTokenIs(lexer.RPAREN) {
		p.nextToken()
		if !p.peekTokenIs(lexer.ARROW) {
			p.errors = append(p.errors, fmt.Sprintf("❌ Ошибка в строке %d: пустые скобки требуют => для лямбды", tok.Line))
			return nil
		}
		p.nextToken() // на '=>'
		p.nextToken() // на тело
		body := p.parseExpression(LOWEST)
		return &ast.FunctionLiteral{
			Token:      tok,
			IsLambda:   true,
			Parameters: []*ast.Identifier{},
			Body: &ast.BlockStatement{
				Statements: []ast.Statement{
					&ast.ReturnStatement{Token: tok, ReturnValue: body},
				},
			},
		}
	}

	p.nextToken()
	exp := p.parseExpression(LOWEST)

	// Запятая после первого выражения = многопараметровая лямбда (a, b) => ...
	if p.peekTokenIs(lexer.COMMA) {
		params := []*ast.Identifier{}
		if id, ok := exp.(*ast.Identifier); ok {
			params = append(params, id)
		} else {
			p.errors = append(p.errors, fmt.Sprintf("❌ Ошибка в строке %d: ожидается список параметров в скобках", tok.Line))
			return nil
		}
		for p.peekTokenIs(lexer.COMMA) {
			p.nextToken()
			p.nextToken()
			if !p.curTokenIs(lexer.IDENT) {
				p.errors = append(p.errors, fmt.Sprintf("❌ Ошибка в строке %d: ожидается имя параметра", p.curToken.Line))
				return nil
			}
			params = append(params, &ast.Identifier{
				Token: p.curToken,
				Value: p.curToken.Literal,
			})
		}
		if !p.expectPeek(lexer.RPAREN) {
			return nil
		}
		if !p.expectPeek(lexer.ARROW) {
			p.errors = append(p.errors, fmt.Sprintf("❌ Ошибка в строке %d: ожидается => после списка параметров", p.curToken.Line))
			return nil
		}
		p.nextToken()
		body := p.parseExpression(LOWEST)
		return &ast.FunctionLiteral{
			Token:      tok,
			IsLambda:   true,
			Parameters: params,
			Body: &ast.BlockStatement{
				Statements: []ast.Statement{
					&ast.ReturnStatement{Token: tok, ReturnValue: body},
				},
			},
		}
	}

	if !p.expectPeek(lexer.RPAREN) {
		return nil
	}

	// Заворачиваем в GroupedExpression, чтобы форматер мог вывести
	// скобки. Эвалуатор для GroupedExpression прозрачен.
	return &ast.GroupedExpression{
		Token: tok,
		Inner: exp,
	}
}

// === Тернарник, pipe, лямбды, range ===

// parseTernaryExpression: условие ? a : b. Краткая форма; для
// идиоматичного кода предпочтительна inline-форма
// 'если c: a иначе: b'.
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

// parsePipeExpression — оператор |> (пайп). Сохраняем как отдельный
// AST-узел PipeExpression для форматера. Семантику разворачивает
// эвалуатор: 'x |> f(a)' эквивалентно 'f(x, a)'.
func (p *Parser) parsePipeExpression(left ast.Expression) ast.Expression {
	pipeTok := p.curToken
	p.nextToken()
	right := p.parseExpression(PIPE)
	if right == nil {
		return left
	}
	return &ast.PipeExpression{
		Token: pipeTok,
		Left:  left,
		Right: right,
	}
}

// parseLambdaExpression: x => выражение или (x, y) => выражение.
// Многопараметровая форма распарсена parseGroupedExpression и приходит
// как CallExpression на уровне операторного парсинга — здесь мы
// разворачиваем её обратно в список параметров.
func (p *Parser) parseLambdaExpression(left ast.Expression) ast.Expression {
	lambda := &ast.FunctionLiteral{Token: p.curToken, IsLambda: true}

	switch node := left.(type) {
	case *ast.Identifier:
		// Один параметр: x => ...
		lambda.Parameters = []*ast.Identifier{node}
	case *ast.CallExpression:
		// (a, b) => ... приходит сюда как CallExpression.
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

	// Тело лямбды — одно выражение, оборачиваем в return.
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

// parseRangeExpression: a..b. Эксклюзивная правая граница (как в Rust).
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
