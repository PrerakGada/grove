package config

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Validate checks a config for the mistakes that would otherwise surface as a
// broken worktree hours later.
func (c *Config) Validate() error {
	var errs []string

	if c.Version > SchemaVersion {
		errs = append(errs, fmt.Sprintf(
			"config version %d is newer than this build understands (%d) — upgrade wt",
			c.Version, SchemaVersion))
	}
	if !nameRe.MatchString(c.Project) {
		errs = append(errs, fmt.Sprintf("project %q must be lowercase alphanumeric with dashes", c.Project))
	}
	if c.MaxSlots < 1 || c.MaxSlots > 999 {
		errs = append(errs, fmt.Sprintf("maxSlots %d out of range (1-999)", c.MaxSlots))
	}
	if len(c.Services) == 0 {
		errs = append(errs, "no services defined — there would be nothing to run")
	}

	seen := map[string]bool{}
	for _, s := range c.Services {
		if !nameRe.MatchString(s.Name) {
			errs = append(errs, fmt.Sprintf("service %q: name must be lowercase alphanumeric with dashes", s.Name))
		}
		if seen[s.Name] {
			errs = append(errs, fmt.Sprintf("service %q defined twice", s.Name))
		}
		seen[s.Name] = true
		if s.Kind != "frontend" && s.Kind != "backend" {
			errs = append(errs, fmt.Sprintf("service %q: kind must be \"frontend\" or \"backend\", got %q", s.Name, s.Kind))
		}
		if s.Cmd == "" {
			errs = append(errs, fmt.Sprintf("service %q: cmd is required", s.Name))
		}
		if s.Port.Via != "env" && s.Port.Via != "flag" {
			errs = append(errs, fmt.Sprintf("service %q: port.via must be \"env\" or \"flag\", got %q", s.Name, s.Port.Via))
		}
		if err := checkBand(s.Band, c.MaxSlots, "service "+s.Name); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if c.HasDB() {
		if err := checkBand(c.DB.Band, c.MaxSlots, "db"); err != nil {
			errs = append(errs, err.Error())
		}
		if c.DB.Driver != "docker-postgres" {
			errs = append(errs, fmt.Sprintf("db.driver %q not supported yet (have: docker-postgres, none)", c.DB.Driver))
		}
		// Empty profiles is allowed: the project may only ever want empty
		// databases, or the commands may not be written yet. `wt db` refuses at
		// run time with a clearer message than a config error could give.
		if c.DB.Default != "" && len(c.DB.Profiles) > 0 {
			if _, ok := c.DB.Profiles[c.DB.Default]; !ok {
				errs = append(errs, fmt.Sprintf("db.default %q is not one of db.profiles", c.DB.Default))
			}
		}
	}

	if err := c.checkBandOverlap(); err != nil {
		errs = append(errs, err.Error())
	}

	if c.Deps != nil && c.Deps.Manager == "" && c.Deps.Install != "" {
		errs = append(errs, "deps.install is set but deps.manager is empty")
	}

	if len(errs) > 0 {
		sort.Strings(errs)
		return fmt.Errorf("%s is invalid:\n  - %s", FileName, strings.Join(errs, "\n  - "))
	}
	return nil
}

func checkBand(band, maxSlots int, what string) error {
	if band < 1024 {
		return fmt.Errorf("%s: band %d is a privileged port (<1024)", what, band)
	}
	if band+maxSlots > 65535 {
		return fmt.Errorf("%s: band %d + %d slots exceeds 65535", what, band, maxSlots)
	}
	return nil
}

// checkBandOverlap catches the failure that capped the bash predecessor at 24
// worktrees: bands spaced closer together than maxSlots, so high slots of one
// service collide with low slots of the next.
func (c *Config) checkBandOverlap() error {
	type entry struct {
		name string
		band int
	}
	var es []entry
	for _, s := range c.Services {
		es = append(es, entry{s.Name, s.Band})
	}
	if c.HasDB() {
		es = append(es, entry{"db", c.DB.Band})
	}
	sort.Slice(es, func(i, j int) bool { return es[i].band < es[j].band })

	for i := 1; i < len(es); i++ {
		prev, cur := es[i-1], es[i]
		if prev.band+c.MaxSlots >= cur.band {
			return fmt.Errorf(
				"bands overlap: %s (%d) reaches %d at slot %d, colliding with %s (%d) — space bands at least %d apart",
				prev.name, prev.band, prev.band+c.MaxSlots, c.MaxSlots, cur.name, cur.band, c.MaxSlots+1)
		}
	}
	return nil
}
