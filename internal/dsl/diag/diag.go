// Package diag provides the diagnostics model shared by the DSL compiler
// packages (ast, schema, cel, ...). Every compiler stage returns its result
// alongside a diag.List instead of an error, so multiple problems in a
// recipe bundle can be reported at once.
package diag

import "fmt"

// Severity classifies a Diagnostic.
type Severity int

const (
	// Error marks a diagnostic that fails compilation.
	Error Severity = iota
	// Warning marks a diagnostic that does not fail compilation.
	Warning
)

// Pos is a 1-based line/column position within a source file.
type Pos struct{ Line, Col int }

// Diagnostic is a single compiler message tied to a location in a recipe
// bundle file.
type Diagnostic struct {
	Severity Severity
	Path     string // файл внутри бандла рецепта
	Pos      Pos
	Message  string
	Module   string // модуль-автор правила (компонент/провайдер), для contract-check
}

// List is an ordered collection of Diagnostic values.
type List []Diagnostic

// Add appends a diagnostic to the list.
func (l *List) Add(d Diagnostic) { *l = append(*l, d) }

// Errorf appends an Error-severity diagnostic built from a format string.
func (l *List) Errorf(path string, pos Pos, format string, args ...any) {
	l.Add(Diagnostic{Severity: Error, Path: path, Pos: pos, Message: fmt.Sprintf(format, args...)})
}

// HasErrors reports whether the list contains at least one Error-severity
// diagnostic.
func (l List) HasErrors() bool {
	for _, d := range l {
		if d.Severity == Error {
			return true
		}
	}
	return false
}
