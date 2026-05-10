package docker

// Container is a single docker image entry with its pull and run commands.
type Container struct {
	Name            string   `yaml:"name"`
	Description     string   `yaml:"description"`
	Command         string   `yaml:"command"`          // docker pull ...
	Run             string   `yaml:"run"`               // docker run ...
	RuntimeComments []string `yaml:"runtime_comments"`
}

// Category groups containers under a named heading within a Group.
type Category struct {
	Name        string      `yaml:"name"`
	Description string      `yaml:"description"`
	Containers  []Container `yaml:"containers"`
}

// Group is a top-level section in docker.yaml. It holds either a flat list
// of Containers or nested Categories — never both.
type Group struct {
	Name        string      `yaml:"name"`
	Description string      `yaml:"description"`
	Containers  []Container `yaml:"containers"` // flat groups
	Categories  []Category  `yaml:"categories"` // categorized groups
}
