package distro

import (
	"bufio"
	"io"
	"os"
	"strings"
)

type Distro int

const (
	Arch Distro = iota
	Kali
	Parrot
	Debian
	Unknown
)

func (d Distro) String() string {
	switch d {
	case Arch:
		return "arch"
	case Kali:
		return "kali"
	case Parrot:
		return "parrot"
	case Debian:
		return "debian"
	default:
		return "unknown"
	}
}

func Detect() Distro {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return Unknown
	}
	defer f.Close()
	return detectFrom(f)
}

func detectFrom(r io.Reader) Distro {
	fields := parseFields(r)
	return classify(
		strings.ToLower(fields["ID"]),
		strings.ToLower(fields["ID_LIKE"]),
	)
}

func parseFields(r io.Reader) map[string]string {
	fields := map[string]string{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.Trim(strings.TrimSpace(parts[1]), `"`)
			fields[key] = val
		}
	}
	return fields
}

func classify(id, idLike string) Distro {
	switch {
	case id == "arch" || id == "berserkarch" || strings.Contains(idLike, "arch"):
		return Arch
	case id == "kali":
		return Kali
	case id == "parrot":
		return Parrot
	case id == "debian" || strings.Contains(idLike, "debian"):
		return Debian
	default:
		return Unknown
	}
}

// parseAndClassify is used by tests to classify os-release content directly.
func parseAndClassify(content string) Distro {
	return detectFrom(strings.NewReader(content))
}
