package driver

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"sinanmohd.com/scid/internal/config"
	"sinanmohd.com/scid/internal/git"
	"sinanmohd.com/scid/internal/slack"
)

func notify(g *git.Git, color, title string, success bool, description string) error {
	status := "success"
	if !success {
		status = "failure"
	}
	slog.Info("job completed", "title", title, "status", status, "description", description)

	if config.Config.Slack != nil {
		return slack.SendMesg(g, color, title, success, description)
	} else {
		return nil
	}
}

func expandPath(path string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", nil
	}

	if path == "~" {
		path = homeDir
	} else if strings.HasPrefix(path, "~/") {
		path = filepath.Join(homeDir, path[2:])
	}

	return os.ExpandEnv(path), nil
}
