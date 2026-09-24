package rde

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestListWarmPools_PathQueryAndParse(t *testing.T) {
	rs := newRecordingServer(t, `{"warmPools":[
		{"id":"p1","name":"ios-devs","templateId":"t1","templateName":"iOS Dev","ownerType":"workspace","ownerId":"my-ws","poolSize":2,
		 "status":{"ready":1,"warming":1,"claimedTotal":12,"coldTotal":3}},
		{"id":"p2","name":"mine","templateId":"t1","ownerType":"user","createdByEmail":"a@b.io"}
	]}`)

	pools, err := rs.client().ListWarmPools(context.Background(), "ws-1", "", false)
	if err != nil {
		t.Fatalf("ListWarmPools: %v", err)
	}
	if want := "/v1/workspaces/ws-1/warm-pools"; rs.lastPath != want {
		t.Errorf("path = %s, want %s", rs.lastPath, want)
	}
	if rs.lastQuery != "" {
		t.Errorf("query = %q, want none for the default scope", rs.lastQuery)
	}
	if len(pools) != 2 || pools[0].PoolSize != 2 || pools[0].Status == nil || pools[0].Status.ClaimedTotal != 12 {
		t.Errorf("pools = %+v", pools)
	}
	if pools[1].CreatedByEmail != "a@b.io" || pools[1].Status != nil {
		t.Errorf("second pool = %+v", pools[1])
	}

	// The template filter and the every-pool scope ride in the query string,
	// the scope by enum name.
	if _, err := rs.client().ListWarmPools(context.Background(), "ws-1", "t1", true); err != nil {
		t.Fatalf("ListWarmPools(filtered): %v", err)
	}
	if want := "scope=WARM_POOL_LIST_SCOPE_ALL&templateId=t1"; rs.lastQuery != want {
		t.Errorf("query = %q, want %q", rs.lastQuery, want)
	}
}

func TestGetWarmPool_PathAndInventory(t *testing.T) {
	rs := newRecordingServer(t, `{"warmPool":{"id":"p1","name":"ios-devs",
		"sessionInputs":[{"key":"GITHUB_TOKEN","isSecret":true},{"key":"REPO","value":"app"},{"key":"SSH_KEY","savedInputId":"si-1"}],
		"status":{"ready":1,"warming":0,"lastError":"quota exceeded",
		  "sessions":[{"sessionId":"s-1","state":"ready","createdAt":"2026-09-17T08:00:00Z","readyAt":"2026-09-17T08:04:00Z"}]}}}`)

	p, err := rs.client().GetWarmPool(context.Background(), "ws-1", "p 1")
	if err != nil {
		t.Fatalf("GetWarmPool: %v", err)
	}
	if want := "/v1/workspaces/ws-1/warm-pools/p%201"; rs.lastURI != want {
		t.Errorf("uri = %s, want %s (pool id escaped)", rs.lastURI, want)
	}
	if len(p.SessionInputs) != 3 || !p.SessionInputs[0].IsSecret || p.SessionInputs[2].SavedInputID != "si-1" {
		t.Errorf("session inputs = %+v", p.SessionInputs)
	}
	if p.Status == nil || p.Status.LastError != "quota exceeded" || len(p.Status.Sessions) != 1 || p.Status.Sessions[0].State != "ready" {
		t.Errorf("status = %+v", p.Status)
	}
}

func TestCreateWarmPool_BodyAndPath(t *testing.T) {
	rs := newRecordingServer(t, `{"warmPool":{"id":"p-new","name":"ios-devs","poolSize":2}}`)

	p, err := rs.client().CreateWarmPool(context.Background(), "ws-1", CreateWarmPoolRequest{
		Name:                    "ios-devs",
		TemplateID:              "t1",
		OwnerType:               "workspace",
		PoolSize:                2,
		SessionInputs:           []SessionInputValue{{Key: "GITHUB_TOKEN", Value: "ghp_x", IsSecret: true}},
		EnabledFeatureFlagNames: []string{"beta"},
		MachineType:             "g2.mac",
	})
	if err != nil {
		t.Fatalf("CreateWarmPool: %v", err)
	}
	if rs.lastMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", rs.lastMethod)
	}
	if want := "/v1/workspaces/ws-1/warm-pools"; rs.lastPath != want {
		t.Errorf("path = %s, want %s", rs.lastPath, want)
	}
	var sent map[string]any
	if err := json.Unmarshal(rs.lastBody, &sent); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sent["name"] != "ios-devs" || sent["templateId"] != "t1" || sent["ownerType"] != "workspace" || sent["poolSize"] != float64(2) {
		t.Errorf("sent = %v", sent)
	}
	inputs, _ := sent["sessionInputs"].([]any)
	if len(inputs) != 1 {
		t.Fatalf("sessionInputs = %v", sent["sessionInputs"])
	}
	if in, _ := inputs[0].(map[string]any); in["key"] != "GITHUB_TOKEN" || in["isSecret"] != true {
		t.Errorf("sessionInputs[0] = %v", inputs[0])
	}
	// Unset overrides must not reach the wire: an empty stackId would still
	// read as "the template's", but keeping the body minimal is the contract.
	for _, k := range []string{"stackId", "cluster", "deviceSpec", "noDevice", "mapSavedToSessionInputs"} {
		if _, ok := sent[k]; ok {
			t.Errorf("%s should be omitted, body = %s", k, rs.lastBody)
		}
	}
	if p.ID != "p-new" || p.PoolSize != 2 {
		t.Errorf("pool = %+v", p)
	}
}

// A count of 0 is the drained-pool request; it is the wire default too, so
// omitting it is equivalent and keeps the body minimal.
func TestCreateWarmPool_ZeroCountOmitted(t *testing.T) {
	rs := newRecordingServer(t, `{"warmPool":{"id":"p-new","name":"preset"}}`)

	if _, err := rs.client().CreateWarmPool(context.Background(), "ws-1", CreateWarmPoolRequest{Name: "preset", TemplateID: "t1"}); err != nil {
		t.Fatalf("CreateWarmPool: %v", err)
	}
	var sent map[string]any
	if err := json.Unmarshal(rs.lastBody, &sent); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := sent["poolSize"]; ok {
		t.Errorf("poolSize 0 should be omitted, body = %s", rs.lastBody)
	}
	if _, ok := sent["ownerType"]; ok {
		t.Errorf("ownerType should be omitted when unset, body = %s", rs.lastBody)
	}
}

func TestUpdateWarmPool_OmitsUnsetAndCarriesSwitches(t *testing.T) {
	rs := newRecordingServer(t, `{"warmPool":{"id":"p1","name":"renamed"}}`)

	name := "renamed"
	zero := 0
	emptyStack := ""
	if _, err := rs.client().UpdateWarmPool(context.Background(), "ws-1", "p1", UpdateWarmPoolRequest{
		Name:                &name,
		PoolSize:            &zero,
		SessionInputs:       []SessionInputValue{{Key: "REPO", Value: "app"}},
		UpdateSessionInputs: true,
		StackID:             &emptyStack,
	}); err != nil {
		t.Fatalf("UpdateWarmPool: %v", err)
	}
	if rs.lastMethod != http.MethodPatch {
		t.Errorf("method = %s, want PATCH", rs.lastMethod)
	}
	if want := "/v1/workspaces/ws-1/warm-pools/p1"; rs.lastPath != want {
		t.Errorf("path = %s, want %s", rs.lastPath, want)
	}
	var sent map[string]any
	if err := json.Unmarshal(rs.lastBody, &sent); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sent["name"] != "renamed" {
		t.Errorf("name = %v, want renamed", sent["name"])
	}
	// A pointer to 0 is a real "drain the pool" request and must survive
	// omitempty — the wire type is *int for exactly this reason.
	if v, ok := sent["poolSize"]; !ok || v != float64(0) {
		t.Errorf("poolSize = %v (present=%v), want 0 present", v, ok)
	}
	// A pointer to "" clears an override and must likewise be sent.
	if v, ok := sent["stackId"]; !ok || v != "" {
		t.Errorf("stackId = %v (present=%v), want \"\" present", v, ok)
	}
	// The replace switch accompanies the list it gates; the others stay out.
	if sent["updateSessionInputs"] != true {
		t.Errorf("updateSessionInputs = %v, want true", sent["updateSessionInputs"])
	}
	for _, k := range []string{"updateEnabledFeatureFlagNames", "updateDeviceSpec", "machineType", "cluster", "noDevice"} {
		if _, ok := sent[k]; ok {
			t.Errorf("%s should be omitted, body = %s", k, rs.lastBody)
		}
	}
}

// Clearing a list is the switch alone: an empty slice drops out under
// omitempty while the switch tells the backend to replace with nothing.
func TestUpdateWarmPool_ClearListIsSwitchAlone(t *testing.T) {
	rs := newRecordingServer(t, `{"warmPool":{"id":"p1"}}`)

	if _, err := rs.client().UpdateWarmPool(context.Background(), "ws-1", "p1", UpdateWarmPoolRequest{
		EnabledFeatureFlagNames:       []string{},
		UpdateEnabledFeatureFlagNames: true,
	}); err != nil {
		t.Fatalf("UpdateWarmPool: %v", err)
	}
	var sent map[string]any
	if err := json.Unmarshal(rs.lastBody, &sent); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sent["updateEnabledFeatureFlagNames"] != true {
		t.Errorf("updateEnabledFeatureFlagNames = %v, want true", sent["updateEnabledFeatureFlagNames"])
	}
	if _, ok := sent["enabledFeatureFlagNames"]; ok {
		t.Errorf("enabledFeatureFlagNames should be omitted when empty, body = %s", rs.lastBody)
	}
}

func TestDeleteWarmPool_Path(t *testing.T) {
	rs := newRecordingServer(t, ``)

	if err := rs.client().DeleteWarmPool(context.Background(), "ws-1", "p1"); err != nil {
		t.Fatalf("DeleteWarmPool: %v", err)
	}
	if rs.lastMethod != http.MethodDelete {
		t.Errorf("method = %s, want DELETE", rs.lastMethod)
	}
	if want := "/v1/workspaces/ws-1/warm-pools/p1"; rs.lastPath != want {
		t.Errorf("path = %s, want %s", rs.lastPath, want)
	}
}

func TestWarmPools_ValidationGuards(t *testing.T) {
	rs := newRecordingServer(t, `{}`)
	c := rs.client()
	ctx := context.Background()

	cases := map[string]func() error{
		"List/no-ws":     func() error { _, err := c.ListWarmPools(ctx, "", "", false); return err },
		"Get/no-ws":      func() error { _, err := c.GetWarmPool(ctx, "", "p1"); return err },
		"Get/no-pool":    func() error { _, err := c.GetWarmPool(ctx, "ws", ""); return err },
		"Create/no-ws":   func() error { _, err := c.CreateWarmPool(ctx, "", CreateWarmPoolRequest{}); return err },
		"Update/no-ws":   func() error { _, err := c.UpdateWarmPool(ctx, "", "p1", UpdateWarmPoolRequest{}); return err },
		"Update/no-pool": func() error { _, err := c.UpdateWarmPool(ctx, "ws", "", UpdateWarmPoolRequest{}); return err },
		"Delete/no-ws":   func() error { return c.DeleteWarmPool(ctx, "", "p1") },
		"Delete/no-pool": func() error { return c.DeleteWarmPool(ctx, "ws", "") },
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

// The warm-pool fields on the session and preview-link requests are sent
// only when set, so every existing request body stays byte-for-byte the same.
func TestWarmPoolID_OnSessionAndPreviewLinkRequests(t *testing.T) {
	rs := newRecordingServer(t, `{"session":{"id":"s1","warmPoolId":"p1","warmState":"claimed"}}`)

	sess, _, err := rs.client().CreateSession(context.Background(), "ws-1", CreateSessionRequest{Name: "dev", WarmPoolID: "p1"})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	var sent map[string]any
	if err := json.Unmarshal(rs.lastBody, &sent); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sent["warmPoolId"] != "p1" || len(sent) != 2 {
		t.Errorf("body = %s, want only name + warmPoolId", rs.lastBody)
	}
	if sess.WarmPoolID != "p1" || sess.WarmState != "claimed" {
		t.Errorf("session warm fields = %q / %q", sess.WarmPoolID, sess.WarmState)
	}

	rs.response = `{"token":"t","url":"u","jti":"j","expiresAt":"2026-09-16T10:00:00Z"}`
	if _, err := rs.client().CreatePreviewLink(context.Background(), "ws-1", CreatePreviewLinkRequest{
		Artifact:   &DeviceArtifact{URL: "https://x/a.apk"},
		WarmPoolID: "p1",
	}); err != nil {
		t.Fatalf("CreatePreviewLink: %v", err)
	}
	sent = nil
	if err := json.Unmarshal(rs.lastBody, &sent); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sent["warmPoolId"] != "p1" {
		t.Errorf("warmPoolId = %v, want p1", sent["warmPoolId"])
	}
	if _, ok := sent["deviceSpec"]; ok {
		t.Errorf("deviceSpec should be omitted when nil, body = %s", rs.lastBody)
	}

	if _, err := rs.client().CreatePreviewLink(context.Background(), "ws-1", CreatePreviewLinkRequest{
		DeviceSpec: &DeviceSpec{Platform: "ios"}, Artifact: &DeviceArtifact{URL: "https://x/a.zip"},
	}); err != nil {
		t.Fatalf("CreatePreviewLink: %v", err)
	}
	sent = nil
	if err := json.Unmarshal(rs.lastBody, &sent); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := sent["warmPoolId"]; ok {
		t.Errorf("warmPoolId should be omitted when unset, body = %s", rs.lastBody)
	}
}
