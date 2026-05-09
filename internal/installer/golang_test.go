package installer

import (
	"testing"

	"github.com/thehackersbrain/berserk/internal/registry"
)

func TestGoBuildPkg(t *testing.T) {
	cases := []struct {
		tool registry.Tool
		want string
	}{
		{
			registry.Tool{Name: "httpx", Repo: "projectdiscovery/httpx/cmd/httpx"},
			"github.com/projectdiscovery/httpx/cmd/httpx@latest",
		},
		{
			registry.Tool{Name: "ffuf", Repo: "ffuf/ffuf/v2"},
			"github.com/ffuf/ffuf/v2@latest",
		},
		{
			// package field takes precedence; already has @
			registry.Tool{Name: "amass", Package: "github.com/owasp-amass/amass/v4/...@master"},
			"github.com/owasp-amass/amass/v4/...@master",
		},
		{
			// package field without @ gets @latest appended
			registry.Tool{Name: "sometool", Package: "github.com/owner/sometool"},
			"github.com/owner/sometool@latest",
		},
		{
			// repo already has @ (edge case — don't double-append)
			registry.Tool{Name: "thing", Repo: "owner/thing@v1.2.3"},
			"github.com/owner/thing@v1.2.3",
		},
	}

	for _, tc := range cases {
		got := goBuildPkg(tc.tool)
		if got != tc.want {
			t.Errorf("tool=%q: got %q, want %q", tc.tool.Name, got, tc.want)
		}
	}
}
