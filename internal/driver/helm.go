package driver

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"sinanmohd.com/scid/internal/config"
	"sinanmohd.com/scid/internal/git"

	"github.com/BurntSushi/toml"
	"github.com/getsops/sops/v3/decrypt"
	"github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"
)

const (
	SCID_HELM_CONFIG_NAME = "scid"
	HELM_COLOR_HEX        = "#10148c"
)

type SCIDToml struct {
	ReleaseName       string   `toml:"release_name" validate:"required"`
	NameSpace         string   `toml:"namespace" validate:"required"`
	ChartPathOverride string   `toml:"chart_path_override"`
	ValuePaths        []string `toml:"value_paths"`
	SopsValuePaths    []string `toml:"sops_value_paths"`
}

func HelmChartUpstallIfChaged(chartPath, scidTomlPath string, bg *git.Git) error {
	var scidToml SCIDToml
	// TODO: potential path traversal vulnerability i dont want to
	// waste time on it. just mention it, if requirements change in the future
	_, err := toml.DecodeFile(scidTomlPath, &scidToml)
	if err != nil {
		return err
	}
	err = validator.New().Struct(scidToml)
	if err != nil {
		return err
	}

	execLine := []string{
		"helm",
		"upgrade",
		"--install",
		"--namespace", scidToml.NameSpace,
		"--create-namespace",
	}

	for _, path := range scidToml.ValuePaths {
		fullPath := filepath.Join(chartPath, path)
		execLine = append(execLine, "--values", fullPath)
	}

	for _, encPath := range scidToml.SopsValuePaths {
		fullEncPath := filepath.Join(chartPath, encPath)
		plainContent, err := decrypt.File(fullEncPath, "yaml")
		if err != nil {
			return err
		}

		plainFile, err := os.CreateTemp("", "scid-helm-sops-enc-*.yaml")
		if err != nil {
			return err
		}
		defer os.Remove(plainFile.Name())

		_, err = plainFile.WriteAt(plainContent, 0)
		if err != nil {
			return err
		}
		err = plainFile.Close()
		if err != nil {
			return err
		}

		execLine = append(execLine, "--values", plainFile.Name())
	}

	var finalChartPath string
	if scidToml.ChartPathOverride == "" {
		finalChartPath = chartPath
	} else {
		finalChartPath = filepath.Join(chartPath, scidToml.ChartPathOverride)
	}
	execLine = append(execLine, scidToml.ReleaseName, finalChartPath)
	changeWatchPaths := []string{
		chartPath,
	}

	output, changedPath, execErr, err := ExecIfChaged(changeWatchPaths, execLine, bg)
	if err != nil {
		return err
	} else if changedPath == "" {
		return nil
	}

	title := fmt.Sprintf("Helm Chart %s", filepath.Base(chartPath))
	if execErr != nil {
		description := fmt.Sprintf("watch path %s changed\n%s: %s", changedPath, execErr.Error(), output)
		err = notify(bg, HELM_COLOR_HEX, title, false, description)
	} else {
		description := fmt.Sprintf("watch path %s changed\n%s", changedPath, output)
		err = notify(bg, HELM_COLOR_HEX, title, true, description)
	}

	return nil
}

func HelmChartUpstallIfChagedWrapped(chartPath, scidTomlPath string, bg *git.Git, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		err := HelmChartUpstallIfChaged(chartPath, scidTomlPath, bg)
		if err != nil {
			log.Error().Err(err).Msgf("Upstalling Helm Chart %s", filepath.Base(chartPath))
		}

		wg.Done()
	}()
}

func HelmChartsUpstallIfChaged(helm *config.Helm, bg *git.Git) error {
	entries, err := os.ReadDir(helm.ChartsPath)
	if err != nil {
		return err
	}
	var configName string
	if helm.Env == "" {
		configName = fmt.Sprintf("%s.toml", SCID_HELM_CONFIG_NAME)
	} else {
		configName = fmt.Sprintf("%s.%s.toml", SCID_HELM_CONFIG_NAME, helm.Env)
	}

	var helmWg sync.WaitGroup
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		chartPath := filepath.Join(helm.ChartsPath, entry.Name())
		scidTomlPath := filepath.Join(chartPath, configName)
		_, err := os.Stat(scidTomlPath)
		if os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}

		HelmChartUpstallIfChagedWrapped(chartPath, scidTomlPath, bg, &helmWg)
	}
	helmWg.Wait()

	return nil
}
