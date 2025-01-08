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
		Statements: make([]Statement, 0),
	}
	lastTok := Token{}
	var errs []*LanguageError
	for {
		lastTok = p.lexer.Peek()
		if lastTok.Type == EOF {
			p.lexer.Next()
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
				Snippet:  p.lexer.buf[lastTok.Offset : lastTok.Offset+len(lastTok.Value)],
			})
			break
		}
		if lastTok.Type == Keyword {
			stmt, err := p.parseStatement()
			if err != nil {
				errs = append(errs, err)
				continue
			}
			program.Statements = append(program.Statements, *stmt)
		}

		p.lexer.Next()
	}
	return program, lastTok, errs
}

func (p *Parser) parseBlock() ([]Statement, *LanguageError) {
	statements := []Statement{}
	doKW := p.lexer.Peek()
	if doKW.Value != "do" {
		return nil, &LanguageError{
			Kind:     ErrorKindParser,
			Line:     doKW.Line,
			Column:   doKW.Column,
			Message:  fmt.Sprintf("expected 'do' after 'if', got %s", doKW.Value),
			Help:     "make sure the block is valid, e.g. do print(\"hello\") end",
			Filename: p.lexer.filename,
			Source:   p.lexer.buf,
			Snippet:  p.lexer.buf[doKW.Offset : doKW.Offset+len(doKW.Value)],
		}
	}
	p.lexer.Next() // consume 'do'
	for p.lexer.Peek().Value != "end" {
		stmt, err := p.parseStatement()
		if err != nil {
			return statements, err
		}
		statements = append(statements, *stmt)
	}
	endKW := p.lexer.Next()
	if endKW.Value != "end" {
		return statements, &LanguageError{
			Kind:    ErrorKindParser,
			Line:    endKW.Line,
			Column:  endKW.Column,
			Message: fmt.Sprintf("expected 'end' after block, got %s", endKW.Value),
		}
	}
	return statements, nil
}

func (p *Parser) parseStatement() (*Statement, *LanguageError) {
	keyword := p.lexer.Next()
	switch keyword.Value {
	case "val":
		assignment := &Assignment{}
		ident := p.lexer.Next()
		if ident.Type == Ident {
			assignment.Variable = ident.Value
			equals := p.lexer.Next()
			if equals.Type == Operator && equals.Value == "=" {
				var err *LanguageError
				assignment.Expr, err = p.parseExpr()
				if err != nil {
					return nil, err
				}
			} else {
				return nil, &LanguageError{
					Kind:     ErrorKindParser,
					Line:     ident.Line,
					Column:   ident.Column,
					Message:  fmt.Sprintf("expected '=' after identifier, got %s", equals.Value),
					Help:     "make sure the assignment is valid, e.g. val foo = 1 + 2",
					Filename: p.lexer.filename,
					Source:   p.lexer.buf,
					Snippet:  p.lexer.buf[ident.Offset : ident.Offset+len(ident.Value)],
				}
			}
		} else {
			return nil, &LanguageError{
				Kind:     ErrorKindParser,
				Line:     ident.Line,
				Column:   ident.Column,
				Message:  fmt.Sprintf("expected identifier after 'val', got %s", ident.Value),
				Help:     "make sure the assignment is valid, e.g. val foo = 1 + 2",
				Filename: p.lexer.filename,
				Source:   p.lexer.buf,
				Snippet:  p.lexer.buf[ident.Offset : ident.Offset+len(ident.Value)],
			}
		}
		return &Statement{
			Assignment: assignment,
		}, nil
	case "if":
		ifStmt := &IfStmt{}
		var err *LanguageError
		ifStmt.Condition, err = p.parseExpr()
		if err != nil {
			return nil, &LanguageError{
				Kind:     ErrorKindParser,
				Line:     keyword.Line,
				Column:   keyword.Column,
				Message:  fmt.Sprintf("expected expression after 'if', got %s", keyword.Value),
				Help:     "make sure the if statement is valid, e.g. if (1 + 2) == 3 then print(\"hello\") end",
				Filename: p.lexer.filename,
				Source:   p.lexer.buf,
				Snippet:  p.lexer.buf[keyword.Offset : keyword.Offset+len(keyword.Value)],
			}
		}
		statements, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		ifStmt.Then = statements
		return &Statement{
			IfStmt: ifStmt,
		}, nil
	}
	return nil, nil
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
				Snippet:  p.lexer.buf[termTok.Offset : termTok.Offset+len(termTok.Value)],
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
				Snippet:  p.lexer.buf[rParen.Offset : rParen.Offset+len(rParen.Value)],
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
			Snippet:  p.lexer.buf[termTok.Offset : termTok.Offset+len(termTok.Value)],
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
		opTok = p.lexer.Next()
		opTok2 := p.lexer.Peek()
		op := opTok.Value
		if opTok2.Type == Operator {
			op = op + opTok2.Value
			p.lexer.Next()
		}
		if opPrecedence, ok := exprOperatorsTable[op]; ok {
			expr.Op = &op
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
	for p.lexer.Peek().Value != ")" && p.lexer.Peek().Type != Keyword {
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
	rParen := p.lexer.Peek()
	if rParen.Value != ")" {
		return term, &LanguageError{
			Kind:     ErrorKindParser,
			Line:     ident.Line,
			Column:   ident.Column,
			Message:  fmt.Sprintf("expected ')' after function call, got %s", rParen.Value),
			Help:     "make sure the function call is valid, e.g. foo(1, 2, 3)",
			Filename: p.lexer.filename,
			Source:   p.lexer.buf,
			Snippet:  p.lexer.buf[ident.Offset : ident.Offset+len(ident.Value)],
		}
	}
	p.lexer.Next() // consume ')'

	return term, err
}
