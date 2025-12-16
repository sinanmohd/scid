package driver

import (
	"log/slog"
	"os/exec"

	"sinanmohd.com/scid/internal/config"
	"sinanmohd.com/scid/internal/git"
)

func ExecIfChaged(title string, paths, execLine []string, g *git.Git) (string, string, error /* exec error */, error) {
	var changed string
	if g.OldHash == nil {
		changed = "/"
	} else {
		changed = g.ArePathsChanged(paths)
	}
	if changed == "" {
		slog.Info("watch paths did not change, skipping", "title", title, "execLine", execLine)
		return "", "", nil, nil
	} else {
		slog.Info("watch path changed, starting", "title", title, "execLine", execLine, "changed", changed)
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
