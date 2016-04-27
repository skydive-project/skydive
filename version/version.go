package version

import (
	"fmt"
	"io"
	"os"
)

// Package is the overall, canonical project import path under which the
// package was built.
var Package = "github.com/redhat-cip/skydive"

// Version indicates which version of the binary is running. This is set to
// the latest release tag by hand, always suffixed by "+unknown". During
// build, it will be replaced by the actual version. The value here will be
// used if Skydive is run after a go get based install.
var Version = "v0.3.0+unknown"

// FprintVersion outputs the version string to the writer, in the following
// format, followed by a newline:
//
//      <cmd> <project> <version>
//
// For example, a binary "registry" built from github.com/docker/distribution
// with version "v1.0.0" would print the following:
//
//      skydive_agent github.com/redhat-cip/skydive v1.0.0
//
func FprintVersion(w io.Writer) {
	fmt.Fprintln(w, os.Args[0], Package, Version)
}

// PrintVersion outputs the version information, from Fprint, to stdout.
func PrintVersion() {
	FprintVersion(os.Stdout)
}
