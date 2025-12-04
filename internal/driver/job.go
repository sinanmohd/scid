package driver

import (
	"fmt"
	"sync"

	"sinanmohd.com/scid/internal/config"
	"sinanmohd.com/scid/internal/git"
	"sinanmohd.com/scid/internal/slack"

	"github.com/rs/zerolog/log"
)

func JobRunIfChaged(name string, job config.JobConfig, g *git.Git) error {
	output, changedPath, execErr, err := ExecIfChaged(job.WatchPaths, job.ExecLine, g)
	if err != nil {
		return err
	} else if changedPath == "" {
		return nil
	}

	var color string
	if job.SlackColor == "" {
		color = "#000000"
	} else {
		color = job.SlackColor
	}

	if execErr != nil {
		extraText := fmt.Sprintf("watch path %s changed\n%s: %s", changedPath, execErr.Error(), output)
		err = slack.SendMesg(g, color, name, false, extraText)
	} else {
		extraText := fmt.Sprintf("watch path %s changed\n%s", changedPath, output)
		err = slack.SendMesg(g, color, name, true, extraText)
	}
	if err != nil {
		return err
	}

	return nil
}

func JobRunIfChagedWrapped(name string, job config.JobConfig, bg *git.Git, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		err := JobRunIfChaged(name, job, bg)
		if err != nil {
			log.Error().Err(err).Msgf("Running Job %s", name)
		}

		wg.Done()
	}()
}

func JobsRunIfChaged(g *git.Git) error {
	var jobWg sync.WaitGroup
	for name, job := range config.Config.Jobs {
		JobRunIfChagedWrapped(name, job, g, &jobWg)
	}
	jobWg.Wait()

	return nil
}
