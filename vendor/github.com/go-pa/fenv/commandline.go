package fenv

import (
	"flag"
	"strings"
	"sync"
)

var (
	commandLineMu sync.Mutex
	commandLine   = NewEnvSet(flag.CommandLine)
)

// CommandLinePrefix sets the prefix used by the package level env set functions.
func CommandLinePrefix(prefix ...string) {
	commandLineMu.Lock()
	defer commandLineMu.Unlock()
	if commandLine.prefix != "" {
		panic("prefix already set: " + commandLine.prefix)
	}
	if commandLine.Parsed() {
		panic("default commandline envset already parsed")
	}
	commandLine.prefix = strings.ToUpper(strings.Join(prefix, "_"))
}

func Var(v interface{}, names ...string) {
	commandLineMu.Lock()
	defer commandLineMu.Unlock()
	commandLine.Var(v, names...)
}

func Parse() error {
	commandLineMu.Lock()
	defer commandLineMu.Unlock()
	return commandLine.Parse()
}

func MustParse() {
	err := Parse()
	if err != nil {
		panic(err)
	}
}

func Parsed() bool {
	commandLineMu.Lock()
	defer commandLineMu.Unlock()
	return commandLine.Parsed()
}

func VisitAll(fn func(e EnvFlag)) {
	commandLineMu.Lock()
	defer commandLineMu.Unlock()
	commandLine.VisitAll(fn)
}
