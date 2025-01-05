package main

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/alecthomas/participle/v2/lexer"
)

type ErrorKind int

const (
	ErrorSyntax ErrorKind = iota
	ErrorType
	ErrorCompile
)

type CompilerError struct {
	Kind    ErrorKind
	Message string
	Pos     lexer.Position
	Source  string
	Help    string
	Context string // Additional context for the error
	Snippet string // The problematic code snippet
}

func (e *CompilerError) Error() string {
	return formatError(e)
}

// formatError creates a visually appealing error message
func formatError(err *CompilerError) string {
	var b strings.Builder

	// Write error header with type and message
	var errorType string
	switch err.Kind {
	case ErrorSyntax:
		errorType = "syntax error"
	case ErrorType:
		errorType = "type error"
	case ErrorCompile:
		errorType = "compilation error"
	default:
		errorType = "unknown error"
	}

	fmt.Fprintf(&b, "\x1b[1;31m%s\x1b[0m: %s\n", errorType, err.Message)

	// Get the line containing the error
	lines := strings.Split(err.Source, "\n")
	if err.Pos.Line > 0 && err.Pos.Line <= len(lines) {
		lineNum := err.Pos.Line
		line := lines[lineNum-1]

		// Print location with filename
		fmt.Fprintf(&b, "\x1b[1;34m-->\x1b[0m %s:%d:%d\n", err.Pos.Filename, err.Pos.Line, err.Pos.Column)

		// Show previous line for context if available
		if lineNum > 1 {
			fmt.Fprintf(&b, "%4d | %s\n", lineNum-1, lines[lineNum-2])
		}

		// Print the error line
		fmt.Fprintf(&b, "%4d | %s\n", lineNum, line)

		// Print the error pointer with squiggly underline
		pointer := strings.Repeat(" ", err.Pos.Column-1) + "\x1b[1;31m^"
		if err.Snippet != "" {
			pointer += strings.Repeat("~", utf8.RuneCountInString(err.Snippet)-1)
		}
		fmt.Fprintf(&b, "     | %s\x1b[0m\n", pointer)

		// Show next line for context if available
		if lineNum < len(lines) {
			fmt.Fprintf(&b, "%4d | %s\n", lineNum+1, lines[lineNum])
		}
	}

	// Print help message if available
	if err.Help != "" {
		fmt.Fprintf(&b, "\n\x1b[1;32mhelp\x1b[0m: %s\n", err.Help)
	}

	// Print additional context if available
	if err.Context != "" {
		fmt.Fprintf(&b, "\n%s\n", err.Context)
	}

	return b.String()
}

// Helper function to create syntax errors
func NewSyntaxError(pos lexer.Position, source, message, help string) error {
	return &CompilerError{
		Kind:    ErrorSyntax,
		Message: message,
		Pos:     pos,
		Source:  source,
		Help:    help,
	}
}

// Helper function to create type errors
func NewTypeError(pos lexer.Position, source, message, help string) error {
	return &CompilerError{
		Kind:    ErrorType,
		Message: message,
		Pos:     pos,
		Source:  source,
		Help:    help,
	}
}

// Helper function to create compilation errors
func NewCompileError(pos lexer.Position, source, message, help string) error {
	return &CompilerError{
		Kind:    ErrorCompile,
		Message: message,
		Pos:     pos,
		Source:  source,
		Help:    help,
	}
}
