package docker

import (
	"fmt"
	"strings"
)

// SearchResult locates a container within the group/category hierarchy.
type SearchResult struct {
	GroupName    string
	CategoryName string // empty for flat groups
	Container    Container
}

// Search returns all containers whose name contains term (case-insensitive).
func Search(groups []Group, term string) []SearchResult {
	lower := strings.ToLower(term)
	var results []SearchResult

	for _, g := range groups {
		for _, c := range g.Containers {
			if strings.Contains(strings.ToLower(c.Name), lower) {
				results = append(results, SearchResult{
					GroupName: g.Name,
					Container: c,
				})
			}
		}
		for _, cat := range g.Categories {
			for _, c := range cat.Containers {
				if strings.Contains(strings.ToLower(c.Name), lower) {
					results = append(results, SearchResult{
						GroupName:    g.Name,
						CategoryName: cat.Name,
						Container:    c,
					})
				}
			}
		}
	}

	return results
}

// GetGroup returns the 1-based indexed group, or an error if out of range.
func GetGroup(groups []Group, idx int) (*Group, error) {
	if idx < 1 || idx > len(groups) {
		return nil, fmt.Errorf("group index %d out of range (valid: 1–%d)", idx, len(groups))
	}
	g := groups[idx-1]
	return &g, nil
}

// GetCategory returns the 1-based indexed category from a group, or an error.
func GetCategory(group *Group, idx int) (*Category, error) {
	if len(group.Categories) == 0 {
		return nil, fmt.Errorf("group %q has no categories", group.Name)
	}
	if idx < 1 || idx > len(group.Categories) {
		return nil, fmt.Errorf("category index %d out of range (valid: 1–%d)", idx, len(group.Categories))
	}
	c := group.Categories[idx-1]
	return &c, nil
}

// AllContainers returns all containers in a group, flattening categories.
func AllContainers(group *Group) []Container {
	var out []Container
	out = append(out, group.Containers...)
	for _, cat := range group.Categories {
		out = append(out, cat.Containers...)
	}
	return out
}
