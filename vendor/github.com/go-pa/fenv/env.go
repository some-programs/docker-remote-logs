package fenv

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// env contains environment variables as key/value map instead of a slice
// like the standard library does it.
type envmap map[string]string

// OSEnv returns os.Environ() as a env.
func OSEnv() map[string]string {
	oe := os.Environ()
	e := make(envmap, len(oe))
	err := e.parse(oe)
	if err != nil {
		panic(err)
	}
	return e

}

// Parse parses and sets environment variables in the KEY=VALUE string format
// used by the standard library.
func (em envmap) parse(oe []string) error {
	for _, v := range oe {
		kv := strings.SplitN(v, "=", 2)
		if len(kv) < 2 {
			return fmt.Errorf("expected format key=value in '%s'", v)
		}
		em[kv[0]] = kv[1]
	}
	return nil
}

// Slice returns the contents of the env in the format of the environment used
// by the standard library foruse with os/exec and similar packages.
// todo: this method can probably be removed
func (em envmap) slice() []string {
	var res []string
	for k, v := range em {
		res = append(res, k+"="+v)
	}
	sort.Strings(res)
	return res
}

// Update updates e with all entries from o.
// todo: this method can probably be removed
func (em envmap) update(o envmap) {
	for k, v := range o {
		em[k] = v
	}
}

// Set sets all variables in the process environment.
// todo: this method can probably be removed
func (em envmap) set() error {
	for k, v := range em {
		err := os.Setenv(k, v)
		if err != nil {
			return err
		}
	}
	return nil
}

func (em envmap) findAll(allNames []string) envmap {
	env := make(envmap)
	for _, k := range allNames {
		if v, ok := em[k]; ok {
			env[k] = v
		}
	}
	return env
}

func (em envmap) findFirst(allNames []string) (string, bool) {
	for _, name := range allNames {
		v := em[name]
		if v != "" {
			return name, true
		}
	}
	return "", false
}
