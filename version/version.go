package version

import (
	"fmt"
	"io"
	"os"
)

// Package is the overall, canonical project import path under which the
// package was built.
var Package = "github.com/skydive-project/skydive"

// Version indicates which version of the binary is running.
var Version = "v0.9.0"

// FprintVersion outputs the version string to the writer, in the following
// format, followed by a newline:
//
//      <cmd> <project> <version>
//
// For example, a binary "registry" built from github.com/docker/distribution
// with version "v1.0.0" would print the following:
//
//      skydive_agent github.com/skydive-project/skydive v1.0.0
//
func FprintVersion(w io.Writer) {
	fmt.Fprintln(w, os.Args[0], Package, Version)
}

// PrintVersion outputs the version information, from Fprint, to stdout.
func PrintVersion() {
	FprintVersion(os.Stdout)
}
