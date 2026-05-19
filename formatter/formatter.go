// Пакет formatter форматирует код на Ясном по канону, описанному
// в СТИЛЬ.md.
package formatter

import (
	"fmt"
	"strings"

	"yasny-lang/ast"
	"yasny-lang/lexer"
	"yasny-lang/parser"
)

// IndentSize — размер одного уровня отступа в пробелах.
const IndentSize = 4

// MaxInlineWidth — максимальная длина строки для однострочных
// массивов/словарей. Длиннее — переключаемся на многострочный вид.
const MaxInlineWidth = 80

// Format форматирует исходный код, возвращая канонический вариант.
// Если в коде синтаксическая ошибка — возвращает исходный код и ошибку.
//
// Форматер сохраняет:
//   - комментарии (на отдельных строках перед кодом)
//   - намеренные пустые строки (схлопывая множественные в одну)
//   - выбор автора между inline и блочной формой функций/if/for/while
func Format(source string) (string, error) {
	l := lexer.New(source)
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		return source, fmt.Errorf("ошибки парсинга, форматирование невозможно:\n%s",
			strings.Join(errs, "\n"))
	}
	w := &writer{
		comments:    l.Comments(),
		blankLines:  l.BlankLineMarks(),
		commentIdx:  0,
	}
	w.formatProgram(program)
	return w.String(), nil
}

// writer аккумулирует форматированный вывод.
type writer struct {
	sb     strings.Builder
	indent int

	// Комментарии и пустые строки из исходника — для восстановления
	// авторской разметки. Курсор commentIdx продвигается по мере
	// вставки комментариев в нужных местах.
	comments        []lexer.Token
	commentIdx      int
	blankLines      []int
	lastEmittedLine int // последняя строка исходника, которую мы вывели
}

func (w *writer) String() string {
	out := w.sb.String()
	// Убираем trailing whitespace на каждой строке.
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	out = strings.Join(lines, "\n")
	// Схлопываем 3+ \n в 2 (= одна пустая строка между блоками).
	for strings.Contains(out, "\n\n\n") {
		out = strings.ReplaceAll(out, "\n\n\n", "\n\n")
	}
	// Гарантируем ровно одну \n в конце.
	out = strings.TrimRight(out, "\n") + "\n"
	return out
}

func (w *writer) pad() {
	w.sb.WriteString(strings.Repeat(" ", w.indent*IndentSize))
}

func (w *writer) write(s string) {
	w.sb.WriteString(s)
}

func (w *writer) nl() {
	w.sb.WriteByte('\n')
}

// emitCommentsBefore выводит все накопленные комментарии, чьи строки
// меньше или равны targetLine. Перед каждым комментарием/statement
// если в исходнике была пустая строка — её сохраняем.
func (w *writer) emitCommentsBefore(targetLine int) {
	for w.commentIdx < len(w.comments) && w.comments[w.commentIdx].Line <= targetLine {
		c := w.comments[w.commentIdx]
		// Если перед этим комментарием в исходнике была пустая
		// строка (после предыдущего вывода) — сохраним.
		if w.lastEmittedLine > 0 && w.hasBlankLineBetween(w.lastEmittedLine, c.Line) {
			w.nl()
		}
		w.commentIdx++
		w.pad()
		text := c.Literal
		if strings.HasPrefix(text, "#") {
			rest := strings.TrimLeft(text[1:], " \t")
			if rest == "" {
				text = "#"
			} else {
				text = "# " + rest
			}
		}
		w.sb.WriteString(text)
		w.nl()
		w.lastEmittedLine = c.Line
	}
}

// hasBlankLineBetween проверяет, была ли в исходнике пустая строка
// между строками low и high (исключая концы — то есть строго внутри).
// Используется для решения, нужна ли пустая строка между двумя statement'ами.
func (w *writer) hasBlankLineBetween(low, high int) bool {
	for _, mark := range w.blankLines {
		if mark > low && mark <= high {
			return true
		}
	}
	return false
}

// stmtLine возвращает номер строки токена statement (для определения
// где он стоит относительно комментариев и пустых строк).
func stmtLine(stmt ast.Statement) int {
	switch s := stmt.(type) {
	case *ast.LetStatement:
		return s.Token.Line
	case *ast.VarStatement:
		return s.Token.Line
	case *ast.ReturnStatement:
		return s.Token.Line
	case *ast.ThrowStatement:
		return s.Token.Line
	case *ast.YieldStatement:
		return s.Token.Line
	case *ast.AssignmentStatement:
		return s.Token.Line
	case *ast.ExpressionStatement:
		return s.Token.Line
	case *ast.ImportStatement:
		return s.Token.Line
	case *ast.ExportStatement:
		return s.Token.Line
	case *ast.DestructuringStatement:
		return s.Token.Line
	case *ast.DecoratedFunctionStatement:
		return s.Token.Line
	case *ast.BreakStatement:
		return s.Token.Line
	case *ast.ContinueStatement:
		return s.Token.Line
	}
	return 0
}

func (w *writer) formatProgram(program *ast.Program) {
	for i, stmt := range program.Statements {
		line := stmtLine(stmt)
		// Проверяем blank между последним выводом и текущим statement —
		// это работает и для случая "blank до первого комментария",
		// и для "blank после комментариев перед statement".
		if w.lastEmittedLine > 0 && w.hasBlankLineBetween(w.lastEmittedLine, line) {
			w.nl()
		}
		w.emitCommentsBefore(line)
		// Пустая строка между предыдущим контентом (комментарием
		// или statement) и текущим, если в исходнике она была.
		if w.lastEmittedLine > 0 && w.hasBlankLineBetween(w.lastEmittedLine, line) {
			w.nl()
		}
		// Между двумя def'ами — обязательная пустая строка по канону.
		if i > 0 {
			cur := isTopLevelDef(stmt)
			prev := isTopLevelDef(program.Statements[i-1])
			if cur && prev {
				prevLine := stmtLine(program.Statements[i-1])
				if !w.hasBlankLineBetween(prevLine, line) {
					w.nl()
				}
			}
		}

		w.formatStatement(stmt)
		w.lastEmittedLine = line
	}
	w.emitCommentsBefore(1 << 30)
}

// isTopLevelDef проверяет — это определение, вокруг которого нужна
// пустая строка (функция, класс, перечисление, декорированная функция).
func isTopLevelDef(stmt ast.Statement) bool {
	switch s := stmt.(type) {
	case *ast.LetStatement:
		if s.Token.Type == lexer.CLASS || s.Token.Type == lexer.ENUM {
			return true
		}
		if _, ok := s.Value.(*ast.FunctionLiteral); ok {
			return true
		}
	case *ast.ExpressionStatement:
		if _, ok := s.Expression.(*ast.FunctionLiteral); ok {
			return true
		}
	case *ast.DecoratedFunctionStatement:
		return true
	}
	return false
}

// formatStatement форматирует одну инструкцию с отступом.
func (w *writer) formatStatement(stmt ast.Statement) {
	w.pad()
	w.formatStatementInline(stmt)
	w.nl()
}

// formatStatementInline форматирует инструкцию без отступа и без \n.
func (w *writer) formatStatementInline(stmt ast.Statement) {
	switch s := stmt.(type) {
	case *ast.LetStatement:
		w.formatLet(s)
	case *ast.VarStatement:
		w.write("перем ")
		w.write(s.Name.Value)
		w.write(" = ")
		w.formatExpression(s.Value)
	case *ast.ReturnStatement:
		w.write("вернуть")
		if s.ReturnValue != nil {
			w.write(" ")
			w.formatExpression(s.ReturnValue)
		}
	case *ast.ThrowStatement:
		w.write("бросить")
		if s.Value != nil {
			w.write(" ")
			w.formatExpression(s.Value)
		}
	case *ast.YieldStatement:
		w.write("выдать ")
		w.formatExpression(s.Value)
	case *ast.AssignmentStatement:
		w.formatExpression(s.Left)
		w.write(" ")
		w.write(s.Operator)
		w.write(" ")
		w.formatExpression(s.Value)
	case *ast.ExpressionStatement:
		w.formatExpression(s.Expression)
	case *ast.ImportStatement:
		w.write("импорт ")
		if s.Name != nil {
			w.write(s.Name.Value)
			w.write(" из ")
		}
		w.write(quote(s.Path))
		if s.Alias != nil {
			w.write(" как ")
			w.write(s.Alias.Value)
		}
	case *ast.ExportStatement:
		w.write("экспорт ")
		w.formatStatementInline(s.Statement)
	case *ast.DestructuringStatement:
		w.write("конст ")
		w.formatExpression(s.Pattern)
		w.write(" = ")
		w.formatExpression(s.Value)
	case *ast.DecoratedFunctionStatement:
		// @декоратор1 @декоратор2 функция f(...) ... конец
		// Каждый декоратор на своей строке перед функцией.
		for i, dec := range s.Decorators {
			if i > 0 {
				w.nl()
				w.pad()
			}
			w.write("@")
			w.formatExpression(dec)
		}
		w.nl()
		w.pad()
		w.formatFunction(s.Function)
	case *ast.BreakStatement:
		w.write("прервать")
	case *ast.ContinueStatement:
		w.write("продолжить")
	default:
		w.write(fmt.Sprintf("/* TODO statement: %T */", stmt))
	}
}

// formatLet форматирует let-statement, распознавая среди них
// замаскированные класс и перечисление по типу токена.
func (w *writer) formatLet(s *ast.LetStatement) {
	switch s.Token.Type {
	case lexer.CLASS:
		w.formatClass(s)
		return
	case lexer.ENUM:
		w.formatEnum(s)
		return
	}
	w.write("конст ")
	w.write(s.Name.Value)
	w.write(" = ")
	w.formatExpression(s.Value)
}

// formatExpression форматирует выражение.
func (w *writer) formatExpression(expr ast.Expression) {
	switch e := expr.(type) {
	case *ast.Identifier:
		w.write(e.Value)
	case *ast.IntegerLiteral:
		w.write(fmt.Sprintf("%d", e.Value))
	case *ast.FloatLiteral:
		w.write(fmt.Sprintf("%g", e.Value))
	case *ast.StringLiteral:
		w.write(quote(e.Value))
	case *ast.Boolean:
		if e.Value {
			w.write("да")
		} else {
			w.write("нет")
		}
	case *ast.NullLiteral:
		w.write("ничего")
	case *ast.PrefixExpression:
		if e.Operator == "не" {
			w.write("не ")
		} else {
			w.write(e.Operator)
		}
		w.formatExpression(e.Right)
	case *ast.InfixExpression:
		w.formatExpression(e.Left)
		w.write(" ")
		w.write(e.Operator)
		w.write(" ")
		w.formatExpression(e.Right)
	case *ast.TernaryExpression:
		w.formatExpression(e.Condition)
		w.write(" ? ")
		w.formatExpression(e.Consequence)
		w.write(" : ")
		w.formatExpression(e.Alternative)
	case *ast.RangeExpression:
		w.formatExpression(e.Start)
		w.write("..")
		w.formatExpression(e.End)
	case *ast.PipeExpression:
		// Многострочный pipeline сохраняем многострочно, иначе
		// однострочно.
		leftLine := exprLine(e.Left)
		rightLine := exprLine(e.Right)
		multiline := leftLine > 0 && rightLine > 0 && leftLine != rightLine
		w.formatExpression(e.Left)
		if multiline {
			w.nl()
			w.indent++
			w.pad()
			w.indent--
		} else {
			w.write(" ")
		}
		w.write("|> ")
		w.formatExpression(e.Right)
	case *ast.GroupedExpression:
		w.write("(")
		w.formatExpression(e.Inner)
		w.write(")")
	case *ast.CallExpression:
		// Специальные конструкции: __тест__("имя", функция() ...)
		// и __проверить__(условие). В исходнике они выглядят как
		// 'тест "имя"' и 'проверить условие'.
		if w.tryFormatTestStmt(e) {
			return
		}
		if w.tryFormatCheckStmt(e) {
			return
		}
		w.formatExpression(e.Function)
		w.write("(")
		for i, arg := range e.Arguments {
			if i > 0 {
				w.write(", ")
			}
			w.formatExpression(arg)
		}
		w.write(")")
	case *ast.IndexExpression:
		w.formatExpression(e.Left)
		// Если это была форма obj.имя — печатаем через точку.
		if e.IsDotAccess {
			if sl, ok := e.Index.(*ast.StringLiteral); ok {
				w.write(".")
				// Убираем маркер интерполяции если есть.
				name := sl.Value
				if strings.HasPrefix(name, "\x00") {
					name = name[1:]
				}
				w.write(name)
				return
			}
		}
		w.write("[")
		w.formatExpression(e.Index)
		w.write("]")
	case *ast.SliceExpression:
		w.formatExpression(e.Left)
		w.write("[")
		if e.Start != nil {
			w.formatExpression(e.Start)
		}
		w.write(":")
		if e.End != nil {
			w.formatExpression(e.End)
		}
		w.write("]")
	case *ast.OptionalExpression:
		w.formatExpression(e.Left)
		w.write("?.")
		w.formatExpression(e.Right)
	case *ast.ArrayLiteral:
		w.formatArray(e)
	case *ast.HashLiteral:
		w.formatHash(e)
	case *ast.IfExpression:
		w.formatIf(e)
	case *ast.MatchExpression:
		w.formatMatch(e)
	case *ast.ForExpression:
		w.formatFor(e)
	case *ast.ForInExpression:
		w.formatForIn(e)
	case *ast.WhileExpression:
		w.formatWhile(e)
	case *ast.FunctionLiteral:
		w.formatFunction(e)
	case *ast.NewExpression:
		w.write("новый ")
		w.write(e.ClassName.Value)
		w.write("(")
		for i, arg := range e.Arguments {
			if i > 0 {
				w.write(", ")
			}
			w.formatExpression(arg)
		}
		w.write(")")
	case *ast.SpreadExpression:
		w.write("...")
		w.formatExpression(e.Value)
	case *ast.AsyncExpression:
		w.write("асинх ")
		w.formatExpression(e.Body)
	case *ast.AwaitExpression:
		w.write("ждать ")
		w.formatExpression(e.Body)
	case *ast.TryExpression:
		w.formatTry(e)
	case *ast.ArrayComprehension:
		w.write("[")
		w.formatExpression(e.Element)
		w.write(" для ")
		w.write(e.Variable.Value)
		w.write(" в ")
		w.formatExpression(e.Iterable)
		if e.Condition != nil {
			w.write(" если ")
			w.formatExpression(e.Condition)
		}
		w.write("]")
	default:
		w.write(fmt.Sprintf("/* TODO expression: %T */", expr))
	}
}

// formatBlock форматирует тело блока с увеличенным отступом.
func (w *writer) formatBlock(block *ast.BlockStatement) {
	if block == nil {
		return
	}
	w.indent++
	defer func() { w.indent-- }()
	for _, stmt := range block.Statements {
		line := stmtLine(stmt)
		// Если last был раньше — обновляем на line (мы уже на этой
		// строке после выхода из условия/заголовка).
		// Но проверяем blank относительно реально предыдущего вывода.
		w.emitCommentsBefore(line)
		if w.lastEmittedLine > 0 && w.lastEmittedLine < line && w.hasBlankLineBetween(w.lastEmittedLine, line) {
			// blank уже выведен в emitCommentsBefore при необходимости
		}
		w.formatStatement(stmt)
		w.lastEmittedLine = line
	}
}

// formatIf форматирует если/иначеесли/иначе.
func (w *writer) formatIf(e *ast.IfExpression) {
	w.write("если ")
	w.formatExpression(e.Condition)

	if e.IsInline && e.Consequence != nil && len(e.Consequence.Statements) == 1 {
		w.write(": ")
		w.formatStatementInline(e.Consequence.Statements[0])
		// Inline-альтернатива: иначе: B (того же типа).
		if e.Alternative != nil && len(e.Alternative.Statements) == 1 {
			w.write(" иначе: ")
			w.formatStatementInline(e.Alternative.Statements[0])
		}
		return
	}

	w.nl()
	w.formatBlock(e.Consequence)

	if e.Alternative != nil {
		// Проверяем — иначеесли (вложенный if в одном statement)?
		if len(e.Alternative.Statements) == 1 {
			if es, ok := e.Alternative.Statements[0].(*ast.ExpressionStatement); ok {
				if nested, ok := es.Expression.(*ast.IfExpression); ok {
					w.pad()
					w.write("иначе")
					w.formatIf(nested)
					return
				}
			}
		}
		w.pad()
		w.write("иначе")
		w.nl()
		w.formatBlock(e.Alternative)
	}
	w.pad()
	w.write("конец")
}

// formatFunction форматирует функцию или лямбду.
func (w *writer) formatFunction(e *ast.FunctionLiteral) {
	// Лямбда: x => тело  или  (a, b) => тело.
	if e.IsLambda {
		w.formatLambda(e)
		return
	}
	w.write("функция")
	if e.Name != nil {
		w.write(" ")
		w.write(e.Name.Value)
	}
	w.write("(")
	w.formatParams(e.Parameters, e.Defaults, e.HasRest)
	w.write(")")

	// Inline-форма "функция f(x): выражение".
	if e.IsInline && e.Body != nil && len(e.Body.Statements) == 1 {
		if ret, ok := e.Body.Statements[0].(*ast.ReturnStatement); ok {
			w.write(": ")
			w.formatExpression(ret.ReturnValue)
			return
		}
	}

	w.nl()
	w.formatBlock(e.Body)
	w.pad()
	w.write("конец")
}

// formatLambda форматирует лямбду в её исходном синтаксисе:
// x => тело (для одного параметра без скобок) или
// (a, b) => тело (для нескольких — со скобками).
func (w *writer) formatLambda(e *ast.FunctionLiteral) {
	if len(e.Parameters) == 1 && !e.HasRest {
		w.write(e.Parameters[0].Value)
	} else {
		w.write("(")
		w.formatParams(e.Parameters, e.Defaults, e.HasRest)
		w.write(")")
	}
	w.write(" => ")
	// Тело лямбды — это блок с одним ReturnStatement.
	if e.Body != nil && len(e.Body.Statements) == 1 {
		if ret, ok := e.Body.Statements[0].(*ast.ReturnStatement); ok && ret.ReturnValue != nil {
			w.formatExpression(ret.ReturnValue)
			return
		}
	}
	// Запасной вариант — печатаем как блок.
	w.write("/* сложная лямбда */")
}

// formatParams форматирует параметры функции с дефолтами и rest.
func (w *writer) formatParams(params []*ast.Identifier, defaults []ast.Expression, hasRest bool) {
	for i, p := range params {
		if i > 0 {
			w.write(", ")
		}
		if hasRest && i == len(params)-1 {
			w.write("...")
		}
		w.write(p.Value)
		if i < len(defaults) && defaults[i] != nil {
			w.write(" = ")
			w.formatExpression(defaults[i])
		}
	}
}

// formatFor форматирует "для i от A до B [по N]".
func (w *writer) formatFor(e *ast.ForExpression) {
	w.write("для ")
	w.write(e.Variable.Value)
	w.write(" от ")
	w.formatExpression(e.From)
	w.write(" до ")
	w.formatExpression(e.To)
	if e.Step != nil {
		w.write(" по ")
		w.formatExpression(e.Step)
	}

	if e.IsInline && e.Body != nil && len(e.Body.Statements) == 1 {
		w.write(": ")
		w.formatStatementInline(e.Body.Statements[0])
		return
	}

	w.nl()
	w.formatBlock(e.Body)
	w.pad()
	w.write("конец")
}

// formatForIn форматирует "для x в коллекция" или "для i, x в коллекция".
func (w *writer) formatForIn(e *ast.ForInExpression) {
	w.write("для ")
	if e.Index != nil {
		w.write(e.Index.Value)
		w.write(", ")
	}
	w.write(e.Variable.Value)
	w.write(" в ")
	w.formatExpression(e.Iterable)

	if e.IsInline && e.Body != nil && len(e.Body.Statements) == 1 {
		w.write(": ")
		w.formatStatementInline(e.Body.Statements[0])
		return
	}

	w.nl()
	w.formatBlock(e.Body)
	w.pad()
	w.write("конец")
}

// formatWhile форматирует "пока условие".
func (w *writer) formatWhile(e *ast.WhileExpression) {
	w.write("пока ")
	w.formatExpression(e.Condition)

	if e.IsInline && e.Body != nil && len(e.Body.Statements) == 1 {
		w.write(": ")
		w.formatStatementInline(e.Body.Statements[0])
		return
	}

	w.nl()
	w.formatBlock(e.Body)
	w.pad()
	w.write("конец")
}

// formatMatch форматирует "совпадение значение когда ... конец".
func (w *writer) formatMatch(e *ast.MatchExpression) {
	w.write("совпадение ")
	w.formatExpression(e.Value)
	w.nl()
	w.indent++
	for _, c := range e.Cases {
		w.pad()
		if c.Pattern == nil {
			w.write("иначе")
		} else {
			w.write("когда ")
			w.formatExpression(c.Pattern)
		}
		w.write(": ")
		w.formatExpression(c.Result)
		w.nl()
	}
	w.indent--
	w.pad()
	w.write("конец")
}

// formatTry форматирует "попытка ... поймать ... всегда ... конец".
func (w *writer) formatTry(e *ast.TryExpression) {
	w.write("попытка")
	w.nl()
	w.formatBlock(e.Body)
	if e.CatchBody != nil {
		w.pad()
		w.write("поймать ")
		if e.CatchVar != nil {
			w.write(e.CatchVar.Value)
		}
		w.nl()
		w.formatBlock(e.CatchBody)
	}
	if e.FinallyBody != nil {
		w.pad()
		w.write("всегда")
		w.nl()
		w.formatBlock(e.FinallyBody)
	}
	w.pad()
	w.write("конец")
}

// formatClass форматирует определение класса (хранится как
// LetStatement с HashLiteral в Value, где ключи — имена методов,
// значения — FunctionLiteral; ключ "__parent_name__" — имя родителя).
func (w *writer) formatClass(s *ast.LetStatement) {
	hash, ok := s.Value.(*ast.HashLiteral)
	if !ok {
		// Не похоже на класс — печатаем как обычный let.
		w.write("конст ")
		w.write(s.Name.Value)
		w.write(" = ")
		w.formatExpression(s.Value)
		return
	}

	w.write("класс ")
	w.write(s.Name.Value)

	// Извлекаем имя родителя из "__parent_name__".
	parentName := ""
	for _, k := range hash.KeyOrder {
		if sl, ok := k.(*ast.StringLiteral); ok && sl.Value == "__parent_name__" {
			if pv, ok := hash.Pairs[k].(*ast.StringLiteral); ok {
				parentName = pv.Value
			}
		}
	}
	if parentName != "" {
		w.write(" наследует ")
		w.write(parentName)
	}
	w.nl()
	w.indent++

	// Обновляем lastEmittedLine на строку заголовка класса, чтобы
	// проверки blank-строк перед методами и комментариями работали
	// корректно.
	w.lastEmittedLine = s.Token.Line

	// Считаем методы (ключи-строки, значения-функции, кроме __parent_name__).
	type method struct {
		name string
		fn   *ast.FunctionLiteral
	}
	methods := []method{}
	for _, k := range hash.KeyOrder {
		sl, ok := k.(*ast.StringLiteral)
		if !ok {
			continue
		}
		if sl.Value == "__parent_name__" {
			continue
		}
		fn, ok := hash.Pairs[k].(*ast.FunctionLiteral)
		if !ok {
			continue
		}
		// Возвращаем имя 'инициализация' обратно в 'создать' для
		// идиоматичной записи.
		name := sl.Value
		if name == "инициализация" {
			name = "создать"
		}
		methods = append(methods, method{name, fn})
	}

	for i, m := range methods {
		// Комментарии перед методом.
		if m.fn.Token.Line > 0 {
			w.emitCommentsBefore(m.fn.Token.Line)
		}
		w.pad()
		w.write("функция ")
		w.write(m.name)
		w.write("(")
		w.formatParams(m.fn.Parameters, m.fn.Defaults, m.fn.HasRest)
		w.write(")")

		// Inline-форма метода.
		isInlineMethod := m.fn.IsInline && m.fn.Body != nil && len(m.fn.Body.Statements) == 1
		if isInlineMethod {
			if ret, ok := m.fn.Body.Statements[0].(*ast.ReturnStatement); ok {
				w.write(": ")
				w.formatExpression(ret.ReturnValue)
				w.nl()
				if i < len(methods)-1 {
					w.nl()
				}
				continue
			}
		}

		w.nl()
		w.formatBlock(m.fn.Body)
		w.pad()
		w.write("конец")
		w.nl()
		if i < len(methods)-1 {
			w.nl()
		}
	}

	w.indent--
	w.pad()
	w.write("конец")
}

// formatEnum форматирует определение перечисления (хранится как
// LetStatement с HashLiteral, где ключи и значения совпадают).
func (w *writer) formatEnum(s *ast.LetStatement) {
	hash, ok := s.Value.(*ast.HashLiteral)
	if !ok {
		w.write("конст ")
		w.write(s.Name.Value)
		w.write(" = ")
		w.formatExpression(s.Value)
		return
	}

	w.write("перечисление ")
	w.write(s.Name.Value)
	w.nl()
	w.indent++
	for _, k := range hash.KeyOrder {
		sl, ok := k.(*ast.StringLiteral)
		if !ok {
			continue
		}
		w.pad()
		w.write(sl.Value)
		w.nl()
	}
	w.indent--
	w.pad()
	w.write("конец")
}

// formatArray — однострочно если коротко, многострочно если длинно
// или если в исходнике уже было многострочно (по позициям токенов).
func (w *writer) formatArray(e *ast.ArrayLiteral) {
	if len(e.Elements) == 0 {
		w.write("[]")
		return
	}
	// Если в исходнике первый элемент на другой строке, чем '[' —
	// автор уже выбрал многострочный вид, уважаем.
	wasMultiline := false
	if len(e.Elements) > 0 {
		firstLine := exprLine(e.Elements[0])
		if firstLine > 0 && firstLine != e.Token.Line {
			wasMultiline = true
		}
	}

	if !wasMultiline {
		if inline := tryInlineArray(e); inline != "" && len(inline) <= MaxInlineWidth {
			w.write(inline)
			return
		}
	}

	w.write("[")
	w.nl()
	w.indent++
	for _, el := range e.Elements {
		w.pad()
		w.formatExpression(el)
		w.write(",")
		w.nl()
	}
	w.indent--
	w.pad()
	w.write("]")
}

// formatHash — то же что для массива.
func (w *writer) formatHash(e *ast.HashLiteral) {
	if len(e.KeyOrder) == 0 {
		w.write("{}")
		return
	}
	wasMultiline := false
	if len(e.KeyOrder) > 0 {
		firstLine := exprLine(e.KeyOrder[0])
		if firstLine > 0 && firstLine != e.Token.Line {
			wasMultiline = true
		}
	}

	if !wasMultiline {
		if inline := tryInlineHash(e); inline != "" && len(inline) <= MaxInlineWidth {
			w.write(inline)
			return
		}
	}

	w.write("{")
	w.nl()
	w.indent++
	for _, key := range e.KeyOrder {
		w.pad()
		w.formatExpression(key)
		w.write(": ")
		w.formatExpression(e.Pairs[key])
		w.write(",")
		w.nl()
	}
	w.indent--
	w.pad()
	w.write("}")
}

// exprLine возвращает номер строки токена выражения (для определения,
// был ли литерал многострочным в исходнике).
func exprLine(expr ast.Expression) int {
	switch e := expr.(type) {
	case *ast.Identifier:
		return e.Token.Line
	case *ast.StringLiteral:
		return e.Token.Line
	case *ast.IntegerLiteral:
		return e.Token.Line
	case *ast.FloatLiteral:
		return e.Token.Line
	case *ast.Boolean:
		return e.Token.Line
	case *ast.NullLiteral:
		return e.Token.Line
	case *ast.HashLiteral:
		return e.Token.Line
	case *ast.ArrayLiteral:
		return e.Token.Line
	case *ast.CallExpression:
		return e.Token.Line
	case *ast.InfixExpression:
		return e.Token.Line
	case *ast.PrefixExpression:
		return e.Token.Line
	case *ast.PipeExpression:
		return e.Token.Line
	case *ast.GroupedExpression:
		return e.Token.Line
	case *ast.IndexExpression:
		return e.Token.Line
	}
	return 0
}

// tryInlineArray пробует напечатать массив одной строкой. Если внутри
// что-то многострочное — возвращает "".
func tryInlineArray(e *ast.ArrayLiteral) string {
	w := &writer{}
	w.write("[")
	for i, el := range e.Elements {
		if i > 0 {
			w.write(", ")
		}
		w.formatExpression(el)
	}
	w.write("]")
	out := w.sb.String()
	if strings.Contains(out, "\n") {
		return ""
	}
	return out
}

// tryInlineHash пробует напечатать словарь одной строкой.
func tryInlineHash(e *ast.HashLiteral) string {
	w := &writer{}
	w.write("{")
	for i, key := range e.KeyOrder {
		if i > 0 {
			w.write(", ")
		}
		w.formatExpression(key)
		w.write(": ")
		w.formatExpression(e.Pairs[key])
	}
	w.write("}")
	out := w.sb.String()
	if strings.Contains(out, "\n") {
		return ""
	}
	return out
}

// tryFormatTestStmt распознаёт паттерн __тест__("имя", функция() ...)
// и печатает его как 'тест "имя" ... конец'. Возвращает true если
// успешно отформатировал.
func (w *writer) tryFormatTestStmt(e *ast.CallExpression) bool {
	if !w.isCallTo(e, "__тест__") {
		return false
	}
	if len(e.Arguments) != 2 {
		return false
	}
	name, ok := e.Arguments[0].(*ast.StringLiteral)
	if !ok {
		return false
	}
	fn, ok := e.Arguments[1].(*ast.FunctionLiteral)
	if !ok {
		return false
	}
	w.write("тест ")
	w.write(quote(name.Value))
	w.nl()
	w.formatBlock(fn.Body)
	w.pad()
	w.write("конец")
	return true
}

// tryFormatCheckStmt распознаёт __проверить__(expr) и печатает как
// 'проверить expr'. Возвращает true если успешно.
func (w *writer) tryFormatCheckStmt(e *ast.CallExpression) bool {
	if !w.isCallTo(e, "__проверить__") {
		return false
	}
	if len(e.Arguments) != 1 {
		return false
	}
	w.write("проверить ")
	w.formatExpression(e.Arguments[0])
	return true
}

// isCallTo проверяет, что вызов делается на функцию с указанным именем.
func (w *writer) isCallTo(e *ast.CallExpression, name string) bool {
	id, ok := e.Function.(*ast.Identifier)
	if !ok {
		return false
	}
	return id.Value == name
}
// Лексер маркирует строки с интерполяцией ведущим байтом \x00 —
// при печати его убираем. Внутри блоков {...} интерполяции
// кавычки и обратные слеши не экранируем (там код, а не литерал).
func quote(s string) string {
	hasInterpolation := strings.HasPrefix(s, "\x00")
	if hasInterpolation {
		s = s[1:]
	}
	var sb strings.Builder
	sb.WriteByte('"')
	braceDepth := 0
	for _, r := range s {
		// Внутри интерполяции отслеживаем баланс {} и не экранируем.
		if hasInterpolation {
			if r == '{' {
				braceDepth++
				sb.WriteRune(r)
				continue
			}
			if r == '}' && braceDepth > 0 {
				braceDepth--
				sb.WriteRune(r)
				continue
			}
			if braceDepth > 0 {
				sb.WriteRune(r)
				continue
			}
		}
		switch r {
		case '"':
			sb.WriteString(`\"`)
		case '\\':
			sb.WriteString(`\\`)
		case '\n':
			sb.WriteString(`\n`)
		case '\t':
			sb.WriteString(`\t`)
		case '\r':
			sb.WriteString(`\r`)
		default:
			sb.WriteRune(r)
		}
	}
	sb.WriteByte('"')
	return sb.String()
}
