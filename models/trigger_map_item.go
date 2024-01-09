package models

import (
	"fmt"

	"github.com/ryanuber/go-glob"
)

// TriggerEventType ...
type TriggerEventType string

const (
	// TriggerEventTypeCodePush ...
	TriggerEventTypeCodePush TriggerEventType = "code-push"
	// TriggerEventTypePullRequest ...
	TriggerEventTypePullRequest TriggerEventType = "pull-request"
	// TriggerEventTypeTag ...
	TriggerEventTypeTag TriggerEventType = "tag"
	// TriggerEventTypeUnknown ...
	TriggerEventTypeUnknown TriggerEventType = "unknown"
)

// TriggerMapItemModel ...
type TriggerMapItemModel struct {
	PushBranch              string `json:"push_branch,omitempty" yaml:"push_branch,omitempty"`
	PullRequestSourceBranch string `json:"pull_request_source_branch,omitempty" yaml:"pull_request_source_branch,omitempty"`
	PullRequestTargetBranch string `json:"pull_request_target_branch,omitempty" yaml:"pull_request_target_branch,omitempty"`
	Tag                     string `json:"tag,omitempty" yaml:"tag,omitempty"`
	PipelineID              string `json:"pipeline,omitempty" yaml:"pipeline,omitempty"`
	WorkflowID              string `json:"workflow,omitempty" yaml:"workflow,omitempty"`

	// deprecated
	Pattern              string `json:"pattern,omitempty" yaml:"pattern,omitempty"`
	IsPullRequestAllowed bool   `json:"is_pull_request_allowed,omitempty" yaml:"is_pull_request_allowed,omitempty"`
}

// Validate ...
func (triggerItem TriggerMapItemModel) Validate() error {
	if triggerItem.PipelineID != "" && triggerItem.WorkflowID != "" {
		return fmt.Errorf("invalid trigger item: (%s), error: pipeline & workflow both defined", triggerItem.Pattern)
	}
	if triggerItem.PipelineID == "" && triggerItem.WorkflowID == "" {
		return fmt.Errorf("invalid trigger item: (%s), error: empty pipeline & workflow", triggerItem.Pattern)
	}

	if triggerItem.Pattern == "" {
		_, err := triggerEventType(triggerItem.PushBranch, triggerItem.PullRequestSourceBranch, triggerItem.PullRequestTargetBranch, triggerItem.Tag)
		if err != nil {
			return fmt.Errorf("trigger map item (%s) validate failed, error: %s", triggerItem.String(true), err)
		}
	} else if triggerItem.PushBranch != "" ||
		triggerItem.PullRequestSourceBranch != "" || triggerItem.PullRequestTargetBranch != "" || triggerItem.Tag != "" {
		return fmt.Errorf("deprecated trigger item (pattern defined), mixed with trigger params (push_branch: %s, pull_request_source_branch: %s, pull_request_target_branch: %s, tag: %s)", triggerItem.PushBranch, triggerItem.PullRequestSourceBranch, triggerItem.PullRequestTargetBranch, triggerItem.Tag)
	}

	return nil
}

// MatchWithParams ...
func (triggerItem TriggerMapItemModel) MatchWithParams(pushBranch, prSourceBranch, prTargetBranch, tag string) (bool, error) {
	paramsEventType, err := triggerEventType(pushBranch, prSourceBranch, prTargetBranch, tag)
	if err != nil {
		return false, err
	}

	migratedTriggerItems := []TriggerMapItemModel{triggerItem}
	if triggerItem.Pattern != "" {
		migratedTriggerItems = migrateDeprecatedTriggerItem(triggerItem)
	}

	for _, migratedTriggerItem := range migratedTriggerItems {
		itemEventType, err := triggerEventType(migratedTriggerItem.PushBranch, migratedTriggerItem.PullRequestSourceBranch, migratedTriggerItem.PullRequestTargetBranch, migratedTriggerItem.Tag)
		if err != nil {
			return false, err
		}

		if paramsEventType != itemEventType {
			continue
		}

		switch itemEventType {
		case TriggerEventTypeCodePush:
			match := glob.Glob(migratedTriggerItem.PushBranch, pushBranch)
			return match, nil
		case TriggerEventTypePullRequest:
			sourceMatch := false
			if migratedTriggerItem.PullRequestSourceBranch == "" {
				sourceMatch = true
			} else {
				sourceMatch = glob.Glob(migratedTriggerItem.PullRequestSourceBranch, prSourceBranch)
			}

			targetMatch := false
			if migratedTriggerItem.PullRequestTargetBranch == "" {
				targetMatch = true
			} else {
				targetMatch = glob.Glob(migratedTriggerItem.PullRequestTargetBranch, prTargetBranch)
			}

			return (sourceMatch && targetMatch), nil
		case TriggerEventTypeTag:
			match := glob.Glob(migratedTriggerItem.Tag, tag)
			return match, nil
		}
	}

	return false, nil
}

func (triggerItem TriggerMapItemModel) String(printTarget bool) string {
	str := ""

	if triggerItem.PushBranch != "" {
		str = fmt.Sprintf("push_branch: %s", triggerItem.PushBranch)
	}

	if triggerItem.PullRequestSourceBranch != "" || triggerItem.PullRequestTargetBranch != "" {
		if str != "" {
			str += " "
		}

		if triggerItem.PullRequestSourceBranch != "" {
			str += fmt.Sprintf("pull_request_source_branch: %s", triggerItem.PullRequestSourceBranch)
		}
		if triggerItem.PullRequestTargetBranch != "" {
			if triggerItem.PullRequestSourceBranch != "" {
				str += " && "
			}

			str += fmt.Sprintf("pull_request_target_branch: %s", triggerItem.PullRequestTargetBranch)
		}
	}

	if triggerItem.Tag != "" {
		if str != "" {
			str += " "
		}

		str += fmt.Sprintf("tag: %s", triggerItem.Tag)
	}

	if triggerItem.Pattern != "" {
		if str != "" {
			str += " "
		}

		str += fmt.Sprintf("pattern: %s && is_pull_request_allowed: %v", triggerItem.Pattern, triggerItem.IsPullRequestAllowed)
	}

	if printTarget {
		if triggerItem.PipelineID != "" {
			str += fmt.Sprintf(" -> pipeline: %s", triggerItem.PipelineID)
		} else {
			str += fmt.Sprintf(" -> workflow: %s", triggerItem.WorkflowID)
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
