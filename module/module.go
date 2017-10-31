package module

import (
	"archive/tar"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

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

func (m Module) GetVersionTreeId(v *version.Version) (string, error) {
	commit, err := m.getVersionCommit(v)
	if err != nil {
		return "", err
	}

	return commit.TreeId().String(), nil
}

func (m Module) getVersionCommit(v *version.Version) (*git.Commit, error) {
	refName := fmt.Sprintf("refs/tags/v%s", v)
	ref, err := m.repo.References.Lookup(refName)
	if err != nil {
		return nil, err
	}

	commitObj, err := ref.Peel(git.ObjectCommit)
	if err != nil {
		return nil, err
	}
	return commitObj.AsCommit()
}

// WriteVersionTar recursively writes the contents of the git tree associated
// with the given version to the given writer. If no such version exists,
// or if there are any other problems when reading the tree, the resulting
// tar archive may be incomplete.
func (m Module) WriteVersionTar(v *version.Version, w io.Writer) error {
	tw := tar.NewWriter(w)
	defer tw.Close()

	commit, err := m.getVersionCommit(v)
	if err != nil {
		return err
	}

	committer := commit.Committer()
	commitTime := committer.When
	rootTree, err := commit.Tree()
	if err != nil {
		return err
	}

	return m.writeGitTreeTar(rootTree, "", commitTime, tw)
}

func (m Module) writeGitTreeTar(tree *git.Tree, prefix string, modTime time.Time, tw *tar.Writer) error {
	ct := tree.EntryCount()

	for i := uint64(0); i < ct; i++ {
		entry := tree.EntryByIndex(i)
		switch entry.Type {
		case git.ObjectTree:
			newPrefix := prefix + entry.Name + "/"
			tw.WriteHeader(&tar.Header{
				Name:       newPrefix,
				Mode:       0755,
				Typeflag:   tar.TypeDir,
				ChangeTime: modTime,
				AccessTime: modTime,
				ModTime:    modTime,
			})
			newTree, err := m.repo.LookupTree(entry.Id)
			if err != nil {
				continue
			}
			err = m.writeGitTreeTar(newTree, newPrefix, modTime, tw)
			if err != nil {
				return err
			}
		case git.ObjectBlob:
			blob, err := m.repo.LookupBlob(entry.Id)
			if err != nil {
				return err
			}

			tw.WriteHeader(&tar.Header{
				Name:       prefix + entry.Name,
				Mode:       int64(entry.Filemode),
				Typeflag:   tar.TypeReg,
				Size:       blob.Size(),
				ChangeTime: modTime,
				AccessTime: modTime,
				ModTime:    modTime,
			})
			_, err = tw.Write(blob.Contents())
			if err != nil {
				return err
			}
		}
	}

	return nil
}
