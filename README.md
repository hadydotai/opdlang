# Opd Lang

Naming is hard okay. Anyway, Opd language is a bytecode compiled toy language
that's not currently meant to be useful. In fact, about the only thing it does
is print strings and do basic math.

The gist of it though is the debugger. Opd is bytecode compiled, here's an
example of a bytecode dump for [simple.dl](./samples/simple.dl)

```sh
go run . compile samples/simple.dl -o simple.bc -r -lnone -d
```

![simple.dl bytecode dump](./public/simple.dl-bytecodedump.png)

Running `go run . compile -h` will give you all the details you need to know
about the flags but for a quick reference regarding the above command:

- `-o` sets the output file for the compiled bytecode
- `-r` will run the compiled bytecode
- `-l` will set the logging level to `none`, we're only interested in the
  program's output
- `-d` will dump the bytecode in the format you see above for inspection

This is all well dandy, the real thing here is the debugging. I wanted to
implement a step, time-travelling debugger. The goal (as you'll see is evident
in the code) was to quickly get from point A to point B so I can start
prototyping the debugger. The compiler is, lackluster to say the least but is
stable.

### Current bytecode limitiations

There's one glaring limitation in the current implementation of the compiler
which I alluded to in [run.go](./run.go) is that we don't store enough of the
metadata with the bytecode needed for the VM and the debugger to run straight
from the bytecode.

And the things we do store we don't read back from the bytecode file. The result
of that is we need to go from AST->Bytecode->VM/Debugger. Hopefully will have
time to fix it in the near future, for now it serves the original purpose.
