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
	mtx      *sync.Mutex
	decls_db *map[string]Decls
}

type Decl struct {
	line uint32 // Note this indicates the last line of function body
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

func (t *Trace) make_decls(path string) Decls {
	var decls Decls
	if true {
		decls = t.get_decls_by_raw(path)
	} else {
		decls = t.get_decls_by_clang(path)
	}
	return decls
}

func (t *Trace) read_1st_func(path string) {

	decls := t.make_decls(path)
	(*t.decls_db)[path] = decls

	for _, decl := range decls {

		if t.entry.line <= decl.line {

			result := ""
			switch decl.kind {
			case clang.Cursor_FunctionDecl:
				result = fmt.Sprintf("-1- Entry point %s@L%d in \x1b[34m%s\x1b[0m function scope.\n",
					t.entry.file, t.entry.line, decl.name)

			case clang.Cursor_StructDecl:
				result = fmt.Sprintf("-1- Entry point %s@L%d in \x1b[31m%s\x1b[0m struct scope.\n",
					t.entry.file, t.entry.line, decl.name)

			}

			print_cached_result(path, decl.name)

			callee := Callee{decl.name, path, decl.line}
			trace := Trace{t.dir, Entry{}, callee, 2, t.maxlevel, result, nil, t.wg, t.mtx, t.decls_db}
			t.nodes = append(t.nodes, &trace)

			filepath.Walk(trace.dir, trace.recur_visit)

			break
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
					t.read_1st_func(path)
				}
			} else if t.level <= t.maxlevel {
				t.read_nth_func(path)
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

func (t *Trace) read_nth_func(path string) {

	var decls Decls

	if _, ok := (*t.decls_db)[path]; ok {
		decls = (*t.decls_db)[path]
	} else {
		decls = t.make_decls(path)
		(*t.mtx).Lock()
		(*t.decls_db)[path] = decls
		(*t.mtx).Unlock()
	}

	fd, err := os.Open(path)
	defer fd.Close()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(2)
	}
	sc := bufio.NewScanner(fd)

	scope := 0
	var lines uint32 = 0
	var last_decl_line uint32 = 1
	comment := false

	re_callee, _ := regexp.Compile("\\w+")

	for sc.Scan() {
		ln := sc.Text()
		lines += 1

		if strings.Contains(ln, "/*") {
			comment = true
		}

		if strings.Contains(ln, "*/") {
			comment = false
		}

		if !comment {

			real_ln := ""
			if strings.Contains(ln, "/*") && strings.Contains(ln, "*/") {
				real_ln = exclude_middle_comment(ln)
			} else if strings.Contains(ln, "*/") {
				real_ln = strings.Split(ln, "*/")[1]
			} else {
				real_ln = ln
			}

			if real_ln == "" {
				continue
			}

			if c := strings.Count(real_ln, "{"); c > 0 {
				scope += c
			}

			if c := strings.Count(real_ln, "}"); c > 0 {
				scope -= c
			}

			if scope > 0 && strings.Contains(real_ln, t.callee.fun) {
				for _, str := range re_callee.FindAllString(real_ln, -1) {
					if str == t.callee.fun {
						last_decl_line = t.go_walk(path, lines, decls, last_decl_line)
						break
					}
				}
			}
		}
	}
}

func (t *Trace) go_walk(path string, lines uint32, decls Decls, last_decl_line uint32) uint32 {

	var decl_line uint32 = 1

	for _, decl := range decls {

		if lines <= decl.line {

			h := fmt.Sprintf("%s-%d-", strings.Repeat(" ", t.level-1), t.level)

			switch decl.kind {
			case clang.Cursor_FunctionDecl:

				path_slice := strings.Split(path, ".")
				ext := path_slice[len(path_slice)-1]

				if ext == "c" {
					if t.callee.fun != decl.name {
						result := fmt.Sprintf("%s %s %s@L%d in \x1b[34m%s\x1b[0m function scope.\n",
							h, t.callee.fun, path, lines, decl.name)

						callee := Callee{decl.name, path, decl.line}
						trace := Trace{t.dir, Entry{}, callee, t.level + 1, t.maxlevel, result, nil, t.wg, t.mtx, t.decls_db}
						t.nodes = append(t.nodes, &trace)

						if decl.line != last_decl_line {
							go t.new_walk(&trace)
						}
					}

				} else {
					result := fmt.Sprintf("%s \x1b[31m%s\x1b[0m defined in %s@L%d.\n",
						h, t.callee.fun, path, decl.line)

					callee := Callee{decl.name, path, decl.line}
					trace := Trace{t.dir, Entry{}, callee, t.level + 1, t.maxlevel, result, nil, t.wg, t.mtx, t.decls_db}
					t.nodes = append(t.nodes, &trace)
				}

			case clang.Cursor_StructDecl:
				result := fmt.Sprintf("%s %s %s@L%d in \x1b[31m%s\x1b[0m struct scope.\n",
					h, t.callee.fun, path, lines, decl.name)

				callee := Callee{decl.name, path, decl.line}
				trace := Trace{t.dir, Entry{}, callee, t.level + 1, t.maxlevel, result, nil, t.wg, t.mtx, t.decls_db}
				t.nodes = append(t.nodes, &trace)
			}

			decl_line = decl.line
			break

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

	decls_db := make(map[string]Decls)
	wg := new(sync.WaitGroup)
	mtx := new(sync.Mutex)
	entry := Entry{file, uint32(line)}
	trace := Trace{dir, entry, Callee{}, 1, maxlevel, ent, nil, wg, mtx, &decls_db}

	filepath.Walk(trace.dir, trace.recur_visit)
	trace.wg.Wait()

	results := ""
	down_tree(&trace, &results)
	fmt.Println(results)
	save_result(&trace, &results)

}
