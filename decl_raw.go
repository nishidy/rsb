package main

import (
	"bufio"
	"fmt"
	"github.com/go-clang/bootstrap/clang"
	"os"
	"strings"
)

func isNotFunc(ln string) bool {
	return strings.ContainsAny(ln, "#;")
}

func reset(s *[]string) {
	*s = []string{}
}

func getFuncName(s []string) string {
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

func exclude(s string) string {
	if strings.Contains(s, "\\\"") {
		s = strings.Replace(s,"\\\"","",-1)
	}

	for {
		if strings.Contains(s, "\"") {
			l := strings.Index(s, "\"")
			r := strings.Index(s[l+1:], "\"") + (l+1) + 1
			if r == -1 || len(s) < r {
				s = s[:l]
			} else {
				s = s[:l] + s[r:]
			}
		} else {
			break
		}
	}

	for {
		if strings.Contains(s, "'") {
			l := strings.Index(s, "'")
			r := strings.Index(s[l+1:], "'") + (l+1) + 1
			if r == -1 || len(s) < r {
				s = s[:l]
			} else {
				s = s[:l] + s[r:]
			}
		} else {
			break
		}
	}

	for {
		if strings.Contains(s, "/*") && strings.Contains(s,"*/") {
			l := strings.Index(s, "/*")
			r := strings.Index(s[l+1:], "*/") + (l+1) + 2
			if len(s) < r {
				s = s[:l]
			} else {
				s = s[:l] + s[r:]
			}
		} else {
			break
		}
	}

	if strings.Contains(s,"/*") {
		l := strings.Index(s, "/*")
		s = s[:l]
	}

	if strings.Contains(s,"*/") {
		r := strings.Index(s, "*/") + 2
		if len(s) < r {
			s = ""
		} else {
			s = s[r:]
		}
	}

	if strings.Contains(s, "//") {
		s = strings.Split(s, "//")[0]
	}

	return s
}

func (t *Trace) getDeclsByRaw(path string) Decls {

	fd, err := os.Open(path)
	defer fd.Close()

	if err != nil {
		panic(err.Error())
	}

	global_scope := 0
	module_scope:= 0

	var line uint32 = 0

	var decls Decls
	var func_decl []string
	sc := bufio.NewScanner(fd)
	for sc.Scan() {
		ln := sc.Text()
		line += 1

		real_ln := exclude(ln)

		if real_ln == "" {
			continue
		}

		if ( global_scope - module_scope ) == 0 {
			if isNotFunc(real_ln) {
				reset(&func_decl)
			} else {
				func_decl = append(func_decl, real_ln)
			}
		}

		if c := strings.Count(real_ln, "{"); c > 0 {

			if ( global_scope - module_scope ) == 0 {
				if strings.Contains(real_ln, "namespace") ||
					strings.Contains(real_ln, "extern") {
					module_scope += 1
				}
			}

			global_scope += c
			//fmt.Println(path,line,"+",global_scope)
		}

		if c := strings.Count(real_ln, "}"); c > 0 {
			global_scope -= c
			//fmt.Println(path,line,"-",global_scope)

			if global_scope < module_scope {
				module_scope -= 1
			}

			if ( global_scope - module_scope ) == 0 && len(func_decl) > 0 {

				if func_name := getFuncName(func_decl); func_name == "" {
					//fmt.Println("No function name found.")
				} else {
					decls = append(decls, Decl{line, clang.Cursor_FunctionDecl, func_name})
				}
				reset(&func_decl)
			}
		}

		if global_scope < 0 {
			fmt.Println(path,line)
			panic("Scope must not be negative.")
		}
	}

	return decls
}
