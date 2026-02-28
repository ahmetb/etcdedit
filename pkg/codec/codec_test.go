package codec

import (
	"encoding/json"
	"testing"
)

func TestIsBuiltIn(t *testing.T) {
	tests := []struct {
		keyPath string
		want    bool
	}{
		{"/registry/pods/default/nginx", true},
		{"/registry/services/kube-system/kube-dns", true},
		{"/registry/namespaces/default", true},
		{"/registry/nodes/worker-1", true},
		{"/registry/deployments/default/my-deploy", true},
		{"/registry/configmaps/default/my-cm", true},
		{"/registry/clusterroles/admin", true},
		{"/registry/customresourcedefinitions/foo", true},
		// CRD paths
		{"/registry/crontabs.stable.example.com/default/my-cron", false},
		{"/registry/clusters.cluster.x-k8s.io/default/my-cluster", false},
		// Edge cases
		{"", false},
		{"/registry/", false},
		{"/registry/unknown/", false},
	}

	for _, tt := range tests {
		t.Run(tt.keyPath, func(t *testing.T) {
			got := IsBuiltIn(tt.keyPath)
			if got != tt.want {
				t.Errorf("IsBuiltIn(%q) = %v, want %v", tt.keyPath, got, tt.want)
			}
		})
	}
}

func TestJSONRoundtrip(t *testing.T) {
	original := map[string]interface{}{
		"apiVersion": "stable.example.com/v1",
		"kind":       "CronTab",
		"metadata": map[string]interface{}{
			"name":      "my-cron",
			"namespace": "default",
		},
		"spec": map[string]interface{}{
			"cronSpec": "* * * * */5",
			"image":    "my-image",
		},
	}

	// Encode to JSON
	encoded, err := encodeJSON(original)
	if err != nil {
		t.Fatalf("encodeJSON: %v", err)
	}

	// Decode back
	result, err := decodeJSON(encoded)
	if err != nil {
		t.Fatalf("decodeJSON: %v", err)
	}

	if result.IsProtobuf {
		t.Error("expected IsProtobuf=false for JSON decoded data")
	}

	// Verify key fields survived roundtrip
	if result.Data["apiVersion"] != "stable.example.com/v1" {
		t.Errorf("apiVersion mismatch: got %v", result.Data["apiVersion"])
	}
	if result.Data["kind"] != "CronTab" {
		t.Errorf("kind mismatch: got %v", result.Data["kind"])
	}
}

func TestYAMLRoundtrip(t *testing.T) {
	original := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      "test",
			"namespace": "default",
		},
		"data": map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		},
	}

	yamlBytes, err := ToYAML(original)
	if err != nil {
		t.Fatalf("ToYAML: %v", err)
	}

	roundtripped, err := FromYAML(yamlBytes)
	if err != nil {
		t.Fatalf("FromYAML: %v", err)
	}

	// Verify fields
	if roundtripped["apiVersion"] != "v1" {
		t.Errorf("apiVersion mismatch: got %v", roundtripped["apiVersion"])
	}
	data, ok := roundtripped["data"].(map[string]interface{})
	if !ok {
		t.Fatal("data field not a map")
	}
	if data["key1"] != "value1" {
		t.Errorf("data.key1 mismatch: got %v", data["key1"])
	}
}

func TestDecodeNonBuiltInJSON(t *testing.T) {
	crd := map[string]interface{}{
		"apiVersion": "stable.example.com/v1",
		"kind":       "CronTab",
		"metadata": map[string]interface{}{
			"name": "my-cron",
		},
	}

	data, err := json.Marshal(crd)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	result, err := Decode("/registry/crontabs.stable.example.com/default/my-cron", data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if result.IsProtobuf {
		t.Error("expected IsProtobuf=false for CRD key")
	}
	if result.Data["kind"] != "CronTab" {
		t.Errorf("kind mismatch: got %v", result.Data["kind"])
	}
}

func TestGetSetRemoveUID(t *testing.T) {
	data := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "test",
			"uid":  "abc-123",
		},
	}

	uid := GetUID(data)
	if uid != "abc-123" {
		t.Errorf("GetUID: got %q, want %q", uid, "abc-123")
	}

	SetUID(data, "new-uid-456")
	uid = GetUID(data)
	if uid != "new-uid-456" {
		t.Errorf("SetUID: got %q, want %q", uid, "new-uid-456")
	}

	RemoveUID(data)
	uid = GetUID(data)
	if uid != "" {
		t.Errorf("RemoveUID: got %q, want empty", uid)
	}
}

func TestGetUIDNoMetadata(t *testing.T) {
	data := map[string]interface{}{
		"kind": "Test",
	}
	uid := GetUID(data)
	if uid != "" {
		t.Errorf("expected empty UID for data without metadata, got %q", uid)
	}
}

func TestFromYAMLInvalid(t *testing.T) {
	_, err := FromYAML([]byte("not: valid: yaml: ["))
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestEncodeForKey(t *testing.T) {
	// CRD path should use JSON
	data := map[string]interface{}{
		"apiVersion": "stable.example.com/v1",
		"kind":       "CronTab",
		"metadata": map[string]interface{}{
			"name": "test",
		},
	}

	encoded, err := EncodeForKey("/registry/crontabs.stable.example.com/default/test", data)
	if err != nil {
		t.Fatalf("EncodeForKey (CRD): %v", err)
	}

	// Should be valid JSON
	var decoded map[string]interface{}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("expected JSON output for CRD key, got: %v", err)
	}
}
