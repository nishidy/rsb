package main

import (
	"fmt"
	"github.com/go-clang/bootstrap/clang"
	"os"
	"reflect"
	"testing"
)

func TestGetDeclByRaw(t *testing.T) {

	tmp := ".tmp"
	source := `#include <stdio.h>

namespace test {
	int hoge(int i, int *j) { // test code
		printf("{ } } } \" } } ");
	}
	
	/* } text code } */
	struct human *get_human() {
		struct human *h = { "name",
		age,height,weight /* test code */
		/*test_code*/};
		struct human *u = { name,age,height,weight };
		return h;
	}
}

static struct ccchar *baz ( char *i, struct *tree ) {
	char a = '{';
	char b = '{{';
	char c = '}}}';
	char d = '}}';

	if ( "a" == '}' ) {
		ccchar *x = strcat(a,b,c,d);
		/*
			XXX: { is not } but {
			/* //
		"test code" */
		while() { /*
		* test
		*/ }
		return *x;
	} else {
		return "x";
	}
}

struct *st f(struct s* _s) {
	if() { /* aaa
		*/
		if() { /* bbb
			*/
			if() { /* ccc
			*/
				if() {
				}
			}
		}
	}
}
`

	file, err := os.Create(tmp)
	if err != nil {
		t.Errorf("Tmp file could not open.")
	}
	file.Write([]byte(source))

	decls := Decls{
		Decl{6, clang.Cursor_FunctionDecl, "hoge", "int hoge(int i, int *j) {"},
		Decl{15, clang.Cursor_FunctionDecl, "get_human", "struct human *get_human() {"},
		Decl{37, clang.Cursor_FunctionDecl, "baz", "static struct ccchar *baz ( char *i, struct *tree ) {"},
		Decl{51, clang.Cursor_FunctionDecl, "f", "struct *st f(struct s* _s) {"},
	}

	trace := Trace{}
	test_decls := trace.getDeclsByRaw(".tmp")
	if !reflect.DeepEqual(decls, test_decls) {
		t.Errorf("Failed.")
		fmt.Println("Assumed result.")
		for i, decl := range decls {
			fmt.Println(i, decl.line, decl.kind, decl.name, decl.head)
		}
		fmt.Println("\nActual result.")
		for i, decl := range test_decls {
			fmt.Println(i, decl.line, decl.kind, decl.name, decl.head)
		}
	}

	os.Remove(tmp)

}

func TestExclude(t *testing.T) {

	a := "aaa /* bbb */ ccc"
	if exclude(a) != "aaa  ccc" {
		t.Errorf("%s failed.", a)
	}

	a = "aaa \" bbb \" ccc"
	if exclude(a) != "aaa  ccc" {
		t.Errorf("%s failed.", a)
	}

	a = "aaa \" bbb \" ccc \" ddd \" eee"
	if exclude(a) != "aaa  ccc  eee" {
		t.Errorf("%s failed.", a)
	}

	a = "aaa \" /* bbb \" ccc \" ddd */ \" eee \" // xxx \" yyy // zzz "
	if exclude(a) != "aaa  ccc  eee  yyy " {
		t.Errorf("%s failed.", a)
	}

	a = "000 ' 111 ' aaa \" /* bbb \" ccc \" ddd */ \" eee \" // xxx \" yyy // zzz "
	if exclude(a) != "000  aaa  ccc  eee  yyy " {
		t.Errorf("%s failed.", a)
	}

	a = "000 ' 111 // ' aaa \" /* bbb \" ccc \" ddd */ \" eee \" // xxx \" yyy // zzz "
	if exclude(a) != "000  aaa  ccc  eee  yyy " {
		t.Errorf("%s failed.", a)
	}

	a = "000 ' 111 // ' \\\" aaa \" /* bbb \" ccc \" ddd */ \" eee \" // xxx \" yyy '---' \\\" // zzz "
	if exclude(a) != "000   aaa  ccc  eee  yyy   " {
		t.Errorf("%s failed.", a)
	}

	a = "000 /*' 111 // ' aaa \" /* bbb \" ccc \" ddd */ \" eee \" // xxx \" yyy //*/ zzz "
	if exclude(a) != "000  zzz " {
		t.Errorf("%s failed.", a)
	}

	a = "0\"1\"2\"3\"4\"5\"6\"7\"8\na'b'c'd'e'f'g'h'i\rA/*B*/C/*D*/E/*F*/G/*H*/I\to\"p\"q'r's/*t*/u//v"
	if exclude(a) != "02468\nacegi\rACEGI\toqsu" {
		t.Errorf("%s failed.", a)
	}

}
