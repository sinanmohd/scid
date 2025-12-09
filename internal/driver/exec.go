package driver

import (
	"os/exec"

	"github.com/rs/zerolog/log"

	"sinanmohd.com/scid/internal/config"
	"sinanmohd.com/scid/internal/git"
)

func ExecIfChaged(paths, execLine []string, g *git.Git) (string, string, error /* exec error */, error) {
	var changed string
	if g.OldHash == nil {
		changed = "/"
	} else {
		changed = g.ArePathsChanged(paths)
	}
	if changed == "" {
		log.Info().Msgf("Skipping %v, because 0 watch paths changed", execLine)
		return "", "", nil, nil
	} else {
		log.Info().Msgf("Execing %v, because watch path %s changed", execLine, changed)
	}

	if config.Config.DryRun {
		return "", changed, nil, nil
	}

	output, err := exec.Command(execLine[0], execLine[1:]...).CombinedOutput()
	if err != nil {
		return string(output), changed, err, nil
	}
	return string(output), changed, nil, nil
}
