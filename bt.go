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

	if strings.HasPrefix(path, ".") {
	} else {
		path_slice := strings.Split(path, ".")
		ext := path_slice[len(path_slice)-1]
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

func (t *Trace) read_fst_level(path string, lines int, save Save) {

	fmt.Printf("-1- Entry point %s@L%d in \x1b[34%s\x1b[0m function scope.\n",
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
			fmt.Printf("%s %s %s@L%d in \x1b[31%s\x1b[0m struct scope.\n",
				h, t.callee.fun, path, lines, save.struct_name)
		} else {

			path_slice := strings.Split(path, ".")
			ext := path_slice[len(path_slice)-1]

			if t.level <= t.maxlevel {
				if ext == "c" {
					fmt.Printf("%s %s %s@L%d in \x1b[34%s\x1b[0m function scope.\n",
						h, t.callee.fun, path, lines, save.func_name)

					entry := Entry{path, lines, t.entry.dir}
					callee := Callee{save.func_name}
					trace := Trace{entry, callee, t.level + 1, t.maxlevel}

					filepath.Walk(trace.entry.dir, trace.recur_visit)
				} else {
					fmt.Printf("%s %s defined in %s@L%d.\n",
						h, t.callee.fun, path, lines)
				}
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
				if lines == t.entry.line {
					t.read_fst_level(path, lines, save)
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

func main() {

	if len(os.Args) != 5 {
		os.Exit(-1)
	}

	file := os.Args[1]

	line, err := strconv.Atoi(os.Args[2])
	if err != nil {
		os.Exit(-2)
	}

	dir := os.Args[3]

	maxlevel, err := strconv.Atoi(os.Args[4])
	if err != nil {
		os.Exit(-2)
	}

	entry := Entry{file, line, dir}
	callee := Callee{""}
	trace := Trace{entry, callee, 1, maxlevel}

	for {
		filepath.Walk(trace.entry.dir, trace.recur_visit)
		//trace.entry.dir = ".."
		//continue
		break
	}

}
