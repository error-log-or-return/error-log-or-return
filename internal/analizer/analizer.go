package analizer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/analysis/singlechecker"
	"golang.org/x/tools/go/ast/inspector"
)

const name = "error_log_or_return"

var nolint = "//nolint:" + name

var analyzer = &analysis.Analyzer{
	Name: name,
	Doc:  "или логируем ошибку, или возвращаем",
	// шаблон для проверки: <receiver>.log.ErrorOr*(&err, ...)
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

// filterFiles фильтрует файлы согласно конфигурации
func filterFiles(pass *analysis.Pass) map[string]bool {
	pathCache := make(map[string]string)
	filteredFilePaths := make(map[string]bool)

	for _, file := range pass.Files {
		filename := pass.Fset.Position(file.Pos()).Filename

		var relPath string
		if cached, ok := pathCache[filename]; ok {
			relPath = cached
		} else {
			var err error
			relPath, err = filepath.Rel(basePath, filename)
			if err != nil {
				relPath = filename
			}
			relPath = strings.ReplaceAll(relPath, "\\", "/")
			pathCache[filename] = relPath
		}

		if cfg.ShouldIgnore(relPath) {
			if verbose {
				fmt.Fprintf(os.Stderr, "[DEBUG] Skipping file: %s\n", relPath)
			}
			continue
		}
		if verbose {
			fmt.Fprintf(os.Stderr, "[DEBUG] File: %s\n", relPath)
		}

		filteredFilePaths[filename] = true
	}

	return filteredFilePaths
}

// hasNoLintComment проверяет наличие комментария //nolint:error_log_or_return
func hasNoLintComment(fn *ast.FuncDecl, pass *analysis.Pass) bool {
	if fn.Doc != nil {
		for _, cg := range fn.Doc.List {
			if strings.Contains(cg.Text, nolint) {
				return true
			}
		}
	}

	var astFile *ast.File
	for _, f := range pass.Files {
		if fn.Pos() >= f.Pos() && fn.Pos() <= f.End() {
			astFile = f
			break
		}
	}
	if astFile != nil {
		file := pass.Fset.File(fn.Pos())
		funcLine := file.Line(fn.Pos())
		for _, cg := range astFile.Comments {
			for _, c := range cg.List {
				commentLine := file.Line(c.Pos())
				if commentLine == funcLine && strings.Contains(c.Text, nolint) {
					return true
				}
			}
		}
	}

	return false
}

// getReceiverName получает имя ресивера функции
func getReceiverName(fn *ast.FuncDecl) string {
	if fn.Recv != nil && len(fn.Recv.List) > 0 && len(fn.Recv.List[0].Names) > 0 {
		return fn.Recv.List[0].Names[0].Name
	}
	return ""
}

// returnsError проверяет, возвращает ли функция error
func returnsError(fn *ast.FuncDecl) bool {
	if fn.Type.Results == nil {
		return false
	}
	for _, res := range fn.Type.Results.List {
		if types.ExprString(res.Type) == "error" {
			return true
		}
	}
	return false
}

// hasErrorVariable проверяет наличие переменной err типа error на уровне функции
func hasErrorVariable(fn *ast.FuncDecl, pass *analysis.Pass) bool {
	for _, stmt := range fn.Body.List {
		if hasErrorInDeclStmt(stmt, pass) || hasErrorInAssignStmt(stmt, pass) {
			return true
		}
	}
	return false
}

// hasErrorInDeclStmt проверяет объявление var err error
func hasErrorInDeclStmt(stmt ast.Stmt, pass *analysis.Pass) bool {
	decl, ok := stmt.(*ast.DeclStmt)
	if !ok {
		return false
	}

	gen, ok := decl.Decl.(*ast.GenDecl)
	if !ok || gen.Tok != token.VAR {
		return false
	}

	for _, spec := range gen.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}

		for i, name := range vs.Names {
			if name.Name != "err" {
				continue
			}

			if vs.Type != nil {
				if ident, ok := vs.Type.(*ast.Ident); ok && ident.Name == "error" {
					return true
				}
			} else if len(vs.Values) > i {
				typ := pass.TypesInfo.TypeOf(vs.Values[i])
				if typ != nil && typ.String() == "error" {
					return true
				}
			}
		}
	}
	return false
}

// hasErrorInAssignStmt проверяет короткое объявление err := ...
func hasErrorInAssignStmt(stmt ast.Stmt, pass *analysis.Pass) bool {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || assign.Tok != token.DEFINE {
		return false
	}

	for _, lhs := range assign.Lhs {
		ident, ok := lhs.(*ast.Ident)
		if !ok || ident.Name != "err" {
			continue
		}

		if obj := pass.TypesInfo.Defs[ident]; obj != nil {
			if obj.Type().String() == "error" {
				return true
			}
		}
	}
	return false
}

// hasDeferWithErrorRef проверяет наличие defer с &err
func hasDeferWithErrorRef(fn *ast.FuncDecl, recvName string) bool {
	hasDeferWithErr := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		deferStmt, ok := n.(*ast.DeferStmt)
		if !ok {
			return true
		}

		call, ok := deferStmt.Call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		xSel, ok := call.X.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		recvIdent, ok := xSel.X.(*ast.Ident)
		if !ok || recvIdent.Name != recvName {
			return true
		}

		if xSel.Sel.Name != "log" {
			return true
		}

		if !strings.HasPrefix(call.Sel.Name, "ErrorOr") &&
			!strings.HasPrefix(call.Sel.Name, "Error") &&
			!strings.HasPrefix(call.Sel.Name, "Debug") {
			return true
		}

		if len(deferStmt.Call.Args) > 0 {
			if starExpr, ok := deferStmt.Call.Args[0].(*ast.UnaryExpr); ok && starExpr.Op == token.AND {
				if ident, ok := starExpr.X.(*ast.Ident); ok && ident.Name == "err" {
					hasDeferWithErr = true
				}
			}
		}
		return true
	})
	return hasDeferWithErr
}

// checkFunctionViolations проверяет нарушения правил для функции
func checkFunctionViolations(fn *ast.FuncDecl, pass *analysis.Pass, filteredFilePaths map[string]bool) {
	fnFile := pass.Fset.Position(fn.Pos()).Filename
	if !filteredFilePaths[fnFile] {
		return
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] Processing function at %s\n", fnFile)
	}

	if hasNoLintComment(fn, pass) {
		return
	}

	recvName := getReceiverName(fn)
	if recvName == "" {
		return // Только методы с ресивером
	}

	returnsErr := returnsError(fn)
	hasErrVar := hasErrorVariable(fn, pass)
	hasDeferWithErr := hasDeferWithErrorRef(fn, recvName)

	// Нарушение: если функция не возвращает error, объявлен var err error, но нет defer с &err
	if !returnsErr && hasErrVar && !hasDeferWithErr {
		pass.Reportf(fn.Pos(), "есть err, нет defer, нет возврата error")
	}

	// Нарушение: если функция возвращает error и есть defer с &err
	if returnsErr && hasDeferWithErr {
		pass.Reportf(fn.Pos(), "возвращает error и есть defer с &err")
	}
}

func run(pass *analysis.Pass) (interface{}, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	filteredFilePaths := filterFiles(pass)

	nodeFilter := []ast.Node{(*ast.FuncDecl)(nil)}
	ins.Preorder(nodeFilter, func(n ast.Node) {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			return
		}

		checkFunctionViolations(fn, pass, filteredFilePaths)
	})

	return nil, nil
}

func Run() {
	singlechecker.Main(analyzer)
}
