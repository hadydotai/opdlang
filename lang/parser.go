package lang

import (
	"fmt"
	"strconv"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

type Term struct {
	Number   *int    `  @Int`
	String   *string `| @String`
	Call     *Call   `| @@`
	Variable *string `| @Ident`
	SubExpr  *Expr   `| "(" @@ ")"`
}

type Call struct {
	Pos      lexer.Position
	Function string  `@Ident`
	Args     []*Expr `"(" (@@? ("," @@)*)? ")"`
}

type IfStmt struct {
	Pos       lexer.Position
	Condition *Expr       `"if" @@ "then"?`
	Then      []Statement `@@+`
	Else      []Statement `("else" @@+)? "end"`
}

type WhileStmt struct {
	Pos       lexer.Position
	Condition *Expr       `"while" @@ "do"`
	Body      []Statement `@@+ "end"`
}

var (
	basicLexer = lexer.MustSimple([]lexer.SimpleRule{
		{Name: "Keyword", Pattern: `\b(val|if|then|else|end|while|do)\b`},
		{Name: "comment", Pattern: `//.*|/\*.*?\*/`},
		{Name: "whitespace", Pattern: `\s+`},
		{Name: "String", Pattern: `"(?:[^"\\]|\\.)*"`},
		{Name: "Ident", Pattern: `\b([a-zA-Z_][a-zA-Z0-9_]*)\b`},
		{Name: "Punct", Pattern: `==|!=|<=|>=|[-,()*/+%{};&!=:<>\[\]]`},
		{Name: "Int", Pattern: `\d+`},
	})

	builtinFunctions = map[string]int{
		"print": 0,
		"add":   1,
	}
)

type Program struct {
	Statements []Statement `@@*`
}

type Statement struct {
	Assignment *Assignment ` 	@@`
	IfStmt     *IfStmt     `| @@`
	WhileStmt  *WhileStmt  `| @@`
	Call       *Call       `| @@`
}

type Assignment struct {
	Pos      lexer.Position
	Variable string `"val" @Ident "="`
	Expr     *Expr  `@@`
}

// First, let's define precedence levels for our operators
const (
	PREC_NONE    = 0
	PREC_COMPARE = 1 // == != < <= > >=
	PREC_TERM    = 2 // + -
	PREC_FACTOR  = 3 // * / %
)

// Define a type for our parser functions
// type prefixFn func(*Compiler) (*Expr, error)
type infixFn func(*Compiler, *Expr) (*Expr, error)

// Define operator info
type operatorInfo struct {
	precedence int
	infix      infixFn
}

// Create operator precedence table
var operators map[string]operatorInfo

func init() {
	operators = map[string]operatorInfo{
		"+":  {PREC_TERM, parseInfixOp},
		"-":  {PREC_TERM, parseInfixOp},
		"*":  {PREC_FACTOR, parseInfixOp},
		"/":  {PREC_FACTOR, parseInfixOp},
		"%":  {PREC_FACTOR, parseInfixOp},
		"==": {PREC_COMPARE, parseInfixOp},
		"!=": {PREC_COMPARE, parseInfixOp},
		"<":  {PREC_COMPARE, parseInfixOp},
		"<=": {PREC_COMPARE, parseInfixOp},
		">":  {PREC_COMPARE, parseInfixOp},
		">=": {PREC_COMPARE, parseInfixOp},
	}
}

// First, update the Expr type to be simpler since we'll handle the parsing ourselves
type Expr struct {
	Left  *Term
	Op    *string
	Right *Expr
}

// Add the Parse method to implement the Parseable interface
func (e *Expr) Parse(lex *lexer.PeekingLexer) error {
	// Parse the initial term
	term := &Term{}
	if err := term.Parse(lex); err != nil {
		return err
	}
	e.Left = term

	// Look for an operator
	token := lex.Peek()
	if token == nil {
		return nil
	}

	// Check if the token is an operator
	if op, isOp := operators[token.Value]; isOp {
		// Consume the operator token
		lex.Next()
		e.Op = &token.Value

		// Parse the right side with the appropriate precedence
		right := &Expr{}
		if err := right.Parse(lex); err != nil {
			return err
		}

		// If the right side has an operator with higher precedence,
		// restructure the tree
		if right.Op != nil {
			rightOp := operators[*right.Op]
			if rightOp.precedence > op.precedence {
				// Create a new expression for the higher precedence operation
				newExpr := &Expr{
					Left:  right.Left,
					Op:    right.Op,
					Right: right.Right,
				}

				// Make the current expression use the higher precedence result
				e.Right = newExpr
				return nil
			}
		}
		e.Right = right
	}

	return nil
}

// func (t *Term) toExpr() *Expr {
// 	return &Expr{Left: t}
// }

func (t *Term) Parse(lex *lexer.PeekingLexer) error {
	token := lex.Peek()
	if token == nil {
		return fmt.Errorf("unexpected end of input")
	}

	switch token.Type {
	case lexer.TokenType(basicLexer.Symbols()["Int"]):
		lex.Next()
		num, err := strconv.Atoi(token.Value)
		if err != nil {
			return err
		}
		t.Number = &num

	case lexer.TokenType(basicLexer.Symbols()["String"]):
		lex.Next()
		t.String = &token.Value

	case lexer.TokenType(basicLexer.Symbols()["Ident"]):
		lex.Next()
		// Look ahead to see if this is a function call
		next := lex.Peek()
		if next != nil && next.Value == "(" {
			// Parse function call
			call := &Call{Function: token.Value}
			call.Pos = token.Pos
			lex.Next() // Consume '('

			// Parse arguments
			for {
				next = lex.Peek()
				if next == nil {
					return fmt.Errorf("unexpected end of input in function call")
				}
				if next.Value == ")" {
					lex.Next() // Consume ')'
					break
				}
				if len(call.Args) > 0 {
					if next.Value != "," {
						return fmt.Errorf("expected ',' between arguments")
					}
					lex.Next() // Consume ','
				}
				arg := &Expr{}
				if err := arg.Parse(lex); err != nil {
					return err
				}
				call.Args = append(call.Args, arg)
			}
			t.Call = call
		} else {
			// It's a variable
			t.Variable = &token.Value
		}

	case lexer.TokenType(basicLexer.Symbols()["Punct"]):
		if token.Value == "(" {
			lex.Next() // Consume '('
			expr := &Expr{}
			if err := expr.Parse(lex); err != nil {
				return err
			}
			next := lex.Peek()
			if next == nil || next.Value != ")" {
				return fmt.Errorf("expected closing parenthesis")
			}
			lex.Next() // Consume ')'
			t.SubExpr = expr
		} else {
			return fmt.Errorf("unexpected token: %s", token.Value)
		}

	default:
		return fmt.Errorf("unexpected token type: %v", token.Type)
	}

	return nil
}

func Parse(sourceFile string, sourceCode string) (program *Program, err error) {
	parser := participle.MustBuild[Program](
		participle.Lexer(basicLexer),
	)

	program, err = parser.ParseString(sourceFile, sourceCode)
	if err != nil {
		err = fmt.Errorf("parse error: %v", err)
	}

	return
}

// Add parsing functions
func parseInfixOp(c *Compiler, left *Expr) (*Expr, error) {
	// Create a new expression with the left operand
	expr := &Expr{
		Left: left.Left,
		Op:   left.Op,
	}

	// Get the operator's precedence
	op := *left.Op
	precedence := operators[op].precedence

	// Parse the right side with precedence
	right, err := parsePrecedence(c, precedence+1)
	if err != nil {
		return nil, err
	}

	expr.Right = right
	return expr, nil
}

func parsePrecedence(c *Compiler, precedence int) (*Expr, error) {
	// Parse the left-hand expression
	left, err := parsePrimary(c)
	if err != nil {
		return nil, err
	}

	for {
		if left.Op == nil {
			break
		}

		op := *left.Op
		info, exists := operators[op]
		if !exists || info.precedence < precedence {
			break
		}

		// Parse the operator
		expr, err := info.infix(c, left)
		if err != nil {
			return nil, err
		}
		left = expr
	}

	return left, nil
}

func parsePrimary(c *Compiler) (*Expr, error) {
	// This will parse a basic term (number, string, variable, etc.)
	return &Expr{
		Left: &Term{
			Number:   nil, // Fill these in based on the token
			String:   nil,
			Variable: nil,
			SubExpr:  nil,
		},
	}, nil
}
