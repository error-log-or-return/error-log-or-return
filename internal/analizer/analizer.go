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

func run(pass *analysis.Pass) (interface{}, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// Создаём кэш для путей файлов
	pathCache := make(map[string]string)

	// Фильтруем файлы согласно конфигурации
	filteredFilePaths := make(map[string]bool) // Для быстрого поиска по абсолютному пути

	for _, file := range pass.Files {
		filename := pass.Fset.Position(file.Pos()).Filename

		// Проверяем кэш путей
		var relPath string
		if cached, ok := pathCache[filename]; ok {
			relPath = cached
		} else {
			var err error
			relPath, err = filepath.Rel(basePath, filename)
			if err != nil {
				relPath = filename
			}
			// Нормализуем разделители путей для консистентности
			relPath = strings.ReplaceAll(relPath, "\\", "/")
			pathCache[filename] = relPath
		}

		// Пропускаем файлы согласно конфигурации из относительного пути
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

	nodeFilter := []ast.Node{(*ast.FuncDecl)(nil)}
	ins.Preorder(nodeFilter, func(n ast.Node) {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			return
		}

		// Проверяем, находится ли функция в одном из отфильтрованных файлов
		fnFile := pass.Fset.Position(fn.Pos()).Filename
		if !filteredFilePaths[fnFile] {
			return
		}
		if verbose {
			fmt.Fprintf(os.Stderr, "[DEBUG] Processing function at %s\n", fnFile)
		}

		// Проверяем, есть ли //nolint:error_log_or_return в комментариях к функции (doc или end-of-line)
		hasNoLint := false
		if fn.Doc != nil {
			for _, cg := range fn.Doc.List {
				if strings.Contains(cg.Text, nolint) {
					hasNoLint = true
					break
				}
			}
		}
		// Проверяем end-of-line комментарии на строке сигнатуры функции
		if !hasNoLint {
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
							hasNoLint = true
							break
						}
					}
					if hasNoLint {
						break
					}
				}
			}
		}
		if hasNoLint {
			return // Пропускаем функцию, если есть //nolint:error_log_or_return
		}

		// Определяем имя ресивера (если есть)
		var recvName string
		if fn.Recv != nil && len(fn.Recv.List) > 0 && len(fn.Recv.List[0].Names) > 0 {
			recvName = fn.Recv.List[0].Names[0].Name
		}
		if recvName == "" {
			return // Только методы с ресивером
		}

		// Проверяем, возвращает ли функция error
		returnsError := false
		if fn.Type.Results != nil {
			for _, res := range fn.Type.Results.List {
				if types.ExprString(res.Type) == "error" {
					returnsError = true
					break
				}
			}
		}

		hasErrVar := false

		// Проверяем объявления переменной err типа error только на уровне функции
		for _, stmt := range fn.Body.List {
			// var err error
			if decl, ok := stmt.(*ast.DeclStmt); ok {
				if gen, ok := decl.Decl.(*ast.GenDecl); ok && gen.Tok == token.VAR {
					for _, spec := range gen.Specs {
						if vs, ok := spec.(*ast.ValueSpec); ok {
							for i, name := range vs.Names {
								if name.Name == "err" {
									// Явно указан тип error
									if vs.Type != nil {
										if ident, ok := vs.Type.(*ast.Ident); ok && ident.Name == "error" {
											hasErrVar = true
										}
									} else if len(vs.Values) > i {
										// err := ... (определение через :=)
										typ := pass.TypesInfo.TypeOf(vs.Values[i])
										if typ != nil && typ.String() == "error" {
											hasErrVar = true
										}
									}
								}
							}
						}
					}
				}
			}
			// err := ... (короткое объявление на уровне функции)
			if assign, ok := stmt.(*ast.AssignStmt); ok && assign.Tok == token.DEFINE {
				for _, lhs := range assign.Lhs {
					if ident, ok := lhs.(*ast.Ident); ok && ident.Name == "err" {
						// Для множественного присваивания нужно проверить тип переменной напрямую
						if obj := pass.TypesInfo.Defs[ident]; obj != nil {
							if obj.Type().String() == "error" {
								hasErrVar = true
							}
						}
					}
				}
			}
		}

		// Проверяем, есть ли defer <ресивер>.log.ErrorOr*(&err, ...)
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
			// Проверяем, что это <ресивер>.log.ErrorOr*
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
			if !strings.HasPrefix(call.Sel.Name, "ErrorOr") && !strings.HasPrefix(call.Sel.Name, "Error") && !strings.HasPrefix(call.Sel.Name, "Debug") {
				return true
			}
			// Проверяем первый аргумент
			if len(deferStmt.Call.Args) > 0 {
				if starExpr, ok := deferStmt.Call.Args[0].(*ast.UnaryExpr); ok && starExpr.Op == token.AND {
					if ident, ok := starExpr.X.(*ast.Ident); ok && ident.Name == "err" {
						hasDeferWithErr = true
					}
				}
			}
			return true
		})

		// Нарушение: если функция не возвращает error, объявлен var err error, но нет defer с &err
		if !returnsError && hasErrVar {
			if !hasDeferWithErr {
				pass.Reportf(fn.Pos(), "есть err, нет defer, нет возврата error")
			}
		}
		// Нарушение: если функция возвращает error и есть defer с &err
		if returnsError && hasDeferWithErr {
			pass.Reportf(fn.Pos(), "возвращает error и есть defer с &err")
		}
	})
	return nil, nil
}

func Run() {
	singlechecker.Main(analyzer)
}
