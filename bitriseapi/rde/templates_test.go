package rde

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestListTemplates_PathAndParse(t *testing.T) {
	rs := newRecordingServer(t, `{"templates":[
		{"id":"t1","name":"Linux Dev","stackId":"linux-ubuntu-24.04","machineType":"m1"},
		{"id":"t2","name":"macOS Dev","stackId":"osx-xcode-16.0.x-edge","machineType":"m2"}
	]}`)

	tmpls, err := rs.client().ListTemplates(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	if want := "/v1/workspaces/ws-1/templates"; rs.lastPath != want {
		t.Errorf("path = %s, want %s", rs.lastPath, want)
	}
	if len(tmpls) != 2 || tmpls[1].Name != "macOS Dev" {
		t.Errorf("templates = %+v", tmpls)
	}
}

func TestGetTemplate_PathAndParse(t *testing.T) {
	rs := newRecordingServer(t, `{"template":{"id":"t1","name":"Dev","templateVariables":[{"key":"FOO","isSecret":true}]}}`)

	tmpl, err := rs.client().GetTemplate(context.Background(), "ws-1", "t1")
	if err != nil {
		t.Fatalf("GetTemplate: %v", err)
	}
	if want := "/v1/workspaces/ws-1/templates/t1"; rs.lastPath != want {
		t.Errorf("path = %s, want %s", rs.lastPath, want)
	}
	if len(tmpl.TemplateVariables) != 1 || !tmpl.TemplateVariables[0].IsSecret {
		t.Errorf("template variables = %+v", tmpl.TemplateVariables)
	}
}

func TestCreateTemplate_BodyAndPath(t *testing.T) {
	rs := newRecordingServer(t, `{"template":{"id":"t-new","name":"Dev"}}`)

	tmpl, err := rs.client().CreateTemplate(context.Background(), "ws-1", CreateTemplateRequest{
		Name:        "Dev",
		StackID:     "osx-xcode-16.0.x-edge",
		MachineType: "g2.mac",
		SessionInputs: []SessionInputCreate{
			{Key: "repo", Required: true},
		},
	})
	if err != nil {
		t.Fatalf("CreateTemplate: %v", err)
	}
	if rs.lastMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", rs.lastMethod)
	}
	if want := "/v1/workspaces/ws-1/templates"; rs.lastPath != want {
		t.Errorf("path = %s, want %s", rs.lastPath, want)
	}
	var sent CreateTemplateRequest
	if err := json.Unmarshal(rs.lastBody, &sent); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sent.StackID != "osx-xcode-16.0.x-edge" || sent.MachineType != "g2.mac" {
		t.Errorf("sent = %+v", sent)
	}
	if len(sent.SessionInputs) != 1 || !sent.SessionInputs[0].Required {
		t.Errorf("sent session inputs = %+v", sent.SessionInputs)
	}
	if tmpl.ID != "t-new" {
		t.Errorf("id = %q, want t-new", tmpl.ID)
	}
}

func TestUpdateTemplate_OmitsUnsetAndCarriesReplaceFlags(t *testing.T) {
	rs := newRecordingServer(t, `{"template":{"id":"t1","name":"Renamed"}}`)

	name := "Renamed"
	if _, err := rs.client().UpdateTemplate(context.Background(), "ws-1", "t1", UpdateTemplateRequest{
		Name:                &name,
		SessionInputs:       []SessionInputCreate{{Key: "repo"}},
		UpdateSessionInputs: true,
	}); err != nil {
		t.Fatalf("UpdateTemplate: %v", err)
	}
	if rs.lastMethod != http.MethodPatch {
		t.Errorf("method = %s, want PATCH", rs.lastMethod)
	}
	var sent map[string]any
	if err := json.Unmarshal(rs.lastBody, &sent); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sent["name"] != "Renamed" {
		t.Errorf("name = %v, want Renamed", sent["name"])
	}
	// Unset scalar pointers drop out entirely.
	if _, ok := sent["stackId"]; ok {
		t.Errorf("stackId should be omitted, body = %s", rs.lastBody)
	}
	// The replace-flag must accompany the array it gates.
	if sent["updateSessionInputs"] != true {
		t.Errorf("updateSessionInputs = %v, want true", sent["updateSessionInputs"])
	}
	// Flags for arrays that weren't sent must stay omitted.
	if _, ok := sent["updateFeatureFlags"]; ok {
		t.Errorf("updateFeatureFlags should be omitted, body = %s", rs.lastBody)
	}
}

func TestDeleteTemplate_Path(t *testing.T) {
	rs := newRecordingServer(t, ``)

	if err := rs.client().DeleteTemplate(context.Background(), "ws-1", "t1"); err != nil {
		t.Fatalf("DeleteTemplate: %v", err)
	}
	if rs.lastMethod != http.MethodDelete {
		t.Errorf("method = %s, want DELETE", rs.lastMethod)
	}
	if want := "/v1/workspaces/ws-1/templates/t1"; rs.lastPath != want {
		t.Errorf("path = %s, want %s", rs.lastPath, want)
	}
}

func TestTemplates_ValidationGuards(t *testing.T) {
	rs := newRecordingServer(t, `{}`)
	c := rs.client()
	ctx := context.Background()

	cases := map[string]func() error{
		"List/no-ws":         func() error { _, err := c.ListTemplates(ctx, ""); return err },
		"Get/no-ws":          func() error { _, err := c.GetTemplate(ctx, "", "t1"); return err },
		"Get/no-template":    func() error { _, err := c.GetTemplate(ctx, "ws", ""); return err },
		"Create/no-ws":       func() error { _, err := c.CreateTemplate(ctx, "", CreateTemplateRequest{}); return err },
		"Update/no-ws":       func() error { _, err := c.UpdateTemplate(ctx, "", "t1", UpdateTemplateRequest{}); return err },
		"Update/no-template": func() error { _, err := c.UpdateTemplate(ctx, "ws", "", UpdateTemplateRequest{}); return err },
		"Delete/no-ws":       func() error { return c.DeleteTemplate(ctx, "", "t1") },
		"Delete/no-template": func() error { return c.DeleteTemplate(ctx, "ws", "") },
	}
	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Error("expected validation error, got nil")
			}
		})
	}
	if rs.hits != 0 {
		t.Errorf("validation guards made %d HTTP call(s); should short-circuit", rs.hits)
	}
}

func TestUpdateTemplate_DeviceSpecFlag(t *testing.T) {
	rs := newRecordingServer(t, `{"template":{"id":"t1"}}`)

	// Clearing: the flag rides alone, with no deviceSpec.
	if _, err := rs.client().UpdateTemplate(context.Background(), "ws-1", "t1", UpdateTemplateRequest{UpdateDeviceSpec: true}); err != nil {
		t.Fatalf("UpdateTemplate: %v", err)
	}
	var sent map[string]any
	if err := json.Unmarshal(rs.lastBody, &sent); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sent["updateDeviceSpec"] != true {
		t.Errorf("updateDeviceSpec = %v, want true", sent["updateDeviceSpec"])
	}
	if _, ok := sent["deviceSpec"]; ok {
		t.Errorf("deviceSpec should be omitted when clearing, body = %s", rs.lastBody)
	}

	// Setting: both go out together.
	if _, err := rs.client().UpdateTemplate(context.Background(), "ws-1", "t1", UpdateTemplateRequest{
		DeviceSpec:       &DeviceSpec{Platform: "ios", DeviceModel: "iPhone 16"},
		UpdateDeviceSpec: true,
	}); err != nil {
		t.Fatalf("UpdateTemplate: %v", err)
	}
	sent = nil
	if err := json.Unmarshal(rs.lastBody, &sent); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	spec, _ := sent["deviceSpec"].(map[string]any)
	if spec["platform"] != "ios" || spec["deviceModel"] != "iPhone 16" || sent["updateDeviceSpec"] != true {
		t.Errorf("body = %s", rs.lastBody)
	}
}

func TestTemplates_HooksOnTheWire(t *testing.T) {
	rs := newRecordingServer(t, `{"template":{"id":"t-new","name":"Hooks","lifecycleMode":"hooks","hooks":{"onCreate":"echo create","onRestore":"echo restore"},"sessionInputs":[{"key":"GH_TOKEN","hook":"on_claim"}]}}`)

	tmpl, err := rs.client().CreateTemplate(context.Background(), "ws-1", CreateTemplateRequest{
		Name: "Hooks", StackID: "linux", MachineType: "standard",
		Hooks:         &TemplateHooks{OnCreate: strPtr("echo create"), OnClaim: strPtr(""), OnRestore: strPtr("echo restore")},
		SessionInputs: []SessionInputCreate{{Key: "GH_TOKEN", Required: true, Hook: "on_claim"}},
	})
	if err != nil {
		t.Fatalf("CreateTemplate: %v", err)
	}
	var sent map[string]any
	if err := json.Unmarshal(rs.lastBody, &sent); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	hooks, _ := sent["hooks"].(map[string]any)
	if hooks["onCreate"] != "echo create" || hooks["onRestore"] != "echo restore" {
		t.Errorf("sent hooks = %v", sent["hooks"])
	}
	if v, ok := hooks["onClaim"]; !ok || v != "" {
		t.Errorf("an added empty hook must be sent as \"\": %v", hooks)
	}
	inputs, _ := sent["sessionInputs"].([]any)
	if in, _ := inputs[0].(map[string]any); in["hook"] != "on_claim" {
		t.Errorf("sent sessionInputs[0].hook = %v", in["hook"])
	}
	if tmpl.LifecycleMode != "hooks" || tmpl.Hooks == nil || tmpl.Hooks.OnRestore == nil || *tmpl.Hooks.OnRestore != "echo restore" || tmpl.Hooks.OnClaim != nil || tmpl.SessionInputs[0].Hook != "on_claim" {
		t.Errorf("parsed template = %+v (hooks %+v)", tmpl, tmpl.Hooks)
	}

	if _, err := rs.client().UpdateTemplate(context.Background(), "ws-1", "t1", UpdateTemplateRequest{
		Hooks: &TemplateHooks{OnCreate: strPtr("echo v2")}, UpdateHooks: true,
	}); err != nil {
		t.Fatalf("UpdateTemplate: %v", err)
	}
	if err := json.Unmarshal(rs.lastBody, &sent); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sent["updateHooks"] != true {
		t.Errorf("updateHooks = %v, want true", sent["updateHooks"])
	}
	if hooks, _ := sent["hooks"].(map[string]any); len(hooks) != 1 {
		t.Errorf("hooks that are not added must be omitted: %v", sent["hooks"])
	}
}

func strPtr(s string) *string { return &s }

func TestTemplates_ScopesAndProtected(t *testing.T) {
	rs := newRecordingServer(t, `{"template":{"id":"t1","name":"Hooks","lifecycleMode":"hooks","templateVariables":[{"key":"REG","isSecret":true,"protected":true,"hook":"on_create"}],"sessionInputs":[{"key":"GH","hook":"on_restore"}]}}`)
	tmpl, err := rs.client().CreateTemplate(context.Background(), "ws-1", CreateTemplateRequest{
		Name: "Hooks", StackID: "linux", MachineType: "standard",
		Hooks:             &TemplateHooks{OnCreate: strPtr("echo")},
		TemplateVariables: []TemplateVariableCreate{{Key: "REG", Value: "tok", Hook: "on_create", Protected: true}, {Key: "APP", Value: "demo"}},
		SessionInputs:     []SessionInputCreate{{Key: "GH", Hook: "on_restore"}},
	})
	if err != nil {
		t.Fatalf("CreateTemplate: %v", err)
	}
	var sent map[string]any
	if err := json.Unmarshal(rs.lastBody, &sent); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	vars, _ := sent["templateVariables"].([]any)
	if v, _ := vars[0].(map[string]any); v["protected"] != true || v["hook"] != "on_create" || v["value"] != "tok" {
		t.Errorf("sent templateVariables[0] = %v", vars[0])
	}
	if v, _ := vars[1].(map[string]any); v["protected"] != nil || v["hook"] != nil {
		t.Errorf("protected and hook must be omitted when unset: %v", vars[1])
	}
	inputs, _ := sent["sessionInputs"].([]any)
	if in, _ := inputs[0].(map[string]any); in["hook"] != "on_restore" {
		t.Errorf("sent sessionInputs[0] = %v", inputs[0])
	}
	if len(tmpl.TemplateVariables) != 1 || !tmpl.TemplateVariables[0].Protected || !tmpl.TemplateVariables[0].IsSecret || tmpl.TemplateVariables[0].Hook != "on_create" {
		t.Errorf("parsed variables = %+v", tmpl.TemplateVariables)
	}
	if len(tmpl.SessionInputs) != 1 || tmpl.SessionInputs[0].Hook != "on_restore" {
		t.Errorf("parsed inputs = %+v", tmpl.SessionInputs)
	}
}
