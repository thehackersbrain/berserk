package docker

import (
	"strings"
)

// Search returns all containers whose name contains term (case-insensitive).
func Search(containers []Container, term string) []Container {
	lower := strings.ToLower(term)
	var results []Container

	for _, c := range containers {
		if strings.Contains(strings.ToLower(c.Name), lower) {
			results = append(results, c)
		}
	}

	return results
}

// SearchByCategory returns all containers whose category list contains term
// (case-insensitive).
func SearchByCategory(containers []Container, term string) []Container {
	lower := strings.ToLower(term)
	var results []Container

	for _, c := range containers {
		for _, cat := range c.Category {
			if strings.EqualFold(cat, lower) || strings.Contains(strings.ToLower(cat), lower) {
				results = append(results, c)
				break
			}
		}
	}

	return results
}
