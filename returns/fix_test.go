// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package returns

import (
	"flag"
	_ "go/importer"
	"testing"
)

var only = flag.String("only", "", "If non-empty, the fix test to run")

var tests = []struct {
	name    string
	skip    bool
	in, out string
}{
	// No-op
	{
		name: "noop",
		in: `package foo
func F() error { return nil }
`,
		out: `package foo

func F() error { return nil }
`,
	},

	// Don't fix naked returns, even when they are erroneous (it's not
	// as clear what we should do with them).
	{
		name: "naked return",
		in: `package foo
func F() error { return }
`,
		out: `package foo

func F() error { return }
`,
	},

	// Add zero value returns for preceding return values.
	{
		name: "preceding",
		in: `package foo
import "errors"
func F() (int, error) { return errors.New("foo") }
`,
		out: `package foo

import "errors"

func F() (int, error) { return 0, errors.New("foo") }
`,
	},

	// Preserve existing rightmost return values when adding preceding
	// zero values.
	{
		name: "preserve rightmost return values",
		in: `package foo
import "errors"
func F() (int, int, error) { return 7, errors.New("foo") }
`,
		out: `package foo

import "errors"

func F() (int, int, error) { return 0, 7, errors.New("foo") }
`,
	},

	// Be aware of direct returns of func calls of funcs that return
	// multiple values.
	{
		name: "direct return of multiple return funcs",
		in: `package foo
import "io/ioutil"
func F() ([]byte, error) { return ioutil.ReadFile("f") }
`,
		out: `package foo

import "io/ioutil"

func F() ([]byte, error) { return ioutil.ReadFile("f") }
`,
	},

	// Determine when local funcs have a single return value.
	{
		name: "local func with single return value",
		in: `package foo
import "errors"
func x() error { return errors.New("foo") }

func F() (int, error) { return x() }
`,
		out: `package foo

import "errors"

func x() error { return errors.New("foo") }

func F() (int, error) { return 0, x() }
`,
	},

	// Determine when external funcs have a single return value.
	{
		name: "ext func with single return value",
		in: `package foo
import "net/http"
func F() (int, error) { return http.ListenAndServe("", nil) }
`,
		out: `package foo

import "net/http"

func F() (int, error) { return 0, http.ListenAndServe("", nil) }
`,
	},

	// Determine when indirect funcs have a single return value.
	{
		name: "indirect func with single return value",
		in: `package foo
import "net/http"
type x func(string, http.Handler) error
func F() (int, error) { return (x(http.ListenAndServe))("", nil) }
`,
		out: `package foo

import "net/http"

type x func(string, http.Handler) error

func F() (int, error) { return 0, (x(http.ListenAndServe))("", nil) }
`,
	},

	// Determine when external funcs have multiple return values.
	{
		name: "ext func with multiple return values",
		in: `package foo
import "strconv"
func x() (int, error) { return strconv.Atoi("7") }

func F() (int, error) { return x() }
`,
		out: `package foo

import "strconv"

func x() (int, error) { return strconv.Atoi("7") }

func F() (int, error) { return x() }
`,
	},

	// Determine when indirect funcs have multiple return values.
	{
		name: "indirect func with multiple return values",
		in: `package foo
import "strconv"
type x func(string) (int, error)
func F() (int, error) { return (x(strconv.Atoi))("7") }
`,
		out: `package foo

import "strconv"

type x func(string) (int, error)

func F() (int, error) { return (x(strconv.Atoi))("7") }
`,
	},

	// Synthesize zero values for all primitives.
	{
		name: "primitives",
		in: `package foo
import "errors"
func F() (uint8, uint16, uint32, uint64, int8, int16, int32, int64, float32, float64, complex64, complex128, byte, rune, uint, int, uintptr, string, bool, error) { return errors.New("foo") }
`,
		out: `package foo

import "errors"

func F() (uint8, uint16, uint32, uint64, int8, int16, int32, int64, float32, float64, complex64, complex128, byte, rune, uint, int, uintptr, string, bool, error) {
	return 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, "", false, errors.New("foo")
}
`,
	},

	// Synthesize zero values (nil) for pointers.
	{
		name: "pointers",
		in: `package foo
import "errors"
func F() (*int, error) { return errors.New("foo") }
`,
		out: `package foo

import "errors"

func F() (*int, error) { return nil, errors.New("foo") }
`,
	},

	// Synthesize zero values (nil) for slices.
	{
		name: "slices",
		in: `package foo
import "errors"
func F() ([]int, error) { return errors.New("foo") }
`,
		out: `package foo

import "errors"

func F() ([]int, error) { return nil, errors.New("foo") }
`,
	},

	// Synthesize zero values for arrays.
	{
		name: "arrays",
		in: `package foo
import "errors"
func F() ([2]int, error) { return errors.New("foo") }
`,
		out: `package foo

import "errors"

func F() ([2]int, error) { return [2]int{}, errors.New("foo") }
`,
	},

	// Synthesize zero values for structs in same package.
	{
		name: "structs",
		skip: true,
		in: `package foo
import "errors"
type T struct {}
func F() (T, error) { return errors.New("foo") }
`,
		out: `package foo

import "errors"

type T struct {}

func F() (T, error) { return T{}, errors.New("foo") }
`,
	},

	// Synthesize zero values for structs in different package.
	{
		name: "external structs",
		skip: true,
		in: `package foo
import (
	"errors"
	"net/url"
)
func F() (url.URL, error) { return errors.New("foo") }
`,
		out: `package foo

import (
	"errors"
	"net/url"
)

func F() (url.URL, error) { return url.URL{}, errors.New("foo") }
`,
	},

	// Synthesize zero values for structs in different package
	// imported using an alias.
	{
		name: "external structs (with import alias)",
		skip: true,
		in: `package foo
import (
	"errors"
	url2 "net/url"
)
func F() (url2.URL, error) { return errors.New("foo") }
`,
		out: `package foo

import (
	"errors"
	url2 "net/url"
)

func F() (url2.URL, error) { return url2.URL{}, errors.New("foo") }
`,
	},

	// Synthesize zero values (nil) for interface types.
	{
		name: "interfaces",
		skip: true,
		in: `package foo
import "errors"
type I interface {}
func F() (I, error) { return errors.New("foo") }
`,
		out: `package foo

import "errors"

type I interface {}

func F() (I, error) { return nil, errors.New("foo") }
`,
	},

	// Synthesize zero values (nil) for interface types in external
	// packages.
	{
		name: "external interfaces",
		skip: true,
		in: `package foo
import (
	"errors"
	"io"
)
func F() (io.Reader, error) { return errors.New("foo") }
`,
		out: `package foo

import (
	"errors"
	"io"
)

func F() (io.Reader, error) { return nil, errors.New("foo") }
`,
	},

	// Preserve original when encountering type checking errors.
	{
		name: "preserve type errors",
		in: `package foo
import "errors"
func F() (X, error) { return errors.New("foo") }
`,
		out: `package foo

import "errors"

func F() (X, error) { return errors.New("foo") }
`,
	},

	// Add return values even when return values do not match
	// rightmost return types.
	{
		name: "return type errors",
		in: `package foo
import "errors"
func F() (int, int) { return errors.New("foo") }
`,
		out: `package foo

import "errors"

func F() (int, int) { return 0, errors.New("foo") }
`,
	},

	// Preserve when return has correct number of values.
	{
		name: "preserve valid-arity returns",
		in: `package foo
import "errors"
func F() (int, error) { return 7, errors.New("foo") }
`,
		out: `package foo

import "errors"

func F() (int, error) { return 7, errors.New("foo") }
`,
	},

	// Process returns in closures (not just top-level func decls).
	{
		name: "closures",
		in: `package foo
import "errors"
func main() { _ = func() (int, error) { return errors.New("foo") } }
`,
		out: `package foo

import "errors"

func main() { _ = func() (int, error) { return 0, errors.New("foo") } }
`,
	},

	// Ensure that closure scopes don't leak
	{
		name: "closure scopes",
		in: `package foo
import "errors"
func outer() (string, error) {
	_ = func() (int, error) { return errors.New("foo") }
	return errors.New("foo")
}
`,
		out: `package foo

import "errors"

func outer() (string, error) {
	_ = func() (int, error) { return 0, errors.New("foo") }
	return "", errors.New("foo")
}
`,
	},
}

func TestFixReturns(t *testing.T) {
	options := &Options{Fragment: true}

	for _, tt := range tests {
		if *only != "" && tt.name != *only {
			continue
		}
		if tt.skip {
			continue
		}
		buf, err := Process("", tt.name+".go", []byte(tt.in), options)
		if err != nil {
			t.Errorf("error on %q: %v", tt.name, err)
			continue
		}
		if got := string(buf); got != tt.out {
			t.Errorf("results diff on %q\nGOT:\n%s\nWANT:\n%s\n", tt.name, got, tt.out)
		}
	}
}
