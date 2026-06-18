package rde

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	rdeapi "github.com/bitrise-io/bitrise-cli/bitriseapi/rde"
)

// catalogServer serves a fixed images / machine-types catalog, switching on the
// request path. The catalog mirrors the real shape: a name can appear in more
// than one cluster, and machine types are compatible with an image when they
// share a cluster.
func catalogServer(t *testing.T) *Service {
	t.Helper()
	const images = `{"images":[
		{"id":"lin-a","name":"linux","clusterName":"a"},
		{"id":"lin-b","name":"linux","clusterName":"b"},
		{"id":"mac-c","name":"mac","clusterName":"c","isDefault":true}
	]}`
	const machineTypes = `{"machineTypes":[
		{"id":"small-a","name":"small","clusterName":"a"},
		{"id":"big-b","name":"big","clusterName":"b"},
		{"id":"m2-c","name":"m2","clusterName":"c"}
	]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/machine-types"):
			_, _ = w.Write([]byte(machineTypes))
		case strings.HasSuffix(r.URL.Path, "/images"):
			_, _ = w.Write([]byte(images))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return NewService(rdeapi.New(srv.URL, "tok"))
}

func TestMachineTypesForImage_FiltersByClusterOverlap(t *testing.T) {
	svc := catalogServer(t)

	// "linux" is offered by clusters a and b, so only machine types in a or b
	// are compatible — "m2" (cluster c) must be excluded.
	got, err := svc.MachineTypesForImage(context.Background(), "ws-1", "linux")
	if err != nil {
		t.Fatalf("MachineTypesForImage: %v", err)
	}
	gotNames := map[string]bool{}
	for _, mt := range got {
		gotNames[mt.Name] = true
	}
	if !gotNames["small"] || !gotNames["big"] {
		t.Errorf("expected small+big for linux, got %+v", got)
	}
	if gotNames["m2"] {
		t.Errorf("m2 (cluster c) should not be compatible with linux, got %+v", got)
	}
}

func TestMachineTypesForImage_SingleCluster(t *testing.T) {
	svc := catalogServer(t)

	// "mac" is only in cluster c, so only "m2" is compatible.
	got, err := svc.MachineTypesForImage(context.Background(), "ws-1", "mac")
	if err != nil {
		t.Fatalf("MachineTypesForImage: %v", err)
	}
	if len(got) != 1 || got[0].Name != "m2" {
		t.Errorf("expected only m2 for mac, got %+v", got)
	}
}

func TestMachineTypesForImage_UnknownImageErrors(t *testing.T) {
	svc := catalogServer(t)

	_, err := svc.MachineTypesForImage(context.Background(), "ws-1", "windows")
	if err == nil || !strings.Contains(err.Error(), "not found in this workspace") {
		t.Errorf("err = %v, want not-found error", err)
	}
}

func TestListImages_CarriesIsDefault(t *testing.T) {
	svc := catalogServer(t)

	images, err := svc.ListImages(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("ListImages: %v", err)
	}
	var defaults []string
	for _, im := range images {
		if im.IsDefault {
			defaults = append(defaults, im.Name)
		}
	}
	if len(defaults) != 1 || defaults[0] != "mac" {
		t.Errorf("expected only mac flagged default, got %v", defaults)
	}
}
