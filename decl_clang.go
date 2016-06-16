package main

import (
	"sort"

	"github.com/go-clang/bootstrap/clang"
)

// TODO : Should find the last line of function body
// TODO : Should find the declaration of function (head)
func (t *Trace) getDeclsByClang(path string) Decls {

	idx := clang.NewIndex(1, 0)
	defer idx.Dispose()

	tu := idx.ParseTranslationUnit(path, []string{}, nil, 0)
	cursor := tu.TranslationUnitCursor()

	decls := Decls{}
	cursor.Visit(func(cursor, parent clang.Cursor) clang.ChildVisitResult {

		_, lines, _, _ := cursor.Location().ExpansionLocation()

		switch cursor.Kind() {
		case clang.Cursor_FunctionDecl:
			decls = append(decls, Decl{lines, clang.Cursor_FunctionDecl, cursor.Spelling(), ""})
		case clang.Cursor_StructDecl:
			decls = append(decls, Decl{lines, clang.Cursor_StructDecl, cursor.Spelling(), ""})
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
	return decls
}
