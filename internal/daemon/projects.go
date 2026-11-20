package daemon

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
)

// The project registry lists every project NeoSwitch turned on, so `neoswitch off all` can restore them before the plugin is removed. It only lists paths; whether a project is still on is read from its own settings.

func projectsPath(dir string) string { return filepath.Join(dir, "projects.json") }

type projectRegistry struct {
	Projects []string `json:"projects"`
}

// Projects returns the registered projects in the order they were first turned on.
func Projects(dir string) ([]string, error) {
	data, err := os.ReadFile(projectsPath(dir))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var registry projectRegistry
	// A damaged registry only loses the list of projects to restore; it must not stop on or off from working.
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil, nil
	}
	return registry.Projects, nil
}

// RecordProject adds project to the registry.
func RecordProject(dir, project string) error {
	projects, err := Projects(dir)
	if err != nil {
		return err
	}
	if slices.Contains(projects, project) {
		return nil
	}
	return saveProjects(dir, append(projects, project))
}

// ForgetProjects removes the given projects from the registry.
func ForgetProjects(dir string, remove ...string) error {
	projects, err := Projects(dir)
	if err != nil {
		return err
	}
	kept := slices.DeleteFunc(slices.Clone(projects), func(p string) bool { return slices.Contains(remove, p) })
	if len(kept) == len(projects) {
		return nil
	}
	return saveProjects(dir, kept)
}

func saveProjects(dir string, projects []string) error {
	data, _ := json.MarshalIndent(projectRegistry{Projects: projects}, "", "  ")
	return writePrivate(projectsPath(dir), append(data, '\n'))
}
