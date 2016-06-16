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
	BTHOME = ".rsb"
)

var (
	cache bool
	vim   bool
)

type Entry struct {
	file string
	line uint32
}

type Callee struct {
	fun  string
	file string
	line uint32
	head string
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
	line uint32 // Note this indicates the last line of function or struct body
	kind clang.CursorKind
	name string
	head string
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

func (t *Trace) makeDecls(path string) Decls {
	var decls Decls
	if true {
		decls = t.getDeclsByRaw(path)
	} else {
		decls = t.getDeclsByClang(path)
	}
	return decls
}

func (t *Trace) read1stFunc(path string) {

	decls := t.makeDecls(path)
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

			if cache {
				printCachedResult(path, decl.name)
			}

			callee := Callee{decl.name, path, decl.line, decl.head}
			trace := Trace{t.dir, Entry{}, callee, 2, t.maxlevel, result, nil, t.wg, t.mtx, t.decls_db}
			t.nodes = append(t.nodes, &trace)

			filepath.Walk(trace.dir, trace.recurVisit)

			break
		}

	}

}

func (t *Trace) recurVisit(path string, info os.FileInfo, err error) error {

	file := filepath.Base(path)

	if strings.HasPrefix(file, ".") {
	} else {

		file_slice := strings.Split(file, ".")
		ext := file_slice[len(file_slice)-1]

		if ext == "c" || ext == "h" {
			if t.level == 1 {
				if t.entry.file == path {
					t.read1stFunc(path)
				}
			} else if t.level <= t.maxlevel {
				t.readNthFunc(path)
			}
		}
	}
	return nil
}

func getHomeEnv() string {
	for _, env := range os.Environ() {
		if strings.Contains(env, "HOME=") {
			return strings.Split(env, "=")[1]
		}
	}
	return ""
}

func getHashedDir(file_path, func_name string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(file_path+func_name)))
}

func dirExists(abs_path string) bool {
	_, err := os.Stat(abs_path)
	return err == nil
}

func getAbsHashedDir(file_path, func_name string) string {
	home_path := getHomeEnv()
	hashed_dir := getHashedDir(file_path, func_name)
	abs_hashed_dir := filepath.Join(home_path, BTHOME, hashed_dir)
	return abs_hashed_dir
}

func getCachedResult(file_path, func_name string) (string, error) {

	abs_hashed_dir := getAbsHashedDir(file_path, func_name)

	if dirExists(abs_hashed_dir) {
		result, err := ioutil.ReadFile(filepath.Join(abs_hashed_dir, "result"))
		if err != nil {
			os.Exit(12)
		}
		return string(result), nil
	} else {
		return "", errors.New("Dir not exists.")
	}

}

func printCachedResult(path, func_name string) {

	file_path, err := filepath.Abs(path)
	if err != nil {
		os.Exit(10)
	}

	result, err := getCachedResult(file_path, func_name)
	if err != nil {
	} else {
		fmt.Println("# Show cached result.")
		fmt.Println(result)
		fmt.Println("# Go on search...")
	}
}

func saveResult(trace *Trace, shows *ShowsInfo) {
	file_path, _ := filepath.Abs(trace.entry.file)
	func_name := trace.nodes[0].callee.fun
	abs_hashed_dir := getAbsHashedDir(file_path, func_name)

	if dirExists(abs_hashed_dir) {
		os.RemoveAll(abs_hashed_dir)
		fmt.Println("# Overwrite the cache.")
	}

	err := os.MkdirAll(abs_hashed_dir, 0755)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(21)
	}

	show := shows.Join("")
	ioutil.WriteFile(filepath.Join(abs_hashed_dir, "result"), []byte(show), 0400)
}

func (t *Trace) readNthFunc(path string) {

	var decls Decls

	if _, ok := (*t.decls_db)[path]; ok {
		decls = (*t.decls_db)[path]
	} else {
		decls = t.makeDecls(path)
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

	global_scope := 0
	module_scope := 0

	var lines uint32 = 0
	var last_decl_line uint32 = 1

	re_callee, _ := regexp.Compile("\\w+")

	real_ln := ""
	comment := false
	comment_start := false
	comment_end := false

	for sc.Scan() {
		ln := sc.Text()
		lines += 1

		real_ln = exclude(ln)
		real_ln, comment_start = excludeCommentStart(real_ln)
		real_ln, comment_end = excludeCommentEnd(real_ln)

		if comment_end {
			comment = false
		}

		if !comment {

			if c := strings.Count(real_ln, "{"); c > 0 {

				if (global_scope - module_scope) == 0 {
					if strings.Contains(real_ln, "namespace") ||
						strings.Contains(real_ln, "extern") {
						module_scope += 1
					}
				}

				global_scope += c
			}

			if c := strings.Count(real_ln, "}"); c > 0 {
				global_scope -= c

				if global_scope < module_scope {
					module_scope -= 1
				}

			}

			if (global_scope-module_scope) > 0 && strings.Contains(real_ln, t.callee.fun) {
				for _, str := range re_callee.FindAllString(real_ln, -1) {
					if str == t.callee.fun {
						last_decl_line = t.goWalk(path, lines, decls, last_decl_line)
						break
					}
				}
			}
		}

		if comment_start {
			comment = true
		}

	}
}

func (t *Trace) goWalk(path string, lines uint32, decls Decls, last_decl_line uint32) uint32 {

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

						callee := Callee{decl.name, path, decl.line, decl.head}
						trace := Trace{t.dir, Entry{}, callee, t.level + 1, t.maxlevel, result, nil, t.wg, t.mtx, t.decls_db}
						t.nodes = append(t.nodes, &trace)

						if decl.line != last_decl_line {
							go t.newWalk(&trace)
						}
					}

				} else {
					result := fmt.Sprintf("%s \x1b[31m%s\x1b[0m defined in %s@L%d.\n",
						h, t.callee.fun, path, decl.line)

					callee := Callee{decl.name, path, decl.line, decl.head}
					trace := Trace{t.dir, Entry{}, callee, t.level + 1, t.maxlevel, result, nil, t.wg, t.mtx, t.decls_db}
					t.nodes = append(t.nodes, &trace)
				}

			case clang.Cursor_StructDecl:
				result := fmt.Sprintf("%s %s %s@L%d in \x1b[31m%s\x1b[0m struct scope.\n",
					h, t.callee.fun, path, lines, decl.name)

				callee := Callee{decl.name, path, decl.line, decl.head}
				trace := Trace{t.dir, Entry{}, callee, t.level + 1, t.maxlevel, result, nil, t.wg, t.mtx, t.decls_db}
				t.nodes = append(t.nodes, &trace)
			}

			decl_line = decl.line
			break

		}

	}

	return decl_line

}

func (t *Trace) newWalk(trace *Trace) {
	t.wg.Add(1)
	filepath.Walk(t.dir, trace.recurVisit)
	t.wg.Done()
}

type ShowInfo struct {
	result string
	head   string
}

type ShowsInfo []ShowInfo

func (shows *ShowsInfo) Join(sep string) string {
	str := []string{}
	for i, show := range *shows {
		if i > 0 {
			str = append(str, " ")
		}
		str = append(str, show.result)
	}
	return strings.Join(str, "")
}

func downTree(root *Trace, shows *ShowsInfo) {
	if root.result == "" {
	} else {
		show := ShowInfo{root.result, root.callee.head}
		*shows = append(*shows, show)
		for _, node := range root.nodes {
			downTree(node, shows)
		}
	}
}

func removeAnsiCode(str string) string {
	str_raw := str
	str_raw = strings.Replace(str_raw, "\x1b[34m", "", -1)
	str_raw = strings.Replace(str_raw, "\x1b[31m", "", -1)
	str_raw = strings.Replace(str_raw, "\x1b[0m", "", -1)
	return str_raw
}

func showResult(shows ShowsInfo) {
	for _, show := range shows {
		real_str := "!"
		if vim {
			real_str = removeAnsiCode(show.result)
		} else {
			real_str = show.result
		}
		fmt.Print(real_str)
	}
	fmt.Println()
}

func main() {

	if len(os.Args) < 5 {
		os.Exit(-1)
	}

	// Mandatory arguments
	var file string
	var line uint64
	var dir string
	var maxlevel int

	// Option arguments with double dash
	raw := false
	cache = false // global variable
	vim = false   // global variable

	i := 0
	var err error

	for _, arg := range os.Args[1:] {

		if arg == "--raw" {
			raw = true
			continue
		}
		if arg == "--cache" {
			cache = true
			continue
		}
		if arg == "--vim" {
			vim = true
			raw = true
			continue
		}

		switch i {
		case 0:
			file = arg
		case 1:
			line, err = strconv.ParseUint(os.Args[2], 10, 32)
			if err != nil {
				os.Exit(-3)
			}
		case 2:
			dir = os.Args[3]
		case 3:
			maxlevel, err = strconv.Atoi(os.Args[4])
			if err != nil {
				os.Exit(-5)
			}
		}
		i += 1
	}

	ent := fmt.Sprintf("Go search from this entry point %s@L%d.\n", file, line)

	decls_db := make(map[string]Decls)
	wg := new(sync.WaitGroup)
	mtx := new(sync.Mutex)
	entry := Entry{file, uint32(line)}
	trace := Trace{dir, entry, Callee{}, 1, maxlevel, ent, nil, wg, mtx, &decls_db}

	filepath.Walk(trace.dir, trace.recurVisit)
	trace.wg.Wait()

	shows := ShowsInfo{}
	downTree(&trace, &shows)

	if !raw {
		term := NewTerm(shows)
		term.Run()
	}

	// Not important to show the first one
	showResult(shows[1:])

	if cache {
		saveResult(&trace, &shows)
	}
}
