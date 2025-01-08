package main

import (
	"fmt"
	"os"
	"strconv"
)

var exprOperatorsTable = map[string]int{
	"+":  1,
	"-":  1,
	"*":  2,
	"/":  2,
	"^":  3,
	"%":  3,
	"<<": 4,
	">>": 4,
	"&":  5,
	"|":  6,
	"&&": 7,
	"||": 8,
	"==": 9,
	"!=": 9,
	"<":  10,
	"<=": 10,
	">":  10,
	">=": 10,
}

type Parser struct {
	lexer *Lexer
}

func NewParser(filename string) (*Parser, error) {
	bytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	lexer := NewLexer(filename, string(bytes))
	return &Parser{lexer: lexer}, nil
}

func (p *Parser) Parse() (*Program, Token, []*LanguageError) {
	program := &Program{
		Statements: []Statement{},
	}
	lastTok := Token{}
	var errs []*LanguageError
	for {
		lastTok = p.lexer.Peek()
		if lastTok.Type == EOF {
			break
		}
		if lastTok.Type == Error {
			errs = append(errs, &LanguageError{
				Kind:     ErrorKindLexer,
				Line:     lastTok.Line,
				Column:   lastTok.Column,
				Message:  lastTok.Value,
				Filename: p.lexer.filename,
				Source:   p.lexer.buf,
				Snippet:  p.lexer.buf[lastTok.Offset:],
			})
			break
		}

		if lastTok.Type == Keyword {
			switch lastTok.Value {
			case "val":
				assignment := &Assignment{}

				// Parse identifier
				p.lexer.Next()          // consume 'val'
				ident := p.lexer.Next() // Consume the next token
				if ident.Type == Ident {
					assignment.Variable = ident.Value
					equals := p.lexer.Next()
					if equals.Type == Operator && equals.Value == "=" {
						expr, err := p.parseExpr()
						if err != nil {
							errs = append(errs, err)
						}
						assignment.Expr = expr
					} else {
						errs = append(errs, &LanguageError{
							Kind:     ErrorKindParser,
							Line:     ident.Line,
							Column:   ident.Column,
							Message:  fmt.Sprintf("expected '=' after identifier, got %s", equals.Value),
							Help:     "make sure the assignment is valid, e.g. val foo = 1 + 2",
							Filename: p.lexer.filename,
							Source:   p.lexer.buf,
							Snippet:  p.lexer.buf[ident.Offset:],
						})
					}
				} else {
					errs = append(errs, &LanguageError{
						Kind:     ErrorKindParser,
						Line:     ident.Line,
						Column:   ident.Column,
						Message:  fmt.Sprintf("expected identifier after 'val', got %s", ident.Value),
						Help:     "make sure the assignment is valid, e.g. val foo = 1 + 2",
						Filename: p.lexer.filename,
						Source:   p.lexer.buf,
						Snippet:  p.lexer.buf[ident.Offset:],
					})
				}
				program.Statements = append(program.Statements, Statement{
					Assignment: assignment,
				})
			}
		}

	}
	return program, lastTok, errs
}

func (p *Parser) parseTerm() (*Term, *LanguageError) {
	term := &Term{}
	termTok := p.lexer.Next()
	if termTok.Type == Int {
		num, err := strconv.ParseInt(termTok.Value, 0, 64)
		if err != nil {
			message := fmt.Sprintf("invalid integer literal: %s", termTok.Value)
			if numErr, ok := err.(*strconv.NumError); ok && numErr.Err == strconv.ErrRange {
				message = "number too large for 64-bit integer"
			}
			return nil, &LanguageError{
				Kind:     ErrorKindParser,
				Line:     termTok.Line,
				Column:   termTok.Column,
				Message:  message,
				Help:     "make sure the number is a valid integer, e.g. 123, 0xfff, 0o007, 0b101",
				Filename: p.lexer.filename,
				Source:   p.lexer.buf,
				Snippet:  p.lexer.buf[termTok.Offset:],
			}
		}
		term.Number = &num
	} else if termTok.Type == Float {
		num, err := strconv.ParseFloat(termTok.Value, 64)
		if err != nil {
			return nil, &LanguageError{
				Kind:    ErrorKindParser,
				Line:    termTok.Line,
				Column:  termTok.Column,
				Message: fmt.Sprintf("invalid float literal: %s", termTok.Value),
			}
		}
		term.Float = &num
	} else if termTok.Type == Str {
		term.String = &termTok.Value
	} else if termTok.Type == Ident {
		if p.lexer.Peek().Type == Operator && p.lexer.Peek().Value == "(" {
			return p.parseFunctionCall(termTok)
		} else {
			term.Variable = &termTok.Value
		}
	} else if termTok.Type == Operator && termTok.Value == "(" {
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		rParen := p.lexer.Next()
		if rParen.Value != ")" {
			return nil, &LanguageError{
				Kind:     ErrorKindParser,
				Line:     rParen.Line,
				Column:   rParen.Column,
				Message:  fmt.Sprintf("expected ')' to close subexpression, got %s", rParen.Value),
				Help:     "make sure subexpressions are valid, e.g. (1 + 2) * 3",
				Filename: p.lexer.filename,
				Source:   p.lexer.buf,
				Snippet:  p.lexer.buf[rParen.Offset:],
			}
		}
		term.SubExpr = expr
	} else {
		return nil, &LanguageError{
			Kind:     ErrorKindParser,
			Line:     termTok.Line,
			Column:   termTok.Column,
			Message:  fmt.Sprintf("expected number, string, or identifier, function call or subexpression, got %s", termTok.Value),
			Help:     "make sure the term is valid, e.g. 123, \"hello\", foo(1, 2, 3), (1 + 2) * 3",
			Filename: p.lexer.filename,
			Source:   p.lexer.buf,
			Snippet:  p.lexer.buf[termTok.Offset:],
		}
	}
	return term, nil
}

func (p *Parser) parseExpr() (*Expr, *LanguageError) {
	expr := &Expr{}
	term, err := p.parseTerm()
	if err != nil {
		return nil, err
	}
	expr.Left = term

	opTok := p.lexer.Peek()

	if opTok.Type == Operator {
		if opPrecedence, ok := exprOperatorsTable[opTok.Value]; ok {
			opTok = p.lexer.Next()

			expr.Op = &opTok.Value
			right, err := p.parseExpr()
			if err != nil {
				return expr, err
			}

			if right.Op != nil {
				rightOpPrecedence := exprOperatorsTable[*right.Op]
				if rightOpPrecedence >= opPrecedence {
					expr.Right = &Expr{
						Left:  right.Left,
						Op:    right.Op,
						Right: right.Right,
					}
					return expr, nil
				}
			}
			expr.Right = right

			return expr, nil
		}
	}
	return expr, nil
}

func (p *Parser) parseFunctionCall(ident Token) (*Term, *LanguageError) {
	p.lexer.Next() // consume '('
	var err *LanguageError
	term := &Term{
		Call: &Call{
			Function: ident.Value,
			Args:     []*Expr{},
		},
	}
	for p.lexer.Peek().Value != ")" {
		var arg *Expr
		arg, err = p.parseExpr()
		if err != nil {
			break
		}
		term.Call.Args = append(term.Call.Args, arg)
		comma := p.lexer.Peek()
		if comma.Type == Operator && comma.Value == "," {
			p.lexer.Next() // consume ','
			continue
		}
	}
	rParen := p.lexer.Next() // consume ')'
	if rParen.Value != ")" {
		return term, &LanguageError{
			Kind:     ErrorKindParser,
			Line:     ident.Line,
			Column:   ident.Column,
			Message:  fmt.Sprintf("expected ')' after function call, got %s", rParen.Value),
			Help:     "make sure the function call is valid, e.g. foo(1, 2, 3)",
			Filename: p.lexer.filename,
			Source:   p.lexer.buf,
			Snippet:  p.lexer.buf[ident.Offset:],
		}
	}

	return term, err
}
