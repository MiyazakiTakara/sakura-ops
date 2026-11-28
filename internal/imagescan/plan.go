package imagescan

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v4"
)

var pinnedRef = regexp.MustCompile(`^([a-z0-9][a-z0-9._/-]*)@(sha256:[0-9a-f]{64})$`)

// External is a registry image that Compose pulls; it must be digest-pinned.
type External struct {
	Label  string // last path element of the repository, used as the image label
	Ref    string // repo@sha256:digest
	Repo   string
	Digest string // sha256:...
}

// Build is a service built locally from a Dockerfile.
type Build struct {
	Label   string // build target, or the service name when no target is set
	Service string
	Target  string // empty means the Dockerfile's last stage
}

// Plan is the set of images a scan must cover.
type Plan struct {
	External []External
	Builds   []Build
}

type composeFile struct {
	Services map[string]struct {
		Image string    `yaml:"image"`
		Build yaml.Node `yaml:"build"`
	} `yaml:"services"`
}

// ParsePlan derives the scan set from a Compose file. Identical pinned
// references collapse into one entry; the same repository with different
// digests, an unpinned external image, a service with neither image nor build,
// or an empty services section is an error.
func ParsePlan(b []byte) (*Plan, error) {
	var c composeFile
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse compose file: %w", err)
	}
	if len(c.Services) == 0 {
		return nil, fmt.Errorf("compose file has no services")
	}
	names := make([]string, 0, len(c.Services))
	for n := range c.Services {
		names = append(names, n)
	}
	sort.Strings(names)

	p := &Plan{}
	byRepo := map[string]string{}  // repo -> digest
	byLabel := map[string]string{} // label -> what claimed it
	for _, n := range names {
		s := c.Services[n]
		if s.Build.Kind != 0 {
			target := ""
			switch s.Build.Kind {
			case yaml.MappingNode:
				var bs struct {
					Target string `yaml:"target"`
				}
				if err := s.Build.Decode(&bs); err != nil {
					return nil, fmt.Errorf("service %s: build: %w", n, err)
				}
				target = strings.TrimSpace(bs.Target)
			case yaml.ScalarNode: // short form: build context only
			default:
				return nil, fmt.Errorf("service %s: unsupported build section", n)
			}
			label := target
			if label == "" {
				label = n
			}
			if prev, dup := byLabel[label]; dup {
				return nil, fmt.Errorf("service %s: image label %q already used by %s", n, label, prev)
			}
			byLabel[label] = "service " + n
			p.Builds = append(p.Builds, Build{Label: label, Service: n, Target: target})
			continue
		}
		if s.Image == "" {
			return nil, fmt.Errorf("service %s: neither image nor build", n)
		}
		m := pinnedRef.FindStringSubmatch(s.Image)
		if m == nil {
			return nil, fmt.Errorf("service %s: image %q is not digest-pinned (want name@sha256:<64 hex>)", n, s.Image)
		}
		repo, digest := m[1], m[2]
		if prev, ok := byRepo[repo]; ok {
			if prev != digest {
				return nil, fmt.Errorf("service %s: repository %s pinned to conflicting digests %s and %s", n, repo, prev, digest)
			}
			continue
		}
		byRepo[repo] = digest
		label := repo[strings.LastIndex(repo, "/")+1:]
		if prev, dup := byLabel[label]; dup {
			return nil, fmt.Errorf("service %s: image label %q already used by %s", n, label, prev)
		}
		byLabel[label] = "repository " + repo
		p.External = append(p.External, External{Label: label, Ref: s.Image, Repo: repo, Digest: digest})
	}
	if len(p.External)+len(p.Builds) == 0 {
		return nil, fmt.Errorf("compose file yields no images")
	}
	return p, nil
}

// Write prints the plan as tab-separated lines for scripts/image-scan.sh:
//
//	external <label> <repo@digest> <digest>
//	build    <label> <target or -> <service>
func (p *Plan) Write(w io.Writer) {
	for _, e := range p.External {
		fmt.Fprintf(w, "external\t%s\t%s\t%s\n", e.Label, e.Ref, e.Digest)
	}
	for _, b := range p.Builds {
		t := b.Target
		if t == "" {
			t = "-"
		}
		fmt.Fprintf(w, "build\t%s\t%s\t%s\n", b.Label, t, b.Service)
	}
}
