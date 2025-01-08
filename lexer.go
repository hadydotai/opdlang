// Base on Eli Bendersky's work on lexers
// https://eli.thegreenplace.net/2022/a-faster-lexer-in-go/
package main

import (
	"fmt"
	"unicode/utf8"
)

type TokenType int

type Token struct {
	Filename string
	Offset   int
	Line     int
	Column   int
	Type     TokenType
	Value    string
}

func (t Token) String() string {
	return [...]string{
		"EOF",
		"Ident",
		"Int",
		"Float",
		"Str",
		"Comment",
		"Directive",
		"Keyword",
		"Operator",
		"L_Paren",
		"R_Paren",
		"Colon",
		"Comma",
		"Dot",
		"Error",
	}[t.Type]
}

var punctTable = map[string]struct{}{
	"+":   {},
	"-":   {},
	"*":   {},
	"/":   {},
	"%":   {},
	"=":   {},
	"==":  {},
	"!=":  {},
	">":   {},
	">=":  {},
	"<":   {},
	"<=":  {},
	"&&":  {},
	"||":  {},
	"!":   {},
	"~":   {},
	"&":   {},
	"|":   {},
	"^":   {},
	"<<":  {},
	">>":  {},
	"(":   {},
	")":   {},
	":":   {},
	",":   {},
	".":   {},
	"...": {},
}

var keywordTable = map[string]struct{}{
	"val":       {},
	"if":        {},
	"elif":      {},
	"else":      {},
	"while":     {},
	"do":        {},
	"end":       {},
	"then":      {},
	"local":     {},
	"return":    {},
	"break":     {},
	"continue":  {},
	"operation": {},
	"depends":   {},
}

const (
	EOF TokenType = iota

	Ident
	Int
	Float
	Str

	Comment
	Directive

	Keyword
	Operator
	L_Paren
	R_Paren
	Colon
	Comma
	Dot

	Error
)

// Lexer
//
// Create a new lexer with NewLexer, then call Next()
// to get the next token. EOF is returned when there's
// no more tokens to return.
// Error is returned when there's an error in the lexer.
type Lexer struct {
	filename string
	buf      string
	line     int
	column   int
	offset   int

	// current rune
	curr rune
	// current position in buf
	currPos int
	// next position in buf
	nextPos int
}

func NewLexer(filename string, buf string) *Lexer {
	lex := &Lexer{buf: buf, filename: filename, line: 1, column: 1}
	lex.next()
	return lex
}

func (lex *Lexer) Next() Token {
	lex.skipWhitespaces()

	if lex.curr == -1 {
		return Token{
			Type:     EOF,
			Value:    "<EOF>",
			Offset:   lex.offset,
			Line:     lex.line,
			Column:   len(lex.buf) - 1,
			Filename: lex.filename,
		}
	}

	if _, ok := punctTable[string(lex.curr)]; ok {
		if lex.curr == '/' { // possibly a comment
			if lex.peekNext() == '/' {
				return lex.scanComment()
			}
		}

		tok := Token{
			Type:     Operator,
			Value:    string(lex.curr),
			Offset:   lex.offset,
			Line:     lex.line,
			Column:   lex.column - 1,
			Filename: lex.filename,
		}
		lex.next()
		return tok
	}

	if isAlpha(lex.curr) {
		return lex.scanIdent()
	} else if isDecimalDigit(lex.curr) {
		return lex.scanNumber()
	} else if lex.curr == '"' || lex.curr == '\'' || lex.curr == '`' {
		return lex.scanString()
	}

	return Token{
		Type:     Error,
		Value:    fmt.Sprintf("unexpected character: %c", lex.curr),
		Offset:   lex.offset,
		Line:     lex.line,
		Column:   lex.column,
		Filename: lex.filename,
	}
}

func (lex *Lexer) Peek() Token {
	// Save current state
	currPos := lex.currPos
	curr := lex.curr
	offset := lex.offset
	line := lex.line
	column := lex.column
	nextPos := lex.nextPos

	// Get next token
	tok := lex.Next()

	// Restore state
	lex.currPos = currPos
	lex.curr = curr
	lex.offset = offset
	lex.line = line
	lex.column = column
	lex.nextPos = nextPos
	return tok
}

func (lex *Lexer) skipWhitespaces() {
	for lex.curr == ' ' || lex.curr == '\t' || lex.curr == '\n' || lex.curr == '\r' {
		if lex.curr == '\n' {
			lex.line++
			lex.column = 1
		}
		lex.next()
	}
}

func (lex *Lexer) scanComment() Token {
	start := lex.currPos
	lex.next() // skip '/'
	for {
		if lex.peekNext() == '\n' {
			break
		}
		lex.next()
	}
	tok := Token{
		Type:     Comment,
		Value:    lex.buf[start:lex.currPos],
		Offset:   lex.offset - len(lex.buf[start:lex.currPos]),
		Line:     lex.line,
		Column:   lex.column - len(lex.buf[start:lex.currPos]) - 1,
		Filename: lex.filename,
	}
	lex.next() // skip '\n'
	return tok
}

func (lex *Lexer) scanIdent() Token {
	start := lex.currPos
	for isAlpha(lex.curr) || isDecimalDigit(lex.curr) {
		lex.next()
	}
	tok := Token{
		Type:     Ident,
		Value:    lex.buf[start:lex.currPos],
		Offset:   lex.offset - len(lex.buf[start:lex.currPos]),
		Line:     lex.line,
		Column:   lex.column - len(lex.buf[start:lex.currPos]) - 1,
		Filename: lex.filename,
	}
	if _, ok := keywordTable[tok.Value]; ok {
		tok.Type = Keyword
	}
	return tok
}

func (lex *Lexer) scanNumber() Token {
	start := lex.currPos
	base := 10
	isFloat := false

	// Check for hex/octal prefix
	if lex.curr == '0' {
		next := lex.peekNext()
		if next == 'x' || next == 'X' {
			lex.next() // consume '0'
			lex.next() // consume 'x'
			base = 16
		} else if next == 'o' || next == 'O' {
			lex.next() // consume '0'
			lex.next() // consume 'o'
			base = 8
		} else if next == 'b' || next == 'B' {
			lex.next() // consume '0'
			lex.next() // consume 'b'
			base = 2
		}
	}

	// Scan digits
	for {
		if base == 16 && (isHexDigit(lex.curr) || lex.curr == '_') ||
			base == 8 && (isOctalDigit(lex.curr) || lex.curr == '_') ||
			base == 2 && (isBinaryDigit(lex.curr) || lex.curr == '_') ||
			base == 10 && (isDecimalDigit(lex.curr) || lex.curr == '_') {
			lex.next()
		} else {
			break
		}
	}

	// Handle decimal point and fractional part
	if base == 10 && lex.curr == '.' {
		isFloat = true
		lex.next() // consume '.'
		for isDecimalDigit(lex.curr) || lex.curr == '_' {
			lex.next()
		}
	}

	// Handle exponent
	if base == 10 && (lex.curr == 'e' || lex.curr == 'E') {
		isFloat = true
		lex.next() // consume 'e'
		if lex.curr == '+' || lex.curr == '-' {
			lex.next()
		}
		for isDecimalDigit(lex.curr) || lex.curr == '_' {
			lex.next()
		}
	}

	tokenType := Int
	if isFloat {
		tokenType = Float
	}

	tok := Token{
		Type:     tokenType,
		Value:    lex.buf[start:lex.currPos],
		Offset:   lex.offset - len(lex.buf[start:lex.currPos]),
		Line:     lex.line,
		Column:   lex.column - len(lex.buf[start:lex.currPos]) - 1,
		Filename: lex.filename,
	}
	return tok
}

func (lex *Lexer) scanString() Token {
	start := lex.currPos
	quote := lex.curr // Save the opening quote character
	lex.next()        // Consume opening quote

	for lex.curr != -1 && lex.curr != quote {
		if quote != '`' && lex.curr == '\\' {
			lex.next() // Consume backslash
			switch lex.curr {
			case 'n', 'r', 't', '\\', '"', '\'':
				lex.next()
			case 'u':
				lex.next()
				// Expect exactly 4 hex digits
				for i := 0; i < 4 && isHexDigit(lex.curr); i++ {
					lex.next()
				}
			default:
				// Invalid escape sequence
				return Token{
					Type:     Error,
					Value:    "invalid escape sequence",
					Offset:   lex.offset,
					Line:     lex.line,
					Column:   lex.column,
					Filename: lex.filename,
				}
			}
		} else {
			if lex.curr == '\n' {
				if quote == '`' {
					// Raw strings can contain newlines
					lex.line++
					lex.column = 1
				} else {
					// Regular strings cannot contain unescaped newlines
					return Token{
						Type:     Error,
						Value:    "unterminated string literal",
						Offset:   lex.offset,
						Line:     lex.line,
						Column:   lex.column,
						Filename: lex.filename,
					}
				}
			}
			lex.next()
		}
	}

	if lex.curr == -1 {
		return Token{
			Type:     Error,
			Value:    "unterminated string literal",
			Offset:   lex.offset,
			Line:     lex.line,
			Column:   lex.column,
			Filename: lex.filename,
		}
	}

	lex.next() // Consume closing quote

	tok := Token{
		Type:     Str,
		Value:    lex.buf[start+1 : lex.currPos-1],
		Offset:   lex.offset - len(lex.buf[start+1:lex.currPos-1]),
		Line:     lex.line,
		Column:   lex.column - len(lex.buf[start+1:lex.currPos-1]) - 1,
		Filename: lex.filename,
	}
	return tok
}

// advances the internal state to point to the next rune in
// the input buffer.
func (lex *Lexer) next() {
	if lex.nextPos >= len(lex.buf) {
		lex.currPos = len(lex.buf)
		lex.curr = -1 // EOF
		lex.offset = len(lex.buf)
		return
	}

	lex.currPos = lex.nextPos
	r, w := rune(lex.buf[lex.nextPos]), 1
	if r >= utf8.RuneSelf {
		r, w = utf8.DecodeRuneInString(lex.buf[lex.nextPos:])
	}
	lex.nextPos += w
	lex.column += w
	lex.curr = r
	lex.offset = lex.nextPos

}

// return the next rune in the input buffer, or -1 if EOF
// without advancing the lexer state.
func (lex *Lexer) peekNext() rune {
	if lex.nextPos < len(lex.buf) {
		return rune(lex.buf[lex.nextPos])
	}
	return -1
}

func isAlpha(r rune) bool {
	return 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || r == '_'
}

func isDecimalDigit(r rune) bool {
	return '0' <= r && r <= '9'
}

func isHexDigit(r rune) bool {
	return '0' <= r && r <= '9' || 'a' <= r && r <= 'f' || 'A' <= r && r <= 'F'
}

func isOctalDigit(r rune) bool {
	return '0' <= r && r <= '7'
}

func isBinaryDigit(r rune) bool {
	return r == '0' || r == '1'
}
