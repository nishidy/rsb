package main

import (
	"github.com/go-clang/bootstrap/clang"
	"os"
	"reflect"
	"testing"
)

func TestGet_decl_by_raw(t *testing.T) {

	tmp := ".tmp"
	source := `
#include <stdio.h>

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
		*/ }
		return *x;
	} else {
		return "x";
	}
}
`

	file, err := os.Create(tmp)
	if err != nil {
		t.Errorf("Tmp file could not open.")
	}
	file.Write([]byte(source))

	decls := Decls{
		Decl{6, clang.Cursor_FunctionDecl, "hoge"},
		Decl{15, clang.Cursor_FunctionDecl, "get_human"},
		Decl{35, clang.Cursor_FunctionDecl, "baz"},
	}

	trace := Trace{}
	if reflect.DeepEqual(decls, trace.get_decls_by_raw(".tmp")) {
		t.Errorf("Failed.")
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
