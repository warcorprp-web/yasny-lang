package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"yasny-lang/ast"
	"yasny-lang/lexer"
	"yasny-lang/parser"

	"github.com/sourcegraph/jsonrpc2"
)

// Server — LSP-сервер для языка Ясный.
type Server struct {
	conn      *jsonrpc2.Conn
	docs      map[string]*Document // URI → документ
	mu        sync.Mutex
	rootURI   string
	shutdown  bool
}

// Document — открытый файл с его AST и диагностикой.
type Document struct {
	URI     string
	Content string
	Version int
	Program *ast.Program
	Errors  []string
	Symbols []Symbol
}

// Symbol — определение в файле.
type Symbol struct {
	Name   string
	Kind   int // SymbolKind из LSP
	Line   int // 0-based
	Col    int
	Detail string
}

const (
	SymbolFunction = 12
	SymbolVariable = 13
	SymbolClass    = 5
	SymbolConstant = 14
	SymbolModule   = 2
)

// Run запускает LSP-сервер на stdin/stdout.
func Run() {
	s := &Server{docs: make(map[string]*Document)}
	ctx := context.Background()
	stream := jsonrpc2.NewBufferedStream(stdinoutStream{}, jsonrpc2.VSCodeObjectCodec{})
	conn := jsonrpc2.NewConn(ctx, stream, jsonrpc2.HandlerWithError(s.handle))
	s.conn = conn
	<-conn.DisconnectNotify()
}

func (s *Server) handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) (interface{}, error) {
	switch req.Method {
	case "initialize":
		return s.initialize(req)
	case "initialized":
		return nil, nil
	case "shutdown":
		s.shutdown = true
		return nil, nil
	case "exit":
		os.Exit(0)
		return nil, nil
	case "textDocument/didOpen":
		s.didOpen(req)
		return nil, nil
	case "textDocument/didChange":
		s.didChange(req)
		return nil, nil
	case "textDocument/didClose":
		s.didClose(req)
		return nil, nil
	case "textDocument/completion":
		return s.completion(req)
	case "textDocument/hover":
		return s.hover(req)
	case "textDocument/definition":
		return s.definition(req)
	case "textDocument/documentSymbol":
		return s.documentSymbol(req)
	case "textDocument/formatting":
		return s.formatting(req)
	case "textDocument/references":
		return s.references(req)
	case "textDocument/rename":
		return s.rename(req)
	case "textDocument/signatureHelp":
		return s.signatureHelp(req)
	}
	return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeMethodNotFound, Message: "method not found: " + req.Method}
}

func (s *Server) initialize(req *jsonrpc2.Request) (interface{}, error) {
	var params struct {
		RootURI string `json:"rootUri"`
	}
	json.Unmarshal(*req.Params, &params)
	s.rootURI = params.RootURI

	return map[string]interface{}{
		"capabilities": map[string]interface{}{
			"textDocumentSync": map[string]interface{}{
				"openClose": true,
				"change":    1, // Full sync
			},
			"completionProvider": map[string]interface{}{
				"triggerCharacters": []string{".", "("},
				"resolveProvider":   false,
			},
			"hoverProvider":            true,
			"definitionProvider":       true,
			"referencesProvider":       true,
			"documentSymbolProvider":   true,
			"renameProvider":           true,
			"signatureHelpProvider":    map[string]interface{}{"triggerCharacters": []string{"(", ","}},
			"documentFormattingProvider": true,
		},
	}, nil
}

// === Document sync ===

func (s *Server) didOpen(req *jsonrpc2.Request) {
	var params struct {
		TextDocument struct {
			URI     string `json:"uri"`
			Text    string `json:"text"`
			Version int    `json:"version"`
		} `json:"textDocument"`
	}
	json.Unmarshal(*req.Params, &params)
	s.mu.Lock()
	doc := &Document{
		URI:     params.TextDocument.URI,
		Content: params.TextDocument.Text,
		Version: params.TextDocument.Version,
	}
	s.docs[params.TextDocument.URI] = doc
	s.mu.Unlock()
	s.analyzeAndPublish(doc)
}

func (s *Server) didChange(req *jsonrpc2.Request) {
	var params struct {
		TextDocument struct {
			URI     string `json:"uri"`
			Version int    `json:"version"`
		} `json:"textDocument"`
		ContentChanges []struct {
			Text string `json:"text"`
		} `json:"contentChanges"`
	}
	json.Unmarshal(*req.Params, &params)
	s.mu.Lock()
	doc, ok := s.docs[params.TextDocument.URI]
	if !ok {
		doc = &Document{URI: params.TextDocument.URI}
		s.docs[params.TextDocument.URI] = doc
	}
	if len(params.ContentChanges) > 0 {
		doc.Content = params.ContentChanges[len(params.ContentChanges)-1].Text
		doc.Version = params.TextDocument.Version
	}
	s.mu.Unlock()
	s.analyzeAndPublish(doc)
}

func (s *Server) didClose(req *jsonrpc2.Request) {
	var params struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	json.Unmarshal(*req.Params, &params)
	s.mu.Lock()
	delete(s.docs, params.TextDocument.URI)
	s.mu.Unlock()
	// Clear diagnostics
	s.conn.Notify(context.Background(), "textDocument/publishDiagnostics", map[string]interface{}{
		"uri":         params.TextDocument.URI,
		"diagnostics": []interface{}{},
	})
}

// === Analysis ===

func (s *Server) analyzeAndPublish(doc *Document) {
	l := lexer.NewWithFilename(doc.Content, uriToPath(doc.URI))
	p := parser.New(l)
	program := p.ParseProgram()

	doc.Program = program
	doc.Errors = p.Errors()
	doc.Symbols = extractSymbols(program)

	// Publish diagnostics
	diags := []interface{}{}
	for _, e := range doc.Errors {
		line := 0
		// Try to extract line number from error
		if idx := strings.Index(e, "строке "); idx >= 0 {
			fmt.Sscanf(e[idx+len("строке "):], "%d", &line)
			if line > 0 {
				line--
			}
		}
		diags = append(diags, map[string]interface{}{
			"range": makeRange(line, 0, line, 100),
			"severity": 1, // Error
			"source":   "yasny",
			"message":  e,
		})
	}
	s.conn.Notify(context.Background(), "textDocument/publishDiagnostics", map[string]interface{}{
		"uri":         doc.URI,
		"diagnostics": diags,
	})
}

func extractSymbols(program *ast.Program) []Symbol {
	if program == nil {
		return nil
	}
	var symbols []Symbol
	for _, stmt := range program.Statements {
		switch s := stmt.(type) {
		case *ast.LetStatement:
			if s.Name == nil {
				continue
			}
			kind := SymbolConstant
			detail := "конст"
			if s.Token.Type == lexer.CLASS {
				kind = SymbolClass
				detail = "класс"
			} else if _, ok := s.Value.(*ast.FunctionLiteral); ok {
				kind = SymbolFunction
				detail = "функция"
			}
			symbols = append(symbols, Symbol{
				Name: s.Name.Value, Kind: kind,
				Line: s.Token.Line - 1, Col: 0, Detail: detail,
			})
		case *ast.VarStatement:
			symbols = append(symbols, Symbol{
				Name: s.Name.Value, Kind: SymbolVariable,
				Line: s.Token.Line - 1, Col: 0, Detail: "перем",
			})
		case *ast.ImportStatement:
			if s.Name != nil {
				symbols = append(symbols, Symbol{
					Name: s.Name.Value, Kind: SymbolModule,
					Line: s.Token.Line - 1, Col: 0, Detail: "импорт из \"" + s.Path + "\"",
				})
			}
		case *ast.ExpressionStatement:
			if fl, ok := s.Expression.(*ast.FunctionLiteral); ok && fl.Name != nil {
				symbols = append(symbols, Symbol{
					Name: fl.Name.Value, Kind: SymbolFunction,
					Line: s.Token.Line - 1, Col: 0, Detail: "функция",
				})
			}
		}
	}
	return symbols
}

// === Helpers ===

func makeRange(startLine, startChar, endLine, endChar int) map[string]interface{} {
	return map[string]interface{}{
		"start": map[string]interface{}{"line": startLine, "character": startChar},
		"end":   map[string]interface{}{"line": endLine, "character": endChar},
	}
}

func uriToPath(uri string) string {
	if strings.HasPrefix(uri, "file://") {
		return strings.TrimPrefix(uri, "file://")
	}
	return uri
}

// stdinoutStream implements jsonrpc2.ObjectStream over stdin/stdout.
type stdinoutStream struct{}

func (s stdinoutStream) Read(p []byte) (int, error)  { return os.Stdin.Read(p) }
func (s stdinoutStream) Write(p []byte) (int, error) { return os.Stdout.Write(p) }
func (s stdinoutStream) Close() error                { return nil }
