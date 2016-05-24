package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Entry struct {
	file string
	line int
	dir  string
}

type Callee struct {
	fun string
}

type Trace struct {
	entry    Entry
	callee   Callee
	level    int
	maxlevel int
}

type Save struct {
	func_name   string
	func_line   int
	struct_name string
	struct_line int
}

func (t *Trace) recur_visit(path string, info os.FileInfo, err error) error {

	if t.level == 1 {
		if t.entry.file == path {
			t.read(path)
		}
	} else {
		t.read(path)
	}
	return nil
}

func (t *Trace) read_fst_level(path string, lines int, save Save) {

	fmt.Printf("-1- Entry point %s@L%d in %s function scope.\n",
		t.entry.file, t.entry.line, save.func_name)

	entry := Entry{path, lines, t.entry.dir}
	callee := Callee{save.func_name}
	trace := Trace{entry, callee, 2, t.maxlevel}

	filepath.Walk(trace.entry.dir, trace.recur_visit)
}

func (t *Trace) read_nth_level(path string, lines int, save Save, is_func bool) {

	if is_func {
		// Skip if this line defines function
	} else if t.entry.file == path && t.entry.line == lines {
		// Skip if this line appears again
	} else {
		h := fmt.Sprintf("%s-%d-", strings.Repeat(" ", t.level-1), t.level)

		if save.func_line < save.struct_line {
			fmt.Printf("%s %s %s@L%d in %s struct scope.\n",
				h, t.callee.fun, path, lines, save.struct_name)
		} else {
			fmt.Printf("%s %s %s@L%d in %s function scope.\n",
				h, t.callee.fun, path, lines, save.func_name)

			if t.level < t.maxlevel {
				entry := Entry{path, lines, t.entry.dir}
				callee := Callee{save.func_name}
				trace := Trace{entry, callee, t.level + 1, t.maxlevel}

				filepath.Walk(trace.entry.dir, trace.recur_visit)
			}

		}
	}

}

func (t *Trace) read(path string) {

	fd, err := os.Open(path)
	if err != nil {
		fmt.Printf("%s file not exist.\n", path)
		os.Exit(1)
	}

	save := Save{"", 0, "", 0}
	lines := 1

	sc := bufio.NewScanner(fd)
	for sc.Scan() {
		ln := sc.Text()

		func_name := get_func_name(ln)
		is_func := false
		if func_name != nil {
			save.func_name = func_name[1]
			save.func_line = lines
			is_func = true
		}

		struct_name := get_struct_name(ln)
		if struct_name != nil {
			save.struct_name = struct_name[1]
			save.struct_line = lines
		}

		if t.level == 1 {
			if lines == t.entry.line {
				t.read_fst_level(path, lines, save)
			}
		} else {
			if strings.Contains(ln, t.callee.fun) {
				t.read_nth_level(path, lines, save, is_func)
			}
		}
		lines += 1
	}
}

func get_struct_name(ln string) []string {
	re_struct, _ := regexp.Compile("\\w *\\w+ struct *(\\w+) *=")
	return re_struct.FindStringSubmatch(ln)
}

func get_func_name(ln string) []string {
	re_func, _ := regexp.Compile("\\w *\\w+ (\\w+) *\\([^\\(\\)]+\\)? *{? *[^;]?$")
	return re_func.FindStringSubmatch(ln)
}

func main() {

	if len(os.Args) != 5 {
		os.Exit(-1)
	}

	line, err := strconv.Atoi(os.Args[2])
	if err != nil {
		os.Exit(-2)
	}

	maxlevel, err := strconv.Atoi(os.Args[4])
	if err != nil {
		os.Exit(-2)
	}

	entry := Entry{os.Args[1], line, os.Args[3]}
	callee := Callee{""}
	trace := Trace{entry, callee, 1, maxlevel}

	for {
		filepath.Walk(trace.entry.dir, trace.recur_visit)
		//trace.entry.dir = ".."
		//continue
		break
	}

}
