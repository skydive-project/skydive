// Copyright 2009 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"io"
	"io/ioutil"
	"os"

	eval "github.com/sbinet/go-eval"
	"golang.org/x/crypto/ssh/terminal"
)

var fset = token.NewFileSet()
var filename = flag.String("f", "", "file to run")

type shell struct {
	r io.Reader
	w io.Writer
}

func (sh *shell) Read(data []byte) (n int, err error) {
	return sh.r.Read(data)
}
func (sh *shell) Write(data []byte) (n int, err error) {
	return sh.w.Write(data)
}

func main() {
	flag.Parse()
	w := eval.NewWorld()
	if *filename != "" {
		data, err := ioutil.ReadFile(*filename)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		file, err := parser.ParseFile(fset, *filename, data, 0)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		files := []*ast.File{file}
		code, err := w.CompilePackage(fset, files, "main")
		if err != nil {
			if list, ok := err.(scanner.ErrorList); ok {
				for _, e := range list {
					fmt.Println(e.Error())
				}
			} else {
				fmt.Println(err.Error())
			}
			os.Exit(1)
		}
		code, err = w.Compile(fset, "main()")
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		_, err = code.Run()
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		os.Exit(0)
	}

	fmt.Println(":: welcome to go-eval...\n(hit ^D to exit)")

	fd := int(os.Stdin.Fd())
	oldState, err := terminal.MakeRaw(fd)
	if err != nil {
		panic(err)
	}
	defer terminal.Restore(fd, oldState)
	term := terminal.NewTerminal(&shell{r: os.Stdin, w: os.Stdout}, "> ")
	if term == nil {
		panic(errors.New("could not create terminal"))
	}

	for {
		line, err := term.ReadLine()
		if err != nil {
			break
		}
		code, err := w.Compile(fset, line)
		if err != nil {
			term.Write([]byte(err.Error() + "\n"))
			continue
		}
		if code == nil {
			term.Write([]byte("failed to compile codelet\n"))
			continue
		}
		v, err := code.Run()
		if err != nil {
			term.Write([]byte(err.Error() + "\n"))
			continue
		}
		if v != nil {
			term.Write([]byte(v.String() + "\n"))
		}
	}
}
