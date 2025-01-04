package main

import (
	"fmt"

	"github.com/alecthomas/participle/v2/lexer"
)

var (
	basicLexer = lexer.MustSimple([]lexer.SimpleRule{
		{Name: "Keyword", Pattern: `\b(val|if|then|else|end|while|do)\b`},
		{Name: "comment", Pattern: `//.*|/\*.*?\*/`},
		{Name: "whitespace", Pattern: `\s+`},
		{Name: "String", Pattern: `"[^"]*"`},
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
	Variable string `"val" @Ident "="`
	Expr     *Expr  `@@`
}

type Expr struct {
	Left  *Term  `@@`
	Op    string `[ @("+" | "-" | "*" | "/" | "%" | "==" | "!=" | "<" | "<=" | ">" | ">=") `
	Right *Expr  `@@ ]`
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
	Condition *Expr       `"if" @@ "then"?`
	Then      []Statement `@@+`
	Else      []Statement `("else" @@+)? "end"`
}

type WhileStmt struct {
	Condition *Expr       `"while" @@ "do"`
	Body      []Statement `@@+ "end"`
}

type Compiler struct {
	code       []byte
	labels     map[string]int
	vars       map[string]int
	funcs      map[string]int
	strings    map[string]int
	nextVar    int
	nextLabel  int
	nextFunc   int
	nextString int
	currentPos int
}

func NewCompiler() *Compiler {
	return &Compiler{
		code:       make([]byte, 0),
		labels:     make(map[string]int),
		vars:       make(map[string]int),
		funcs:      make(map[string]int),
		strings:    make(map[string]int),
		nextVar:    0,
		nextLabel:  0,
		nextFunc:   0,
		nextString: 0,
		currentPos: 0,
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

func (c *Compiler) compileProgram(program *Program) []byte {
	for _, stmt := range program.Statements {
		c.compileStatement(&stmt)
	}
	// Add HALT instruction at the end of the program
	c.emit(InstrHalt)
	return c.code
}

func (c *Compiler) compileStatement(stmt *Statement) {
	switch {
	case stmt.Assignment != nil:
		c.compileExpr(stmt.Assignment.Expr)
		varIdx := c.getVarIdx(stmt.Assignment.Variable)
		c.emit(InstrStore, byte(varIdx))
	case stmt.IfStmt != nil:
		endLabel := c.createLabel()
		elseLabel := c.createLabel()

		c.compileExpr(stmt.IfStmt.Condition)
		c.emit(InstrJmpIfZero)
		jumpPos := c.currentPos
		c.code = append(c.code, 0, 0) // Reserve 2 bytes for jump address
		c.currentPos += 2

		for _, s := range stmt.IfStmt.Then {
			c.compileStatement(&s)
		}

		c.emit(InstrJmp)
		endJumpPos := c.currentPos
		c.code = append(c.code, 0, 0) // Reserve 2 bytes for jump address
		c.currentPos += 2

		c.setLabel(elseLabel)
		if stmt.IfStmt.Else != nil {
			for _, s := range stmt.IfStmt.Else {
				c.compileStatement(&s)
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
			c.compileStatement(&s)
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
		c.compileCall(stmt.Call)
	}
}

func (c *Compiler) compileExpr(expr *Expr) {
	c.compileTerm(expr.Left)
	if expr.Right != nil {
		// Compile right operand first
		c.compileExpr(expr.Right)
		// Then emit the operator
		switch expr.Op {
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
	}
}

func (c *Compiler) compileTerm(term *Term) {
	switch {
	case term.Number != nil:
		c.emit(InstrPush, byte(*term.Number))
	case term.String != nil:
		stringIdx := c.internString(*term.String)
		fmt.Printf("Compiling string: %q -> str_%d\n", *term.String, stringIdx)
		c.emit(InstrPushStr, byte(stringIdx))
	case term.Variable != nil:
		varIdx := c.getVarIdx(*term.Variable)
		c.emit(InstrLoad, byte(varIdx))
	case term.Call != nil:
		c.compileCall(term.Call)
	case term.SubExpr != nil:
		c.compileExpr(term.SubExpr)
	}
}

func (c *Compiler) compileCall(call *Call) {
	fmt.Printf("Compiling call to %s with %d arguments\n", call.Function, len(call.Args))

	// Compile arguments in reverse order
	// for i := len(call.Args) - 1; i >= 0; i-- {
	// 	arg := call.Args[i]
	// 	if arg.Left.String != nil {
	// 		// Handle string literals directly
	// 		stringIdx := c.internString(*arg.Left.String)
	// 		c.emit(InstrPushStr, byte(stringIdx))
	// 	} else {
	// 		c.compileExpr(arg)
	// 	}
	// }

	for _, arg := range call.Args {
		c.compileExpr(arg)
	}

	// Get function index and emit call instruction
	funcIdx := c.getFuncIdx(call.Function)
	c.emit(InstrCall, byte(funcIdx), byte(len(call.Args)))
}

func (c *Compiler) internString(s string) int {
	// Remove quotes from the string literal
	s = s[1 : len(s)-1]

	if idx, ok := c.strings[s]; ok {
		return idx
	}
	c.strings[s] = c.nextString
	c.nextString++
	return c.nextString - 1
}
