package rde

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestGetWarmPool_MappingMasksSecretsAndParsesStatus(t *testing.T) {
	rs := newRecordingServer(t, `{"warmPool":{
		"id":"p1","name":"ios-devs","workspaceId":"ws-1","templateId":"t1","templateName":"iOS Dev",
		"ownerType":"workspace","ownerId":"my-ws","createdByEmail":"a@b.io","desiredCount":2,
		"sessionInputs":[
			{"key":"GITHUB_TOKEN","value":"leaked","isSecret":true},
			{"key":"REPO","value":"app"},
			{"key":"SSH_KEY","savedInputId":"si-1"}
		],
		"enabledFeatureFlagNames":["beta"],
		"machineType":"g2.mac",
		"deviceSpec":{"platform":"ios","deviceModel":"iPhone 16"},
		"status":{"ready":1,"warming":1,"claimedTotal":12,"coldTotal":3,"lastError":"quota","configError":"stack retired",
		  "pausedUntil":"2026-09-17T09:00:00Z",
		  "sessions":[{"sessionId":"s-1","state":"ready","createdAt":"2026-09-17T08:00:00Z","readyAt":"2026-09-17T08:04:00Z"},
		              {"sessionId":"s-2","state":"warming","createdAt":"2026-09-17T08:10:00Z"}]},
		"createdAt":"2026-09-01T00:00:00Z","updatedAt":"2026-09-17T07:00:00Z"}}`)

	p, err := rs.service().GetWarmPool(context.Background(), "ws-1", "p1")
	if err != nil {
		t.Fatalf("GetWarmPool: %v", err)
	}
	if want := "/v1/workspaces/ws-1/warm-pools/p1"; rs.lastPath != want {
		t.Errorf("path = %s, want %s", rs.lastPath, want)
	}
	if p.ID != "p1" || p.TemplateName != "iOS Dev" || p.OwnerType != SessionOwnerWorkspace || p.DesiredCount != 2 || p.MachineType != "g2.mac" {
		t.Errorf("pool = %+v", p)
	}
	if len(p.SessionInputs) != 3 {
		t.Fatalf("session inputs = %+v", p.SessionInputs)
	}
	// Secret values are masked at the boundary even if the backend leaks one.
	if got := p.SessionInputs[0]; got.Value != "" || !got.IsSecret || got.Key != "GITHUB_TOKEN" {
		t.Errorf("secret input not masked: %+v", got)
	}
	if got := p.SessionInputs[1]; got.Value != "app" || got.IsSecret {
		t.Errorf("plain input wrong: %+v", got)
	}
	if got := p.SessionInputs[2]; got.SavedInputID != "si-1" {
		t.Errorf("saved input wrong: %+v", got)
	}
	if p.DeviceSpec == nil || p.DeviceSpec.DeviceModel != "iPhone 16" {
		t.Errorf("device spec = %+v", p.DeviceSpec)
	}
	st := p.Status
	if st == nil {
		t.Fatal("status is nil")
	}
	if st.Ready != 1 || st.Warming != 1 || st.ClaimedTotal != 12 || st.ColdTotal != 3 || st.LastError != "quota" || st.ConfigError != "stack retired" {
		t.Errorf("status = %+v", st)
	}
	if st.PausedUntil == nil || !st.PausedUntil.Equal(time.Date(2026, 9, 17, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("paused_until = %v", st.PausedUntil)
	}
	if len(st.Sessions) != 2 || st.Sessions[0].ReadyAt == nil || st.Sessions[1].ReadyAt != nil || st.Sessions[1].State != "warming" {
		t.Errorf("inventory = %+v", st.Sessions)
	}
	if p.CreatedAt == nil || p.UpdatedAt == nil {
		t.Errorf("timestamps not parsed: %v / %v", p.CreatedAt, p.UpdatedAt)
	}

	// The stable JSON shape: snake_case keys, counters always present, the
	// masked secret carries only key + is_secret.
	out, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{`"desired_count":2`, `"template_name":"iOS Dev"`, `"owner_type":"workspace"`, `"claimed_total":12`, `"cold_total":3`, `"session_id":"s-1"`, `"saved_input_id":"si-1"`, `"paused_until"`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("JSON missing %s:\n%s", want, out)
		}
	}
	if strings.Contains(string(out), "leaked") {
		t.Errorf("JSON leaks the secret value:\n%s", out)
	}
}

// A drained pool reports 0 everywhere, and 0 must be visible in JSON — a
// caller deciding whether to scale up reads exactly these fields.
func TestWarmPool_ZeroCountsStayInJSON(t *testing.T) {
	rs := newRecordingServer(t, `{"warmPools":[{"id":"p1","name":"preset","status":{}}]}`)

	pools, err := rs.service().ListWarmPools(context.Background(), "ws-1", "", false)
	if err != nil {
		t.Fatalf("ListWarmPools: %v", err)
	}
	out, err := json.Marshal(pools[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{`"desired_count":0`, `"ready":0`, `"warming":0`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("JSON missing %s:\n%s", want, out)
		}
	}
}

func TestListWarmPools_ForwardsFilterAndScope(t *testing.T) {
	rs := newRecordingServer(t, `{"warmPools":[]}`)

	if _, err := rs.service().ListWarmPools(context.Background(), "ws-1", "t1", true); err != nil {
		t.Fatalf("ListWarmPools: %v", err)
	}
	if rs.lastMethod != http.MethodGet {
		t.Errorf("method = %s, want GET", rs.lastMethod)
	}
	if !strings.Contains(rs.lastQuery, "templateId=t1") || !strings.Contains(rs.lastQuery, "scope=WARM_POOL_LIST_SCOPE_ALL") {
		t.Errorf("query = %q", rs.lastQuery)
	}
}

func TestCreateWarmPool_MapsRequestAndValidates(t *testing.T) {
	rs := newRecordingServer(t, `{"warmPool":{"id":"p-new","name":"ios-devs","desiredCount":2}}`)
	svc := rs.service()

	p, err := svc.CreateWarmPool(context.Background(), "ws-1", CreateWarmPoolRequest{
		Name:                    "ios-devs",
		TemplateID:              "t1",
		OwnerType:               SessionOwnerWorkspace,
		DesiredCount:            2,
		SessionInputs:           []SessionInputValue{{Key: "GITHUB_TOKEN", Value: "ghp_x", IsSecret: true}, {Key: "SSH", SavedInputID: "si-1"}},
		EnabledFeatureFlagNames: []string{"beta"},
		StackID:                 "osx-27-edge",
		Cluster:                 "c1",
	})
	if err != nil {
		t.Fatalf("CreateWarmPool: %v", err)
	}
	if rs.lastMethod != http.MethodPost || rs.lastPath != "/v1/workspaces/ws-1/warm-pools" {
		t.Errorf("request = %s %s", rs.lastMethod, rs.lastPath)
	}
	var sent map[string]any
	if err := json.Unmarshal(rs.lastBody, &sent); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sent["templateId"] != "t1" || sent["ownerType"] != "workspace" || sent["desiredCount"] != float64(2) || sent["stackId"] != "osx-27-edge" || sent["cluster"] != "c1" {
		t.Errorf("body = %s", rs.lastBody)
	}
	inputs, _ := sent["sessionInputs"].([]any)
	if len(inputs) != 2 {
		t.Fatalf("sessionInputs = %v", sent["sessionInputs"])
	}
	if in, _ := inputs[1].(map[string]any); in["savedInputId"] != "si-1" {
		t.Errorf("saved input not forwarded: %v", inputs[1])
	}
	if p.ID != "p-new" {
		t.Errorf("pool = %+v", p)
	}

	// Local guards short-circuit before any HTTP call.
	hitsBefore := rs.lastMethod
	for name, req := range map[string]CreateWarmPoolRequest{
		"no name":        {TemplateID: "t1"},
		"no template":    {Name: "x"},
		"negative count": {Name: "x", TemplateID: "t1", DesiredCount: -1},
	} {
		if _, err := svc.CreateWarmPool(context.Background(), "ws-1", req); err == nil {
			t.Errorf("%s: expected a validation error", name)
		}
	}
	if rs.lastMethod != hitsBefore {
		t.Error("validation guards must not reach the server")
	}
}

func TestUpdateWarmPool_SwitchesFollowTheLists(t *testing.T) {
	rs := newRecordingServer(t, `{"warmPool":{"id":"p1","name":"ios-devs"}}`)
	svc := rs.service()

	// Name + count only: no list, no switch.
	name := "renamed"
	zero := 0
	if _, err := svc.UpdateWarmPool(context.Background(), "ws-1", "p1", UpdateWarmPoolRequest{Name: &name, DesiredCount: &zero}); err != nil {
		t.Fatalf("UpdateWarmPool: %v", err)
	}
	if rs.lastMethod != http.MethodPatch || rs.lastPath != "/v1/workspaces/ws-1/warm-pools/p1" {
		t.Errorf("request = %s %s", rs.lastMethod, rs.lastPath)
	}
	var sent map[string]any
	if err := json.Unmarshal(rs.lastBody, &sent); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sent["name"] != "renamed" || sent["desiredCount"] != float64(0) {
		t.Errorf("body = %s", rs.lastBody)
	}
	for _, k := range []string{"updateSessionInputs", "updateEnabledFeatureFlagNames", "updateDeviceSpec", "stackId"} {
		if _, ok := sent[k]; ok {
			t.Errorf("%s should be absent, body = %s", k, rs.lastBody)
		}
	}

	// A non-nil (even empty) list replaces the pool's and carries its
	// switch; an empty-string override clears it; ClearDeviceSpec is the
	// device switch alone.
	empty := []SessionInputValue{}
	flags := []string{"beta"}
	noStack := ""
	if _, err := svc.UpdateWarmPool(context.Background(), "ws-1", "p1", UpdateWarmPoolRequest{
		SessionInputs:           &empty,
		EnabledFeatureFlagNames: &flags,
		StackID:                 &noStack,
		ClearDeviceSpec:         true,
	}); err != nil {
		t.Fatalf("UpdateWarmPool: %v", err)
	}
	sent = nil
	if err := json.Unmarshal(rs.lastBody, &sent); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sent["updateSessionInputs"] != true || sent["updateEnabledFeatureFlagNames"] != true || sent["updateDeviceSpec"] != true {
		t.Errorf("switches missing, body = %s", rs.lastBody)
	}
	if _, ok := sent["sessionInputs"]; ok {
		t.Errorf("an empty list rides as the switch alone, body = %s", rs.lastBody)
	}
	if got, _ := sent["enabledFeatureFlagNames"].([]any); len(got) != 1 || got[0] != "beta" {
		t.Errorf("enabledFeatureFlagNames = %v", sent["enabledFeatureFlagNames"])
	}
	if v, ok := sent["stackId"]; !ok || v != "" {
		t.Errorf("stackId = %v (present=%v), want \"\" present to clear the override", v, ok)
	}
	if _, ok := sent["deviceSpec"]; ok {
		t.Errorf("deviceSpec must be absent when clearing, body = %s", rs.lastBody)
	}

	neg := -1
	if _, err := svc.UpdateWarmPool(context.Background(), "ws-1", "p1", UpdateWarmPoolRequest{DesiredCount: &neg}); err == nil {
		t.Error("expected a validation error for a negative count")
	}
}

func TestDeleteWarmPool_Path(t *testing.T) {
	rs := newRecordingServer(t, ``)

	if err := rs.service().DeleteWarmPool(context.Background(), "ws-1", "p1"); err != nil {
		t.Fatalf("DeleteWarmPool: %v", err)
	}
	if rs.lastMethod != http.MethodDelete || rs.lastPath != "/v1/workspaces/ws-1/warm-pools/p1" {
		t.Errorf("request = %s %s", rs.lastMethod, rs.lastPath)
	}
}

func TestResolveWarmPoolID(t *testing.T) {
	rs := newRecordingServer(t, `{"warmPools":[{"id":"p-1","name":"ios-devs"},{"id":"p-2","name":"dup"},{"id":"p-3","name":"Dup"}]}`)
	svc := rs.service()
	ctx := context.Background()

	// A UUID short-circuits with no network call.
	const uuid = "33333333-4444-4444-8444-555555555555"
	if got, err := svc.ResolveWarmPoolID(ctx, "ws-1", uuid); err != nil || got != uuid {
		t.Errorf("uuid: got %q, %v", got, err)
	}
	if rs.lastMethod != "" {
		t.Error("UUID resolution must not call the server")
	}

	if got, err := svc.ResolveWarmPoolID(ctx, "ws-1", "IOS-DEVS"); err != nil || got != "p-1" {
		t.Errorf("name: got %q, %v", got, err)
	}
	if strings.Contains(rs.lastQuery, "SCOPE_ALL") {
		t.Errorf("name resolution must use the visible scope, query = %q", rs.lastQuery)
	}
	if _, err := svc.ResolveWarmPoolID(ctx, "ws-1", "nope"); err == nil || !strings.Contains(err.Error(), "no warm pool named") {
		t.Errorf("unknown name: err = %v", err)
	}
	if _, err := svc.ResolveWarmPoolID(ctx, "ws-1", "dup"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("ambiguous name: err = %v", err)
	}
	if _, err := svc.ResolveWarmPoolID(ctx, "ws-1", ""); err == nil {
		t.Error("empty value: expected an error")
	}
}

// The warm fields on a session reach the stable type verbatim and are sent
// on create only when set.
func TestSession_WarmFields(t *testing.T) {
	rs := newRecordingServer(t, `{"session":{"id":"s1","name":"dev","status":"SESSION_STATUS_RUNNING","warmPoolId":"p1","warmState":"claimed"}}`)
	svc := rs.service()

	res, err := svc.CreateSession(context.Background(), "ws-1", CreateSessionRequest{Name: "dev", WarmPoolID: "p1"})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	var sent map[string]any
	if err := json.Unmarshal(rs.lastBody, &sent); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sent["warmPoolId"] != "p1" {
		t.Errorf("warmPoolId = %v, want p1", sent["warmPoolId"])
	}
	if res.Session.WarmPoolID != "p1" || res.Session.WarmState != WarmStateClaimed {
		t.Errorf("session warm fields = %q / %q", res.Session.WarmPoolID, res.Session.WarmState)
	}
	out, _ := json.Marshal(res.Session)
	if !strings.Contains(string(out), `"warm_pool_id":"p1"`) || !strings.Contains(string(out), `"warm_state":"claimed"`) {
		t.Errorf("JSON missing warm fields:\n%s", out)
	}

	// A session without a pool must not grow new keys — the JSON contract
	// changes additively only.
	rs.lastBody = nil
	res, err = svc.CreateSession(context.Background(), "ws-1", CreateSessionRequest{Name: "dev", TemplateID: "t1"})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	sent = nil
	if err := json.Unmarshal(rs.lastBody, &sent); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := sent["warmPoolId"]; ok {
		t.Errorf("warmPoolId must be omitted when unset, body = %s", rs.lastBody)
	}
	out, _ = json.Marshal(Session{ID: "s2", Name: "plain"})
	if strings.Contains(string(out), "warm_") {
		t.Errorf("a pool-less session must not emit warm_* keys:\n%s", out)
	}
	_ = res
}
