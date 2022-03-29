package analytics

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/bitrise-io/bitrise/configs"
	"github.com/bitrise-io/bitrise/models"
	"github.com/bitrise-io/bitrise/version"
	"github.com/bitrise-io/go-utils/pointers"
	"github.com/bitrise-io/go-utils/v2/analytics"
	"github.com/bitrise-io/go-utils/v2/env"
	stepmanModels "github.com/bitrise-io/stepman/models"
)

const (
	// BuildExecutionID ...
	BuildExecutionID = "build_execution_id"
	// WorkflowExecutionID ...
	WorkflowExecutionID = "workflow_execution_id"
	// StepExecutionID ...
	StepExecutionID = "step_execution_id"

	workflowStartedEventName       = "workflow_started"
	workflowFinishedEventName      = "workflow_finished"
	stepStartedEventName           = "step_started"
	stepFinishedEventName          = "step_finished"
	stepPreparationFailedEventName = "step_preparation_failed"
	stepSkippedEventName           = "step_skipped"
	cliWarningEventName            = "cli_warning"

	workflowNameProperty          = "workflow_name"
	ciModeProperty                = "ci_mode"
	prModeProperty                = "pr_mode"
	debugModeProperty             = "debug_mode"
	secretFilteringProperty       = "secret_filtering"
	secretEnvsFilteringProperty   = "secret_envs_filtering"
	buildSlugProperty             = "build_slug"
	parentStepExecutionIDProperty = "parent_step_execution_id"
	cliVersionProperty            = "cli_version"
	envmanVersionProperty         = "envman_version"
	stepmanVersionProperty        = "stepman_version"
	statusProperty                = "status"
	inputsProperty                = "inputs"
	errorMessageProperty          = "error_message"
	reasonProperty                = "reason"
	messageProperty               = "message"
	stepIDProperty                = "step_id"
	stepTitleProperty             = "step_title"
	stepVersionProperty           = "step_version"
	stepSourceProperty            = "step_source"
	skippableProperty             = "skippable"

	failedValue      = "failed"
	successfulValue  = "successful"
	buildFailedValue = "build_failed"
	runIfValue       = "run_if"

	buildSlugEnvKey = "BITRISE_BUILD_SLUG"
	// StepExecutionIDEnvKey ...
	StepExecutionIDEnvKey = "BITRISE_STEP_EXECUTION_ID"

	bitriseVersionKey = "bitrise"
	envmanVersionKey  = "envman"
	stepmanVersionKey = "stepman"
)

// Tracker ...
type Tracker interface {
	SendWorkflowStarted(properties analytics.Properties, title string)
	SendWorkflowFinished(properties analytics.Properties, failed bool)
	SendStepStartedEvent(properties analytics.Properties, infoModel stepmanModels.StepInfoModel, inputs map[string]interface{})
	SendStepFinishedEvent(properties analytics.Properties, results models.StepRunResultsModel)
	Wait()
}

type tracker struct {
	tracker       analytics.Tracker
	envRepository env.Repository
	stateChecker  StateChecker
}

// NewTracker ...
func NewTracker(analyticsTracker analytics.Tracker, envRepository env.Repository, stateChecker StateChecker) Tracker {
	return tracker{tracker: analyticsTracker, envRepository: envRepository, stateChecker: stateChecker}
}

// NewDefaultTracker ...
func NewDefaultTracker() Tracker {
	envRepository := env.NewRepository()
	return NewTracker(analytics.NewDefaultTracker(), envRepository, NewStateChecker(envRepository))
}

// SendWorkflowStarted sends `workflow_started` events. `parent_step_execution_id` can be used to filter those
// Bitrise CLI events that were started as part of a step (like script).
func (t tracker) SendWorkflowStarted(properties analytics.Properties, title string) {
	if !t.stateChecker.Enabled() {
		return
	}

	isCI := t.envRepository.Get(configs.CIModeEnvKey) == "true"
	isPR := t.envRepository.Get(configs.PRModeEnvKey) == "true"
	isDebug := t.envRepository.Get(configs.DebugModeEnvKey) == "true"
	isSecretFiltering := t.envRepository.Get(configs.IsSecretFilteringKey) == "true"
	isSecretEnvsFiltering := t.envRepository.Get(configs.IsSecretEnvsFilteringKey) == "true"
	buildSlug := t.envRepository.Get(buildSlugEnvKey)
	parentStepExecutionID := t.envRepository.Get(StepExecutionIDEnvKey)

	var bitriseVersion string
	var envmanVersion string
	var stepmanVersion string
	currentVersionMap, err := version.ToolVersionMap(os.Args[0])
	if err == nil {
		if bv, ok := currentVersionMap[bitriseVersionKey]; ok {
			bitriseVersion = bv.String()
		}
		if ev, ok := currentVersionMap[envmanVersionKey]; ok {
			envmanVersion = ev.String()
		}
		if sv, ok := currentVersionMap[stepmanVersionKey]; ok {
			stepmanVersion = sv.String()
		}
	} else {
		t.SendCLIWarning(fmt.Sprintf("Couldn't get tool versions: %s", err.Error()))
	}

	stateProperties := analytics.Properties{
		workflowNameProperty:        title,
		ciModeProperty:              isCI,
		prModeProperty:              isPR,
		debugModeProperty:           isDebug,
		secretFilteringProperty:     isSecretFiltering,
		secretEnvsFilteringProperty: isSecretEnvsFiltering,
	}
	stateProperties.AppendIfNotEmpty(buildSlugProperty, buildSlug)
	stateProperties.AppendIfNotEmpty(parentStepExecutionIDProperty, parentStepExecutionID)
	stateProperties.AppendIfNotEmpty(cliVersionProperty, bitriseVersion)
	stateProperties.AppendIfNotEmpty(envmanVersionProperty, envmanVersion)
	stateProperties.AppendIfNotEmpty(stepmanVersionProperty, stepmanVersion)

	t.tracker.Enqueue(workflowStartedEventName, properties, stateProperties)
}

// SendWorkflowFinished ...
func (t tracker) SendWorkflowFinished(properties analytics.Properties, failed bool) {
	if !t.stateChecker.Enabled() {
		return
	}

	var statusMessage string
	if failed {
		statusMessage = failedValue
	} else {
		statusMessage = successfulValue
	}

	t.tracker.Enqueue(workflowFinishedEventName, properties, analytics.Properties{statusProperty: statusMessage})
}

// SendStepStartedEvent ...
func (t tracker) SendStepStartedEvent(properties analytics.Properties, infoModel stepmanModels.StepInfoModel, inputs map[string]interface{}) {
	if !t.stateChecker.Enabled() {
		return
	}

	extraProperties := []analytics.Properties{properties, prepareStartProperties(infoModel)}
	if len(inputs) > 0 {
		inputBytes, err := json.Marshal(inputs)
		if err != nil {
			t.SendCLIWarning(fmt.Sprintf("Failed to marshal inputs: %s", err.Error()))
		} else {
			extraProperties = append(extraProperties, analytics.Properties{inputsProperty: string(inputBytes)})
		}
	}

	t.tracker.Enqueue(stepStartedEventName, extraProperties...)
}

// SendStepFinishedEvent ...
func (t tracker) SendStepFinishedEvent(properties analytics.Properties, results models.StepRunResultsModel) {
	if !t.stateChecker.Enabled() {
		return
	}

	var eventName string
	var extraProperties analytics.Properties

	switch results.Status {
	case models.StepRunStatusCodeSuccess:
		eventName = stepFinishedEventName
		extraProperties = analytics.Properties{statusProperty: successfulValue}
		break
	case models.StepRunStatusCodeFailed, models.StepRunStatusCodeFailedSkippable:
		eventName = stepFinishedEventName
		extraProperties = analytics.Properties{statusProperty: failedValue}
		extraProperties.AppendIfNotEmpty(errorMessageProperty, results.ErrorStr)
		break
	case models.StepRunStatusCodePreparationFailed:
		eventName = stepPreparationFailedEventName
		extraProperties = prepareStartProperties(results.StepInfo)
		extraProperties.AppendIfNotEmpty("error_message", results.ErrorStr)
	case models.StepRunStatusCodeSkipped, models.StepRunStatusCodeSkippedWithRunIf:
		eventName = stepSkippedEventName
		extraProperties = prepareStartProperties(results.StepInfo)
		if results.Status == models.StepRunStatusCodeSkipped {
			extraProperties[reasonProperty] = buildFailedValue
		} else {
			extraProperties[reasonProperty] = runIfValue
		}
	default:
		panic(fmt.Sprintf("Unknown step status code: %d", results.Status))
	}

	t.tracker.Enqueue(eventName, properties, extraProperties)
}

// SendCLIWarning ...
func (t tracker) SendCLIWarning(message string) {
	if !t.stateChecker.Enabled() {
		return
	}

	t.tracker.Enqueue(cliWarningEventName, analytics.Properties{messageProperty: message})
}

// Wait ...
func (t tracker) Wait() {
	if !t.stateChecker.Enabled() {
		return
	}
	t.tracker.Wait()
}

func prepareStartProperties(infoModel stepmanModels.StepInfoModel) analytics.Properties {
	properties := analytics.Properties{}
	properties.AppendIfNotEmpty(stepIDProperty, infoModel.ID)
	properties.AppendIfNotEmpty(stepTitleProperty, pointers.StringWithDefault(infoModel.Step.Title, ""))
	properties.AppendIfNotEmpty(stepVersionProperty, infoModel.Version)
	properties.AppendIfNotEmpty(stepSourceProperty, pointers.StringWithDefault(infoModel.Step.SourceCodeURL, ""))
	properties[skippableProperty] = pointers.BoolWithDefault(infoModel.Step.IsSkippable, false)
	return properties
}
