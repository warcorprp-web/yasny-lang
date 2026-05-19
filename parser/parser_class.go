package parser

import (
	"yasny-lang/ast"
	"yasny-lang/lexer"
)

// parseEnumStatement: перечисление ИМЯ <члены> конец.
// Парсит в LetStatement с Hash-литералом, где значение каждого
// члена равно его имени (как строка).
func (p *Parser) parseEnumStatement() ast.Statement {
	enumTok := p.curToken

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}
	name := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	p.nextToken()

	pairs := []ast.Expression{}
	for !p.curTokenIs(lexer.END) && !p.curTokenIs(lexer.EOF) {
		if p.curTokenIs(lexer.IDENT) {
			memberName := p.curToken.Literal
			pairs = append(pairs, &ast.StringLiteral{Token: p.curToken, Value: memberName})
			pairs = append(pairs, &ast.StringLiteral{Token: p.curToken, Value: memberName})
		}
		p.nextToken()
	}

	hashLit := &ast.HashLiteral{Token: enumTok, Pairs: make(map[ast.Expression]ast.Expression), KeyOrder: []ast.Expression{}}
	for i := 0; i < len(pairs); i += 2 {
		hashLit.Pairs[pairs[i]] = pairs[i+1]
		hashLit.KeyOrder = append(hashLit.KeyOrder, pairs[i])
	}

	return &ast.LetStatement{
		Token: enumTok,
		Name:  name,
		Value: hashLit,
	}
}

// parseClassStatement: класс ИМЯ [наследует РОДИТЕЛЬ] <методы> конец.
// Парсит в LetStatement с Hash-литералом, где ключи — имена методов,
// значения — функции. Имя 'создать' автоматически переименовывается
// в 'инициализация' (это канонический ключ конструктора).
func (p *Parser) parseClassStatement() ast.Statement {
	className := p.curToken

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}

	name := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	var parentName *ast.Identifier
	if p.peekTokenIs(lexer.EXTENDS) {
		p.nextToken()
		if !p.expectPeek(lexer.IDENT) {
			return nil
		}
		parentName = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	}

	// Сохраняем методы в map для удобства, но порядок отслеживаем
	// отдельно — он важен для форматера и для удобочитаемости.
	methods := make(map[string]*ast.FunctionLiteral)
	methodOrder := []string{}

	p.nextToken()

	for !p.curTokenIs(lexer.END) && !p.curTokenIs(lexer.EOF) {
		if p.curTokenIs(lexer.FUNCTION) {
			methodTok := p.curToken
			p.nextToken()
			if !p.curTokenIs(lexer.IDENT) {
				p.nextToken()
				continue
			}
			methodName := p.curToken.Literal
			if methodName == "создать" {
				methodName = "инициализация"
			}

			if !p.expectPeek(lexer.LPAREN) {
				continue
			}

			lit := &ast.FunctionLiteral{Token: methodTok}
			lit.Parameters, lit.Defaults, lit.HasRest = p.parseFunctionParametersFull()

			if !p.expectPeek(lexer.RPAREN) {
				return nil
			}

			body, inline := p.parseInlineOrBlockImplicitReturn()
			lit.Body = body
			lit.IsInline = inline
			if _, ok := methods[methodName]; !ok {
				methodOrder = append(methodOrder, methodName)
			}
			methods[methodName] = lit
		}
		p.nextToken()
	}

	pairs := []ast.Expression{}
	for _, methodName := range methodOrder {
		pairs = append(pairs, &ast.StringLiteral{Value: methodName})
		pairs = append(pairs, methods[methodName])
	}

	if parentName != nil {
		pairs = append(pairs, &ast.StringLiteral{Value: "__parent_name__"})
		pairs = append(pairs, &ast.StringLiteral{Value: parentName.Value})
	}

	hashLit := &ast.HashLiteral{Pairs: make(map[ast.Expression]ast.Expression), KeyOrder: []ast.Expression{}}
	for i := 0; i < len(pairs); i += 2 {
		hashLit.Pairs[pairs[i]] = pairs[i+1]
		hashLit.KeyOrder = append(hashLit.KeyOrder, pairs[i])
	}

	return &ast.LetStatement{
		Token: className,
		Name:  name,
		Value: hashLit,
	}
}

// parseNewExpression: новый ИМЯ_КЛАССА(аргументы).
// Альтернативная форма создания экземпляра класса. Эквивалентно
// просто ИМЯ_КЛАССА(аргументы).
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
