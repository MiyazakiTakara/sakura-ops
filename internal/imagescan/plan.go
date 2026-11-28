package imagescan

import (
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v4"
)

// The only builds the scan reproduces (scripts/image-scan.sh): Compose context
// is always the repository root, the Dockerfile one of the allowlisted files,
// and the only accepted build args are the ones that change file ownership,
// never package content. Any other build section is rejected rather than
// silently scanning a different image.
const buildContext = "../.."

var allowedDockerfiles = map[string]bool{
	"deployment/docker/hub.Dockerfile":      true,
	"deployment/docker/postgres.Dockerfile": true,
}

var allowedBuildArgs = map[string]bool{"POSTGRES_UID": true, "POSTGRES_GID": true}

var pinnedRef = regexp.MustCompile(`^([a-z0-9][a-z0-9._/-]*)@(sha256:[0-9a-f]{64})$`)

// External is a registry image that Compose pulls; it must be digest-pinned.
type External struct {
	Label  string // last path element of the repository, used as the image label
	Ref    string // repo@sha256:digest
	Repo   string
	Digest string // sha256:...
}

// Build is an image built locally from an allowlisted Dockerfile. Services
// that build the same (Dockerfile, target) collapse into one entry.
type Build struct {
	Label      string // build target, or the Dockerfile stem (postgres.Dockerfile -> postgres) when no target is set
	Service    string // first service (alphabetical) using this build
	Dockerfile string // repository-relative path, allowlisted
	Target     string // empty means the Dockerfile's last stage
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
	byBuild := map[string]string{} // dockerfile|target -> first service
	for _, n := range names {
		s := c.Services[n]
		if s.Build.Kind != 0 {
			target := ""
			if s.Build.Kind != yaml.MappingNode {
				return nil, fmt.Errorf("service %s: build must be a mapping with context %s and an allowlisted dockerfile (the scan builds only that)", n, buildContext)
			}
			var bs struct {
				Context    string         `yaml:"context"`
				Dockerfile string         `yaml:"dockerfile"`
				Target     string         `yaml:"target"`
				Args       map[string]any `yaml:"args"`
			}
			if err := s.Build.Decode(&bs); err != nil {
				return nil, fmt.Errorf("service %s: build: %w", n, err)
			}
			if bs.Context == "" || path.Clean(bs.Context) != buildContext {
				return nil, fmt.Errorf("service %s: build context %q is not %s (repository root relative to the compose file); the scan would not build what Compose builds", n, bs.Context, buildContext)
			}
			df := path.Clean(bs.Dockerfile)
			if bs.Dockerfile == "" || !allowedDockerfiles[df] {
				return nil, fmt.Errorf("service %s: build dockerfile %q is not one of the allowlisted Dockerfiles (hub.Dockerfile, postgres.Dockerfile under deployment/docker); the scan would not build what Compose builds", n, bs.Dockerfile)
			}
			for k := range bs.Args {
				if !allowedBuildArgs[k] {
					return nil, fmt.Errorf("service %s: build arg %q is not allowed (the scan builds with Dockerfile defaults)", n, k)
				}
			}
			target = strings.TrimSpace(bs.Target)
			key := df + "|" + target
			if _, dup := byBuild[key]; dup {
				continue // identical build: already in the plan
			}
			label := target
			if label == "" {
				base := path.Base(df)
				label = strings.TrimSuffix(base, ".Dockerfile")
			}
			byBuild[key] = n
			if prev, dup := byLabel[label]; dup {
				return nil, fmt.Errorf("service %s: image label %q already used by %s", n, label, prev)
			}
			byLabel[label] = "service " + n
			p.Builds = append(p.Builds, Build{Label: label, Service: n, Dockerfile: df, Target: target})
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
//	build    <label> <dockerfile> <target or -> <service>
func (p *Plan) Write(w io.Writer) {
	for _, e := range p.External {
		fmt.Fprintf(w, "external\t%s\t%s\t%s\n", e.Label, e.Ref, e.Digest)
	}
	for _, b := range p.Builds {
		t := b.Target
		if t == "" {
			t = "-"
		}
		fmt.Fprintf(w, "build\t%s\t%s\t%s\t%s\n", b.Label, b.Dockerfile, t, b.Service)
	}
}

var (
	fromLine   = regexp.MustCompile(`(?im)^\s*FROM\s+(?:--\S+\s+)*(\S+)(?:\s+AS\s+(\S+))?\s*$`)
	pinnedFrom = regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]*(:[A-Za-z0-9._-]+)?@sha256:[0-9a-f]{64}$`)
)

// CheckDockerfilePins verifies that every FROM of a Dockerfile is digest-pinned
// (name@sha256:<64 hex>, a tag may accompany the digest) or refers to an
// earlier stage of the same file. Line continuations are not supported; a FROM
// that does not match the simple form is an error rather than skipped.
func CheckDockerfilePins(name string, b []byte) error {
	stages := map[string]bool{}
	n := 0
	for i, line := range strings.Split(string(b), "\n") {
		t := strings.TrimSpace(line)
		if len(t) < 4 || !strings.EqualFold(t[:4], "FROM") || (len(t) > 4 && t[4] != ' ' && t[4] != '\t') {
			continue
		}
		m := fromLine.FindStringSubmatch(t)
		if m == nil {
			return fmt.Errorf("%s:%d: unsupported FROM syntax %q", name, i+1, t)
		}
		n++
		ref := m[1]
		if !stages[strings.ToLower(ref)] && !pinnedFrom.MatchString(ref) {
			return fmt.Errorf("%s:%d: base image %q is not digest-pinned (want name@sha256:<64 hex>)", name, i+1, ref)
		}
		if m[2] != "" {
			stages[strings.ToLower(m[2])] = true
		}
	}
	if n == 0 {
		return fmt.Errorf("%s: no FROM found", name)
	}
	return nil
}
