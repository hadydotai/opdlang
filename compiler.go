package main

import (
	"fmt"
	"strconv"

	"github.com/alecthomas/participle/v2/lexer"
)

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
type prefixFn func(*Compiler) (*Expr, error)
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

// Add a helper method to Term to convert it to an Expr
func (t *Term) toExpr() *Expr {
	return &Expr{Left: t}
}

// Update the Term type to also implement Parseable
func (t *Term) Parse(lex *lexer.PeekingLexer) error {
	token := lex.Peek()
	if token == nil {
		return fmt.Errorf("unexpected end of input")
	}

	switch token.Type {
	case lexer.TokenType(basicLexer.Symbols()["Int"]):
		lex.Next() // Consume the token
		num, err := strconv.Atoi(token.Value)
		if err != nil {
			return err
		}
		t.Number = &num

	case lexer.TokenType(basicLexer.Symbols()["String"]):
		lex.Next() // Consume the token
		t.String = &token.Value

	case lexer.TokenType(basicLexer.Symbols()["Ident"]):
		lex.Next() // Consume the token
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

// Update the compiler's expression compilation
func (c *Compiler) compileExpr(expr *Expr) error {
	// If there's no operator, just compile the term
	if expr.Op == nil {
		return c.compileTerm(expr.Left)
	}

	// Compile left operand
	if err := c.compileTerm(expr.Left); err != nil {
		return err
	}

	// Compile right operand
	if err := c.compileExpr(expr.Right); err != nil {
		return err
	}

	// Emit the operator instruction
	switch *expr.Op {
	case "+":
		c.emit(InstrAdd)
	case "-":
		c.emit(InstrSub)
	case "*":
		c.emit(InstrMul)
	case "/":
		c.emit(InstrDiv)
	case "%":
		c.emit(InstrMod)
	case "==":
		c.emit(InstrEq)
	case "!=":
		c.emit(InstrNeq)
	case "<":
		c.emit(InstrLt)
	case "<=":
		c.emit(InstrLte)
	case ">":
		c.emit(InstrGt)
	case ">=":
		c.emit(InstrGte)
	}

	return nil
}

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

type Compiler struct {
	code        []byte
	labels      map[string]int
	vars        map[string]int
	funcs       map[string]int
	strings     map[string]int
	nextVar     int
	nextLabel   int
	nextFunc    int
	nextString  int
	currentPos  int
	currentLine int
	sourceMap   map[int]int
}

func NewCompiler() *Compiler {
	return &Compiler{
		code:        make([]byte, 0),
		labels:      make(map[string]int),
		vars:        make(map[string]int),
		funcs:       make(map[string]int),
		strings:     make(map[string]int),
		nextVar:     0,
		nextLabel:   0,
		nextFunc:    0,
		nextString:  0,
		currentPos:  0,
		currentLine: 1,
		sourceMap:   make(map[int]int),
	}
}

func (c *Compiler) DebugPrint() {
	fmt.Println("\033[1;36mBytecode:\033[0m")
	i := 0
	for i < len(c.code) {
		instr := Instr(c.code[i])
		fmt.Printf("\033[90m%04d:\033[0m \033[1;33m%-12v\033[0m", i, instr)

		switch instr {
		case InstrPush:
			if i+1 < len(c.code) {
				fmt.Printf("    \033[1;32mvalue:\033[0m %-20d", c.code[i+1])
				i++
			}
		case InstrPushStr:
			if i+1 < len(c.code) {
				strIdx := c.code[i+1]
				var foundStr string
				for str, idx := range c.strings {
					if idx == int(strIdx) {
						foundStr = str
						break
					}
				}
				fmt.Printf("    \033[1;32mstring:\033[0m %-20q    \033[90m(str_%d)\033[0m", foundStr, strIdx)
				i++
			}
		case InstrCall:
			if i+2 < len(c.code) {
				funcIdx := int(c.code[i+1])
				funcName := "?"
				for name, idx := range builtinFunctions {
					if idx == funcIdx {
						funcName = name
						break
					}
				}
				fmt.Printf("    \033[1;32mfunc:\033[0m   %-20s    \033[90m(func_%d, args=%d)\033[0m",
					funcName, funcIdx, c.code[i+2])
				i += 2
			}
		case InstrLoad, InstrStore:
			if i+1 < len(c.code) {
				varIdx := c.code[i+1]
				varName := "?"
				for name, idx := range c.vars {
					if idx == int(varIdx) {
						varName = name
						break
					}
				}
				fmt.Printf("    \033[1;32mvar:\033[0m    %-20s    \033[90m(var_%d)\033[0m", varName, varIdx)
				i++
			}
		case InstrJmp, InstrJmpIfZero:
			if i+2 < len(c.code) {
				jumpAddr := (int(c.code[i+1]) << 8) | int(c.code[i+2])
				fmt.Printf("    \033[1;32mjump:\033[0m   %-20d", jumpAddr)
				i += 2
			}
		}
		fmt.Println()
		i++
	}

	fmt.Println("\n\033[1;36mSymbol Tables:\033[0m")

	fmt.Println("\n\033[1;35mVariables:\033[0m")
	for name, idx := range c.vars {
		fmt.Printf("    %-30s \033[90m-> var_%d\033[0m\n", name, idx)
	}

	fmt.Println("\n\033[1;35mFunctions:\033[0m")
	for name, idx := range c.funcs {
		fmt.Printf("    %-30s \033[90m-> func_%d\033[0m\n", name, idx)
	}

	fmt.Println("\n\033[1;35mBuiltin Functions:\033[0m")
	for name, idx := range builtinFunctions {
		fmt.Printf("    %-30s \033[90m-> func_%d\033[0m\n", name, idx)
	}

	fmt.Println("\n\033[1;35mLabels:\033[0m")
	for name, addr := range c.labels {
		fmt.Printf("    %-30s \033[90m-> addr_%d\033[0m\n", name, addr)
	}

	fmt.Println("\n\033[1;35mStrings:\033[0m")
	for str, idx := range c.strings {
		fmt.Printf("    %-30q \033[90m-> str_%d\033[0m\n", str, idx)
	}
}

func (c *Compiler) emit(op Instr, operands ...byte) {
	c.code = append(c.code, byte(op))
	c.currentPos++
	c.code = append(c.code, operands...)
	c.currentPos += len(operands)
}

func (c *Compiler) getVarIdx(name string) int {
	if idx, ok := c.vars[name]; ok {
		return idx
	}
	c.vars[name] = c.nextVar
	c.nextVar++
	return c.nextVar - 1
}

func (c *Compiler) getFuncIdx(name string) int {
	if idx, ok := builtinFunctions[name]; ok {
		return idx
	}
	if idx, ok := c.funcs[name]; ok {
		return idx
	}
	c.funcs[name] = c.nextFunc
	c.nextFunc++
	return c.nextFunc - 1
}

func (c *Compiler) createLabel() string {
	label := fmt.Sprintf("L%d", c.nextLabel)
	c.nextLabel++
	return label
}

func (c *Compiler) setLabel(label string) {
	c.labels[label] = c.currentPos
}

func (c *Compiler) compileProgram(program *Program) ([]byte, error) {
	for _, stmt := range program.Statements {
		if err := c.compileStatement(&stmt); err != nil {
			return nil, err
		}
	}
	c.emit(InstrHalt)
	return c.code, nil
}

func (c *Compiler) compileStatement(stmt *Statement) error {
	switch {
	case stmt.Assignment != nil:
		c.registerLine(stmt.Assignment.Pos)
		if err := c.compileExpr(stmt.Assignment.Expr); err != nil {
			return err
		}
		varIdx := c.getVarIdx(stmt.Assignment.Variable)
		c.emit(InstrStore, byte(varIdx))
	case stmt.IfStmt != nil:
		c.registerLine(stmt.IfStmt.Pos)
		endLabel := c.createLabel()
		elseLabel := c.createLabel()

		c.compileExpr(stmt.IfStmt.Condition)
		c.emit(InstrJmpIfZero)
		jumpPos := c.currentPos
		c.code = append(c.code, 0, 0) // Reserve 2 bytes for jump address
		c.currentPos += 2

		for _, s := range stmt.IfStmt.Then {
			if err := c.compileStatement(&s); err != nil {
				return err
			}
		}

		c.emit(InstrJmp)
		endJumpPos := c.currentPos
		c.code = append(c.code, 0, 0) // Reserve 2 bytes for jump address
		c.currentPos += 2

		c.setLabel(elseLabel)
		if stmt.IfStmt.Else != nil {
			for _, s := range stmt.IfStmt.Else {
				if err := c.compileStatement(&s); err != nil {
					return err
				}
			}
		}

		c.setLabel(endLabel)

		// Update jump positions using actual instruction positions
		elseAddr := c.labels[elseLabel]
		c.code = append(c.code[:jumpPos], append([]byte{
			byte(elseAddr >> 8),
			byte(elseAddr & 0xff),
		}, c.code[jumpPos+2:]...)...)

		endAddr := c.labels[endLabel]
		c.code = append(c.code[:endJumpPos], append([]byte{
			byte(endAddr >> 8),
			byte(endAddr & 0xff),
		}, c.code[endJumpPos+2:]...)...)
	case stmt.WhileStmt != nil:
		c.registerLine(stmt.WhileStmt.Pos)
		startLabel := c.createLabel()
		endLabel := c.createLabel()

		// Start of loop
		c.setLabel(startLabel)
		c.compileExpr(stmt.WhileStmt.Condition)

		// Jump to end if condition is false
		c.emit(InstrJmpIfZero)
		jumpToEndPos := c.currentPos
		c.code = append(c.code, 0, 0) // Reserve 2 bytes for jump address
		c.currentPos += 2

		// Compile loop body
		for _, s := range stmt.WhileStmt.Body {
			if err := c.compileStatement(&s); err != nil {
				return err
			}
		}

		// Jump back to start of loop
		c.emit(InstrJmp)
		startAddr := c.labels[startLabel]
		c.code = append(c.code, byte(startAddr>>8), byte(startAddr&0xff))
		c.currentPos += 2

		// Set end label and patch the conditional jump
		c.setLabel(endLabel)
		endAddr := c.labels[endLabel]

		// Patch the jump-if-zero instruction with the correct end address
		c.code[jumpToEndPos] = byte(endAddr >> 8)
		c.code[jumpToEndPos+1] = byte(endAddr & 0xff)

	case stmt.Call != nil:
		c.registerLine(stmt.Call.Pos)
		return c.compileCall(stmt.Call)
	}
	return nil
}

func (c *Compiler) compileTerm(term *Term) error {
	switch {
	case term.Number != nil:
		c.emit(InstrPush, byte(*term.Number))
	case term.String != nil:
		stringIdx := c.internString(*term.String)
		c.emit(InstrPushStr, byte(stringIdx))
	case term.Variable != nil:
		varIdx := c.getVarIdx(*term.Variable)
		c.emit(InstrLoad, byte(varIdx))
	case term.Call != nil:
		return c.compileCall(term.Call)
	case term.SubExpr != nil:
		return c.compileExpr(term.SubExpr)
	}
	return nil
}

func (c *Compiler) compileCall(call *Call) error {
	for _, arg := range call.Args {
		if err := c.compileExpr(arg); err != nil {
			return err
		}
	}

	// Get function index and emit call instruction
	funcIdx := c.getFuncIdx(call.Function)
	c.emit(InstrCall, byte(funcIdx), byte(len(call.Args)))
	return nil
}

func (c *Compiler) internString(s string) int {
	unescaped := unescapeString(s)

	if idx, ok := c.strings[unescaped]; ok {
		return idx
	}
	c.strings[unescaped] = c.nextString
	c.nextString++
	return c.nextString - 1
}

func (c *Compiler) registerLine(pos lexer.Position) {
	if pos.Line != c.currentLine {
		c.currentLine = pos.Line
		c.sourceMap[c.currentPos] = c.currentLine
	}
}

func (c *Compiler) GetPCForLine(line int) int {
	for pc, l := range c.sourceMap {
		if l == line {
			return pc
		}
	}
	return -1
}

// Add a method to get the source map
func (c *Compiler) GetSourceMap() map[int]int {
	return c.sourceMap
}

func (c *Compiler) GetLineForPC(pc int) int {
	for pc, line := range c.sourceMap {
		if pc == pc {
			return line
		}
	}
	return -1
}

// Add this helper function to handle string escapes
func unescapeString(s string) string {
	// Remove surrounding quotes first
	s = s[1 : len(s)-1]

	var result []rune
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++ // Skip the backslash
			switch s[i] {
			case 'n':
				result = append(result, '\n')
			case 't':
				result = append(result, '\t')
			case 'r':
				result = append(result, '\r')
			case '"':
				result = append(result, '"')
			case '\\':
				result = append(result, '\\')
			default:
				// For unsupported escape sequences, keep them as-is
				result = append(result, '\\', rune(s[i]))
			}
		} else {
			result = append(result, rune(s[i]))
		}
	}
	return string(result)
}
