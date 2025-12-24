package driver

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"sinanmohd.com/scid/internal/config"
	"sinanmohd.com/scid/internal/git"

	"github.com/BurntSushi/toml"
	"github.com/getsops/sops/v3/decrypt"
	"github.com/go-playground/validator/v10"
	"github.com/hmdsefi/gograph"
)

const (
	scidHelmConfigName = "scid"
	helmColorHex       = "#10148c"
)

type SCIDConf struct {
	ReleaseName        string   `toml:"release_name" validate:"required"`
	NameSpace          string   `toml:"namespace" validate:"required"`
	ChartPathOverride  string   `toml:"chart_path_override"`
	ValuePaths         []string `toml:"value_paths"`
	OptionalValuePaths []string `toml:"optional_value_paths"`
	SopsValuePaths     []string `toml:"sops_value_paths"`
	Dependencies       []string `toml:"dependencies"`

	chartPath string
}

func HelmChartUpstallIfChaged(scidToml *SCIDConf, bg *git.Git) error {
	execLine := []string{
		"helm",
		"upgrade",
		"--install",
		"--wait",
		"--namespace", scidToml.NameSpace,
		"--create-namespace",
	}

	for _, path := range scidToml.ValuePaths {
		fullPath := filepath.Join(scidToml.chartPath, path)
		execLine = append(execLine, "--values", fullPath)
	}

	for _, path := range scidToml.OptionalValuePaths {
		path, err := expandPath(path)
		if err != nil {
			return err
		}
		_, err = os.Stat(path)
		if err != nil {
			continue
		}

		execLine = append(execLine, "--values", path)
	}

	for _, encPath := range scidToml.SopsValuePaths {
		fullEncPath := filepath.Join(scidToml.chartPath, encPath)
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
		finalChartPath = scidToml.chartPath
	} else {
		finalChartPath = filepath.Join(scidToml.chartPath, scidToml.ChartPathOverride)
	}
	execLine = append(execLine, scidToml.ReleaseName, finalChartPath)
	changeWatchPaths := []string{
		scidToml.chartPath,
	}

	output, changedPath, execErr, err := ExecIfChaged(filepath.Base(scidToml.chartPath), changeWatchPaths, execLine, bg)
	if err != nil {
		return err
	} else if changedPath == "" {
		return nil
	}

	title := fmt.Sprintf("Helm Chart %s", filepath.Base(scidToml.chartPath))
	if execErr != nil {
		description := fmt.Sprintf("watch path %s changed\n%s: %s", changedPath, execErr.Error(), output)
		err = notify(bg, helmColorHex, title, false, description)
	} else {
		description := fmt.Sprintf("watch path %s changed\n%s", changedPath, output)
		err = notify(bg, helmColorHex, title, true, description)
	}

	return nil
}

func HelmChartUpstallGraph(dependencyGraph gograph.Graph[*SCIDConf], bg *git.Git) {
	var graphMutex sync.Mutex
	var helmWg sync.WaitGroup
	scheduled := make(map[*SCIDConf]bool)
	jobComplete := make(chan bool, 1)
	jobComplete <- true

	for {
		graphMutex.Lock()
		scidTomls := dependencyGraph.GetAllVertices()
		graphMutex.Unlock()
		if len(scidTomls) == 0 {
			break
		}

		// wait for atleast one job to complete before trying
		// to find vertices(scidConf job) where outDegree == 0
		<-jobComplete

		for _, scidTomlVertex := range scidTomls {
			if scidTomlVertex.OutDegree() != 0 {
				continue
			}

			scidToml := scidTomlVertex.Label()
			_, found := scheduled[scidToml]
			if found {
				continue
			} else {
				scheduled[scidToml] = true
			}

			helmWg.Add(1)
			go func() {
				err := HelmChartUpstallIfChaged(scidToml, bg)
				if err != nil {
					slog.Error("upstalling Helm chart", "chartPath", scidToml.chartPath)
				}

				graphMutex.Lock()
				dependencyGraph.RemoveVertices(scidTomlVertex)
				graphMutex.Unlock()

				// only keep one value in buffer
				select {
				case <-jobComplete:
					jobComplete <- true
				default:
					jobComplete <- true
				}

				helmWg.Done()
			}()
		}

	}

	helmWg.Wait()
}

func scidConfGet(helm *config.Helm) (map[string]*SCIDConf, error) {
	entries, err := os.ReadDir(helm.ChartsPath)
	if err != nil {
		return nil, err
	}

	var configName string
	if helm.Env == "" {
		configName = fmt.Sprintf("%s.toml", scidHelmConfigName)
	} else {
		configName = fmt.Sprintf("%s.%s.toml", scidHelmConfigName, helm.Env)
	}
	scidTomls := make(map[string]*SCIDConf)
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
			return nil, err
		}

		scidToml := new(SCIDConf)
		_, err = toml.DecodeFile(scidTomlPath, scidToml)
		if err != nil {
			return nil, err
		}
		err = validator.New().Struct(scidToml)
		if err != nil {
			return nil, err
		}
		scidToml.chartPath = chartPath
		scidTomls[entry.Name()] = scidToml
	}

	return scidTomls, nil
}

func helmDependencyGraph(scidTomls map[string]*SCIDConf) (gograph.Graph[*SCIDConf], error) {
	dependencyGraph := gograph.New[*SCIDConf](gograph.Acyclic())
	for _, scidToml := range scidTomls {
		dependencyGraph.AddVertex(gograph.NewVertex(scidToml))
		for _, dependencyName := range scidToml.Dependencies {
			dependency, ok := scidTomls[dependencyName]
			if !ok {
				return nil, fmt.Errorf("did not find dependency %s", dependencyName)
			}

			_, err := dependencyGraph.AddEdge(
				gograph.NewVertex(scidToml),
				gograph.NewVertex(dependency),
			)
			if err != nil {
				return nil, err
			}
		}
	}

	return dependencyGraph, nil
}

func HelmChartsHandle(helm *config.Helm, bg *git.Git) error {
	scidTomls, err := scidConfGet(helm)
	if err != nil {
		return err
	}
	dependencyGraph, err := helmDependencyGraph(scidTomls)
	if err != nil {
		return err
	}
	HelmChartUpstallGraph(dependencyGraph, bg)

	return nil
}
