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
)

const (
	BTHOME = ".bt"
)

type Entry struct {
	file string
	line int
}

type Callee struct {
	fun  string
	file string
	line int
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

type Save struct {
	func_name   string
	func_line   int
	struct_name string
	struct_line int
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
					t.read(path)
				}
			} else {
				t.read(path)
			}
		}
	}
	return nil
}

func get_home() string {
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
	home_path := get_home()
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

func (t *Trace) read_fst_level(path string, lines int, save Save) {

	result := fmt.Sprintf("-1- Entry point %s@L%d in \x1b[34m%s\x1b[0m function scope.\n",
		t.entry.file, t.entry.line, save.func_name)
	fmt.Print(result)

	callee := Callee{save.func_name, path, lines}
	trace := Trace{t.dir, Entry{}, callee, 2, t.maxlevel, result, nil, t.wg}

	t.nodes = append(t.nodes, &trace)

	print_cached_result(path, save.func_name)

	filepath.Walk(trace.dir, trace.recur_visit)
}

func (t *Trace) skip_same_callee(save Save) bool {
	if len(t.nodes) > 0 {
		if t.nodes[len(t.nodes)-1].callee.fun == save.func_name ||
			t.nodes[len(t.nodes)-1].callee.fun == save.struct_name {
			return true
		}
	}
	return false
}

func (t *Trace) read_nth_level(path string, lines int, save Save, is_func bool) {

	if is_func {
		// Skip if this line defines function
	} else if t.callee.file == path && t.callee.line == lines {
		// Skip if this line appears again
	} else if t.maxlevel < t.level {
		//
	} else if t.skip_same_callee(save) {
		// Skip if the same callee is found in a row
	} else {

		h := fmt.Sprintf("%s-%d-", strings.Repeat(" ", t.level-1), t.level)

		if save.func_line < save.struct_line {
			result := fmt.Sprintf("%s %s %s@L%d in \x1b[31m%s\x1b[0m struct scope.\n",
				h, t.callee.fun, path, lines, save.struct_name)
			fmt.Print(result)

			callee := Callee{save.struct_name, path, lines}
			trace := Trace{t.dir, Entry{}, callee, t.level + 1, t.maxlevel, result, nil, t.wg}
			t.nodes = append(t.nodes, &trace)
		} else {

			path_slice := strings.Split(path, ".")
			ext := path_slice[len(path_slice)-1]

			if ext == "c" {
				result := fmt.Sprintf("%s %s %s@L%d in \x1b[34m%s\x1b[0m function scope.\n",
					h, t.callee.fun, path, lines, save.func_name)
				fmt.Print(result)

				callee := Callee{save.func_name, path, lines}
				trace := Trace{t.dir, Entry{}, callee, t.level + 1, t.maxlevel, result, nil, t.wg}
				t.nodes = append(t.nodes, &trace)

				go t.new_walk(&trace)

			} else {
				result := fmt.Sprintf("%s \x1b[31m%s\x1b[0m defined in %s@L%d.\n",
					h, t.callee.fun, path, lines)
				fmt.Print(result)

				callee := Callee{save.func_name, path, lines}
				trace := Trace{t.dir, Entry{}, callee, t.level + 1, t.maxlevel, result, nil, t.wg}
				t.nodes = append(t.nodes, &trace)
			}
		}

	}

}

func (t *Trace) new_walk(trace *Trace) {
	t.wg.Add(1)
	filepath.Walk(t.dir, trace.recur_visit)
	t.wg.Done()
}

func (t *Trace) read(path string) {

	fd, err := os.Open(path)
	if err != nil {
		fmt.Printf("%s file not exist.\n", path)
		os.Exit(1)
	}

	save := Save{"", 0, "", 0}
	lines := 1

	re_cstart, _ := regexp.Compile("/\\*")
	re_cend, _ := regexp.Compile("\\*/")
	re_callee, _ := regexp.Compile("\\w+")

	comment := false

	sc := bufio.NewScanner(fd)
	for sc.Scan() {
		ln := sc.Text()

		if re_cstart.MatchString(ln) {
			comment = true
		}

		if !comment {

			func_name := get_func_name(ln)
			is_func := false
			if func_name != nil {
				save.func_name = func_name[1]
				save.func_line = lines
				is_func = true
				//fmt.Printf("Now the scoped is changed to %s\n",save.func_name)
			}

			struct_name := get_struct_name(ln)
			if struct_name != nil {
				save.struct_name = struct_name[1]
				save.struct_line = lines
			}

			if t.level == 1 {
				if t.entry.line <= lines {
					t.read_fst_level(path, lines, save)
					break
				}
			} else {
				if strings.Contains(ln, t.callee.fun) {
					for _, str := range re_callee.FindAllString(ln, -1) {
						if str == t.callee.fun {
							t.read_nth_level(path, lines, save, is_func)
							break
						}
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

func get_struct_name(ln string) []string {
	re_struct, _ := regexp.Compile("^\\w+ *\\w+ struct +(\\w+) .*=")
	return re_struct.FindStringSubmatch(ln)
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

	line, err := strconv.Atoi(os.Args[2])
	if err != nil {
		os.Exit(-3)
	}

	dir := os.Args[3]

	maxlevel, err := strconv.Atoi(os.Args[4])
	if err != nil {
		os.Exit(-5)
	}

	ent := fmt.Sprintf("Go search from this entry point %s@L%d.\n", file, line)
	fmt.Print(ent)

	wg := new(sync.WaitGroup)
	entry := Entry{file, line}
	trace := Trace{dir, entry, Callee{}, 1, maxlevel, ent, nil, wg}

	filepath.Walk(trace.dir, trace.recur_visit)
	trace.wg.Wait()

	results := ""
	down_tree(&trace, &results)
	save_result(&trace, &results)

}
