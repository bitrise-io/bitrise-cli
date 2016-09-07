package toolkits

import (
	"path/filepath"

	"github.com/bitrise-io/bitrise/configs"
	stepmanModels "github.com/bitrise-io/stepman/models"
)

// ToolkitCheckResult ...
type ToolkitCheckResult struct {
	Path    string
	Version string
}

// Toolkit ...
type Toolkit interface {
	// Check the toolkit - first returned value (bool) indicates
	// whether the toolkit is "operational", or have to be installed.
	// "Have to be installed" can be true if the toolkit is not installed,
	// or if an older version is installed, and an update/newer version is required.
	Check() (bool, ToolkitCheckResult, error)
	// Install the toolkit
	Install() error
	// Bootstrap : initialize the toolkit for use,
	// e.g. setting Env Vars
	Bootstrap() error
	// ToolkitName : a one liner name/id of the toolkit, for logging purposes
	ToolkitName() string
	// StepRunCommandArguments ...
	StepRunCommandArguments(stepDirPath string) ([]string, error)
}

//
// === Utils ===

// ToolkitForStep ...
func ToolkitForStep(step stepmanModels.StepModel) Toolkit {
	if step.Toolkit != nil {
		stepToolkit := step.Toolkit
		if stepToolkit.Go != nil {
			return GoToolkit{}
		} else if stepToolkit.Bash != nil {
			return BashToolkit{}
		}
	}

	// default
	return BashToolkit{}
}

// AllSupportedToolkits ...
func AllSupportedToolkits() []Toolkit {
	return []Toolkit{GoToolkit{}, BashToolkit{}}
}

func getBitriseToolkitsTmpDirPath() string {
	bitriseToolkitsDirPath := configs.GetBitriseToolkitsDirPath()
	return filepath.Join(bitriseToolkitsDirPath, "tmp")
}
