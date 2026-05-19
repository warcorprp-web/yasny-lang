package linter

import (
	"fmt"
	"strings"
	"yasny-lang/ast"
	"yasny-lang/lexer"
	"yasny-lang/parser"
)

// Severity — уровень предупреждения.
type Severity int

const (
	Warning Severity = iota
	Error
)

// Issue — одно предупреждение линтера.
type Issue struct {
	File     string
	Line     int
	Column   int
	Severity Severity
	Code     string // короткий код правила, например "unused-var"
	Message  string
}

func (i Issue) String() string {
	sev := "⚠"
	if i.Severity == Error {
		sev = "✗"
	}
	loc := ""
	if i.File != "" {
		loc = fmt.Sprintf("%s:%d", i.File, i.Line)
	} else {
		loc = fmt.Sprintf("строка %d", i.Line)
	}
	return fmt.Sprintf("%s %s [%s] %s", sev, loc, i.Code, i.Message)
}

// Lint анализирует исходный код и возвращает список проблем.
func Lint(source, filename string) []Issue {
	l := lexer.NewWithFilename(source, filename)
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		issues := make([]Issue, 0, len(p.Errors()))
		for _, e := range p.Errors() {
			issues = append(issues, Issue{
				File:     filename,
				Line:     1,
				Severity: Error,
				Code:     "parse-error",
				Message:  e,
			})
		}
		return issues
	}

	var issues []Issue
	ctx := &lintContext{filename: filename}

	// Собираем все определения и использования
	ctx.collectDefinitions(program)
	ctx.collectUsages(program)

	// Правила
	issues = append(issues, ctx.checkUnusedVariables()...)
	issues = append(issues, ctx.checkUnusedFunctions()...)
	issues = append(issues, ctx.checkShadowing()...)
	issues = append(issues, ctx.checkUnreachableCode(program)...)
	issues = append(issues, ctx.checkEmptyBlocks(program)...)
	issues = append(issues, ctx.checkReassignConst(program)...)

	return issues
}

// === Контекст линтера ===

type definition struct {
	name  string
	line  int
	kind  string // "переменная", "константа", "функция", "параметр"
	scope int
}

type usage struct {
	name string
	line int
}

type lintContext struct {
	filename    string
	definitions []definition
	usages      []usage
	scopeDepth  int
}

func (ctx *lintContext) collectDefinitions(program *ast.Program) {
	ctx.scopeDepth = 0
	for _, stmt := range program.Statements {
		ctx.collectDefsFromStatement(stmt)
	}
}

func (ctx *lintContext) collectDefsFromStatement(stmt ast.Statement) {
	switch s := stmt.(type) {
	case *ast.LetStatement:
		if s.Name != nil {
			kind := "константа"
			// Проверяем — это функция или класс?
			if s.Token.Type == lexer.CLASS {
				kind = "класс"
			} else if _, ok := s.Value.(*ast.FunctionLiteral); ok {
				kind = "функция"
			}
			ctx.definitions = append(ctx.definitions, definition{
				name: s.Name.Value, line: s.Token.Line, kind: kind, scope: ctx.scopeDepth,
			})
		}
		if s.Value != nil {
			ctx.collectDefsFromExpression(s.Value)
		}
	case *ast.VarStatement:
		ctx.definitions = append(ctx.definitions, definition{
			name: s.Name.Value, line: s.Token.Line, kind: "переменная", scope: ctx.scopeDepth,
		})
		if s.Value != nil {
			ctx.collectDefsFromExpression(s.Value)
		}
	case *ast.ExpressionStatement:
		if s.Expression != nil {
			ctx.collectDefsFromExpression(s.Expression)
		}
	case *ast.ImportStatement:
		if s.Name != nil {
			ctx.definitions = append(ctx.definitions, definition{
				name: s.Name.Value, line: s.Token.Line, kind: "импорт", scope: ctx.scopeDepth,
			})
		}
	}
}

func (ctx *lintContext) collectDefsFromExpression(expr ast.Expression) {
	switch e := expr.(type) {
	case *ast.FunctionLiteral:
		if e.Name != nil {
			ctx.definitions = append(ctx.definitions, definition{
				name: e.Name.Value, line: e.Token.Line, kind: "функция", scope: ctx.scopeDepth,
			})
		}
		// Параметры
		ctx.scopeDepth++
		for _, p := range e.Parameters {
			ctx.definitions = append(ctx.definitions, definition{
				name: p.Value, line: p.Token.Line, kind: "параметр", scope: ctx.scopeDepth,
			})
		}
		if e.Body != nil {
			for _, stmt := range e.Body.Statements {
				ctx.collectDefsFromStatement(stmt)
			}
		}
		ctx.scopeDepth--
	case *ast.IfExpression:
		if e.Consequence != nil {
			ctx.scopeDepth++
			for _, stmt := range e.Consequence.Statements {
				ctx.collectDefsFromStatement(stmt)
			}
			ctx.scopeDepth--
		}
		if e.Alternative != nil {
			ctx.scopeDepth++
			for _, stmt := range e.Alternative.Statements {
				ctx.collectDefsFromStatement(stmt)
			}
			ctx.scopeDepth--
		}
	case *ast.ForExpression:
		ctx.scopeDepth++
		if e.Body != nil {
			for _, stmt := range e.Body.Statements {
				ctx.collectDefsFromStatement(stmt)
			}
		}
		ctx.scopeDepth--
	case *ast.ForInExpression:
		ctx.scopeDepth++
		if e.Body != nil {
			for _, stmt := range e.Body.Statements {
				ctx.collectDefsFromStatement(stmt)
			}
		}
		ctx.scopeDepth--
	}
}

func (ctx *lintContext) collectUsages(program *ast.Program) {
	for _, stmt := range program.Statements {
		ctx.collectUsagesFromStatement(stmt)
	}
}

func (ctx *lintContext) collectUsagesFromStatement(stmt ast.Statement) {
	switch s := stmt.(type) {
	case *ast.LetStatement:
		if s.Value != nil {
			ctx.collectUsagesFromExpression(s.Value)
		}
	case *ast.VarStatement:
		if s.Value != nil {
			ctx.collectUsagesFromExpression(s.Value)
		}
	case *ast.ReturnStatement:
		if s.ReturnValue != nil {
			ctx.collectUsagesFromExpression(s.ReturnValue)
		}
	case *ast.ExpressionStatement:
		if s.Expression != nil {
			ctx.collectUsagesFromExpression(s.Expression)
		}
	case *ast.AssignmentStatement:
		// Правая часть += содержит использование переменной
		if id, ok := s.Left.(*ast.Identifier); ok {
			if s.Operator != "=" {
				// += -= *= /= — переменная используется в правой части
				ctx.usages = append(ctx.usages, usage{name: id.Value, line: s.Token.Line})
			}
			ctx.usages = append(ctx.usages, usage{name: id.Value, line: s.Token.Line})
		}
		if s.Value != nil {
			ctx.collectUsagesFromExpression(s.Value)
		}
	case *ast.ExportStatement:
		if s.Statement != nil {
			ctx.collectUsagesFromStatement(s.Statement)
		}
	}
}

func (ctx *lintContext) collectUsagesFromExpression(expr ast.Expression) {
	if expr == nil {
		return
	}
	switch e := expr.(type) {
	case *ast.Identifier:
		ctx.usages = append(ctx.usages, usage{name: e.Value, line: e.Token.Line})
	case *ast.StringLiteral:
		// Интерполяция: строки с маркером \x00 содержат {переменная}
		if len(e.Value) > 0 && e.Value[0] == '\x00' {
			ctx.extractInterpolationUsages(e.Value[1:], e.Token.Line)
		}
	case *ast.CallExpression:
		ctx.collectUsagesFromExpression(e.Function)
		for _, a := range e.Arguments {
			ctx.collectUsagesFromExpression(a)
		}
	case *ast.InfixExpression:
		ctx.collectUsagesFromExpression(e.Left)
		ctx.collectUsagesFromExpression(e.Right)
	case *ast.PrefixExpression:
		ctx.collectUsagesFromExpression(e.Right)
	case *ast.IndexExpression:
		ctx.collectUsagesFromExpression(e.Left)
		ctx.collectUsagesFromExpression(e.Index)
	case *ast.IfExpression:
		ctx.collectUsagesFromExpression(e.Condition)
		if e.Consequence != nil {
			for _, stmt := range e.Consequence.Statements {
				ctx.collectUsagesFromStatement(stmt)
			}
		}
		if e.Alternative != nil {
			for _, stmt := range e.Alternative.Statements {
				ctx.collectUsagesFromStatement(stmt)
			}
		}
	case *ast.FunctionLiteral:
		if e.Body != nil {
			for _, stmt := range e.Body.Statements {
				ctx.collectUsagesFromStatement(stmt)
			}
		}
	case *ast.ArrayLiteral:
		for _, el := range e.Elements {
			ctx.collectUsagesFromExpression(el)
		}
	case *ast.HashLiteral:
		for k, v := range e.Pairs {
			ctx.collectUsagesFromExpression(k)
			ctx.collectUsagesFromExpression(v)
		}
	case *ast.ForExpression:
		if e.Body != nil {
			for _, stmt := range e.Body.Statements {
				ctx.collectUsagesFromStatement(stmt)
			}
		}
	case *ast.ForInExpression:
		ctx.collectUsagesFromExpression(e.Iterable)
		if e.Body != nil {
			for _, stmt := range e.Body.Statements {
				ctx.collectUsagesFromStatement(stmt)
			}
		}
	case *ast.WhileExpression:
		ctx.collectUsagesFromExpression(e.Condition)
		if e.Body != nil {
			for _, stmt := range e.Body.Statements {
				ctx.collectUsagesFromStatement(stmt)
			}
		}
	case *ast.TryExpression:
		if e.Body != nil {
			for _, stmt := range e.Body.Statements {
				ctx.collectUsagesFromStatement(stmt)
			}
		}
		if e.CatchBody != nil {
			for _, stmt := range e.CatchBody.Statements {
				ctx.collectUsagesFromStatement(stmt)
			}
		}
		if e.FinallyBody != nil {
			for _, stmt := range e.FinallyBody.Statements {
				ctx.collectUsagesFromStatement(stmt)
			}
		}
	}
}

// === Правила ===

// Неиспользуемые переменные/константы (не функции, не параметры с _)
func (ctx *lintContext) checkUnusedVariables() []Issue {
	var issues []Issue
	for _, def := range ctx.definitions {
		if def.kind != "переменная" && def.kind != "константа" && def.kind != "импорт" {
			continue
		}
		if strings.HasPrefix(def.name, "_") {
			continue // _ — явно игнорируемая
		}
		used := false
		for _, u := range ctx.usages {
			if u.name == def.name && u.line != def.line {
				used = true
				break
			}
		}
		if !used {
			issues = append(issues, Issue{
				File:     ctx.filename,
				Line:     def.line,
				Severity: Warning,
				Code:     "unused-var",
				Message:  fmt.Sprintf("%s '%s' объявлена но не используется", def.kind, def.name),
			})
		}
	}
	return issues
}

// Неиспользуемые функции (кроме экспортируемых и создать)
func (ctx *lintContext) checkUnusedFunctions() []Issue {
	var issues []Issue
	for _, def := range ctx.definitions {
		if def.kind != "функция" {
			continue
		}
		if def.name == "создать" || strings.HasPrefix(def.name, "_") {
			continue
		}
		used := false
		for _, u := range ctx.usages {
			if u.name == def.name {
				used = true
				break
			}
		}
		if !used {
			issues = append(issues, Issue{
				File:     ctx.filename,
				Line:     def.line,
				Severity: Warning,
				Code:     "unused-func",
				Message:  fmt.Sprintf("функция '%s' объявлена но не вызывается", def.name),
			})
		}
	}
	return issues
}

// Затенение переменных (одно имя в разных скоупах)
func (ctx *lintContext) checkShadowing() []Issue {
	var issues []Issue
	seen := map[string]definition{}
	for _, def := range ctx.definitions {
		if def.kind == "параметр" || def.kind == "класс" {
			continue
		}
		if prev, ok := seen[def.name]; ok {
			if def.scope > prev.scope {
				issues = append(issues, Issue{
					File:     ctx.filename,
					Line:     def.line,
					Severity: Warning,
					Code:     "shadow",
					Message:  fmt.Sprintf("'%s' затеняет определение на строке %d", def.name, prev.line),
				})
			}
		} else {
			seen[def.name] = def
		}
	}
	return issues
}

// Недостижимый код после вернуть/бросить/прервать
func (ctx *lintContext) checkUnreachableCode(program *ast.Program) []Issue {
	var issues []Issue
	for _, stmt := range program.Statements {
		issues = append(issues, ctx.checkUnreachableInBlock(stmt)...)
	}
	return issues
}

func (ctx *lintContext) checkUnreachableInBlock(stmt ast.Statement) []Issue {
	var issues []Issue
	switch s := stmt.(type) {
	case *ast.LetStatement:
		if fl, ok := s.Value.(*ast.FunctionLiteral); ok && fl.Body != nil {
			issues = append(issues, ctx.findUnreachableInStatements(fl.Body.Statements)...)
		}
	case *ast.ExpressionStatement:
		if fl, ok := s.Expression.(*ast.FunctionLiteral); ok && fl.Body != nil {
			issues = append(issues, ctx.findUnreachableInStatements(fl.Body.Statements)...)
		}
	}
	return issues
}

func (ctx *lintContext) findUnreachableInStatements(stmts []ast.Statement) []Issue {
	var issues []Issue
	for i, stmt := range stmts {
		isTerminator := false
		switch stmt.(type) {
		case *ast.ReturnStatement:
			isTerminator = true
		case *ast.ThrowStatement:
			isTerminator = true
		case *ast.BreakStatement:
			isTerminator = true
		case *ast.ContinueStatement:
			isTerminator = true
		}
		if isTerminator && i < len(stmts)-1 {
			next := stmts[i+1]
			line := 0
			switch n := next.(type) {
			case *ast.ExpressionStatement:
				line = n.Token.Line
			case *ast.LetStatement:
				line = n.Token.Line
			case *ast.VarStatement:
				line = n.Token.Line
			case *ast.ReturnStatement:
				line = n.Token.Line
			}
			if line > 0 {
				issues = append(issues, Issue{
					File:     ctx.filename,
					Line:     line,
					Severity: Warning,
					Code:     "unreachable",
					Message:  "недостижимый код после вернуть/бросить/прервать",
				})
			}
			break
		}
	}
	return issues
}

// Пустые блоки (если/для/пока без тела)
func (ctx *lintContext) checkEmptyBlocks(program *ast.Program) []Issue {
	var issues []Issue
	for _, stmt := range program.Statements {
		ctx.findEmptyBlocks(stmt, &issues)
	}
	return issues
}

func (ctx *lintContext) findEmptyBlocks(stmt ast.Statement, issues *[]Issue) {
	switch s := stmt.(type) {
	case *ast.ExpressionStatement:
		if s.Expression != nil {
			ctx.findEmptyBlocksInExpr(s.Expression, issues)
		}
	case *ast.LetStatement:
		if s.Value != nil {
			ctx.findEmptyBlocksInExpr(s.Value, issues)
		}
	}
}

func (ctx *lintContext) findEmptyBlocksInExpr(expr ast.Expression, issues *[]Issue) {
	switch e := expr.(type) {
	case *ast.IfExpression:
		if e.Consequence != nil && len(e.Consequence.Statements) == 0 {
			*issues = append(*issues, Issue{
				File:     ctx.filename,
				Line:     e.Token.Line,
				Severity: Warning,
				Code:     "empty-block",
				Message:  "пустой блок 'если'",
			})
		}
	case *ast.ForExpression:
		if e.Body != nil && len(e.Body.Statements) == 0 {
			*issues = append(*issues, Issue{
				File:     ctx.filename,
				Line:     e.Token.Line,
				Severity: Warning,
				Code:     "empty-block",
				Message:  "пустой блок 'для'",
			})
		}
	case *ast.FunctionLiteral:
		if e.Body != nil {
			for _, stmt := range e.Body.Statements {
				ctx.findEmptyBlocks(stmt, issues)
			}
		}
	}
}

// Переприсваивание конст
func (ctx *lintContext) checkReassignConst(program *ast.Program) []Issue {
	var issues []Issue
	consts := map[string]int{}
	for _, def := range ctx.definitions {
		if def.kind == "константа" && def.scope == 0 {
			consts[def.name] = def.line
		}
	}
	for _, stmt := range program.Statements {
		if as, ok := stmt.(*ast.AssignmentStatement); ok {
			if _, isConst := consts[getAssignName(as)]; isConst {
				issues = append(issues, Issue{
					File:     ctx.filename,
					Line:     as.Token.Line,
					Severity: Error,
					Code:     "const-reassign",
					Message:  fmt.Sprintf("попытка изменить константу '%s'", getAssignName(as)),
				})
			}
		}
	}
	return issues
}

// getAssignName извлекает имя из AssignmentStatement.Left.
func getAssignName(as *ast.AssignmentStatement) string {
	if id, ok := as.Left.(*ast.Identifier); ok {
		return id.Value
	}
	return ""
}

// extractInterpolationUsages извлекает идентификаторы из интерполированной строки.
// Формат: "текст {выражение} текст {выражение}..."
func (ctx *lintContext) extractInterpolationUsages(s string, line int) {
	inBrace := false
	var ident strings.Builder
	for _, r := range s {
		if r == '{' {
			inBrace = true
			ident.Reset()
		} else if r == '}' && inBrace {
			inBrace = false
			name := ident.String()
			// Простой идентификатор или доступ через точку (берём первую часть)
			if idx := strings.IndexByte(name, '.'); idx > 0 {
				name = name[:idx]
			}
			if idx := strings.IndexByte(name, '['); idx > 0 {
				name = name[:idx]
			}
			if idx := strings.IndexByte(name, '('); idx > 0 {
				name = name[:idx]
			}
			// Пропускаем выражения с операторами
			if len(name) > 0 && !strings.ContainsAny(name, " +-*/") {
				ctx.usages = append(ctx.usages, usage{name: strings.TrimSpace(name), line: line})
			}
		} else if inBrace {
			ident.WriteRune(r)
		}
	}
}
