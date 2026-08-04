package config

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var placeholder = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.]+)\s*\}\}`)

// Vars are the substitutions available to templated config values.
//
// Deliberately a closed set with hard errors on unknown keys: a typo in
// `.wt.json` should fail at provisioning time, not silently write the literal
// string "{{port.backand}}" into a worktree's .env.
type Vars map[string]string

// NewVars builds the standard variable set for one worktree.
func NewVars(cfg *Config, name, mainRoot, wtPath string, slot int) Vars {
	v := Vars{
		"name": name,
		"slot": strconv.Itoa(slot),
		"main": mainRoot,
		"wt":   wtPath,
	}
	for _, s := range cfg.Services {
		v["port."+s.Name] = strconv.Itoa(s.Band + slot)
	}
	if cfg.HasDB() {
		v["port.db"] = strconv.Itoa(cfg.DB.Band + slot)
		v["db.port"] = strconv.Itoa(cfg.DB.Band + slot)
		// db.name is templated itself, so render it with what we have so far.
		if n, err := Render(cfg.DB.Name, v); err == nil {
			v["db.name"] = n
		}
		if n, err := Render(cfg.DB.Container, v); err == nil {
			v["db.container"] = n
		}
		if n, err := Render(cfg.DB.Volume, v); err == nil {
			v["db.volume"] = n
		}
		v["db.user"] = cfg.DB.User
		v["db.password"] = cfg.DB.Password
	}
	return v
}

// Render substitutes {{key}} placeholders, failing on any key it does not know.
func Render(s string, v Vars) (string, error) {
	var bad []string
	out := placeholder.ReplaceAllStringFunc(s, func(m string) string {
		key := placeholder.FindStringSubmatch(m)[1]
		val, ok := v[key]
		if !ok {
			bad = append(bad, key)
			return m
		}
		return val
	})
	if len(bad) > 0 {
		sort.Strings(bad)
		return "", fmt.Errorf("unknown template variable(s) %s in %q (known: %s)",
			strings.Join(uniq(bad), ", "), s, strings.Join(v.keys(), ", "))
	}
	return out, nil
}

// RenderMap renders every value in m, preserving keys.
func RenderMap(m map[string]string, v Vars) (map[string]string, error) {
	out := make(map[string]string, len(m))
	for k, val := range m {
		r, err := Render(val, v)
		if err != nil {
			return nil, fmt.Errorf("key %s: %w", k, err)
		}
		out[k] = r
	}
	return out, nil
}

func (v Vars) keys() []string {
	ks := make([]string, 0, len(v))
	for k := range v {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func uniq(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
