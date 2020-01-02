package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"

	"golang.org/x/tools/imports"
)

type Config struct {
	// Accept fragment of a source file (no package statement)
	Fragment *bool `json:"fragment,omitempty"`

	// Print non-fatal typechecking errors to stderr (interferes with some tools that use gofmt/goimports and expect them to only print code or diffs to stdout + stderr)
	PrintErrors *bool `json:"printErrors,omitempty"`

	// Report all errors (not just the first 10 on different lines)
	AllErrors *bool `json:"allErrors,omitempty"`

	// Remove bare returns
	RemoveBareReturns *bool `json:"removeBareReturns,omitempty"`

	// put imports beginning with this string after 3rd-party packages (see goimports)
	Local string `json:"local,omitempty"`
}

func loadConfigFile() error {
	userObj, err := user.Current()
	if err != nil {
		return err
	}
	fpath := filepath.Join(userObj.HomeDir, ".goreturns.json")
	jsonBytes, err := ioutil.ReadFile(fpath)
	if os.IsNotExist(err) {
		return nil
	}
	c := &Config{}
	err = json.Unmarshal(jsonBytes, c)
	if err != nil {
		return err
	}
	if c.Fragment != nil {
		options.Fragment = *c.Fragment
	}
	if c.PrintErrors != nil {
		options.PrintErrors = *c.PrintErrors
	}
	if c.AllErrors != nil {
		options.AllErrors = *c.AllErrors
	}
	if c.RemoveBareReturns != nil {
		options.RemoveBareReturns = *c.RemoveBareReturns
	}
	if c.Local != "" {
		imports.LocalPrefix = c.Local
	}
	return nil
}
