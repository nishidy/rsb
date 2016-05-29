package main

import (
	"bufio"
	"crypto/md5"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/go-clang/bootstrap/clang"
)

const (
	BTHOME = ".bt"
)

type Entry struct {
	file string
	line uint32
}

type Callee struct {
	fun  string
	file string
	line uint32
}

type Trace struct {
	dir      string
	entry    Entry
	callee   Callee
	level    int
	maxlevel int
	result   string
	nodes    []*Trace
	wg       *sync.WaitGroup
}

type Decl struct {
	line uint32
	kind clang.CursorKind
	name string
}

type Decls []Decl

func (d Decls) Less(i, j int) bool {
	return d[i].line < d[j].line
}

func (d Decls) Len() int {
	return len(d)
}

func (d Decls) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}

func (t *Trace) read_1st_ast(path string) {

	idx := clang.NewIndex(1, 0)
	defer idx.Dispose()

	tu := idx.ParseTranslationUnit(path, []string{}, nil, 0)
	cursor := tu.TranslationUnitCursor()

	decls := Decls{}
	cursor.Visit(func(cursor, parent clang.Cursor) clang.ChildVisitResult {

		_, lines, _, _ := cursor.Location().ExpansionLocation()

		switch cursor.Kind() {
		case clang.Cursor_FunctionDecl:
			decls = append(decls, Decl{lines, clang.Cursor_FunctionDecl, cursor.Spelling()})
		case clang.Cursor_StructDecl:
			decls = append(decls, Decl{lines, clang.Cursor_StructDecl, cursor.Spelling()})
		}

		switch cursor.Kind() {
		case clang.Cursor_ClassDecl,
			clang.Cursor_EnumDecl,
			clang.Cursor_StructDecl,
			clang.Cursor_Namespace,
			clang.Cursor_FunctionDecl,
			clang.Cursor_CompoundStmt:
			return clang.ChildVisit_Recurse
		}

		return clang.ChildVisit_Continue
	})

	sort.Sort(decls)

	predecl := Decl{}
	for _, decl := range decls {

		if t.entry.line <= decl.line {

			result := ""
			switch predecl.kind {
			case clang.Cursor_FunctionDecl:
				result = fmt.Sprintf("-1- Entry point %s@L%d in \x1b[34m%s\x1b[0m function scope.\n",
					t.entry.file, t.entry.line, predecl.name)

			case clang.Cursor_StructDecl:
				result = fmt.Sprintf("-1- Entry point %s@L%d in \x1b[31m%s\x1b[0m struct scope.\n",
					t.entry.file, t.entry.line, predecl.name)

			}

			print_cached_result(path, predecl.name)

			callee := Callee{predecl.name, path, predecl.line}
			trace := Trace{t.dir, Entry{}, callee, 2, t.maxlevel, result, nil, t.wg}
			t.nodes = append(t.nodes, &trace)

			filepath.Walk(trace.dir, trace.recur_visit)

			break
		}

		switch decl.kind {
		case clang.Cursor_FunctionDecl,
			clang.Cursor_StructDecl:
			predecl = decl
		}

	}

}

func (t *Trace) recur_visit(path string, info os.FileInfo, err error) error {

	file := filepath.Base(path)

	if strings.HasPrefix(file, ".") {
	} else {

		file_slice := strings.Split(file, ".")
		ext := file_slice[len(file_slice)-1]

		if ext == "c" || ext == "h" {
			if t.level == 1 {
				if t.entry.file == path {
					t.read_1st_ast(path)
				}
			} else if t.level <= t.maxlevel {
				t.read_nth_ast(path)
			}
		}
	}
	return nil
}

func get_home_env() string {
	for _, env := range os.Environ() {
		if strings.Contains(env, "HOME=") {
			return strings.Split(env, "=")[1]
		}
	}
	return ""
}

func get_hashed_dir(file_path, func_name string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(file_path+func_name)))
}

func dir_exists(abs_path string) bool {
	_, err := os.Stat(abs_path)
	return err == nil
}

func get_abs_hashed_dir(file_path, func_name string) string {
	home_path := get_home_env()
	hashed_dir := get_hashed_dir(file_path, func_name)
	abs_hashed_dir := filepath.Join(home_path, BTHOME, hashed_dir)
	return abs_hashed_dir
}

func get_cached_result(file_path, func_name string) (string, error) {

	abs_hashed_dir := get_abs_hashed_dir(file_path, func_name)

	if dir_exists(abs_hashed_dir) {
		result, err := ioutil.ReadFile(filepath.Join(abs_hashed_dir, "result"))
		if err != nil {
			os.Exit(12)
		}
		return string(result), nil
	} else {
		return "", errors.New("Dir not exists.")
	}

}

func print_cached_result(path, func_name string) {

	file_path, err := filepath.Abs(path)
	if err != nil {
		os.Exit(10)
	}

	result, err := get_cached_result(file_path, func_name)
	if err != nil {
	} else {
		fmt.Println("# Show cached result.")
		fmt.Println(result)
		fmt.Println("# Go on search...")
	}
}

func save_result(trace *Trace, results *string) {
	file_path, _ := filepath.Abs(trace.entry.file)
	func_name := trace.nodes[0].callee.fun
	abs_hashed_dir := get_abs_hashed_dir(file_path, func_name)

	if dir_exists(abs_hashed_dir) {
		os.RemoveAll(abs_hashed_dir)
		fmt.Println("# Overwrite the cache.")
	}

	err := os.MkdirAll(abs_hashed_dir, 0755)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(21)
	}

	ioutil.WriteFile(filepath.Join(abs_hashed_dir, "result"), []byte(*results), 0400)
}

func (t *Trace) read_nth_ast(path string) {

	idx := clang.NewIndex(1, 0)
	defer idx.Dispose()

	tu := idx.ParseTranslationUnit(path, []string{}, nil, 0)
	cursor := tu.TranslationUnitCursor()

	decls := Decls{}
	cursor.Visit(func(cursor, parent clang.Cursor) clang.ChildVisitResult {

		_, lines, _, _ := cursor.Location().ExpansionLocation()

		switch cursor.Kind() {
		case clang.Cursor_FunctionDecl:
			decls = append(decls, Decl{lines, clang.Cursor_FunctionDecl, cursor.Spelling()})
		case clang.Cursor_StructDecl:
			decls = append(decls, Decl{lines, clang.Cursor_StructDecl, cursor.Spelling()})
		}

		switch cursor.Kind() {
		case clang.Cursor_ClassDecl,
			clang.Cursor_EnumDecl,
			clang.Cursor_StructDecl,
			clang.Cursor_Namespace,
			clang.Cursor_FunctionDecl,
			clang.Cursor_CompoundStmt:
			return clang.ChildVisit_Recurse
		}

		return clang.ChildVisit_Continue
	})

	sort.Sort(decls)

	fd, err := os.Open(path)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(2)
	}
	sc := bufio.NewScanner(fd)

	var lines uint32 = 1
	var predecl_line uint32 = 1
	comment := false

	re_cstart, _ := regexp.Compile("/\\*")
	re_cend, _ := regexp.Compile("\\*")
	re_callee, _ := regexp.Compile("\\w+")

	for sc.Scan() {
		ln := sc.Text()
		if re_cstart.MatchString(ln) {
			comment = true
		}

		if !comment {
			if strings.Contains(ln, t.callee.fun) {
				for _, str := range re_callee.FindAllString(ln, -1) {
					if str == t.callee.fun {
						predecl_line = t.go_walk(path, lines, decls, predecl_line)
					}
				}
			}
		}

		if re_cend.MatchString(ln) {
			comment = false
		}

		lines += 1
	}
}

func (t *Trace) go_walk(path string, lines uint32, decls Decls, predecl_line uint32) uint32 {

	predecl := Decl{}
	var decl_line uint32 = 1

	for _, decl := range decls {

		if lines < decl.line {

			h := fmt.Sprintf("%s-%d-", strings.Repeat(" ", t.level-1), t.level)

			switch predecl.kind {
			case clang.Cursor_FunctionDecl:

				path_slice := strings.Split(path, ".")
				ext := path_slice[len(path_slice)-1]

				if ext == "c" {
					result := fmt.Sprintf("%s %s %s@L%d in \x1b[34m%s\x1b[0m function scope.\n",
						h, t.callee.fun, path, lines, predecl.name)

					callee := Callee{predecl.name, path, predecl.line}
					trace := Trace{t.dir, Entry{}, callee, t.level + 1, t.maxlevel, result, nil, t.wg}
					t.nodes = append(t.nodes, &trace)

					if decl.line != predecl_line {
						go t.new_walk(&trace)
					}

				} else {
					result := fmt.Sprintf("%s \x1b[31m%s\x1b[0m defined in %s@L%d.\n",
						h, t.callee.fun, path, predecl.line)

					callee := Callee{predecl.name, path, predecl.line}
					trace := Trace{t.dir, Entry{}, callee, t.level + 1, t.maxlevel, result, nil, t.wg}
					t.nodes = append(t.nodes, &trace)
				}

			case clang.Cursor_StructDecl:
				result := fmt.Sprintf("%s %s %s@L%d in \x1b[31m%s\x1b[0m struct scope.\n",
					h, t.callee.fun, path, lines, predecl.name)

				callee := Callee{predecl.name, path, predecl.line}
				trace := Trace{t.dir, Entry{}, callee, t.level + 1, t.maxlevel, result, nil, t.wg}
				t.nodes = append(t.nodes, &trace)
			}

			decl_line = decl.line
			break

		}

		switch decl.kind {
		case clang.Cursor_FunctionDecl,
			clang.Cursor_StructDecl:
			predecl = decl
		}
	}

	return decl_line

}

func (t *Trace) new_walk(trace *Trace) {
	t.wg.Add(1)
	filepath.Walk(t.dir, trace.recur_visit)
	t.wg.Done()
}

func get_struct_name(ln string) []string {
	re_struct, _ := regexp.Compile("^\\w+ *\\w+ struct +(\\w+) .*=")
	if re_struct.FindString(ln) == "" {
		re_struct_b, _ := regexp.Compile("^struct +(\\w+) *{")
		if re_struct_b.FindString(ln) == "" {
			return nil
		} else {
			return re_struct_b.FindStringSubmatch(ln)
		}
	} else {
		return re_struct.FindStringSubmatch(ln)
	}
}

func get_func_name(ln string) []string {
	re_not_func, _ := regexp.Compile("[%!\\?\\+\\-]|//")
	if re_not_func.FindString(ln) == "" {
		re_func, _ := regexp.Compile("^\\w+ *\\w+ (\\w+) *\\([^\\(\\)]+\\)? *{? *[^;]?$")
		return re_func.FindStringSubmatch(ln)
	} else {
		return nil
	}
}

func down_tree(root *Trace, results *string) {
	if root.result == "" {
	} else {
		*results += root.result
		for _, node := range root.nodes {
			down_tree(node, results)
		}
	}
}

func main() {

	if len(os.Args) != 5 {
		os.Exit(-1)
	}

	file := os.Args[1]

	line, err := strconv.ParseUint(os.Args[2], 10, 32)
	if err != nil {
		os.Exit(-3)
	}

	dir := os.Args[3]

	maxlevel, err := strconv.Atoi(os.Args[4])
	if err != nil {
		os.Exit(-5)
	}

	ent := fmt.Sprintf("Go search from this entry point %s@L%d.\n", file, line)

	wg := new(sync.WaitGroup)
	entry := Entry{file, uint32(line)}
	trace := Trace{dir, entry, Callee{}, 1, maxlevel, ent, nil, wg}

	filepath.Walk(trace.dir, trace.recur_visit)
	trace.wg.Wait()

	results := ""
	down_tree(&trace, &results)
	fmt.Println(results)
	save_result(&trace, &results)

}
