package main

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type ErrorKind int

const (
	ErrorKindLexer ErrorKind = iota
	ErrorKindParser
	ErrorKindCompiler
)

type LanguageError struct {
	Kind     ErrorKind
	Line     int
	Column   int
	Message  string
	Filename string
	Help     string
	Source   string
	Snippet  string
}

func (e LanguageError) Error() string {
	return formatError(&e)
}

// formatError creates a visually appealing error message
func formatError(err *LanguageError) string {
	var b strings.Builder

	// Write error header with type and message
	var errorType string
	switch err.Kind {
	case ErrorKindLexer:
		errorType = "lexer error"
	case ErrorKindParser:
		errorType = "parser error"
	default:
		errorType = "unknown error"
	}

	fmt.Fprintf(&b, "\x1b[1;31m%s\x1b[0m: %s\n", errorType, err.Message)

	// Get the line containing the error
	lines := strings.Split(err.Source, "\n")
	if err.Line > 0 && err.Line <= len(lines) {
		lineNum := err.Line
		line := lines[lineNum-1]

		// Print location with filename
		fmt.Fprintf(&b, "\x1b[1;34m-->\x1b[0m %s:%d:%d\n", err.Filename, err.Line, err.Column)

		// Show previous line for context if available
		if lineNum > 1 {
			fmt.Fprintf(&b, "%4d | %s\n", lineNum-1, lines[lineNum-2])
		}

		// Print the error line
		fmt.Fprintf(&b, "%4d | %s\n", lineNum, line)

		// Print the error pointer with squiggly underline
		pointer := strings.Repeat(" ", err.Column-1) + "\x1b[1;31m^"
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

	return b.String()
}
