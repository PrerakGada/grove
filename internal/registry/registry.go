// Package registry owns Grove's machine-local state, kept in ~/.wt.
//
// The split matters: `.wt.json` in a repository describes the project's *shape*
// and is committed so a team shares one definition. The registry records what
// this particular machine has handed out — which port block each project owns
// and which worktree holds which slot. None of that belongs in git.
//
// Reserving bands globally is what makes Grove safe to use in more than one
// repository at once. Without it, two projects independently choosing "3100"
// fight for ports the moment both are running.
package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const Version = 1

// ReservedPorts are defaults of common tooling. A band is rejected if any of
// these falls inside it, so Grove never hands a project a range that will
// collide with an unrelated dev server on the same machine.
var ReservedPorts = []int{
	3000, 3001, // Next.js, Create React App
	4200,  // Angular
	5000,  // macOS AirPlay Receiver
	5173,  // Vite
	5432,  // PostgreSQL
	3306,  // MySQL
	6379,  // Redis
	8000,  // Django, misc
	8080,  // Tomcat, misc
	8081,  // misc
	9000,  // misc
	9200,  // Elasticsearch
	27017, // MongoDB
}

// BandStep is the spacing between adjacent bands. One service owns one block;
// a worktree's slot is a +1 offset inside it, so the trailing digits of any
// port are always the slot number.
const BandStep = 100

// SearchLow and SearchHigh bound automatic band suggestions.
const (
	SearchLow  = 3100
	SearchHigh = 9900
)

type Registry struct {
	Version  int                 `json:"version"`
	Projects map[string]*Project `json:"projects"`
}

type Project struct {
	Root     string         `json:"root"`
	Bands    []int          `json:"bands"`
	MaxSlots int            `json:"maxSlots"`
	Slots    map[string]int `json:"slots"`
}

// Home is the Grove state directory, ~/.wt unless WT_HOME overrides it.
func Home() string {
	if h := os.Getenv("WT_HOME"); h != "" {
		return h
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".wt"
	}
	return filepath.Join(home, ".wt")
}

func path() string { return filepath.Join(Home(), "registry.json") }

// Load reads the registry, returning an empty one if it does not exist yet.
func Load() (*Registry, error) {
	b, err := os.ReadFile(path())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Registry{Version: Version, Projects: map[string]*Project{}}, nil
		}
		return nil, err
	}
	var r Registry
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("%s: %w", path(), err)
	}
	if r.Projects == nil {
		r.Projects = map[string]*Project{}
	}
	return &r, nil
}

// Save writes the registry atomically.
func (r *Registry) Save() error {
	if err := os.MkdirAll(Home(), 0o755); err != nil {
		return err
	}
	r.Version = Version
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')

	tmp, err := os.CreateTemp(Home(), "registry-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path())
}

// Update runs fn against the registry while holding an exclusive lock, then
// saves. Grove is routinely driven by several concurrent agents, so
// read-modify-write without a lock loses slot allocations.
func Update(fn func(*Registry) error) error {
	unlock, err := lock()
	if err != nil {
		return err
	}
	defer unlock()

	r, err := Load()
	if err != nil {
		return err
	}
	if err := fn(r); err != nil {
		return err
	}
	return r.Save()
}

// Register records a project's band reservation, replacing any previous one.
func (r *Registry) Register(name, root string, bands []int, maxSlots int) {
	p := r.Projects[name]
	if p == nil {
		p = &Project{Slots: map[string]int{}}
		r.Projects[name] = p
	}
	p.Root = root
	p.Bands = append([]int(nil), bands...)
	sort.Ints(p.Bands)
	p.MaxSlots = maxSlots
	if p.Slots == nil {
		p.Slots = map[string]int{}
	}
}

// ProjectConflicts reports bands that overlap another registered project.
//
// These are hard conflicts: two projects on one machine genuinely cannot both
// own a port, and Grove is the only thing that can see the collision coming.
func (r *Registry) ProjectConflicts(self string, bands []int, maxSlots int) []string {
	var out []string
	for _, b := range bands {
		lo, hi := b, b+maxSlots
		for name, p := range r.Projects {
			if name == self {
				continue
			}
			for _, ob := range p.Bands {
				olo, ohi := ob, ob+p.MaxSlots
				if lo <= ohi && olo <= hi {
					out = append(out, fmt.Sprintf(
						"band %d-%d overlaps project %q band %d-%d", lo, hi, name, olo, ohi))
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

// ReservedConflicts reports bands containing a well-known default port.
//
// Softer than a project conflict: nothing is necessarily listening there. It
// matters only at the slot whose offset lands on that port, so it is worth
// avoiding when choosing a new band and worth reporting — but not worth
// relocating a band the project already uses.
func (r *Registry) ReservedConflicts(bands []int, maxSlots int) []string {
	var out []string
	for _, b := range bands {
		lo, hi := b, b+maxSlots
		for _, rp := range ReservedPorts {
			if rp >= lo && rp <= hi {
				out = append(out, fmt.Sprintf(
					"band %d-%d reaches %d at slot %d, a well-known default port",
					lo, hi, rp, rp-lo))
			}
		}
	}
	sort.Strings(out)
	return out
}

// Conflicts reports every conflict, hard and soft.
func (r *Registry) Conflicts(self string, bands []int, maxSlots int) []string {
	return append(r.ProjectConflicts(self, bands, maxSlots),
		r.ReservedConflicts(bands, maxSlots)...)
}

// SuggestBands returns n free, non-overlapping bands for a new project.
func (r *Registry) SuggestBands(self string, n, maxSlots int) []int {
	var out []int
	for b := SearchLow; b <= SearchHigh && len(out) < n; b += BandStep {
		cand := append(append([]int(nil), out...), b)
		if len(r.Conflicts(self, cand, maxSlots)) == 0 {
			out = append(out, b)
		}
	}
	return out
}

// Slot returns the slot held by a worktree, if any.
func (p *Project) Slot(worktree string) (int, bool) {
	s, ok := p.Slots[worktree]
	return s, ok
}

// Allocate hands a worktree the lowest free slot, or returns its existing one.
func (p *Project) Allocate(worktree string) (int, error) {
	if s, ok := p.Slots[worktree]; ok {
		return s, nil
	}
	used := map[int]bool{}
	for _, s := range p.Slots {
		used[s] = true
	}
	for s := 1; s <= p.MaxSlots; s++ {
		if !used[s] {
			p.Slots[worktree] = s
			return s, nil
		}
	}
	return 0, fmt.Errorf("all %d slots are in use — delete a worktree first", p.MaxSlots)
}

// Release frees a worktree's slot.
func (p *Project) Release(worktree string) { delete(p.Slots, worktree) }

// Ports maps service name to port for a slot, given the project's bands in the
// same order as the config's services.
func Port(band, slot int) int { return band + slot }
