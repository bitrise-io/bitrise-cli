package rde

import (
	"context"
	"net/http"
	"testing"
)

func TestGetWorkspaceScript_PathQueryAndParse(t *testing.T) {
	rs := newRecordingServer(t, `{"workspaceScript":{
		"configured":true,"enabled":true,"script":"echo hi\n",
		"variables":[
			{"key":"REGION","value":"eu"},
			{"key":"TOKEN","value":"s3cret","isSecret":true},
			{"key":"KEY","isSecret":true,"protected":true,"valueStored":true}
		],
		"updatedBy":"a@b.io","updatedAt":"2026-11-01T10:00:00Z"}}`)

	ws, err := rs.client().GetWorkspaceScript(context.Background(), "ws-1", true)
	if err != nil {
		t.Fatalf("GetWorkspaceScript: %v", err)
	}
	if rs.lastMethod != http.MethodGet || rs.lastPath != "/v1/workspaces/ws-1/workspace-script" {
		t.Errorf("request = %s %s", rs.lastMethod, rs.lastPath)
	}
	if rs.lastQuery != "include_secrets=true" {
		t.Errorf("query = %q, want include_secrets=true", rs.lastQuery)
	}
	if !ws.Configured || !ws.Enabled || ws.Script != "echo hi\n" || ws.UpdatedBy != "a@b.io" || ws.UpdatedAt != "2026-11-01T10:00:00Z" {
		t.Errorf("script = %+v", ws)
	}
	if len(ws.Variables) != 3 || ws.Variables[1].Value != "s3cret" || !ws.Variables[2].Protected || !ws.Variables[2].ValueStored {
		t.Errorf("variables = %+v", ws.Variables)
	}
}

// Without secrets no query is sent; an empty object (nothing configured,
// proto3 omits every zero value) decodes to the zero script.
func TestGetWorkspaceScript_NoSecretsAndEmpty(t *testing.T) {
	rs := newRecordingServer(t, `{"workspaceScript":{}}`)
	ws, err := rs.client().GetWorkspaceScript(context.Background(), "ws-1", false)
	if err != nil {
		t.Fatalf("GetWorkspaceScript: %v", err)
	}
	if rs.lastQuery != "" {
		t.Errorf("query = %q, want none", rs.lastQuery)
	}
	if ws.Configured || ws.Enabled || ws.Script != "" || len(ws.Variables) != 0 {
		t.Errorf("script = %+v, want zero", ws)
	}
}

// The PUT is a full replacement: every field goes out, even false and
// empty, and no variables is [] rather than null.
func TestUpdateWorkspaceScript_SendsFullBody(t *testing.T) {
	rs := newRecordingServer(t, `{"workspaceScript":{"configured":true}}`)
	if _, err := rs.client().UpdateWorkspaceScript(context.Background(), "ws-1", UpdateWorkspaceScriptRequest{}); err != nil {
		t.Fatalf("UpdateWorkspaceScript: %v", err)
	}
	if rs.lastMethod != http.MethodPut || rs.lastPath != "/v1/workspaces/ws-1/workspace-script" {
		t.Errorf("request = %s %s", rs.lastMethod, rs.lastPath)
	}
	if got, want := string(rs.lastBody), `{"enabled":false,"script":"","variables":[]}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

func TestWorkspaceScript_ForbiddenSurfacesMessage(t *testing.T) {
	rs := newRecordingServer(t, `{"code":7,"message":"The workspace script is not available for this workspace."}`)
	rs.status = http.StatusForbidden
	_, err := rs.client().GetWorkspaceScript(context.Background(), "ws-1", false)
	if err == nil || err.Error() != "RDE API 403: The workspace script is not available for this workspace." {
		t.Errorf("err = %v", err)
	}
}
