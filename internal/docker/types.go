package docker

// Container is a single docker image entry with its pull and run commands.
type Container struct {
	Name            string   `yaml:"name"`
	Description     string   `yaml:"description"`
	Category        []string `yaml:"category"`
	Command         string   `yaml:"command"` // docker pull ...
	Run             string   `yaml:"run"`     // docker run ...
	RuntimeComments []string `yaml:"runtime_comments"`
}
