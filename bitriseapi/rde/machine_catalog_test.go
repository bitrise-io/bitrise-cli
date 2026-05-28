package rde

import (
	"context"
	"encoding/json"
	"net/http"
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

func TestResolveClusters_BodyPathAndParse(t *testing.T) {
	rs := newRecordingServer(t, `{"clusters":[{"clusterName":"c1","imageId":"i","machineTypeId":"m"}]}`)

	clusters, err := rs.client().ResolveClusters(context.Background(), "ws-1", ResolveClustersRequest{
		Image:       "osx-xcode-edge",
		MachineType: "g2.mac",
	})
	if err != nil {
		t.Fatalf("ResolveClusters: %v", err)
	}
	if rs.lastMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", rs.lastMethod)
	}
	if want := "/v1/workspaces/ws-1/resolve-clusters"; rs.lastPath != want {
		t.Errorf("path = %s, want %s", rs.lastPath, want)
	}
	var sent ResolveClustersRequest
	if err := json.Unmarshal(rs.lastBody, &sent); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sent.Image != "osx-xcode-edge" || sent.MachineType != "g2.mac" {
		t.Errorf("sent = %+v", sent)
	}
	if len(clusters) != 1 || clusters[0].ClusterName != "c1" {
		t.Errorf("clusters = %+v", clusters)
	}
}

func TestMachineCatalog_ValidationGuards(t *testing.T) {
	rs := newRecordingServer(t, `{}`)
	c := rs.client()
	ctx := context.Background()

	cases := map[string]func() error{
		"Images/no-ws":       func() error { _, err := c.ListImages(ctx, ""); return err },
		"MachineTypes/no-ws": func() error { _, err := c.ListMachineTypes(ctx, ""); return err },
		"Resolve/no-ws": func() error {
			_, err := c.ResolveClusters(ctx, "", ResolveClustersRequest{Image: "i", MachineType: "m"})
			return err
		},
		"Resolve/no-image": func() error {
			_, err := c.ResolveClusters(ctx, "ws", ResolveClustersRequest{MachineType: "m"})
			return err
		},
		"Resolve/no-machinetype": func() error { _, err := c.ResolveClusters(ctx, "ws", ResolveClustersRequest{Image: "i"}); return err },
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
