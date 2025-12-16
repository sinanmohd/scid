package main

import (
	"log"
	"log/slog"
	"os"
	"sync"

	"github.com/lmittmann/tint"
	"sinanmohd.com/scid/internal/config"
	"sinanmohd.com/scid/internal/driver"
	"sinanmohd.com/scid/internal/git"
)

func scid(g *git.Git) {
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		if config.Config.Helm != nil {
			err := driver.HelmChartsHandle(config.Config.Helm, g)
			if err != nil {
				slog.Error("running helm driver", "error", err)
			}
		}
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		driver.JobsRunIfChaged(g)
		wg.Done()
	}()

	wg.Wait()
}

func main() {
	logger := slog.New(tint.NewHandler(os.Stderr, nil))
	slog.SetDefault(logger)

	err := config.Init()
	if err != nil {
		log.Fatal("creating config: ", err)
	}

	g, err := git.New(config.Config.RepoUrl, config.Config.Branch, &config.Config.Tag)
	if err != nil {
		log.Fatal("pulling git repo: ", err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	err = os.Chdir(g.LocalPath)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		err = os.Chdir(originalDir)
		if err != nil {
			log.Fatal(err)
		}
	}()

	if !g.HeadMoved() {
		slog.Info("no new commits : (")
		return
	}

	slog.Info("branch HEAD moved", "oldHash", g.OldHash, "newHash", g.NewHash)
	scid(g)
}
