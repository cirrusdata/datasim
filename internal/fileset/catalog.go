package fileset

import (
	"fmt"
	"slices"
)

// Catalog stores the built-in fileset profiles.
type Catalog struct {
	profiles map[string]Profile
}

// NewCatalog constructs the built-in fileset profile catalog.
func NewCatalog() *Catalog {
	profiles := map[string]Profile{
		"corporate": corporateProfile(),
		"school":    schoolProfile(),
		"nasa":      nasaProfile(),
	}

	return &Catalog{profiles: profiles}
}

// DefaultProfileName returns the default fileset profile name.
func (c *Catalog) DefaultProfileName() string {
	return "corporate"
}

// Get returns a fileset profile by name.
func (c *Catalog) Get(name string) (Profile, error) {
	profile, ok := c.profiles[name]
	if !ok {
		return Profile{}, fmt.Errorf("unknown fileset profile %q", name)
	}

	return profile, nil
}

// Names returns the built-in profile names in sorted order.
func (c *Catalog) Names() []string {
	names := make([]string, 0, len(c.profiles))
	for name := range c.profiles {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
