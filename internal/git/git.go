package git

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"golang.org/x/mod/semver"
	"lukechampine.com/blake3"
	"sinanmohd.com/scid/internal/config"
)

type Git struct {
	LocalPath        string
	repo             *git.Repository
	NewHash, OldHash *plumbing.Hash
	changedPaths     []string
}

func hashFromTag(tag *config.Tag, repo *git.Repository) (*plumbing.Hash, error) {
	switch tag.Model {
	case config.TagModelStatic:
		hash, err := repo.ResolveRevision(plumbing.Revision(tag.Value))
		return hash, err
	case config.TagModelSemver:
		tagRefs, err := repo.Tags()
		if err != nil {
			return nil, err
		}

		var versions []string
		tagRefs.ForEach(func(tafRef *plumbing.Reference) error {
			// golnag semver is not spec compliant
			version := "v" + tafRef.Name().Short()
			if semver.IsValid(version) {
				versions = append(versions, version)
			}
			return nil
		})
		if len(versions) <= 0 {
			return nil, errors.New("No semver tag")
		}

		// golnag semver is not spec compliant
		latestVersion := slices.MaxFunc(versions, semver.Compare)[1:]
		hash, err := repo.ResolveRevision(plumbing.Revision(latestVersion))
		return hash, err
	default:
		return nil, fmt.Errorf("unsupported tag model: %s", tag.Model)

	}
}

func checkoutTag(tag *config.Tag, repo *git.Repository) error {
	hash, err := hashFromTag(tag, repo)
	if err != nil {
		return err
	}

	workTree, err := repo.Worktree()
	if err != nil {
		return err
	}

	workTree.Checkout(&git.CheckoutOptions{
		Hash: *hash,
	})

	return nil
}

func cloneRepo(localPath, repoUrl, branchName string, tag *config.Tag) (*Git, error) {
	repo, err := git.PlainClone(localPath, &git.CloneOptions{
		URL:           repoUrl,
		SingleBranch:  true,
		ReferenceName: plumbing.NewBranchReferenceName(branchName),
		Progress:      os.Stdout,
	})
	if err != nil {
		return nil, err
	}

	if tag.Model != config.TagModelDisabled {
		err = checkoutTag(tag, repo)
		if err != nil {
			return nil, err
		}
	}

	headRef, err := repo.Head()
	if err != nil {
		return nil, err
	}
	newHash := headRef.Hash()

	return &Git{
		LocalPath: localPath,
		repo:      repo,
		NewHash:   &newHash,
		OldHash:   nil,
	}, nil
}

func pullBranch(workTree *git.Worktree, branchName string) error {
	err := workTree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(branchName),
	})
	if err != nil {
		return err
	}

	err = workTree.Pull(&git.PullOptions{
		SingleBranch: true,
	})

	if err != nil && err != git.NoErrAlreadyUpToDate {
		return err
	}

	return nil
}

func updateRepo(localPath, branchName string, tag *config.Tag) (*Git, error) {
	// get oldHash
	repo, err := git.PlainOpen(localPath)
	if err != nil {
		return nil, err
	}
	headRef, err := repo.Head()
	if err != nil {
		return nil, err
	}
	oldHash := headRef.Hash()

	// get newHash
	workTree, err := repo.Worktree()
	if err != nil {
		return nil, err
	}
	err = pullBranch(workTree, branchName)
	if err != nil {
		return nil, err
	}
	if tag.Model != config.TagModelDisabled {
		err = checkoutTag(tag, repo)
		if err != nil {
			return nil, err
		}
	}
	headRef, err = repo.Head()
	if err != nil {
		return nil, err
	}
	newHash := headRef.Hash()

	// get changed paths
	g := Git{
		LocalPath: localPath,
		repo:      repo,
		NewHash:   &newHash,
		OldHash:   &oldHash,
	}
	err = g.changedPathsSet()
	if err != nil {
		return nil, err
	}

	return &g, nil
}

func New(repoUrl, branchName string, tag *config.Tag) (*Git, error) {
	sum256 := blake3.Sum256([]byte(repoUrl + branchName))
	localPath := fmt.Sprintf("%x", sum256)

	_, err := os.Stat(localPath)
	if os.IsNotExist(err) {
		return cloneRepo(localPath, repoUrl, branchName, tag)
	} else if err != nil {
		return nil, err
	}

	return updateRepo(localPath, branchName, tag)
}

// go-git has concurrency issues: https://github.com/go-git/go-git/issues/773
// doing this concurrently with coroutines can cause "zlib: invalid header" error
// so it would require a mutex and bottleneck concurrency
// also in-memory should be faster than reading it from disk every time
func (g *Git) changedPathsSet() error {
	coOld, err := g.repo.CommitObject(*g.OldHash)
	if err != nil {
		return err
	}
	treeOld, err := coOld.Tree()
	if err != nil {
		return err
	}

	coNew, err := g.repo.CommitObject(*g.NewHash)
	if err != nil {
		return err
	}
	treeNew, err := coNew.Tree()
	if err != nil {
		return err
	}

	changes, err := treeOld.Diff(treeNew)
	if err != nil {
		return err
	}

	for _, change := range changes {
		if change.From.Name != "" {
			g.changedPaths = append(g.changedPaths, change.From.Name)
		}
		if change.To.Name != "" {
			g.changedPaths = append(g.changedPaths, change.To.Name)
		}
	}

	return err
}

func (g *Git) HeadMoved() bool {
	if config.Config.ForceReRun {
		return true
	}

	if config.Config.DryRun {
		return true
	} else if g.OldHash == nil {
		return true
	}

	return *g.NewHash != *g.OldHash
}

func (g *Git) ArePathsChanged(prefixPaths []string) string {
	if config.Config.ForceReRun {
		return "/force-re-run"
	}

	for _, changedPath := range g.changedPaths {
		for _, prefixPath := range prefixPaths {
			if strings.HasPrefix(changedPath, prefixPath) {
				return changedPath
			}
		}
	}

	return ""
}
