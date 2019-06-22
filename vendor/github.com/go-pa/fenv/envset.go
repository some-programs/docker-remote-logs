package fenv

import (
	"errors"
	"flag"
	"fmt"
	"reflect"
	"strings"
)

// NewEnvSet returns a *EnvSet
func NewEnvSet(fs *flag.FlagSet, opt ...Option) *EnvSet {
	es := &EnvSet{
		fs:      fs,
		names:   make(map[string][]string),
		exclude: make(map[string]bool),
		self:    make(map[string]bool),
		env:     make(map[string]string),
		errs:    make(map[string]error),
	}
	for _, o := range opt {
		o(es)
	}
	return es
}

func ParseSet(fs *flag.FlagSet, opt ...Option) error {
	es := NewEnvSet(fs, opt...)
	return es.Parse()
}

// EnvFlag is used bu the EnvSet.Visit* funcs.
type EnvFlag struct {
	// the associated flag.Flag
	Flag *flag.Flag

	// the environment variable name which the value for flag parsing was
	// extracted from. This field is set regardless if setting the flag succeeded or not
	// succeeds or fails.
	Name string

	// Value is the value of the environmet variable Name.
	Value string

	// all the environment variable names mapped associated with the flag.
	Names []string

	// Values for all corresponding Names which were present when EnvSet.Parse() was called.
	Env map[string]string

	// IsSet is true if the flag has been set by the owning EnvSet.Parse() function or by the associated FlagSet.
	IsSet bool

	// IsSelfSet is true if the flag value was successfully set by the
	// associated EnvSet.Parse() function.
	IsSelfSet bool

	// error caused by flag.Set when the envset tried to set it
	Err error
}

// ErrAlreadyParsed is returned by EnvSet.Parse() if the EnvSet already was parsed.
var ErrAlreadyParsed = errors.New("the envset is already parsed")

// ErrMultipleSet is returned by EnvSet.Parse() if the ContinueOnError is enabled and more than one flag failed to be set.
var ErrMultipleSet = errors.New("multiple errors encountered when calling flag.Set()")

// FlagError
type FlagError EnvFlag

func (f FlagError) Error() string {
	return fmt.Sprintf("failed to set flag %q with value %q", f.Flag.Name, f.Value)
}

// EnvSet adds environment variable support for flag.FlagSet.
type EnvSet struct {
	fs              *flag.FlagSet // associated FlagSet
	prefix          string        // the environment variable name prefix
	continueOnError bool          // continue to parse flags after failing to set
	parsed          bool          // true after Parse() has been run

	//  the key is the environment variable name
	env envmap // environmenet variables collected by Parse(), may be partial

	// the key for all these maps are based on the flag.Flag.Name
	names   map[string][]string // all configured env var names
	exclude map[string]bool     // excluded from env vars
	errs    map[string]error    // errors which occured during flag.Set()
	self    map[string]bool     // was the flag set by this EnvSet

}

// Var enables associattion with environment variable names other than the default auto generated ones
//
// If no name argument is supplied the variable will be excluded from
// environment pasrsing and the EnvSet.VisitAll method. The special name value
// emtpy string "" will be translated to the automatically generated
// environment variable name. This function will panic if given an
func (s *EnvSet) Var(value interface{}, names ...string) {
	f, err := s.findFlag(value)
	if err != nil {
		panic(err)
	}
	if f == nil {
		panic(fmt.Sprintf("%T (%v) is not a member of the flagset", value, value))
	}
	if len(names) == 0 {
		s.exclude[f.Name] = true
		delete(s.names, f.Name)
		return
	}
	for i, v := range names {
		names[i] = fmtEnv(v)
	}
	s.names[f.Name] = names
	delete(s.exclude, f.Name)
}

func (s *EnvSet) Flag(flagName string, names ...string) {
	var flg *flag.Flag
	s.fs.VisitAll(func(f *flag.Flag) {
		if flg != nil {
			return
		}
		if f.Name == flagName {
			flg = f
		}
	})
	if flg == nil {
		panic(fmt.Sprintf("%s is not a registed flag in the flagset", flagName))
	}
	s.Var(flg.Value, names...)
}

// Parsed reports whether s.Parse has been called.
func (s *EnvSet) Parsed() bool {
	return s.parsed
}

func (s *EnvSet) Parse() error {
	return s.ParseEnv(OSEnv())
}

func (s *EnvSet) ParseEnv(env map[string]string) error {
	if s.parsed {
		return ErrAlreadyParsed
	}
	s.parsed = true
	actual := make(map[string]bool)
	s.fs.Visit(func(f *flag.Flag) { actual[f.Name] = true })
	em := envmap(env)
	var (
		err  error
		nerr int
	)
	s.fs.VisitAll(func(f *flag.Flag) {
		if s.exclude[f.Name] {
			return
		}
		names := s.allNames(f)
		s.names[f.Name] = names
		env := em.findAll(names)
		s.env.update(env)
		if actual[f.Name] {
			return // skip if already set
		}
		if err != nil && !s.continueOnError {
			return
		}
		name, found := env.findFirst(names)
		if found {
			if ferr := f.Value.Set(env[name]); ferr != nil {
				nerr++
				s.errs[f.Name] = ferr
				err = FlagError{
					Flag:  f,
					Env:   env,
					Name:  name,
					Value: env[name],
					Names: names,
					Err:   ferr,
				}
			} else {
				s.self[f.Name] = true
			}

		}
	})
	if nerr > 1 {
		return ErrMultipleSet
	}
	return err
}

// Visit visits all non exluded EnvFlags in the flagset
func (s *EnvSet) VisitAll(fn func(e EnvFlag)) {
	actual := make(map[string]bool)
	s.fs.Visit(func(f *flag.Flag) { actual[f.Name] = true })
	s.fs.VisitAll(func(f *flag.Flag) {
		n := f.Name
		if s.exclude[n] {
			return
		}
		allNames := s.allNames(f)
		name, _ := s.env.findFirst(allNames)
		fn(EnvFlag{
			Flag:      f,
			Name:      name,
			Value:     s.env[name],
			Names:     allNames,
			IsSet:     actual[f.Name] || s.self[f.Name],
			IsSelfSet: s.self[f.Name],
			Env:       s.env.findAll(allNames),
			Err:       s.errs[f.Name],
		})
	})
}

// returns the flag.Flag instace bound to ref or nil if not found
func (s *EnvSet) findFlag(v interface{}) (*flag.Flag, error) {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("not a pointer: %v", v)
	}
	vp := rv.Pointer()
	var flg *flag.Flag
	s.fs.VisitAll(func(f *flag.Flag) {
		p := reflect.ValueOf(f.Value).Pointer()
		if vp == p {
			// todo: find out which PLUGIN_ or other env var we are refering to
			// so that a better message can be printed
			flg = f
		}
	})
	return flg, nil
}

// allNames return all environment names for a given flag
func (s *EnvSet) allNames(f *flag.Flag) []string {
	var allNames []string
	if names, ok := s.names[f.Name]; ok {
		for _, name := range names {
			if name == "" {
				name = fmtEnv(f.Name, s.prefix)
			}
			allNames = append(allNames, name)
		}
	}
	if len(allNames) == 0 {
		allNames = append(allNames, fmtEnv(f.Name, s.prefix))
	}
	return allNames
}

// fmtEnv formats a environment variable name as expected by this package.
func fmtEnv(s string, prefix ...string) string {
	s = strings.Join(prefix, "_") + s
	s = strings.Replace(s, ".", "_", -1)
	s = strings.Replace(s, "-", "_", -1)
	s = strings.ToUpper(s)
	return s
}
