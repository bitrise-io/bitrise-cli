package rde

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	rdeapi "github.com/bitrise-io/bitrise-cli/bitriseapi/rde"
)

func ptr[T any](v T) *T { return &v }

// storedWorkspaceScript is a stored script as GET ?include_secrets=true returns it.
var storedWorkspaceScript = rdeapi.WorkspaceScript{
	Configured: true,
	Enabled:    true,
	Script:     "old\n",
	Variables: []rdeapi.WorkspaceScriptVariable{
		{Key: "REGION", Value: "eu"},
		{Key: "TOKEN", Value: "s3cret", IsSecret: true},
		{Key: "KEY", IsSecret: true, Protected: true, ValueStored: true},
	},
}

func TestApplyWorkspaceScriptChanges(t *testing.T) {
	unchanged := []rdeapi.WorkspaceScriptVariableInput{
		{Key: "REGION", Value: "eu"},
		{Key: "TOKEN", Value: "s3cret", IsSecret: true},
		// Protected: re-sent with "" so the stored value is kept.
		{Key: "KEY", Value: "", IsSecret: true, Protected: true},
	}
	cases := []struct {
		name    string
		changes WorkspaceScriptChanges
		want    rdeapi.UpdateWorkspaceScriptRequest
	}{
		{
			name:    "script only keeps variables and enabled",
			changes: WorkspaceScriptChanges{Script: ptr("new\n")},
			want:    rdeapi.UpdateWorkspaceScriptRequest{Enabled: true, Script: "new\n", Variables: unchanged},
		},
		{
			name:    "disable keeps the script",
			changes: WorkspaceScriptChanges{Enabled: ptr(false)},
			want:    rdeapi.UpdateWorkspaceScriptRequest{Enabled: false, Script: "old\n", Variables: unchanged},
		},
		{
			name: "set replaces in place, adds at the end, remove drops",
			changes: WorkspaceScriptChanges{
				SetVariables: []WorkspaceScriptVariableChange{
					{Key: "NEW", Value: "1", Visibility: VariableVisibilityPlain},
					{Key: "REGION", Value: "us", Visibility: VariableVisibilityPlain},
				},
				RemoveVariables: []string{"TOKEN"},
			},
			want: rdeapi.UpdateWorkspaceScriptRequest{Enabled: true, Script: "old\n", Variables: []rdeapi.WorkspaceScriptVariableInput{
				{Key: "REGION", Value: "us"},
				{Key: "KEY", Value: "", IsSecret: true, Protected: true},
				{Key: "NEW", Value: "1"},
			}},
		},
		{
			name: "visibility change",
			changes: WorkspaceScriptChanges{SetVariables: []WorkspaceScriptVariableChange{
				{Key: "REGION", Value: "eu", Visibility: VariableVisibilitySecret},
				{Key: "TOKEN", Value: "rotated", Visibility: VariableVisibilityProtected},
				{Key: "KEY", Value: "now-plain", Visibility: VariableVisibilityPlain},
			}},
			want: rdeapi.UpdateWorkspaceScriptRequest{Enabled: true, Script: "old\n", Variables: []rdeapi.WorkspaceScriptVariableInput{
				{Key: "REGION", Value: "eu", IsSecret: true},
				{Key: "TOKEN", Value: "rotated", IsSecret: true, Protected: true},
				{Key: "KEY", Value: "now-plain"},
			}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := applyWorkspaceScriptChanges(storedWorkspaceScript, tc.changes)
			if err != nil {
				t.Fatalf("apply: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got  %+v\nwant %+v", got, tc.want)
			}
		})
	}
}

// Input-only mistakes are caught by Validate, which UpdateWorkspaceScript
// runs before its GET: no request is made, so a flag error never turns into
// an auth or network error. Errors name keys, never values.
func TestUpdateWorkspaceScript_ValidatesBeforeAnyRequest(t *testing.T) {
	plain := func(k string) WorkspaceScriptVariableChange {
		return WorkspaceScriptVariableChange{Key: k, Value: "v", Visibility: VariableVisibilityPlain}
	}
	cases := map[string]struct {
		changes WorkspaceScriptChanges
		want    string
	}{
		"nothing":            {WorkspaceScriptChanges{}, "nothing to update"},
		"empty key":          {WorkspaceScriptChanges{SetVariables: []WorkspaceScriptVariableChange{{Key: "", Value: "s3cret", Visibility: VariableVisibilitySecret}}}, "must not be empty"},
		"reserved key":       {WorkspaceScriptChanges{SetVariables: []WorkspaceScriptVariableChange{plain("RDE_X")}}, "reserved"},
		"set twice":          {WorkspaceScriptChanges{SetVariables: []WorkspaceScriptVariableChange{plain("A"), plain("A")}}, "more than once"},
		"set and removed":    {WorkspaceScriptChanges{SetVariables: []WorkspaceScriptVariableChange{plain("REGION")}, RemoveVariables: []string{"REGION"}}, "both set and removed"},
		"empty remove key":   {WorkspaceScriptChanges{RemoveVariables: []string{""}}, "must not be empty"},
		"empty protected":    {WorkspaceScriptChanges{SetVariables: []WorkspaceScriptVariableChange{{Key: "P", Visibility: VariableVisibilityProtected}}}, "needs a value"},
		"unknown visibility": {WorkspaceScriptChanges{SetVariables: []WorkspaceScriptVariableChange{{Key: "P", Value: "s3cret", Visibility: "public"}}}, "unknown visibility"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rs := newRecordingServer(t, `{}`)
			_, err := rs.service().UpdateWorkspaceScript(context.Background(), "ws-1", tc.changes)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want it to contain %q", err, tc.want)
			}
			if err != nil && strings.Contains(err.Error(), "s3cret") {
				t.Errorf("error leaks a value: %v", err)
			}
			if rs.lastMethod != "" {
				t.Errorf("a %s request was made, want none", rs.lastMethod)
			}
		})
	}
}

// Whether a removed variable exists is the one check that needs the stored
// script.
func TestApplyWorkspaceScriptChanges_RemoveUnknown(t *testing.T) {
	_, err := applyWorkspaceScriptChanges(storedWorkspaceScript, WorkspaceScriptChanges{RemoveVariables: []string{"NOPE"}})
	if err == nil || !strings.Contains(err.Error(), "no variable NOPE") {
		t.Errorf("err = %v", err)
	}
}

// The mapper names each variable's visibility and never lets a protected
// value through, even if the backend sent one.
func TestGetWorkspaceScript_Mapping(t *testing.T) {
	rs := newRecordingServer(t, `{"workspaceScript":{"configured":true,"script":"x",
		"variables":[{"key":"A","value":"1"},{"key":"B","isSecret":true},{"key":"C","value":"leak","isSecret":true,"protected":true,"valueStored":true}],
		"updatedBy":"a@b.io","updatedAt":"2026-11-01T10:00:00Z"}}`)
	ws, err := rs.service().GetWorkspaceScript(context.Background(), "ws-1", false)
	if err != nil {
		t.Fatalf("GetWorkspaceScript: %v", err)
	}
	want := []WorkspaceScriptVariable{
		{Key: "A", Value: "1", Visibility: VariableVisibilityPlain},
		{Key: "B", Visibility: VariableVisibilitySecret},
		{Key: "C", Visibility: VariableVisibilityProtected, ValueStored: true},
	}
	if !reflect.DeepEqual(ws.Variables, want) {
		t.Errorf("variables = %+v, want %+v", ws.Variables, want)
	}
	if ws.Enabled || !ws.Configured || ws.UpdatedAt == nil || ws.UpdatedBy != "a@b.io" {
		t.Errorf("script = %+v", ws)
	}
}

// An unconfigured script still renders a dense JSON shape: the booleans,
// the script and an empty variables list are always present.
func TestWorkspaceScript_JSONShapeIsDense(t *testing.T) {
	data, err := json.Marshal(workspaceScriptFromAPI(rdeapi.WorkspaceScript{}))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), `{"configured":false,"enabled":false,"script":"","variables":[]}`; got != want {
		t.Errorf("json = %s, want %s", got, want)
	}
}
