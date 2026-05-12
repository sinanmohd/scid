package main

import (
	"log"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/lmittmann/tint"
	"sinanmohd.com/scid/internal/config"
	"sinanmohd.com/scid/internal/driver"
	"sinanmohd.com/scid/internal/git"
	"sinanmohd.com/scid/internal/health"
)

func driverRun(g *git.Git) {
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

func scid(config config.SCIDonfig) (err error) {
	slog.Debug("pulling new changes :)")
	g, err := git.New(config.RepoUrl, config.Branch, &config.Tag, config.SSH)
	if err != nil {
		return err
	}
	if !g.HeadMoved() {
		slog.Debug("no new commits ;(")
		return
	}

	originalDir, err := os.Getwd()
	if err != nil {
		return err
	}
	err = os.Chdir(g.LocalPath)
	if err != nil {
		return err
	}
	defer func() {
		err = os.Chdir(originalDir)
		if err != nil {
			log.Fatal(err)
		}
	}()

	slog.Info("branch HEAD moved", "oldHash", g.OldHash, "newHash", g.NewHash)
	driverRun(g)
	return nil
}

func main() {
	logger := slog.New(tint.NewHandler(os.Stderr, nil))
	slog.SetDefault(logger)

	err := config.Init()
	if err != nil {
		log.Fatal("creating config: ", err)
	}

	interval, err := time.ParseDuration(config.Config.PullInterval)
	if err != nil {
		log.Fatal("parsing pull interval: ", err)
	}

	health.Init()
	for {
		start := time.Now()

		err = scid(config.Config)
		if err != nil {
			slog.Error("running scid", "err", err)
		}

		elapsed := time.Since(start)
		if elapsed < interval {
			slog.Debug("sleeping", "duration", interval-elapsed)
			time.Sleep(interval - elapsed)
		}
	}
}
