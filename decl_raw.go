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

func getStructName(s string) string {
	tokens := strings.Split(s, " ")
	for i, token := range tokens {
		if token == "struct" {
			if i < len(tokens)-2 {
				return tokens[i+2]
			}
		}
	}
	return ""
}

func getFuncName(s string) string {
	func_decl := strings.Split(s, "(")
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
		s = strings.Replace(s, "\\\"", "", -1)
	}

	for {
		if strings.Contains(s, "\"") {
			l := strings.Index(s, "\"")
			r := strings.Index(s[l+1:], "\"") + (l + 1) + 1
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
			r := strings.Index(s[l+1:], "'") + (l + 1) + 1
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
		if strings.Contains(s, "/*") && strings.Contains(s, "*/") {
			l := strings.Index(s, "/*")
			r := strings.Index(s[l+1:], "*/") + (l + 1) + 2
			if len(s) < r {
				s = s[:l]
			} else {
				s = s[:l] + s[r:]
			}
		} else {
			break
		}
	}

	if strings.Contains(s, "//") {
		s = strings.Split(s, "//")[0]
	}

	return s
}

func exclude_comment_start(s string) (string, bool) {
	comment_start := false
	if strings.Contains(s, "/*") {
		l := strings.Index(s, "/*")
		s = s[:l]
		comment_start = true
	}

	return s, comment_start
}

func exclude_comment_end(s string) (string, bool) {
	comment_end := false
	if strings.Contains(s, "*/") {
		r := strings.Index(s, "*/") + 2
		if len(s) < r {
			s = ""
		} else {
			s = s[r:]
		}
		comment_end = true
	}

	return s, comment_end
}

func (t *Trace) getDeclsByRaw(path string) Decls {

	fd, err := os.Open(path)
	defer fd.Close()

	if err != nil {
		panic(err.Error())
	}

	global_scope := 0
	module_scope := 0

	var line uint32 = 0

	var decls Decls
	var decl_slice []string
	sc := bufio.NewScanner(fd)

	real_ln := ""
	comment := false
	comment_start := false
	comment_end := false

	for sc.Scan() {

		ln := sc.Text()
		line += 1

		real_ln = exclude(ln)

		real_ln, comment_start = exclude_comment_start(real_ln)

		real_ln, comment_end = exclude_comment_end(real_ln)

		if comment_end {
			comment = false
		}

		if !comment {

			if (global_scope - module_scope) == 0 {
				if isNotFunc(real_ln) {
					reset(&decl_slice)
				} else {
					decl_slice = append(decl_slice, real_ln)
				}
			}

			if c := strings.Count(real_ln, "{"); c > 0 {

				if (global_scope - module_scope) == 0 {
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

				if (global_scope-module_scope) == 0 && len(decl_slice) > 0 {

					decl_str := strings.Join(decl_slice, " ")
					if func_name := getFuncName(decl_str); func_name == "" {
						//fmt.Println("No function name found.")
						if struct_name := getStructName(decl_str); struct_name == "" {
							//fmt.Println("No struct name found.")
						} else {
							decls = append(decls, Decl{line, clang.Cursor_StructDecl, struct_name, decl_str})
						}
					} else {
						decls = append(decls, Decl{line, clang.Cursor_FunctionDecl, func_name, decl_str})
					}
					reset(&decl_slice)
				}
			}

			if global_scope < 0 {
				fmt.Println(path, line)
				panic("Scope must not be negative.")
			}
		}

		if comment_start {
			comment = true
		}

	}

	return decls
}
