// Copyright 2009 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eval

import (
	"fmt"
	"runtime"
)

// Abort aborts the thread's current computation,
// causing the innermost Try to return err.
func (t *Thread) Abort(err error) {
	if t.abort == nil {
		panic("abort: " + err.Error())
	}
	t.abort <- err
	runtime.Goexit()
}

// Try executes a computation; if the computation
// Aborts, Try returns the error passed to abort.
func (t *Thread) Try(f func(t *Thread)) error {
	oc := t.abort
	c := make(chan error)
	t.abort = c
	go func() {
		f(t)
		c <- nil
	}()
	err := <-c
	t.abort = oc
	return err
}

type DivByZeroError struct{}

func (DivByZeroError) Error() string { return "divide by zero" }

type NilPointerError struct{}

func (NilPointerError) Error() string { return "nil pointer dereference" }

type IndexError struct {
	Idx, Len int64
}

func (e IndexError) Error() string {
	if e.Idx < 0 {
		return fmt.Sprintf("negative index: %d", e.Idx)
	}
	return fmt.Sprintf("index %d exceeds length %d", e.Idx, e.Len)
}

type SliceError struct {
	Lo, Hi, Cap int64
}

func (e SliceError) Error() string {
	return fmt.Sprintf("slice [%d:%d]; cap %d", e.Lo, e.Hi, e.Cap)
}

type KeyError struct {
	Key interface{}
}

func (e KeyError) Error() string { return fmt.Sprintf("key '%v' not found in map", e.Key) }

type NegativeLengthError struct {
	Len int64
}

func (e NegativeLengthError) Error() string {
	return fmt.Sprintf("negative length: %d", e.Len)
}

type NegativeCapacityError struct {
	Len int64
}

func (e NegativeCapacityError) Error() string {
	return fmt.Sprintf("negative capacity: %d", e.Len)
}
