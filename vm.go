package main

import (
	"fmt"
	"sync"
)

type DebuggerCmd int

const (
	DebuggerCmdContinue DebuggerCmd = iota
	DebuggerCmdStepNext
	DebuggerCmdStepBack
	DebuggerCmdPause
)

type VMState struct {
	PC          int
	Stack       []Value
	Locals      []Value
	Memory      []byte
	CallStack   []int
	ReturnStack []int
	Strings     []string
	SourceLine  int
}

func (vm *VMState) Clone() *VMState {
	newState := &VMState{
		PC:          vm.PC,
		Stack:       make([]Value, len(vm.Stack)),
		Locals:      make([]Value, len(vm.Locals)),
		Memory:      make([]byte, len(vm.Memory)),
		CallStack:   make([]int, len(vm.CallStack)),
		ReturnStack: make([]int, len(vm.ReturnStack)),
		Strings:     make([]string, len(vm.Strings)),
		SourceLine:  vm.SourceLine,
	}
	copy(newState.Stack, vm.Stack)
	copy(newState.Locals, vm.Locals)
	copy(newState.Memory, vm.Memory)
	copy(newState.CallStack, vm.CallStack)
	copy(newState.ReturnStack, vm.ReturnStack)
	copy(newState.Strings, vm.Strings)
	return newState
}

type Instr byte

const (
	InstrPush Instr = iota
	InstrPushStr
	InstrPop
	InstrAdd
	InstrSub
	InstrMul
	InstrDiv
	InstrMod
	InstrEq
	InstrNeq
	InstrLt
	InstrGt
	InstrLte
	InstrGte
	InstrLoad
	InstrStore
	InstrJmp
	InstrJmpIfZero
	InstrCall
	InstrRet
	InstrHalt
)

func (instr Instr) String() string {
	names := []string{
		"PUSH", "PUSH_STR", "POP", "ADD", "SUB", "MUL", "DIV", "MOD",
		"EQ", "NEQ", "LT", "GT", "LTE", "GTE", "LOAD",
		"STORE", "JMP", "JMP_IF_ZERO", "CALL", "RET", "HALT",
	}
	if int(instr) < len(names) {
		return names[instr]
	}
	return fmt.Sprintf("UNKNOWN(%d)", instr)
}

type ValueType int

const (
	ValueTypeInt ValueType = iota
	ValueTypeString
)

type Value interface {
	Type() ValueType
}

type IntValue int

func (i IntValue) Type() ValueType { return ValueTypeInt }

type StringValue struct {
	Index int
}

func (s StringValue) Type() ValueType { return ValueTypeString }

type GoFunction func(args []Value) Value

type VM struct {
	currentState *VMState
	history      []*VMState
	bytecode     []byte

	debugChan chan DebuggerCmd
	stateChan chan *VMState

	mu              sync.RWMutex
	running         bool
	functions       map[int]GoFunction
	sourceMap       map[int]int
	lineBreakpoints map[int]bool
}

func NewVmState(bytecode []byte, stackSize, localsSize int) *VMState {
	return &VMState{
		Stack:      make([]Value, 0, stackSize),
		Locals:     make([]Value, 0, localsSize),
		Memory:     make([]byte, 0, 1024),
		Strings:    make([]string, 0),
		SourceLine: 1,
	}
}

func NewVM(bytecode []byte, stackSize, localsSize int) *VM {
	return &VM{
		bytecode:        bytecode,
		currentState:    NewVmState(bytecode, stackSize, localsSize),
		debugChan:       make(chan DebuggerCmd),
		stateChan:       make(chan *VMState),
		history:         make([]*VMState, 0),
		running:         false,
		functions:       make(map[int]GoFunction),
		sourceMap:       make(map[int]int),
		lineBreakpoints: make(map[int]bool),
	}
}

// func (vm *VM) SetBreakpoint(pc int, enabled bool) {
// 	vm.mu.Lock()
// 	defer vm.mu.Unlock()
// 	vm.breakpoints[pc] = enabled
// 	if !enabled {
// 		delete(vm.breakpoints, pc)
// 	}
// }

func (vm *VM) SetLineBreakpoint(line int, enabled bool) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.lineBreakpoints[line] = enabled
	if !enabled {
		delete(vm.lineBreakpoints, line)
	}
}

func (vm *VM) Run() {
	vm.mu.Lock()
	// Clear any pending states from previous runs
	select {
	case <-vm.stateChan:
	default:
	}
	vm.running = true
	vm.mu.Unlock()

	// Start with sending initial state
	go func() {
		vm.stateChan <- vm.currentState.Clone()
		vm.execute()
	}()
}

func (vm *VM) RunBlock() {
	vm.running = true
	vm.execute()
}

func (vm *VM) RunUntil(pc int) {
	vm.running = true
	vm.currentState.PC = pc
	go vm.execute()
}

func (vm *VM) Debug(cmd DebuggerCmd) *VMState {
	// Don't start a new execution if already running
	if !vm.running {
		vm.Run()
	}

	// Send command and wait for response
	vm.debugChan <- cmd
	return <-vm.stateChan
}

func (vm *VM) StepNext() *VMState {
	return vm.Debug(DebuggerCmdStepNext)
}

func (vm *VM) StepBack() *VMState {
	return vm.Debug(DebuggerCmdStepBack)
}

func (vm *VM) Pause() {
	vm.Debug(DebuggerCmdPause)
}

func (vm *VM) Continue() {
	vm.Debug(DebuggerCmdContinue)
}

func (vm *VM) State() *VMState {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	return vm.currentState.Clone()
}

func (vm *VM) execute() {
	for vm.running && vm.currentState.PC < len(vm.bytecode) {
		cmd := <-vm.debugChan

		switch cmd {
		case DebuggerCmdPause:
			vm.running = false
			vm.stateChan <- vm.currentState.Clone()
			return

		case DebuggerCmdStepNext:
			err := vm.stepToNextLine()
			if err != nil {
				fmt.Println("Execution error:", err)
				vm.running = false
				vm.stateChan <- vm.currentState.Clone()
				return
			}
			vm.stateChan <- vm.currentState.Clone()

		case DebuggerCmdStepBack:
			if len(vm.history) > 0 {
				vm.stepToPreviousLine()
				vm.stateChan <- vm.currentState.Clone()
			} else {
				vm.stateChan <- vm.currentState.Clone()
			}
		case DebuggerCmdContinue:
			for vm.running && vm.currentState.PC < len(vm.bytecode) {
				currentLine := vm.sourceMap[vm.currentState.PC]
				if vm.lineBreakpoints[currentLine] {
					break
				}
				err := vm.executeInstruction()
				if err != nil {
					fmt.Println("Execution error:", err)
					vm.running = false
					vm.stateChan <- vm.currentState.Clone()
					return
				}
				vm.history = append(vm.history, vm.currentState.Clone())
			}
			vm.stateChan <- vm.currentState.Clone()
		}
	}

	// Program finished
	vm.mu.Lock()
	vm.running = false
	vm.mu.Unlock()

	// Send final state
	vm.stateChan <- vm.currentState.Clone()
}

func (vm *VM) executeInstruction() error {
	instruction := vm.bytecode[vm.currentState.PC]
	// fmt.Printf("Executing instruction at PC=%d: %v\n", vm.currentState.PC, Instr(instruction))
	// fmt.Printf("Stack before: %v\n", vm.currentState.Stack)
	vm.currentState.PC++

	switch inst := Instr(instruction); inst {
	case InstrPush:
		return vm.executePush()
	case InstrPop:
		return vm.executePop()
	case InstrAdd:
		return vm.executeAdd()
	case InstrSub:
		return vm.executeSub()
	case InstrMul:
		return vm.executeMul()
	case InstrDiv:
		return vm.executeDiv()
	case InstrMod:
		return vm.executeMod()
	case InstrEq:
		return vm.executeEq()
	case InstrNeq:
		return vm.executeNeq()
	case InstrLt:
		return vm.executeLt()
	case InstrGt:
		return vm.executeGt()
	case InstrLte:
		return vm.executeLte()
	case InstrGte:
		return vm.executeGte()
	case InstrLoad:
		return vm.executeLoad()
	case InstrStore:
		return vm.executeStore()
	case InstrJmp:
		return vm.executeJmp()
	case InstrJmpIfZero:
		return vm.executeJmpIfZero()
	case InstrCall:
		return vm.executeCall()
	case InstrRet:
		return vm.executeRet()
	case InstrHalt:
		return vm.executeHalt()
	case InstrPushStr:
		return vm.executePushStr()
	default:
		return fmt.Errorf("unknown instruction: %d", instruction)
	}

	// fmt.Printf("Stack after: %v\n", vm.currentState.Stack)
	// return nil
}

func (vm *VM) executePush() error {
	if vm.currentState.PC >= len(vm.bytecode) {
		return fmt.Errorf("program counter out of bounds")
	}
	value := int(vm.bytecode[vm.currentState.PC])
	vm.currentState.Stack = append(vm.currentState.Stack, IntValue(value))
	vm.currentState.PC++
	return nil
}

func (vm *VM) executePop() error {
	if len(vm.currentState.Stack) == 0 {
		return fmt.Errorf("stack underflow")
	}
	if vm.currentState.PC >= len(vm.bytecode) {
		return fmt.Errorf("program counter out of bounds")
	}
	varIdx := int(vm.bytecode[vm.currentState.PC])
	if varIdx >= len(vm.currentState.Locals) {
		return fmt.Errorf("variable index out of bounds: %d", varIdx)
	}
	vm.currentState.Locals[varIdx] = vm.currentState.Stack[len(vm.currentState.Stack)-1]
	vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-1]
	vm.currentState.PC++
	return nil
}

func (vm *VM) executeAdd() error {
	if len(vm.currentState.Stack) < 2 {
		return fmt.Errorf("stack underflow")
	}
	b := vm.currentState.Stack[len(vm.currentState.Stack)-1]
	a := vm.currentState.Stack[len(vm.currentState.Stack)-2]
	vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-2]

	switch va := a.(type) {
	case IntValue:
		if vb, ok := b.(IntValue); ok {
			vm.currentState.Stack = append(vm.currentState.Stack, IntValue(int(va)+int(vb)))
			return nil
		}
	case StringValue:
		if vb, ok := b.(StringValue); ok {
			// String concatenation
			newStr := vm.currentState.Strings[va.Index] + vm.currentState.Strings[vb.Index]
			newIdx := vm.RegisterString(newStr)
			vm.currentState.Stack = append(vm.currentState.Stack, StringValue{Index: newIdx})
			return nil
		}
	}
	return fmt.Errorf("invalid operand types for add")
}

func (vm *VM) executeSub() error {
	if len(vm.currentState.Stack) < 2 {
		return fmt.Errorf("stack underflow")
	}
	a := vm.currentState.Stack[len(vm.currentState.Stack)-1]
	b := vm.currentState.Stack[len(vm.currentState.Stack)-2]
	vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-2]

	if va, ok := a.(IntValue); ok {
		if vb, ok := b.(IntValue); ok {
			vm.currentState.Stack = append(vm.currentState.Stack, IntValue(int(va)-int(vb)))
			return nil
		}
	}
	return fmt.Errorf("invalid operand types for sub")
}

func (vm *VM) executeMul() error {
	if len(vm.currentState.Stack) < 2 {
		return fmt.Errorf("stack underflow")
	}
	b := vm.currentState.Stack[len(vm.currentState.Stack)-1]
	a := vm.currentState.Stack[len(vm.currentState.Stack)-2]
	vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-2]

	if va, ok := a.(IntValue); ok {
		if vb, ok := b.(IntValue); ok {
			vm.currentState.Stack = append(vm.currentState.Stack, IntValue(int(va)*int(vb)))
			return nil
		}
	}
	return fmt.Errorf("invalid operand types for mul")
}

func (vm *VM) executeDiv() error {
	if len(vm.currentState.Stack) < 2 {
		return fmt.Errorf("stack underflow")
	}
	a := vm.currentState.Stack[len(vm.currentState.Stack)-1]
	b := vm.currentState.Stack[len(vm.currentState.Stack)-2]
	vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-2]

	if va, ok := a.(IntValue); ok {
		if vb, ok := b.(IntValue); ok {
			vm.currentState.Stack = append(vm.currentState.Stack, IntValue(int(va)/int(vb)))
			return nil
		}
	}
	return fmt.Errorf("invalid operand types for div")
}

func (vm *VM) executeMod() error {
	if len(vm.currentState.Stack) < 2 {
		return fmt.Errorf("stack underflow")
	}
	a := vm.currentState.Stack[len(vm.currentState.Stack)-1]
	b := vm.currentState.Stack[len(vm.currentState.Stack)-2]
	vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-2]

	if va, ok := a.(IntValue); ok {
		if vb, ok := b.(IntValue); ok {
			vm.currentState.Stack = append(vm.currentState.Stack, IntValue(int(va)%int(vb)))
			return nil
		}
	}
	return fmt.Errorf("invalid operand types for mod")
}

func (vm *VM) executeEq() error {
	if len(vm.currentState.Stack) < 2 {
		return fmt.Errorf("stack underflow")
	}
	b := vm.currentState.Stack[len(vm.currentState.Stack)-1]
	a := vm.currentState.Stack[len(vm.currentState.Stack)-2]
	vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-2]

	switch va := a.(type) {
	case IntValue:
		if vb, ok := b.(IntValue); ok {
			result := 0
			if int(va) == int(vb) {
				result = 1
			}
			vm.currentState.Stack = append(vm.currentState.Stack, IntValue(result))
			return nil
		}
	case StringValue:
		if vb, ok := b.(StringValue); ok {
			result := 0
			if vm.currentState.Strings[va.Index] == vm.currentState.Strings[vb.Index] {
				result = 1
			}
			vm.currentState.Stack = append(vm.currentState.Stack, IntValue(result))
			return nil
		}
	}
	return fmt.Errorf("invalid operand types for equality")
}

func (vm *VM) executeNeq() error {
	if len(vm.currentState.Stack) < 2 {
		return fmt.Errorf("stack underflow")
	}
	b := vm.currentState.Stack[len(vm.currentState.Stack)-1]
	a := vm.currentState.Stack[len(vm.currentState.Stack)-2]
	vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-2]
	switch va := a.(type) {
	case IntValue:
		if vb, ok := b.(IntValue); ok {
			result := 0
			if int(va) != int(vb) {
				result = 1
			}
			vm.currentState.Stack = append(vm.currentState.Stack, IntValue(result))
			return nil
		}
	case StringValue:
		if vb, ok := b.(StringValue); ok {
			result := 0
			if vm.currentState.Strings[va.Index] != vm.currentState.Strings[vb.Index] {
				result = 1
			}
			vm.currentState.Stack = append(vm.currentState.Stack, IntValue(result))
			return nil
		}
	}
	return nil
}

func (vm *VM) executeLt() error {
	if len(vm.currentState.Stack) < 2 {
		return fmt.Errorf("stack underflow")
	}
	b := vm.currentState.Stack[len(vm.currentState.Stack)-1]
	a := vm.currentState.Stack[len(vm.currentState.Stack)-2]
	vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-2]

	if va, ok := a.(IntValue); ok {
		if vb, ok := b.(IntValue); ok {
			result := 0
			if int(va) < int(vb) {
				result = 1
			}
			vm.currentState.Stack = append(vm.currentState.Stack, IntValue(result))
			return nil
		}
	}
	return fmt.Errorf("invalid operand types for less than")
}

func (vm *VM) executeGt() error {
	if len(vm.currentState.Stack) < 2 {
		return fmt.Errorf("stack underflow")
	}
	a := vm.currentState.Stack[len(vm.currentState.Stack)-1]
	b := vm.currentState.Stack[len(vm.currentState.Stack)-2]
	vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-2]
	if va, ok := a.(IntValue); ok {
		if vb, ok := b.(IntValue); ok {
			result := 0
			if int(va) > int(vb) {
				result = 1
			}
			vm.currentState.Stack = append(vm.currentState.Stack, IntValue(result))
			return nil
		}
	}
	return nil
}

func (vm *VM) executeLte() error {
	if len(vm.currentState.Stack) < 2 {
		return fmt.Errorf("stack underflow")
	}
	a := vm.currentState.Stack[len(vm.currentState.Stack)-1]
	b := vm.currentState.Stack[len(vm.currentState.Stack)-2]
	vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-2]
	if va, ok := a.(IntValue); ok {
		if vb, ok := b.(IntValue); ok {
			result := 0
			if int(va) <= int(vb) {
				result = 1
			}
			vm.currentState.Stack = append(vm.currentState.Stack, IntValue(result))
			return nil
		}
	}
	return nil
}

func (vm *VM) executeGte() error {
	if len(vm.currentState.Stack) < 2 {
		return fmt.Errorf("stack underflow")
	}
	a := vm.currentState.Stack[len(vm.currentState.Stack)-1]
	b := vm.currentState.Stack[len(vm.currentState.Stack)-2]
	vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-2]
	if va, ok := a.(IntValue); ok {
		if vb, ok := b.(IntValue); ok {
			result := 0
			if int(va) >= int(vb) {
				result = 1
			}
			vm.currentState.Stack = append(vm.currentState.Stack, IntValue(result))
			return nil
		}
	}
	return nil
}

func (vm *VM) executeLoad() error {
	if vm.currentState.PC >= len(vm.bytecode) {
		return fmt.Errorf("program counter out of bounds")
	}
	varIdx := int(vm.bytecode[vm.currentState.PC])
	if varIdx >= len(vm.currentState.Locals) {
		return fmt.Errorf("variable index out of bounds: %d", varIdx)
	}
	vm.currentState.Stack = append(vm.currentState.Stack, vm.currentState.Locals[varIdx])
	vm.currentState.PC++
	return nil
}

func (vm *VM) executeStore() error {
	if vm.currentState.PC >= len(vm.bytecode) {
		return fmt.Errorf("program counter out of bounds")
	}
	if len(vm.currentState.Stack) == 0 {
		return fmt.Errorf("stack underflow")
	}
	varIdx := int(vm.bytecode[vm.currentState.PC])
	if varIdx >= len(vm.currentState.Locals) {
		vm.currentState.Locals = append(vm.currentState.Locals, nil)
	}
	vm.currentState.Locals[varIdx] = vm.currentState.Stack[len(vm.currentState.Stack)-1]
	vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-1]
	vm.currentState.PC++
	return nil
}

func (vm *VM) executeJmp() error {
	// Read two bytes for jump address
	if vm.currentState.PC+1 >= len(vm.bytecode) {
		return fmt.Errorf("invalid jump address")
	}
	highByte := int(vm.bytecode[vm.currentState.PC])
	lowByte := int(vm.bytecode[vm.currentState.PC+1])
	jumpAddr := (highByte << 8) | lowByte
	vm.currentState.PC = jumpAddr
	return nil
}

func (vm *VM) executeJmpIfZero() error {
	if len(vm.currentState.Stack) == 0 {
		return fmt.Errorf("stack underflow")
	}
	// Read two bytes for jump address
	if vm.currentState.PC+1 >= len(vm.bytecode) {
		return fmt.Errorf("invalid jump address")
	}
	highByte := int(vm.bytecode[vm.currentState.PC])
	lowByte := int(vm.bytecode[vm.currentState.PC+1])
	jumpAddr := (highByte << 8) | lowByte

	condition := vm.currentState.Stack[len(vm.currentState.Stack)-1]
	// Pop the condition value
	vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-1]

	if condition == IntValue(0) {
		vm.currentState.PC = jumpAddr
	} else {
		vm.currentState.PC += 2 // Skip over jump address
	}
	return nil
}

func (vm *VM) executeCall() error {
	funcIdx := int(vm.bytecode[vm.currentState.PC])
	numArgs := int(vm.bytecode[vm.currentState.PC+1])

	if fn, ok := vm.functions[funcIdx]; ok {
		// Get arguments in the correct order
		args := make([]Value, numArgs)
		for i := numArgs - 1; i >= 0; i-- {
			if len(vm.currentState.Stack) == 0 {
				return fmt.Errorf("stack underflow while getting function arguments")
			}
			args[i] = vm.currentState.Stack[len(vm.currentState.Stack)-1]
			vm.currentState.Stack = vm.currentState.Stack[:len(vm.currentState.Stack)-1]
		}

		result := fn(args)
		vm.currentState.Stack = append(vm.currentState.Stack, result)
		vm.currentState.PC += 2
		return nil
	}

	vm.currentState.CallStack = append(vm.currentState.CallStack, vm.currentState.PC)
	vm.currentState.PC = int(vm.currentState.Locals[vm.currentState.PC].(IntValue))
	vm.currentState.PC++
	return fmt.Errorf("unknown function index: %d", funcIdx)
}

func (vm *VM) executeRet() error {
	if len(vm.currentState.CallStack) == 0 {
		return fmt.Errorf("call stack underflow")
	}
	vm.currentState.PC = vm.currentState.CallStack[len(vm.currentState.CallStack)-1]
	vm.currentState.CallStack = vm.currentState.CallStack[:len(vm.currentState.CallStack)-1]
	vm.currentState.PC++
	return nil
}

func (vm *VM) executeHalt() error {
	vm.running = false
	return nil
}

func (vm *VM) executePushStr() error {
	if vm.currentState.PC >= len(vm.bytecode) {
		return fmt.Errorf("program counter out of bounds")
	}
	strIdx := int(vm.bytecode[vm.currentState.PC])
	if strIdx >= len(vm.currentState.Strings) {
		return fmt.Errorf("string index out of bounds: %d", strIdx)
	}
	vm.currentState.Stack = append(vm.currentState.Stack, StringValue{Index: strIdx})
	vm.currentState.PC++
	return nil
}

func (vm *VM) RegisterFunction(idx int, fn GoFunction) {
	vm.functions[idx] = fn
}

func (vm *VM) RegisterString(s string) int {
	vm.currentState.Strings = append(vm.currentState.Strings, s)
	return len(vm.currentState.Strings) - 1
}

func (vm *VM) RegisterStrings(strings map[string]int) {
	// Pre-allocate space in the strings slice
	maxIdx := -1
	for _, idx := range strings {
		if idx > maxIdx {
			maxIdx = idx
		}
	}

	// Resize the strings slice if needed
	if maxIdx >= len(vm.currentState.Strings) {
		newStrings := make([]string, maxIdx+1)
		copy(newStrings, vm.currentState.Strings)
		vm.currentState.Strings = newStrings
	}

	// Register all strings at their correct indices
	for str, idx := range strings {
		vm.currentState.Strings[idx] = str
	}
}

func (vm *VM) RegisterSourceMap(pc, line int) {
	vm.sourceMap[pc] = line
}

func (vm *VM) stepToNextLine() error {
	currentLine := vm.sourceMap[vm.currentState.PC]

	for vm.currentState.PC < len(vm.bytecode) {
		// Store the current state BEFORE executing the instruction
		vm.history = append(vm.history, vm.currentState.Clone())

		err := vm.executeInstruction()
		if err != nil {
			return err
		}

		// If we've reached an instruction from a different line, stop
		if newLine := vm.sourceMap[vm.currentState.PC]; newLine != currentLine && newLine != 0 {
			vm.currentState.SourceLine = newLine
			return nil
		}
	}
	return nil
}

func (vm *VM) stepToPreviousLine() {
	if len(vm.history) == 0 {
		return
	}

	currentLine := vm.sourceMap[vm.currentState.PC]
	var previousState *VMState

	for len(vm.history) > 0 {
		previousState = vm.history[len(vm.history)-1].Clone()
		vm.history = vm.history[:len(vm.history)-1]

		if newLine := vm.sourceMap[previousState.PC]; newLine != currentLine && newLine != 0 {
			vm.currentState = previousState
			vm.currentState.SourceLine = newLine
			break
		}
	}
}

// Add a new method to properly stop the VM
func (vm *VM) Stop() {
	vm.mu.Lock()
	vm.running = false
	vm.mu.Unlock()
	// Drain any pending state from the channel
	select {
	case <-vm.stateChan:
	default:
	}
}
