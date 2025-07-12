# error-log-or-return

[![Go Version](https://img.shields.io/github/go-mod/go-version/error-log-or-return/error-log-or-return)](https://go.dev/doc/install)
[![Go Report Card](https://goreportcard.com/badge/github.com/error-log-or-return/error-log-or-return)](https://goreportcard.com/report/github.com/error-log-or-return/error-log-or-return)
[![coverage](https://img.shields.io/badge/coverage-75.4%25-brightgreen)](https://htmlpreview.github.io/?https://github.com/error-log-or-return/error-log-or-return/blob/main/.coverage/.html)
[![Last Commit](https://img.shields.io/github/last-commit/error-log-or-return/error-log-or-return)](https://github.com/error-log-or-return/error-log-or-return/commits/main/)
[![Project Status](https://img.shields.io/github/release/error-log-or-return/error-log-or-return.svg)](https://github.com/error-log-or-return/error-log-or-return/releases/latest)

> 🚀 Golang linter: или логируем ошибку, или возвращаем. 

## 🎯 Overview

`error-log-or-return` is a **powerful static analysis tool**. Built on top of [golang.org/x/tools/go/analysis](https://pkg.go.dev/golang.org/x/tools/go/analysis), it seamlessly integrates with your development workflow. 

## 🤔 Problem Example

```go
func (s *Service) GetStatus() error {
  var err error
  defer s.log.ErrorOrDebug(&err, "")
  return err
}
```

- Если функция возвращает ошибку (error) — она НЕ должна логировать ошибку внутри.
- Если функция НЕ возвращает ошибку, но внутри есть обработка ошибки — она ДОЛЖНА логировать ошибку.
- Если функция не возвращает ошибку и не обрабатывает ошибку — всё ОК.
- Если функция возвращает ошибку и не логирует — всё ОК.

## 🧠 Эволюция

Правило выработано постепенно:
- [Прощай error-hell: альтернативная обработка ошибок](https://habr.com/ru/articles/912150/)
- [Соглашение по обработке ошибок](https://habr.com/ru/articles/912788/)
- [Структурированные логи + локальный стек вызовов: эволюция обработки ошибок в Go](https://habr.com/ru/articles/915660/)

🧪 Пример применения - [пет-проект](https://github.com/comerc/budva43).

## ✨ Features

- 📊 **Clean Output**: Sorted by file path and line numbers
- 🔌 **Editor Integration**: Works with `go vet`, `gopls`, and your favorite IDE
- 🌍 **Cross-Platform**: Full support for Windows, Linux, and macOS

## 🚀 Quick Start

```bash
# Install the tool globally
go install github.com/error-log-or-return/error-log-or-return@latest

error-log-or-return ./...
```

## ⚙️ Configuration

```yaml
# error-log-or-return.yml
ignore:
  - "**/*_test.go"
  - "test/**"
  - "**/*_mock.go"
  - "**/mock/**"
  - "**/mocks/**"
```

The configuration file is automatically searched in the current directory (or `.config/`) with an optional dot prefix.

## 🔧 VS Code Integration

`Ctrl+Shift+P` (`Cmd+Shift+P` on Mac) → "Tasks: Run Task" → "Go: Check Error Log or Return"

- ✅ **Real-time highlighting** of rule violations
- ✅ **Problems panel** integration with clickable errors
- ✅ **File explorer markers** showing files with issues

🔄 **Optional**: Install the [Trigger Task on Save](https://marketplace.visualstudio.com/items?itemName=Gruntfuggly.triggertaskonsave) extension to automatically run the task silently on file save.

## 📋 Sample Output

```
path/to/file.go:25:1: возвращает error и есть defer с &err
path/to/file.go:31:1: есть err, нет defer, нет возврата error
```

> 💡 **Pro Tip**: Output format is identical to `go vet` - your editor will highlight issues automatically!

## 🔧 Integration with other analyzers

```go
import (
    "golang.org/x/tools/go/analysis"
    "github.com/error-log-or-return/error-log-or-return"
)

// Add to your multichecker
analyzers := []*analysis.Analyzer{
    errorLogOrReturn.Analyzer,
    // ... other analyzers
}
```

## 🔨 Development

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Clone and build
git clone https://github.com/error-log-or-return/error-log-or-return.git
cd error-log-or-return
make build
```

## 🤝 Contributing

We ❤️ contributions! Please include:

1. 🐛 **Reproducer** (code snippet or minimal repo)
2. 📊 **Expected vs actual output**
3. 🔖 **Go version** (`go version`)

📬 PRs are welcome too.

⭐ [Star this repo](https://github.com/error-log-or-return/error-log-or-return) if it helped you write cleaner Go code.
