package main

import (
	"fmt"
	"github.com/nsf/termbox-go"
	"os"
	"os/exec"
	"strings"
)

type Term struct {
	ypos int
	strs []string
}

func get_func_line(s string) (string, string) {
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
	item := t.strs[t.ypos]
	file, line := get_func_line(item)
	if file != "" && line != "" {
		cmd := exec.Command("/usr/local/bin/vim", file, "+"+line)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Run()
	}
}

func (t *Term) draw() {
	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)

	for y, str := range t.strs {
		bgcolor := termbox.ColorDefault
		if y == t.ypos {
			bgcolor = termbox.ColorWhite
		}
		for x, r := range str {
			termbox.SetCell(x, y, r, termbox.ColorDefault, bgcolor)
		}
	}
	termbox.Flush()
}

func (t *Term) Run() {

	fmt.Println("# Finished search. You can open with vim to hit enter on next page.")
	fmt.Println("# Other available keys: ↓(C-j) ↑(C-k) Esc(C-q)")
	a := 0
	fmt.Scanln(&a)

	_ = termbox.Init()
	defer termbox.Close()

	t.draw()
	for {
		switch ev := termbox.PollEvent(); ev.Type {
		case termbox.EventKey:
			switch ev.Key {
			case termbox.KeyEsc,
				termbox.KeyCtrlQ:
				return
			case termbox.KeyArrowDown,
				termbox.KeyCtrlJ:
				if t.ypos < len(t.strs)-1 {
					t.ypos += 1
				}
				t.draw()
			case termbox.KeyArrowUp,
				termbox.KeyCtrlK:
				if t.ypos > 0 {
					t.ypos -= 1
				}
				t.draw()
			case termbox.KeyEnter:
				t.exec()
				return
			}
		default:
			t.draw()
		}
	}
}
