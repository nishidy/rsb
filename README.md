# Purpose

This is to run a Recursive Static Backtrace for C code.
This runs depth-first search for functions with goroutines and show the backtrace tree after completing the tree.
Supported for the use in vim command.

```
$ bt ENTRYFILE ENTRYLINE ROOTDIR MAXBACKTRACELEVEL
```

# Installation

Necessary to install go-clang/bootstrap.

https://github.com/go-clang/bootstrap

# Note

Currently, clang is only used for variable definitions. It is ToDo as of now to implement clang for totally safe tracing. In addition to that, directives are not treated properly.

