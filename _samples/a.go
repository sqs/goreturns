package foo

import "errors"

func F() (int, error) {
	return errors.New("foo")
}
