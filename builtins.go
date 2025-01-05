package main

import "fmt"

// RegisterBuiltins registers all built-in functions with the VM
func RegisterBuiltins(vm *VM) {
	// Print function
	vm.RegisterFunction(builtinFunctions["print"], func(args []Value) Value {
		vm.wg.Add(1)       // Add before print operation
		defer vm.wg.Done() // Ensure Done is called after printing

		for _, arg := range args {
			switch v := arg.(type) {
			case IntValue:
				fmt.Print(int(v))
			case StringValue:
				fmt.Print(vm.currentState.Strings[v.Index])
			}
		}
		return IntValue(0)
	})
}
