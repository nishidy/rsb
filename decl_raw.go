package main

import (
	"bufio"
	"fmt"
	"github.com/go-clang/bootstrap/clang"
	"os"
	"strings"
)

func is_func(ln string) bool {
	return !strings.ContainsAny(ln, "#;")
}

func reset(s []string) {
	s = s[:0]
}

func get_func_name(s []string) string {
	func_decl := strings.Split(strings.Join(s, " "), "(")
	if len(func_decl) > 1 {
		tokens := strings.Split(strings.TrimSpace(func_decl[0]), " ")
		if len(tokens) > 0 {
			func_name := tokens[len(tokens)-1]
			if len(func_name) == 0 {
				//panic("The function name is not taken successfully.")
				return ""
			}
			if func_name[0] == '*' {
				return func_name[1:]
			} else {
				return func_name
			}
		}
	}
	return ""
}

func (t *Trace) get_decls_by_raw(path string) Decls {
	fd, err := os.Open(path)
	if err != nil {
		panic(err.Error())
	}

	scope := 0
	var line uint32 = 0
	comment := false

	var decls Decls
	var func_decl []string
	sc := bufio.NewScanner(fd)
	for sc.Scan() {
		ln := sc.Text()
		line += 1

		if strings.Contains(ln, "/*") {
			comment = true
		}

		if strings.Contains(ln, "*/") {
			comment = false
		}

		if !comment {

			if ln == "" {
				continue
			}

			real_ln := ""
			if strings.Contains(ln, "//") {
				real_ln = strings.Split(ln, "//")[0]
			} else if strings.Contains(ln, "/*") {
				real_ln = strings.Split(ln, "/*")[0]
			} else {
				real_ln = ln
			}

			if is_func(real_ln) && scope == 0 {
				func_decl = append(func_decl, real_ln)
			}

			if c := strings.Count(real_ln, "{"); c > 0 {
				scope += c
			}

			if c := strings.Count(real_ln, "}"); c > 0 {
				scope -= c

				if scope == 0 && len(func_decl) > 0 {
					if func_name := get_func_name(func_decl); func_name == "" {
						//fmt.Println("No function name found.")
					} else {
						decls = append(decls, Decl{line, clang.Cursor_FunctionDecl, func_name})
					}
					reset(func_decl)
				}
			}
		}

		if scope < 0 {
			fmt.Println(line)
			panic("Scope must not be negative.")
		}
	}

	return decls
}
