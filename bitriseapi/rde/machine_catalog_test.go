package rde

import (
	"context"
	"testing"
)

func TestListImages_PathAndParse(t *testing.T) {
	rs := newRecordingServer(t, `{"images":[{"id":"i1","name":"osx-xcode","clusterName":"c1"}]}`)

	images, err := rs.client().ListImages(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("ListImages: %v", err)
	}
	if want := "/v1/workspaces/ws-1/images"; rs.lastPath != want {
		t.Errorf("path = %s, want %s", rs.lastPath, want)
	}
	if len(images) != 1 || images[0].Name != "osx-xcode" {
		t.Errorf("images = %+v", images)
	}
}

func TestListMachineTypes_PathAndParse(t *testing.T) {
	rs := newRecordingServer(t, `{"machineTypes":[{"id":"m1","name":"g2.mac"}]}`)

	types, err := rs.client().ListMachineTypes(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("ListMachineTypes: %v", err)
	}
	if want := "/v1/workspaces/ws-1/machine-types"; rs.lastPath != want {
		t.Errorf("path = %s, want %s", rs.lastPath, want)
	}
	if len(types) != 1 || types[0].Name != "g2.mac" {
		t.Errorf("machine types = %+v", types)
	}
}

func TestMachineCatalog_ValidationGuards(t *testing.T) {
	rs := newRecordingServer(t, `{}`)
	c := rs.client()
	ctx := context.Background()

	cases := map[string]func() error{
		"Images/no-ws":       func() error { _, err := c.ListImages(ctx, ""); return err },
		"MachineTypes/no-ws": func() error { _, err := c.ListMachineTypes(ctx, ""); return err },
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
