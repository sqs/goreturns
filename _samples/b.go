package foo

import "errors"

func Fb() (res int, err error) {
	err = errors.New("foo")
	return
}
