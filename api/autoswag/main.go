// This code is tailor-made for api package and very brittle,
// can't do much about it due to the very nature of it.
// It will find func names that start with "enable" (hardcoded)
// then inside look for RegisterMethod calls (based on func signature)
// parse the URL, method and handler func of those RegisterMethod
// and then look for those handler funcs (in the same file)
// to replace (or add) @Router tags in the handler func doc comment

package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"log"
	"os"
	"strings"
)

type PathMethod struct {
	path, method string
}

func main() {
	if len(os.Args) > 1 {
		err := ParseFile(os.Args[1])
		if err != nil {
			fmt.Println(err)
		}
		return
	}

	// else parse the whole directory
	log.SetOutput(io.Discard) // without debug logs
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	files, err := os.ReadDir(cwd)
	if err != nil {
		panic(err)
	}
	for _, f := range files {
		if !f.IsDir() {
			err := ParseFile(f.Name())
			if err != nil {
				fmt.Println(err)
			}
		}
	}
}

// ParseFile rewrites (in place) the given file
func ParseFile(file string) error {
	// Parse the Go file
	fset := token.NewFileSet()
	parsedFile, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		return err
	}
	fmap := ParseRegisterMethodCalls(parsedFile)
	UpdateRouterTags(fset, parsedFile, fmap)

	// Print the updated file
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, parsedFile); err != nil {
		panic(err)
	}

	fd, err := os.Create(file)
	if err != nil {
		panic(err)
	}
	if _, err := fd.Write(buf.Bytes()); err != nil {
		panic(err)
	}
	return nil
}

// ParseRegisterMethodCalls returns a map of func names
// with the URL and method that are registered as handlers for
func ParseRegisterMethodCalls(parsedFile *ast.File) map[string][]PathMethod {
	fmap := make(map[string][]PathMethod)
	// Find the `RegisterMethod` calls in the file
	for _, decl := range parsedFile.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		if !strings.HasPrefix(fn.Name.Name, "enable") {
			continue
		}

		// Extract the path and method from the `RegisterMethod` call
		path := ""
		method := ""
		fname := ""
		for _, stmt := range fn.Body.List {
			ifstmt, ok := stmt.(*ast.IfStmt)
			if !ok {
				continue
			}
			assign, ok := ifstmt.Init.(*ast.AssignStmt)
			if !ok {
				continue
			}
			call, ok := assign.Rhs[0].(*ast.CallExpr)
			if !ok {
				continue
			}
			if len(call.Args) != 4 {
				continue
			}

			url, ok := call.Args[0].(*ast.BasicLit)
			if !ok {
				continue
			}
			if url.Kind != token.STRING {
				continue
			}
			path = strings.Trim(url.Value, "\"")
			log.Printf("url %s ", url.Value)

			httpmethod, ok := call.Args[1].(*ast.BasicLit)
			if !ok {
				continue
			}
			if httpmethod.Kind != token.STRING {
				continue
			}
			method = strings.ToLower(strings.Trim(httpmethod.Value, "\""))
			log.Printf("[%s] ", httpmethod.Value)

			fn, ok := call.Args[3].(*ast.SelectorExpr)
			if !ok {
				continue
			}
			log.Printf("-> %s\n", fn.Sel.Name)
			fname = fn.Sel.Name

			// Add the method to the list if a path and method were found
			if path != "" && method != "" && fname != "" {
				fmap[fname] = append(fmap[fname], PathMethod{path: path, method: method})
			}
		}
	}
	return fmap
}

// UpdateRouterTags updates the @Router tags of the funcs passed in fmap
func UpdateRouterTags(fset *token.FileSet, parsedFile *ast.File, fmap map[string][]PathMethod) {
	cmap := ast.NewCommentMap(fset, parsedFile, parsedFile.Comments)

	ast.Inspect(parsedFile, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			if pms, found := fmap[x.Name.Name]; found {
				list := []*ast.Comment{}
				for _, line := range cmap[x][0].List {
					if strings.Contains(line.Text, "@Router") {
						continue
					}
					line.Slash = token.Pos(int(x.Pos()) - 1)
					list = append(list, line)
				}

				for _, pm := range pms {
					line := &ast.Comment{
						Text:  fmt.Sprintf(`//	@Router %s [%s]`, pm.path, pm.method),
						Slash: token.Pos(int(x.Pos() - 1)),
					}
					list = append(list, line)
				}

				cmap[x] = []*ast.CommentGroup{
					{
						List: list,
					},
				}
			}
		}
		return true
	})
	parsedFile.Comments = cmap.Filter(parsedFile).Comments()
}

// InitComments replaces the whole func doc with a hardcoded template (this is currently unused)
func InitComments(fset *token.FileSet, parsedFile *ast.File, fmap map[string][]PathMethod) {
	cmap := ast.NewCommentMap(fset, parsedFile, parsedFile.Comments)

	ast.Inspect(parsedFile, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			if _, found := fmap[x.Name.Name]; found {
				list := []*ast.Comment{}
				list = append(list, &ast.Comment{
					Text:  fmt.Sprintf("// %s", x.Name.Name),
					Slash: token.Pos(int(x.Pos() - 1)),
				})
				list = append(list, &ast.Comment{
					Text:  "//",
					Slash: token.Pos(int(x.Pos() - 1)),
				})
				list = append(list, &ast.Comment{
					Text:  "// @Summary TODO",
					Slash: token.Pos(int(x.Pos() - 1)),
				})
				list = append(list, &ast.Comment{
					Text:  "// @Description TODO",
					Slash: token.Pos(int(x.Pos() - 1)),
				})
				list = append(list, &ast.Comment{
					Text:  "// @Success 200 {object} object",
					Slash: token.Pos(int(x.Pos() - 1)),
				})

				cmap[x] = []*ast.CommentGroup{
					{
						List: list,
					},
				}
			}
		}
		return true
	})
	parsedFile.Comments = cmap.Filter(parsedFile).Comments()
}
