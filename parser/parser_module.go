package parser

import (
	"yasny-lang/ast"
	"yasny-lang/lexer"
)

// parseImportStatement: импорт ИМЯ из "путь.ya" [как АЛИАС]
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

	if p.peekTokenIs(lexer.AS) {
		p.nextToken()
		if !p.expectPeek(lexer.IDENT) {
			return nil
		}
		stmt.Alias = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	}

	return stmt
}

// parseExportStatement: экспорт <statement>
func (p *Parser) parseExportStatement() *ast.ExportStatement {
	stmt := &ast.ExportStatement{Token: p.curToken}

	p.nextToken()
	stmt.Statement = p.parseStatement()

	if stmt.Statement == nil {
		return nil
	}

	return stmt
}
