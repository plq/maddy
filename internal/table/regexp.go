package table

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/foxcpp/maddy/framework/config"
	"github.com/foxcpp/maddy/framework/module"
)

type Regexp struct {
	modName    string
	instName   string
	inlineArgs []string

	re          *regexp.Regexp
	replacement string

	expandPlaceholders bool
}

func NewRegexp(modName, instName string, _, inlineArgs []string) (module.Module, error) {
	return &Regexp{
		modName:    modName,
		instName:   instName,
		inlineArgs: inlineArgs,
	}, nil
}

func (r *Regexp) Init(cfg *config.Map) error {
	var (
		fullMatch       bool
		caseInsensitive bool
	)
	cfg.Bool("full_match", false, true, &fullMatch)
	cfg.Bool("case_insensitive", false, true, &caseInsensitive)
	cfg.Bool("expand_replaceholders", false, true, &r.expandPlaceholders)
	if _, err := cfg.Process(); err != nil {
		return err
	}

	if len(r.inlineArgs) > 2 {
		return fmt.Errorf("%s: at most two arguments accepted", r.modName)
	}
	regex := r.inlineArgs[0]
	if len(r.inlineArgs) == 2 {
		r.replacement = r.inlineArgs[1]
	}

	if fullMatch {
		if !strings.HasPrefix(regex, "^") {
			regex = "^" + regex
		}
		if !strings.HasSuffix(regex, "$") {
			regex = regex + "$"
		}
	}

	if caseInsensitive {
		regex = "(?i)" + regex
	}

	var err error
	r.re, err = regexp.Compile(regex)
	if err != nil {
		return fmt.Errorf("%s: %v", r.modName, err)
	}
	return nil
}

func (r *Regexp) Name() string {
	return r.modName
}

func (r *Regexp) InstanceName() string {
	return r.modName
}

func (r *Regexp) Lookup(key string) (string, bool, error) {
	matches := r.re.FindStringSubmatchIndex(key)
	if matches == nil {
		return "", false, nil
	}

	if !r.expandPlaceholders {
		return r.replacement, true, nil
	}

	return string(r.re.ExpandString([]byte{}, r.replacement, key, matches)), true, nil
}

func init() {
	module.RegisterDeprecated("regexp", "table.regexp", NewRegexp)
	module.Register("table.regexp", NewRegexp)
}
