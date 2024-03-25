package models

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/bitrise-io/go-utils/sliceutil"
	"github.com/ryanuber/go-glob"
)

type TriggerEventType string

const (
	TriggerEventTypeCodePush    TriggerEventType = "code-push"
	TriggerEventTypePullRequest TriggerEventType = "pull-request"
	TriggerEventTypeTag         TriggerEventType = "tag"
	TriggerEventTypeUnknown     TriggerEventType = "unknown"
)

type PullRequestReadyState string

const (
	PullRequestReadyStateDraft                     PullRequestReadyState = "draft"
	PullRequestReadyStateReadyForReview            PullRequestReadyState = "ready_for_review"
	PullRequestReadyStateConvertedToReadyForReview PullRequestReadyState = "converted_to_ready_for_review"
)

const defaultDraftPullRequestEnabled = true

type TriggerItemConditionStringValue string

type TriggerItemConditionRegexValue struct {
	Regex string `json:"regex" yaml:"regex"`
}

type TriggerItemType string

const (
	CodePushType    TriggerItemType = "code-push"
	PullRequestType TriggerItemType = "pull-request"
	TagPushType     TriggerItemType = "tag-push"
)

type TriggerMapItemModel struct {
	// Trigger Item shared properties
	Type       TriggerItemType `json:"type" yaml:"type"`
	Enabled    bool            `json:"enabled" yaml:"enabled"`
	PipelineID string          `json:"pipeline,omitempty" yaml:"pipeline,omitempty"`
	WorkflowID string          `json:"workflow,omitempty" yaml:"workflow,omitempty"`

	// Code Push Item conditions
	PushBranch    interface{} `json:"push_branch,omitempty" yaml:"push_branch,omitempty"`
	CommitMessage interface{} `json:"commit_message" yaml:"commit_message"`
	ChangedFiles  interface{} `json:"changed_files" yaml:"changed_files"`

	// Tag Push Item conditions
	Tag interface{} `json:"tag,omitempty" yaml:"tag,omitempty"`

	// Pull Request Item conditions
	PullRequestSourceBranch interface{} `json:"pull_request_source_branch,omitempty" yaml:"pull_request_source_branch,omitempty"`
	PullRequestTargetBranch interface{} `json:"pull_request_target_branch,omitempty" yaml:"pull_request_target_branch,omitempty"`
	DraftPullRequestEnabled *bool       `json:"draft_pull_request_enabled,omitempty" yaml:"draft_pull_request_enabled,omitempty"`
	PullRequestLabel        interface{} `json:"pull_request_label" yaml:"pull_request_label"`

	// Deprecated properties
	Pattern              string `json:"pattern,omitempty" yaml:"pattern,omitempty"`
	IsPullRequestAllowed bool   `json:"is_pull_request_allowed,omitempty" yaml:"is_pull_request_allowed,omitempty"`
}

func (triggerItem TriggerMapItemModel) Validate(idx int, workflows, pipelines []string) ([]string, error) {
	warnings, err := triggerItem.validateTarget(idx, workflows, pipelines)
	if err != nil {
		return warnings, err
	}

	if triggerItem.Pattern != "" {
		if err := triggerItem.validateTypeOfLegacyItem(idx); err != nil {
			return warnings, err
		}
	} else if triggerItem.Type == "" {
		if err := triggerItem.validateTypeOfItem(idx); err != nil {
			return warnings, err
		}
	} else {
		if err := triggerItem.validateTypeOfItemWithExplicitType(idx); err != nil {
			return warnings, err
		}
		if err := triggerItem.validateValuesOfItemWithExplicitType(idx); err != nil {
			return warnings, err
		}
	}

	return warnings, nil
}

func (triggerItem TriggerMapItemModel) validateTypeOfLegacyItem(idx int) error {
	if err := triggerItem.validateNoCodePushConditionsSet(idx, "pattern"); err != nil {
		return err
	}
	if err := triggerItem.validateNoTagPushConditionsSet(idx, "pattern"); err != nil {
		return err
	}
	if err := triggerItem.validateNoPullRequestConditionsSet(idx, "pattern"); err != nil {
		return err
	}
	return nil
}

func (triggerItem TriggerMapItemModel) validateTypeOfItem(idx int) error {
	if isStringLiteralOrRegexSet(triggerItem.PushBranch) {
		if err := triggerItem.validateNoTagPushConditionsSet(idx, "push_branch"); err != nil {
			return err
		}
		if err := triggerItem.validateNoPullRequestConditionsSet(idx, "push_branch"); err != nil {
			return err
		}
	} else if isStringLiteralOrRegexSet(triggerItem.PullRequestSourceBranch) {
		if err := triggerItem.validateNoCodePushConditionsSet(idx, "pull_request_source_branch"); err != nil {
			return err
		}
		if err := triggerItem.validateNoTagPushConditionsSet(idx, "pull_request_source_branch"); err != nil {
			return err
		}
	} else if isStringLiteralOrRegexSet(triggerItem.PullRequestTargetBranch) {
		if err := triggerItem.validateNoCodePushConditionsSet(idx, "pull_request_target_branch"); err != nil {
			return err
		}
		if err := triggerItem.validateNoTagPushConditionsSet(idx, "pull_request_target_branch"); err != nil {
			return err
		}
	} else if isStringLiteralOrRegexSet(triggerItem.Tag) {
		if err := triggerItem.validateNoCodePushConditionsSet(idx, "tag"); err != nil {
			return err
		}
		if err := triggerItem.validateNoPullRequestConditionsSet(idx, "tag"); err != nil {
			return err
		}
	} else if !isStringLiteralOrRegexSet(triggerItem.Tag) {
		return fmt.Errorf("no trigger condition defined defined in the %d. trigger item", idx+1)
	}

	return nil
}

func (triggerItem TriggerMapItemModel) validateTypeOfItemWithExplicitType(idx int) error {
	switch triggerItem.Type {
	case CodePushType:
		if err := triggerItem.validateNoTagPushConditionsSet(idx, fmt.Sprintf("%s type", CodePushType)); err != nil {
			return err
		}
		if err := triggerItem.validateNoPullRequestConditionsSet(idx, fmt.Sprintf("%s type", CodePushType)); err != nil {
			return err
		}
	case PullRequestType:
		if err := triggerItem.validateNoCodePushConditionsSet(idx, fmt.Sprintf("%s type", PullRequestType)); err != nil {
			return err
		}
		if err := triggerItem.validateNoTagPushConditionsSet(idx, fmt.Sprintf("%s type", PullRequestType)); err != nil {
			return err
		}
	case TagPushType:
		if err := triggerItem.validateNoCodePushConditionsSet(idx, fmt.Sprintf("%s type", TagPushType)); err != nil {
			return err
		}
		if err := triggerItem.validateNoPullRequestConditionsSet(idx, fmt.Sprintf("%s type", TagPushType)); err != nil {
			return err
		}
	}
	return nil
}

func (triggerItem TriggerMapItemModel) validateNoCodePushConditionsSet(idx int, field string) error {
	if isStringLiteralOrRegexSet(triggerItem.PushBranch) {
		return fmt.Errorf("both %s and push_branch defined in the %d. trigger item", field, idx+1)
	}
	if isStringLiteralOrRegexSet(triggerItem.CommitMessage) {
		return fmt.Errorf("both %s and commit_message defined in the %d. trigger item", field, idx+1)
	}
	if isStringLiteralOrRegexSet(triggerItem.ChangedFiles) {
		return fmt.Errorf("both %s and changed_files defined in the %d. trigger item", field, idx+1)
	}
	return nil
}

func (triggerItem TriggerMapItemModel) validateNoTagPushConditionsSet(idx int, field string) error {
	if isStringLiteralOrRegexSet(triggerItem.Tag) {
		return fmt.Errorf("both %s and tag defined in the %d. trigger item", field, idx+1)
	}
	return nil
}

func (triggerItem TriggerMapItemModel) validateNoPullRequestConditionsSet(idx int, field string) error {
	if isStringLiteralOrRegexSet(triggerItem.PullRequestSourceBranch) {
		return fmt.Errorf("both %s and pull_request_source_branch defined in the %d. trigger item", field, idx+1)
	}
	if isStringLiteralOrRegexSet(triggerItem.PullRequestTargetBranch) {
		return fmt.Errorf("both %s and pull_request_target_branch defined in the %d. trigger item", field, idx+1)
	}
	if triggerItem.IsDraftPullRequestEnabled() != defaultDraftPullRequestEnabled {
		return fmt.Errorf("both %s and draft_pull_request_enabled defined in the %d. trigger item", field, idx+1)
	}
	if isStringLiteralOrRegexSet(triggerItem.PullRequestLabel) {
		return fmt.Errorf("both %s and pull_request_label defined in the %d. trigger item", field, idx+1)
	}
	return nil
}

func (triggerItem TriggerMapItemModel) validateValuesOfItemWithExplicitType(idx int) error {
	return nil
}

func stringFromTriggerCondition(condition interface{}) string {
	if condition == nil {
		return ""
	}
	return condition.(string)
}

func stringLiteralOrRegex(value interface{}) string {
	if value == nil {
		return ""
	}
	str, ok := value.(TriggerItemConditionStringValue)
	if ok {
		return string(str)
	}

	regex, ok := value.(TriggerItemConditionRegexValue)
	if ok {
		return regex.Regex
	}
	return ""
}

func isStringLiteralOrRegexSet(value interface{}) bool {
	return stringLiteralOrRegex(value) != ""
}

func (triggerItem TriggerMapItemModel) validateTarget(idx int, workflows, pipelines []string) ([]string, error) {
	var warnings []string

	// Validate target
	if triggerItem.PipelineID != "" && triggerItem.WorkflowID != "" {
		return warnings, fmt.Errorf("both pipeline and workflow are defined as trigger target for the %d. trigger item", idx+1)
	}
	if triggerItem.PipelineID == "" && triggerItem.WorkflowID == "" {
		return warnings, fmt.Errorf("no pipeline nor workflow is defined as a trigger target for the %d. trigger item", idx+1)
	}

	if strings.HasPrefix(triggerItem.WorkflowID, "_") {
		warnings = append(warnings, fmt.Sprintf("utility workflow (%s) defined as trigger target for the %d. trigger item, but utility workflows can't be triggered directly", triggerItem.WorkflowID, idx+1))
	}

	if triggerItem.PipelineID != "" {
		if !sliceutil.IsStringInSlice(triggerItem.PipelineID, pipelines) {
			return warnings, fmt.Errorf("pipeline (%s) defined in the %d. trigger item, but does not exist", triggerItem.PipelineID, idx+1)
		}
	} else {
		if !sliceutil.IsStringInSlice(triggerItem.WorkflowID, workflows) {
			return warnings, fmt.Errorf("workflow (%s) defined in the %d. trigger item, but does not exist", triggerItem.WorkflowID, idx+1)
		}
	}

	return warnings, nil
}

func (triggerItem TriggerMapItemModel) MatchWithParams(pushBranch, prSourceBranch, prTargetBranch string, prReadyState PullRequestReadyState, tag string) (bool, error) {
	paramsEventType, err := triggerEventType(pushBranch, prSourceBranch, prTargetBranch, tag)
	if err != nil {
		return false, err
	}

	migratedTriggerItems := []TriggerMapItemModel{triggerItem}
	if triggerItem.Pattern != "" {
		migratedTriggerItems = migrateDeprecatedTriggerItem(triggerItem)
	}

	for _, migratedTriggerItem := range migratedTriggerItems {
		itemEventType, err := triggerEventType(stringFromTriggerCondition(migratedTriggerItem.PushBranch),
			stringFromTriggerCondition(migratedTriggerItem.PullRequestSourceBranch),
			stringFromTriggerCondition(migratedTriggerItem.PullRequestTargetBranch),
			stringFromTriggerCondition(migratedTriggerItem.Tag))
		if err != nil {
			return false, err
		}

		if paramsEventType != itemEventType {
			continue
		}

		switch itemEventType {
		case TriggerEventTypeCodePush:
			match := glob.Glob(stringFromTriggerCondition(migratedTriggerItem.PushBranch), pushBranch)
			return match, nil
		case TriggerEventTypePullRequest:
			sourceMatch := false
			if migratedTriggerItem.PullRequestSourceBranch == "" {
				sourceMatch = true
			} else {
				sourceMatch = glob.Glob(stringFromTriggerCondition(migratedTriggerItem.PullRequestSourceBranch), prSourceBranch)
			}

			targetMatch := false
			if migratedTriggerItem.PullRequestTargetBranch == "" {
				targetMatch = true
			} else {
				targetMatch = glob.Glob(stringFromTriggerCondition(migratedTriggerItem.PullRequestTargetBranch), prTargetBranch)
			}

			// When a PR is converted to ready for review:
			// - if draft PR trigger is enabled, this event is just a status change on the PR
			// 	 and the given status of the code base already triggered a build.
			// - if draft PR trigger is disabled, the given status of the code base didn't trigger a build yet.
			stateMismatch := false
			if migratedTriggerItem.IsDraftPullRequestEnabled() {
				if prReadyState == PullRequestReadyStateConvertedToReadyForReview {
					stateMismatch = true
				}
			} else {
				if prReadyState == PullRequestReadyStateDraft {
					stateMismatch = true
				}
			}

			return sourceMatch && targetMatch && !stateMismatch, nil
		case TriggerEventTypeTag:
			match := glob.Glob(stringFromTriggerCondition(migratedTriggerItem.Tag), tag)
			return match, nil
		}
	}

	return false, nil
}

func (triggerItem TriggerMapItemModel) IsDraftPullRequestEnabled() bool {
	draftPullRequestEnabled := defaultDraftPullRequestEnabled
	if triggerItem.DraftPullRequestEnabled != nil {
		draftPullRequestEnabled = *triggerItem.DraftPullRequestEnabled
	}
	return draftPullRequestEnabled
}

func (triggerItem TriggerMapItemModel) String() string {
	str := ""

	rv := reflect.Indirect(reflect.ValueOf(&triggerItem))
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		tag := field.Tag.Get("yaml")
		tag = strings.TrimSuffix(tag, ",omitempty")
		if tag == "pipeline" || tag == "workflow" || tag == "type" || tag == "enabled" {
			continue
		}

		value := rv.FieldByName(field.Name).Interface()
		str += fmt.Sprintf("%s:%v", tag, value)

		if i < rt.NumField()-1 {
			str += "&"
		}
	}

	return str
}

func triggerEventType(pushBranch, prSourceBranch, prTargetBranch, tag string) (TriggerEventType, error) {
	if pushBranch != "" {
		// Ensure not mixed with code-push event
		if prSourceBranch != "" {
			return TriggerEventTypeUnknown, fmt.Errorf("push_branch (%s) selects code-push trigger event, but pull_request_source_branch (%s) also provided", pushBranch, prSourceBranch)
		}
		if prTargetBranch != "" {
			return TriggerEventTypeUnknown, fmt.Errorf("push_branch (%s) selects code-push trigger event, but pull_request_target_branch (%s) also provided", pushBranch, prTargetBranch)
		}

		// Ensure not mixed with tag event
		if tag != "" {
			return TriggerEventTypeUnknown, fmt.Errorf("push_branch (%s) selects code-push trigger event, but tag (%s) also provided", pushBranch, tag)
		}

		return TriggerEventTypeCodePush, nil
	} else if prSourceBranch != "" || prTargetBranch != "" {
		// Ensure not mixed with tag event
		if tag != "" {
			return TriggerEventTypeUnknown, fmt.Errorf("pull_request_source_branch (%s) and pull_request_target_branch (%s) selects pull-request trigger event, but tag (%s) also provided", prSourceBranch, prTargetBranch, tag)
		}

		return TriggerEventTypePullRequest, nil
	} else if tag != "" {
		return TriggerEventTypeTag, nil
	}

	return TriggerEventTypeUnknown, fmt.Errorf("failed to determin trigger event from params: push-branch: %s, pr-source-branch: %s, pr-target-branch: %s, tag: %s", pushBranch, prSourceBranch, prTargetBranch, tag)
}

func migrateDeprecatedTriggerItem(triggerItem TriggerMapItemModel) []TriggerMapItemModel {
	migratedItems := []TriggerMapItemModel{
		TriggerMapItemModel{
			PushBranch: triggerItem.Pattern,
			WorkflowID: triggerItem.WorkflowID,
		},
	}
	if triggerItem.IsPullRequestAllowed {
		migratedItems = append(migratedItems, TriggerMapItemModel{
			PullRequestSourceBranch: triggerItem.Pattern,
			WorkflowID:              triggerItem.WorkflowID,
		})
	}
	return migratedItems
}
