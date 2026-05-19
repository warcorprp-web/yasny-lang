package interpreter

import (
	"fmt"
	"os"
	"sort"

	"yasny-lang/ast"
	"yasny-lang/lexer"
	"yasny-lang/parser"
)

// evalLoad — встроенная функция загрузить("путь.ya"): читает файл и
// выполняет его в текущем окружении.
func evalLoad(tok lexer.Token, path string, env *Environment) Object {
	content, err := os.ReadFile(path)
	if err != nil {
		return ErrorFileNotFound(tok, path)
	}

	l := lexer.NewWithFilename(string(content), path)
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) != 0 {
		errMsg := fmt.Sprintf("при загрузке '%s':", path)
		for _, msg := range p.Errors() {
			errMsg += "\n  " + msg
		}
		return &Error{
			Message: fmt.Sprintf("❌ ОШИБКА %s", errMsg),
			Line:    tok.Line,
			Column:  tok.Column,
		}
	}

	return Eval(program, env)
}

// evalImportStatement выполняет: импорт ИМЯ из "путь.ya" [как АЛИАС].
// Создаёт изолированное окружение, выполняет файл, собирает экспорты
// в словарь и связывает его с именем модуля в текущем окружении.
func evalImportStatement(node *ast.ImportStatement, env *Environment) Object {
	path := node.Path

	content, err := os.ReadFile(path)
	if err != nil {
		return newErrorWithToken(node.Token, "не удалось прочитать файл: %s", err.Error())
	}

	l := lexer.NewWithFilename(string(content), path)
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		return &Error{
			Message: fmt.Sprintf("❌ ОШИБКА при импорте из '%s':\n  %s", path, p.Errors()[0]),
			Line:    node.Token.Line,
			Column:  node.Token.Column,
		}
	}

	moduleEnv := NewEnvironment()
	moduleEnv.outer = env

	result := Eval(program, moduleEnv)
	if isError(result) {
		return result
	}

	moduleObj := NewHash()

	// Сортируем имена, чтобы порядок ключей в собранном модуле
	// был стабильным.
	exportNames := make([]string, 0, len(moduleEnv.exports))
	for k := range moduleEnv.exports {
		exportNames = append(exportNames, k)
	}
	sort.Strings(exportNames)
	for _, k := range exportNames {
		moduleObj.Set(&String{Value: k}, moduleEnv.exports[k])
	}

	moduleName := node.Name.Value
	if node.Alias != nil {
		moduleName = node.Alias.Value
	}
	env.Set(moduleName, moduleObj)

	return moduleObj
}

// evalExportStatement помечает объявление как экспортируемое.
func evalExportStatement(node *ast.ExportStatement, env *Environment) Object {
	result := Eval(node.Statement, env)
	if isError(result) {
		return result
	}

	switch stmt := node.Statement.(type) {
	case *ast.LetStatement:
		env.Export(stmt.Name.Value)
	case *ast.ExpressionStatement:
		if fn, ok := stmt.Expression.(*ast.FunctionLiteral); ok {
			if fn.Name != nil {
				env.Export(fn.Name.Value)
			}
		}
	}

	return result
}
