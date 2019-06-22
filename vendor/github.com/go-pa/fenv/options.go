package fenv

import "strings"

type Option func(e *EnvSet)

func Prefix(prefix ...string) Option {
	return func(e *EnvSet) {
		e.prefix = strings.ToUpper(strings.Join(prefix, "_"))
	}
}

func ContinueOnError() Option {
	return func(e *EnvSet) {
		e.continueOnError = true
	}
}
