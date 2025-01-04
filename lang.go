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
	nextVar    int
	nextLabel  int
	nextFunc   int
	currentPos int
}

func NewCompiler() *Compiler {
	return &Compiler{
		code:       make([]byte, 0),
		labels:     make(map[string]int),
		vars:       make(map[string]int),
		funcs:      make(map[string]int),
		nextVar:    0,
		nextLabel:  0,
		nextFunc:   0,
		currentPos: 0,
	}
}

func (c *Compiler) DebugPrint() {
	fmt.Println("Bytecode:")
	i := 0
	for i < len(c.code) {
		instr := Instr(c.code[i])

		// Check if this is actually part of a jump instruction
		isJumpOperand := false
		if i > 0 {
			prevInstr := Instr(c.code[i-1])
			if prevInstr == InstrJmp || prevInstr == InstrJmpIfZero ||
				prevInstr == InstrJmpIfNeg || prevInstr == InstrJmpIfPos {
				isJumpOperand = true
			}
		}

		if !isJumpOperand {
			fmt.Printf("%04d: %-12v", i, instr)

			// Instructions that have operands
			switch instr {
			case InstrPush:
				if i+1 < len(c.code) {
					fmt.Printf("\t\x1b[36mvalue: %d\x1b[0m", c.code[i+1])
					i++
				}
			case InstrPop:
				if i+1 < len(c.code) {
					// Find variable name by index
					varName := "?"
					for name, idx := range c.vars {
						if idx == int(c.code[i+1]) {
							varName = name
							break
						}
					}
					if varName == "?" {
						fmt.Printf("\t\x1b[31m→ %s\x1b[0m (var_%d)", varName, c.code[i+1])
					} else {
						fmt.Printf("\t\x1b[36m→ %s (var_%d)\x1b[0m", varName, c.code[i+1])
					}
					i++
				}
			case InstrLoad:
				if i+1 < len(c.code) {
					// Find variable name by index
					varName := "?"
					for name, idx := range c.vars {
						if idx == int(c.code[i+1]) {
							varName = name
							break
						}
					}
					if varName == "?" {
						fmt.Printf("\t\x1b[31mload %s (var_%d)\x1b[0m", varName, c.code[i+1])
					} else {
						fmt.Printf("\t\x1b[36mload %s (var_%d)\x1b[0m", varName, c.code[i+1])
					}
					i++
				}
			case InstrStore:
				if i+1 < len(c.code) {
					// Find variable name by index
					varName := "?"
					for name, idx := range c.vars {
						if idx == int(c.code[i+1]) {
							varName = name
							break
						}
					}
					if varName == "?" {
						fmt.Printf("\t\x1b[31m→ %s (var_%d)\x1b[0m", varName, c.code[i+1])
					} else {
						fmt.Printf("\t\x1b[36m→ %s (var_%d)\x1b[0m", varName, c.code[i+1])
					}
					i++
				}
			case InstrCall:
				if i+2 < len(c.code) {
					// Find function name by index
					funcName := "?"
					funcIdx := int(c.code[i+1])
					for name, idx := range builtinFunctions {
						if idx == funcIdx {
							funcName = name
							break
						}
					}
					if funcName == "?" {
						fmt.Printf("\t\x1b[31m%s (func_%d) with %d args\x1b[0m",
							funcName, c.code[i+1], c.code[i+2])
					} else {
						fmt.Printf("\t\x1b[36m%s (func_%d) with %d args\x1b[0m",
							funcName, c.code[i+1], c.code[i+2])
					}
					i += 2
				}
			case InstrJmp, InstrJmpIfZero, InstrJmpIfNeg, InstrJmpIfPos:
				if i+2 < len(c.code) {
					jumpAddr := (int(c.code[i+1]) << 8) | int(c.code[i+2])
					// Find label by address
					labelName := "?"
					for name, addr := range c.labels {
						if addr == jumpAddr {
							labelName = name
							break
						}
					}
					if labelName == "?" {
						switch instr {
						case InstrJmp:
							fmt.Printf("\t\x1b[31m→ %s (addr: %d)\x1b[0m", labelName, jumpAddr)
						case InstrJmpIfZero:
							fmt.Printf("\t\x1b[31m→ %s (addr: %d) if zero\x1b[0m", labelName, jumpAddr)
						case InstrJmpIfNeg:
							fmt.Printf("\t\x1b[31m→ %s (addr: %d) if neg\x1b[0m", labelName, jumpAddr)
						case InstrJmpIfPos:
							fmt.Printf("\t\x1b[31m→ %s (addr: %d) if pos\x1b[0m", labelName, jumpAddr)
						}
					} else {
						switch instr {
						case InstrJmp:
							fmt.Printf("\t\x1b[36m→ %s (addr: %d)\x1b[0m", labelName, jumpAddr)
						case InstrJmpIfZero:
							fmt.Printf("\t\x1b[36m→ %s (addr: %d) if zero\x1b[0m", labelName, jumpAddr)
						case InstrJmpIfNeg:
							fmt.Printf("\t\x1b[36m→ %s (addr: %d) if neg\x1b[0m", labelName, jumpAddr)
						case InstrJmpIfPos:
							fmt.Printf("\t\x1b[36m→ %s (addr: %d) if pos\x1b[0m", labelName, jumpAddr)
						}
					}
					i += 2
				}
			default:
				fmt.Printf("\t\x1b[36m%v\x1b[0m", instr)
			}
			fmt.Println()
		}
		i++
	}

	// Print symbol tables
	fmt.Println("\nVariables:")
	for name, idx := range c.vars {
		fmt.Printf("  %s -> var_%d\n", name, idx)
	}

	fmt.Println("\nLabels:")
	for name, addr := range c.labels {
		fmt.Printf("  %s -> addr %d\n", name, addr)
	}

	fmt.Println("\nFunctions:")
	for name, idx := range builtinFunctions {
		fmt.Printf("  %s -> func_%d (builtin)\n", name, idx)
	}
	for name, idx := range c.funcs {
		fmt.Printf("  %s -> func_%d\n", name, idx)
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
		endAddr := c.currentPos // Use current position BEFORE setting the label
		c.setLabel(endLabel)    // Now set the label

		// Patch the jump-if-zero instruction with the correct end address
		c.code = append(c.code[:jumpToEndPos], append([]byte{
			byte(endAddr >> 8),
			byte(endAddr & 0xff),
		}, c.code[jumpToEndPos+2:]...)...)

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
	// First compile the arguments
	for _, arg := range call.Args {
		c.compileExpr(arg)
	}

	// Get function index and emit call instruction
	funcIdx := c.getFuncIdx(call.Function)
	c.emit(InstrCall, byte(funcIdx), byte(len(call.Args)))
}
