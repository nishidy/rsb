package main

import (
	"os"
	"os/exec"
	"strings"

	"github.com/nsf/termbox-go"
)

type Term struct {
	yabs  int
	ybase int
	strs  []string
	head  []string
}

func NewTerm(shows ShowsInfo) Term {
	term := Term{0, 0, []string{}, []string{}}
	for _, show := range shows[1:] {
		term.strs = append(term.strs, show.result)
		term.head = append(term.strs, show.head)
	}

	return term
}

func getFuncLine(s string) (string, string) {
	for _, str := range strings.Split(s, " ") {
		if strings.Contains(str, "@L") {
			str_slice := strings.Split(str, "@L")
			file := str_slice[0]
			line := str_slice[1]
			return file, line
		}
	}
	return "", ""
}

func (t *Term) exec() {
	item := t.strs[t.yabs]
	file, line := getFuncLine(item)
	if file != "" && line != "" {
		vim_path := ""
		if _, err := os.Stat("/usr/bin/vim"); err == nil {
			vim_path = "/usr/bin/vim"
		} else if _, err := os.Stat("/usr/local/bin/vim"); err == nil {
			vim_path = "/usr/local/bin/vim"
		}
		cmd := exec.Command(vim_path, file, "+"+line)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Run()
	}
}

func replaceAnsiCode(str string) string {
	str_raw := str
	str_raw = strings.Replace(str_raw, "\x1b[34m", "|", -1)
	str_raw = strings.Replace(str_raw, "\x1b[31m", "~", -1)
	str_raw = strings.Replace(str_raw, "\x1b[0m", "^", -1)
	return str_raw
}

func drawTitle(str_raw string, bgAttr termbox.Attribute, y int) {
	color := termbox.ColorDefault
	x := 0
	for _, r := range str_raw {
		termbox.SetCell(x, y, r, color, bgAttr)
		x += 1
	}
}
func drawLine(str_raw string, bgAttr termbox.Attribute, y int) {
	color := termbox.ColorDefault
	x := 0
	for _, r := range str_raw {
		if r == '|' {
			color = termbox.ColorBlue
			continue
		}
		if r == '~' {
			color = termbox.ColorRed
			continue
		}
		if r == '^' {
			color = termbox.ColorDefault
			continue
		}
		termbox.SetCell(x, y+1, r, color, bgAttr)
		x += 1
	}
}

func (t *Term) draw() {
	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)

	exp := "# Available keys: vim[enter] up[↓/C-j] down[↑/C-k] head[C-h] bottom[C-b] quit[Esc/C-q]"
	drawTitle(exp, termbox.ColorDefault, 0)

	for y, str := range t.strs[t.ybase:] {
		bgAttr := termbox.ColorDefault

		if y == t.yabs-t.ybase {
			bgAttr = termbox.AttrReverse
		}

		str_raw := replaceAnsiCode(str)
		drawLine(str_raw, bgAttr, y)
	}

	termbox.Flush()
}

func (t *Term) Run() {

	_ = termbox.Init()

	t.draw()
	for {
		switch ev := termbox.PollEvent(); ev.Type {
		case termbox.EventKey:
			switch ev.Key {
			case termbox.KeyEsc,
				termbox.KeyCtrlQ:
				termbox.Close()
				return
			case termbox.KeyArrowDown,
				termbox.KeyCtrlJ:
				if t.yabs < len(t.strs)-1 {
					t.yabs += 1
					_, height := termbox.Size()
					// 1 : index which starts from 0
					// 2 : the first line is title
					height -= 2
					if height < t.yabs-t.ybase {
						t.ybase += 1
					}
				}
			case termbox.KeyArrowUp,
				termbox.KeyCtrlK:
				if t.yabs > 0 {
					t.yabs -= 1
					if t.yabs < t.ybase {
						t.ybase = t.yabs
					}
				}
			case termbox.KeyCtrlH:
				t.yabs = 0
				t.ybase = 0
			case termbox.KeyCtrlB:
				_, height := termbox.Size()
				height -= 2
				if len(t.strs) < height {
					t.yabs = len(t.strs) - 1
				} else {
					t.yabs = len(t.strs) - 1
					t.ybase = len(t.strs) - 1 - height
				}
			case termbox.KeyEnter:
				t.exec()
				termbox.Close()
				t.Run()
				return
			default:
			}
		}
		t.draw()
	}
}
