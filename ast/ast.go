package ast

import "yasny-lang/lexer"

// Node - базовый интерфейс для всех узлов AST
type Node interface {
	TokenLiteral() string
	GetToken() lexer.Token
}

// Statement - узел-выражение (не возвращает значение)
type Statement interface {
	Node
	statementNode()
}

// Expression - узел-выражение (возвращает значение)
type Expression interface {
	Node
	expressionNode()
}

// Program - корневой узел программы
type Program struct {
	Statements []Statement
}

func (p *Program) TokenLiteral() string {
	if len(p.Statements) > 0 {
		return p.Statements[0].TokenLiteral()
	}
	return ""
}

// ImportStatement - импорт модуль из "файл.pr"
type ImportStatement struct {
	Token    lexer.Token // токен "импорт"
	Name     *Identifier // имя модуля
	Path     string      // путь к файлу
	Alias    *Identifier // алиас (опционально)
}

func (is *ImportStatement) statementNode()       {}
func (is *ImportStatement) TokenLiteral() string { return is.Token.Literal }
func (is *ImportStatement) GetToken() lexer.Token { return is.Token }

// ExportStatement - экспорт функция/переменная
type ExportStatement struct {
	Token     lexer.Token // токен "экспорт"
	Statement Statement   // что экспортируем
}

func (es *ExportStatement) statementNode()       {}
func (es *ExportStatement) TokenLiteral() string { return es.Token.Literal }
func (es *ExportStatement) GetToken() lexer.Token { return es.Token }

// LetStatement - пусть имя = значение
type LetStatement struct {
	Token lexer.Token // токен "пусть"
	Name  *Identifier
	Value Expression
}

func (ls *LetStatement) statementNode()       {}
func (ls *LetStatement) TokenLiteral() string { return ls.Token.Literal }
func (ls *LetStatement) GetToken() lexer.Token { return ls.Token }

// DestructuringStatement - конст [a, b] = массив или конст {x, y} = объект
type DestructuringStatement struct {
	Token   lexer.Token // токен "конст"
	Pattern Expression  // ArrayLiteral или HashLiteral с идентификаторами
	Value   Expression
}

func (ds *DestructuringStatement) statementNode()       {}
func (ds *DestructuringStatement) TokenLiteral() string { return ds.Token.Literal }

// VarStatement - var имя = значение
type VarStatement struct {
	Token lexer.Token
	Name  *Identifier
	Value Expression
}

func (vs *VarStatement) statementNode()       {}
func (vs *VarStatement) TokenLiteral() string { return vs.Token.Literal }

// AssignmentStatement - имя = значение или obj.field = значение
type AssignmentStatement struct {
	Token    lexer.Token // токен идентификатора
	Left     Expression  // может быть Identifier или IndexExpression (для obj.field)
	Operator string      // "=", "+=", "-=", "*=", "/="
	Value    Expression
}

func (as *AssignmentStatement) statementNode()       {}
func (as *AssignmentStatement) TokenLiteral() string { return as.Token.Literal }

// ReturnStatement - вернуть значение
type ReturnStatement struct {
	Token       lexer.Token
	ReturnValue Expression
}

func (rs *ReturnStatement) statementNode()       {}
func (rs *ReturnStatement) TokenLiteral() string { return rs.Token.Literal }

// YieldStatement - выдать значение из генератора
type YieldStatement struct {
	Token lexer.Token
	Value Expression
}

func (ys *YieldStatement) statementNode()       {}
func (ys *YieldStatement) TokenLiteral() string { return ys.Token.Literal }

// ExpressionStatement - выражение как statement
type ExpressionStatement struct {
	Token      lexer.Token
	Expression Expression
}

func (es *ExpressionStatement) statementNode()       {}
func (es *ExpressionStatement) TokenLiteral() string { return es.Token.Literal }

// BlockStatement - блок кода
type BlockStatement struct {
	Token      lexer.Token
	Statements []Statement
}

func (bs *BlockStatement) statementNode()       {}
func (bs *BlockStatement) TokenLiteral() string { return bs.Token.Literal }

// Identifier - имя переменной/функции
type Identifier struct {
	Token lexer.Token
	Value string
}

func (i *Identifier) expressionNode()      {}
func (i *Identifier) TokenLiteral() string { return i.Token.Literal }

// IntegerLiteral - целое число
type IntegerLiteral struct {
	Token lexer.Token
	Value int64
}

func (il *IntegerLiteral) expressionNode()      {}
func (il *IntegerLiteral) TokenLiteral() string { return il.Token.Literal }

// FloatLiteral - дробное число
type FloatLiteral struct {
	Token lexer.Token
	Value float64
}

func (fl *FloatLiteral) expressionNode()      {}
func (fl *FloatLiteral) TokenLiteral() string { return fl.Token.Literal }

// StringLiteral - строка
type StringLiteral struct {
	Token lexer.Token
	Value string
}

func (sl *StringLiteral) expressionNode()      {}
func (sl *StringLiteral) TokenLiteral() string { return sl.Token.Literal }

// Boolean - да/нет
type Boolean struct {
	Token lexer.Token
	Value bool
}

func (b *Boolean) expressionNode()      {}
func (b *Boolean) TokenLiteral() string { return b.Token.Literal }

// NullLiteral - ничего
type NullLiteral struct {
	Token lexer.Token
}

func (n *NullLiteral) expressionNode()      {}
func (n *NullLiteral) TokenLiteral() string { return n.Token.Literal }
func (n *NullLiteral) GetToken() lexer.Token { return n.Token }

// PrefixExpression - унарный оператор (не x, -5)
type PrefixExpression struct {
	Token    lexer.Token
	Operator string
	Right    Expression
}

func (pe *PrefixExpression) expressionNode()      {}
func (pe *PrefixExpression) TokenLiteral() string { return pe.Token.Literal }

// InfixExpression - бинарный оператор (x + y)
type InfixExpression struct {
	Token    lexer.Token
	Left     Expression
	Operator string
	Right    Expression
}

func (ie *InfixExpression) expressionNode()      {}
func (ie *InfixExpression) TokenLiteral() string { return ie.Token.Literal }

// IfExpression - если условие ... иначе ...
type IfExpression struct {
	Token       lexer.Token
	Condition   Expression
	Consequence *BlockStatement
	Alternative *BlockStatement
}

func (ie *IfExpression) expressionNode()      {}
func (ie *IfExpression) TokenLiteral() string { return ie.Token.Literal }

// MatchExpression - совпадение значение когда ... конец
type MatchExpression struct {
	Token lexer.Token // токен "совпадение"
	Value Expression
	Cases []*MatchCase
}

func (me *MatchExpression) expressionNode()      {}
func (me *MatchExpression) TokenLiteral() string { return me.Token.Literal }

// MatchCase - когда паттерн => результат
type MatchCase struct {
	Token   lexer.Token // токен "когда"
	Pattern Expression  // паттерн для сравнения (или "иначе" для default)
	Result  Expression
}

// FunctionLiteral - функция(параметры) ... конец
type FunctionLiteral struct {
	Token      lexer.Token
	Name       *Identifier
	Parameters []*Identifier
	Defaults   []Expression // дефолтные значения параметров (nil если нет)
	HasRest    bool         // последний параметр - rest (...args)
	Body       *BlockStatement
}

func (fl *FunctionLiteral) expressionNode()      {}
func (fl *FunctionLiteral) TokenLiteral() string { return fl.Token.Literal }

// CallExpression - вызов функции
type CallExpression struct {
	Token     lexer.Token
	Function  Expression
	Arguments []Expression
}

func (ce *CallExpression) expressionNode()      {}
func (ce *CallExpression) TokenLiteral() string { return ce.Token.Literal }

// ArrayLiteral - [1, 2, 3]
type ArrayLiteral struct {
	Token    lexer.Token
	Elements []Expression
}

func (al *ArrayLiteral) expressionNode()      {}
func (al *ArrayLiteral) TokenLiteral() string { return al.Token.Literal }

// IndexExpression - массив[индекс]
type IndexExpression struct {
	Token    lexer.Token
	Left     Expression
	Index    Expression
	Optional bool // true для ?.[индекс]
}

func (ie *IndexExpression) expressionNode()      {}
func (ie *IndexExpression) TokenLiteral() string { return ie.Token.Literal }

// SliceExpression - массив[начало:конец] / строка[1:5]
// Любая граница может быть nil (open-ended): [:5], [2:], [:]
type SliceExpression struct {
	Token lexer.Token
	Left  Expression
	Start Expression // может быть nil
	End   Expression // может быть nil
}

func (se *SliceExpression) expressionNode()      {}
func (se *SliceExpression) TokenLiteral() string { return se.Token.Literal }

// AsyncExpression - асинх <выражение>: запускает в горутине, возвращает Future
type AsyncExpression struct {
	Token lexer.Token
	Body  Expression
}

func (ae *AsyncExpression) expressionNode()      {}
func (ae *AsyncExpression) TokenLiteral() string { return ae.Token.Literal }

// AwaitExpression - ждать <Future>: ждёт результат
type AwaitExpression struct {
	Token lexer.Token
	Body  Expression
}

func (ae *AwaitExpression) expressionNode()      {}
func (ae *AwaitExpression) TokenLiteral() string { return ae.Token.Literal }

// OptionalExpression - obj?.field
type OptionalExpression struct {
	Token lexer.Token // токен ?.
	Left  Expression
	Right Expression
}

func (oe *OptionalExpression) expressionNode()      {}
func (oe *OptionalExpression) TokenLiteral() string { return oe.Token.Literal }

// ForExpression - для i от 1 до 10 [по N]
type ForExpression struct {
	Token     lexer.Token
	Variable  *Identifier
	From      Expression
	To        Expression
	Step      Expression // опциональный шаг (по N), может быть nil
	Body      *BlockStatement
}

func (fe *ForExpression) expressionNode()      {}
func (fe *ForExpression) TokenLiteral() string { return fe.Token.Literal }

// ForInExpression - для элемент в массив
type ForInExpression struct {
	Token    lexer.Token
	Index    *Identifier // опционально - для "для i, элемент в массив"
	Variable *Identifier
	Iterable Expression
	Body     *BlockStatement
}

func (fie *ForInExpression) expressionNode()      {}
func (fie *ForInExpression) TokenLiteral() string { return fie.Token.Literal }

// WhileExpression - пока условие
type WhileExpression struct {
	Token     lexer.Token
	Condition Expression
	Body      *BlockStatement
}

func (we *WhileExpression) expressionNode()      {}
func (we *WhileExpression) TokenLiteral() string { return we.Token.Literal }

// HashLiteral - словарь {"ключ": значение}.
// Pairs хранит соответствие, KeyOrder — порядок появления в исходнике
// (нужен, чтобы при выводе словаря ключи были в порядке вставки).
type HashLiteral struct {
	Token    lexer.Token
	Pairs    map[Expression]Expression
	KeyOrder []Expression
}

func (hl *HashLiteral) expressionNode()      {}
func (hl *HashLiteral) TokenLiteral() string { return hl.Token.Literal }

// SpreadExpression - ...массив
type SpreadExpression struct {
	Token lexer.Token
	Value Expression
}

func (se *SpreadExpression) expressionNode()      {}
func (se *SpreadExpression) TokenLiteral() string { return se.Token.Literal }
func (se *SpreadExpression) GetToken() lexer.Token { return se.Token }


// TryExpression - попытка ... поймать ...
type TryExpression struct {
	Token       lexer.Token
	Body        *BlockStatement
	CatchVar    *Identifier
	CatchBody   *BlockStatement
	FinallyBody *BlockStatement
}

func (te *TryExpression) expressionNode()      {}
func (te *TryExpression) TokenLiteral() string { return te.Token.Literal }

// ThrowStatement - бросить ошибку
type ThrowStatement struct {
	Token lexer.Token
	Value Expression
}

func (ts *ThrowStatement) statementNode()       {}
func (ts *ThrowStatement) TokenLiteral() string { return ts.Token.Literal }

// BreakStatement - прервать цикл
type BreakStatement struct {
	Token lexer.Token
}

func (bs *BreakStatement) statementNode()       {}
func (bs *BreakStatement) TokenLiteral() string { return bs.Token.Literal }

// ContinueStatement - продолжить цикл
type ContinueStatement struct {
	Token lexer.Token
}

func (cs *ContinueStatement) statementNode()       {}
func (cs *ContinueStatement) TokenLiteral() string { return cs.Token.Literal }

// NewExpression - новый Класс(...)
type NewExpression struct {
	Token     lexer.Token
	ClassName *Identifier
	Arguments []Expression
}

func (ne *NewExpression) expressionNode()      {}
func (ne *NewExpression) TokenLiteral() string { return ne.Token.Literal }

// TernaryExpression - условие ? да : нет
type TernaryExpression struct {
	Token       lexer.Token
	Condition   Expression
	Consequence Expression
	Alternative Expression
}

func (te *TernaryExpression) expressionNode()      {}
func (te *TernaryExpression) TokenLiteral() string { return te.Token.Literal }


// GetToken implementations for all node types
func (p *Program) GetToken() lexer.Token {
	if len(p.Statements) > 0 {
		return p.Statements[0].GetToken()
	}
	return lexer.Token{}
}
func (vs *VarStatement) GetToken() lexer.Token         { return vs.Token }
func (ds *DestructuringStatement) GetToken() lexer.Token { return ds.Token }
func (as *AssignmentStatement) GetToken() lexer.Token  { return as.Token }
func (rs *ReturnStatement) GetToken() lexer.Token      { return rs.Token }
func (es *ExpressionStatement) GetToken() lexer.Token  { return es.Token }
func (bs *BlockStatement) GetToken() lexer.Token       { return bs.Token }
func (i *Identifier) GetToken() lexer.Token            { return i.Token }
func (il *IntegerLiteral) GetToken() lexer.Token       { return il.Token }
func (fl *FloatLiteral) GetToken() lexer.Token         { return fl.Token }
func (sl *StringLiteral) GetToken() lexer.Token        { return sl.Token }
func (b *Boolean) GetToken() lexer.Token               { return b.Token }
func (pe *PrefixExpression) GetToken() lexer.Token     { return pe.Token }
func (ie *InfixExpression) GetToken() lexer.Token      { return ie.Token }
func (ife *IfExpression) GetToken() lexer.Token        { return ife.Token }
func (me *MatchExpression) GetToken() lexer.Token      { return me.Token }
func (fl *FunctionLiteral) GetToken() lexer.Token      { return fl.Token }
func (ce *CallExpression) GetToken() lexer.Token       { return ce.Token }
func (al *ArrayLiteral) GetToken() lexer.Token         { return al.Token }
func (ie *IndexExpression) GetToken() lexer.Token      { return ie.Token }
func (se *SliceExpression) GetToken() lexer.Token      { return se.Token }
func (ys *YieldStatement) GetToken() lexer.Token       { return ys.Token }
func (ae *AsyncExpression) GetToken() lexer.Token      { return ae.Token }
func (awe *AwaitExpression) GetToken() lexer.Token     { return awe.Token }
func (oe *OptionalExpression) GetToken() lexer.Token   { return oe.Token }
func (fe *ForExpression) GetToken() lexer.Token        { return fe.Token }
func (fie *ForInExpression) GetToken() lexer.Token     { return fie.Token }
func (we *WhileExpression) GetToken() lexer.Token      { return we.Token }
func (hl *HashLiteral) GetToken() lexer.Token          { return hl.Token }
func (te *TryExpression) GetToken() lexer.Token        { return te.Token }
func (ts *ThrowStatement) GetToken() lexer.Token       { return ts.Token }
func (bs *BreakStatement) GetToken() lexer.Token       { return bs.Token }
func (cs *ContinueStatement) GetToken() lexer.Token    { return cs.Token }
func (ne *NewExpression) GetToken() lexer.Token        { return ne.Token }
func (te *TernaryExpression) GetToken() lexer.Token    { return te.Token }
func (re *RangeExpression) GetToken() lexer.Token      { return re.Token }
func (ac *ArrayComprehension) GetToken() lexer.Token   { return ac.Token }

// RangeExpression - диапазон start..end
type RangeExpression struct {
	Token lexer.Token
	Start Expression
	End   Expression
}

func (re *RangeExpression) expressionNode()      {}
func (re *RangeExpression) TokenLiteral() string { return re.Token.Literal }

// ArrayComprehension - [выражение для переменная в итератор если условие]
type ArrayComprehension struct {
	Token     lexer.Token
	Element   Expression  // выражение для каждого элемента
	Variable  *Identifier // переменная цикла
	Iterable  Expression  // по чему итерируемся
	Condition Expression  // опциональное условие (если)
}

func (ac *ArrayComprehension) expressionNode()      {}
func (ac *ArrayComprehension) TokenLiteral() string { return ac.Token.Literal }
