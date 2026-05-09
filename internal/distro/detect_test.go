package distro

import "testing"

func TestDetect(t *testing.T) {
	cases := []struct {
		content string
		want    Distro
	}{
		{"ID=arch\n", Arch},
		{"ID=berserkarch\n", Arch},
		{"ID=manjaro\nID_LIKE=arch\n", Arch},
		{"ID=kali\n", Kali},
		{"ID=parrot\n", Parrot},
		{"ID=debian\n", Debian},
		{"ID=ubuntu\nID_LIKE=debian\n", Debian},
		{"ID=linuxmint\nID_LIKE=\"ubuntu debian\"\n", Debian},
		{"ID=something_weird\n", Unknown},
		{"", Unknown},
	}

	for _, tc := range cases {
		got := parseAndClassify(tc.content)
		if got != tc.want {
			t.Errorf("content=%q: got %v, want %v", tc.content, got, tc.want)
		}
	}
}

func TestDistroString(t *testing.T) {
	cases := map[Distro]string{
		Arch:    "arch",
		Kali:    "kali",
		Parrot:  "parrot",
		Debian:  "debian",
		Unknown: "unknown",
	}
	for d, want := range cases {
		if got := d.String(); got != want {
			t.Errorf("Distro(%d).String() = %q, want %q", d, got, want)
		}
	}
}
