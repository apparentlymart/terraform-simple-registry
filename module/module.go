package module

import (
	"sort"
	"strings"

	version "github.com/hashicorp/go-version"
	git "gopkg.in/libgit2/git2go.v24"
)

type Module struct {
	repo *git.Repository
}

// Load creates a new Module object that reads its data from the given
// git repository directory.
//
// This function returns nil if the given directory cannot be opened as
// a git repository for any reason.
func Load(gitDir string) *Module {
	repo, err := git.OpenRepository(gitDir)
	if err != nil {
		return nil
	}

	return &Module{
		repo: repo,
	}
}

// AllVersions returns all of the available versions for the receiving module,
// in reverse order such that the latest version is at index 0.
//
// The result may be an empty (or nil) slice if the underlying repository
// has no version-shaped tags.
func (m Module) AllVersions() ([]*version.Version, error) {
	it, err := m.repo.NewReferenceNameIterator()
	if err != nil {
		return nil, err
	}

	var ret []*version.Version
	for {
		name, err := it.Next()

		if err, ok := err.(*git.GitError); ok && err.Code == git.ErrIterOver {
			break
		}
		if err != nil {
			return nil, err
		}

		if strings.HasPrefix(name, "refs/tags/v") {
			versionStr := name[11:]
			v, err := version.NewVersion(versionStr)
			if err != nil {
				continue
			}
			ret = append(ret, v)
		}
	}

	sort.Slice(ret, func(i, j int) bool {
		// j and i are inverted here because we want reverse order
		return ret[j].LessThan(ret[i])
	})

	return ret, nil
}

// LatestVersion returns the latest version available for the receiving module,
// or nil if it has no versions.
func (m Module) LatestVersion() (*version.Version, error) {
	versions, err := m.AllVersions()
	if err != nil {
		return nil, err
	}

	if len(versions) == 0 {
		return nil, nil
	}

	return versions[0], nil
}

// HasVersion returns true if the receiving module has a tag for the given
// version number.
func (m Module) HasVersion(v *version.Version) (bool, error) {
	it, err := m.repo.NewReferenceNameIterator()
	if err != nil {
		return false, err
	}

	for {
		name, err := it.Next()

		if err, ok := err.(*git.GitError); ok && err.Code == git.ErrIterOver {
			break
		}
		if err != nil {
			return false, err
		}

		if strings.HasPrefix(name, "refs/tags/v") {
			versionStr := name[11:]
			gotV, err := version.NewVersion(versionStr)
			if err != nil {
				continue
			}

			if gotV.Equal(v) {
				return true, nil
			}
		}
	}

	return false, nil
}
