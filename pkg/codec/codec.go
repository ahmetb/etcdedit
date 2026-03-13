package codec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/protobuf"
	"k8s.io/kubectl/pkg/scheme"
	sigsyaml "sigs.k8s.io/yaml"
)

// builtInPaths lists etcd key prefixes that use protobuf encoding.
var builtInPaths = []string{
	"/registry/pods/",
	"/registry/services/",
	"/registry/endpoints/",
	"/registry/configmaps/",
	"/registry/secrets/",
	"/registry/namespaces/",
	"/registry/nodes/",
	"/registry/events/",
	"/registry/limitranges/",
	"/registry/resourcequotas/",
	"/registry/serviceaccounts/",
	"/registry/persistentvolumes/",
	"/registry/persistentvolumeclaims/",
	"/registry/replicationcontrollers/",
	"/registry/deployments/",
	"/registry/replicasets/",
	"/registry/statefulsets/",
	"/registry/daemonsets/",
	"/registry/jobs/",
	"/registry/cronjobs/",
	"/registry/roles/",
	"/registry/rolebindings/",
	"/registry/clusterroles/",
	"/registry/clusterrolebindings/",
	"/registry/storageclasses/",
	"/registry/csistoragecapacities/",
	"/registry/csidrivers/",
	"/registry/csinodes/",
	"/registry/volumeattachments/",
	"/registry/leases/",
	"/registry/priorityclasses/",
	"/registry/runtimeclasses/",
	"/registry/networkpolicies/",
	"/registry/ingresses/",
	"/registry/ingressclasses/",
	"/registry/endpointslices/",
	"/registry/flowschemas/",
	"/registry/prioritylevelconfigurations/",
	"/registry/mutatingwebhookconfigurations/",
	"/registry/validatingwebhookconfigurations/",
	"/registry/customresourcedefinitions/",
	"/registry/controllerrevisions/",
	"/registry/mutatingadmissionpolicies/",
	"/registry/mutatingadmissionpolicybindings/",
	"/registry/validatingadmissionpolicies/",
	"/registry/validatingadmissionpolicybindings/",
	"/registry/clustertrustbundles/",
	"/registry/podcertificaterequests/",
	"/registry/leasecandidates/",
	"/registry/servicecidrs/",
	"/registry/ipaddresses/",
	"/registry/workloads/",
	"/registry/horizontalpodautoscalers/",
	"/registry/storageversions/",
	"/registry/selfsubjectreviews/",
	"/registry/selfsubjectaccessreviews/",
	"/registry/selfsubjectrulesreviews/",
	"/registry/subjectaccessreviews/",
	"/registry/localsubjectaccessreviews/",
	"/registry/tokenreviews/",
	"/registry/poddisruptionbudgets/",
	"/registry/podtemplates/",
	"/registry/componentstatuses/",
	"/registry/rangeallocations/",
	"/registry/devicelclasses/",
	"/registry/devicetaintrules/",
	"/registry/resourceclaims/",
	"/registry/resourceclaimtemplates/",
	"/registry/resourceslices/",
	"/registry/csioperatorstates/",
	"/registry/volumeattributesclasses/",
	"/registry/storagemigrations/",
}

// k8s protobuf magic prefix
var k8sMagic = []byte("k8s\x00")

// IsBuiltIn returns true if the key path matches a known built-in Kubernetes resource path.
func IsBuiltIn(keyPath string) bool {
	for _, prefix := range builtInPaths {
		if strings.HasPrefix(keyPath, prefix) {
			return true
		}
	}
	// Also match exact paths without trailing content (e.g., "/registry/namespaces" for cluster-scoped listing)
	return false
}

// DecodeResult holds the decoded data along with encoding metadata.
type DecodeResult struct {
	Data       map[string]interface{}
	IsProtobuf bool
	GVK        *schema.GroupVersionKind
}

// Decode detects the encoding of etcd value bytes and decodes to an unstructured map.
// Built-in key paths are decoded as protobuf. All other paths are decoded as JSON.
func Decode(keyPath string, data []byte) (*DecodeResult, error) {
	if IsBuiltIn(keyPath) {
		return decodeProtobuf(data)
	}
	return decodeJSON(data)
}

// Encode converts an unstructured map back to bytes using the specified encoding.
func Encode(keyPath string, data map[string]interface{}, asProtobuf bool) ([]byte, error) {
	if asProtobuf {
		return encodeProtobuf(data)
	}
	return encodeJSON(data)
}

// ToYAML converts an unstructured map to YAML bytes.
func ToYAML(data map[string]interface{}) ([]byte, error) {
	return sigsyaml.Marshal(data)
}

// FromYAML parses YAML bytes into an unstructured map.
func FromYAML(yamlBytes []byte) (map[string]interface{}, error) {
	var obj map[string]interface{}
	if err := sigsyaml.Unmarshal(yamlBytes, &obj); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	return obj, nil
}

// ToJSON converts an unstructured map to indented JSON bytes.
func ToJSON(data map[string]interface{}) ([]byte, error) {
	return json.MarshalIndent(data, "", "  ")
}

func decodeProtobuf(data []byte) (*DecodeResult, error) {
	if len(data) < 4 || !bytes.HasPrefix(data, k8sMagic) {
		return nil, fmt.Errorf("data does not have k8s protobuf magic prefix")
	}

	codec := protobuf.NewSerializer(scheme.Scheme, scheme.Scheme)

	// First decode to Unknown to get GVK
	obj := &runtime.Unknown{}
	_, gvk, err := codec.Decode(data, nil, obj)
	if err != nil {
		return nil, fmt.Errorf("protobuf decode to Unknown: %w", err)
	}

	// Then decode to the concrete typed object
	intoObj, err := scheme.Scheme.New(*gvk)
	if err != nil {
		return nil, fmt.Errorf("no registered type for %s: %w", gvk, err)
	}

	_, _, err = codec.Decode(data, nil, intoObj)
	if err != nil {
		return nil, fmt.Errorf("protobuf decode to typed object: %w", err)
	}

	// Convert typed object to unstructured map
	unstrMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(intoObj)
	if err != nil {
		return nil, fmt.Errorf("convert to unstructured: %w", err)
	}

	return &DecodeResult{
		Data:       unstrMap,
		IsProtobuf: true,
		GVK:        gvk,
	}, nil
}

func decodeJSON(data []byte) (*DecodeResult, error) {
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("JSON decode: %w", err)
	}
	return &DecodeResult{
		Data:       obj,
		IsProtobuf: false,
	}, nil
}

func encodeProtobuf(data map[string]interface{}) ([]byte, error) {
	// Extract apiVersion and kind to determine GVK
	apiVersion, _ := data["apiVersion"].(string)
	kind, _ := data["kind"].(string)
	if apiVersion == "" || kind == "" {
		return nil, fmt.Errorf("manifest must have apiVersion and kind for protobuf encoding")
	}

	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid apiVersion %q: %w", apiVersion, err)
	}
	gvk := gv.WithKind(kind)

	// Create a new typed object for this GVK
	typedObj, err := scheme.Scheme.New(gvk)
	if err != nil {
		return nil, fmt.Errorf("no registered type for %s: %w", gvk, err)
	}

	// Convert unstructured map to typed object
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(data, typedObj); err != nil {
		return nil, fmt.Errorf("convert from unstructured: %w", err)
	}

	// Encode to protobuf
	codec := protobuf.NewSerializer(scheme.Scheme, scheme.Scheme)
	buf := &bytes.Buffer{}
	if err := codec.Encode(typedObj, buf); err != nil {
		return nil, fmt.Errorf("protobuf encode: %w", err)
	}

	return buf.Bytes(), nil
}

func encodeJSON(data map[string]interface{}) ([]byte, error) {
	return json.Marshal(data)
}

// UnstructuredToYAML is a convenience to decode etcd bytes and return YAML.
func UnstructuredToYAML(keyPath string, data []byte) ([]byte, *DecodeResult, error) {
	result, err := Decode(keyPath, data)
	if err != nil {
		return nil, nil, err
	}
	yamlBytes, err := ToYAML(result.Data)
	if err != nil {
		return nil, nil, err
	}
	return yamlBytes, result, nil
}

// YAMLToEncoded parses YAML and encodes back to the original format.
func YAMLToEncoded(keyPath string, yamlBytes []byte, asProtobuf bool) ([]byte, error) {
	data, err := FromYAML(yamlBytes)
	if err != nil {
		return nil, err
	}
	return Encode(keyPath, data, asProtobuf)
}

// GetUID extracts metadata.uid from an unstructured map, returns empty string if not present.
func GetUID(data map[string]interface{}) string {
	metadata, ok := data["metadata"].(map[string]interface{})
	if !ok {
		return ""
	}
	uid, _ := metadata["uid"].(string)
	return uid
}

// SetUID sets metadata.uid in an unstructured map.
func SetUID(data map[string]interface{}, uid string) {
	metadata, ok := data["metadata"].(map[string]interface{})
	if !ok {
		metadata = make(map[string]interface{})
		data["metadata"] = metadata
	}
	metadata["uid"] = uid
}

// RemoveUID removes metadata.uid from an unstructured map.
func RemoveUID(data map[string]interface{}) {
	metadata, ok := data["metadata"].(map[string]interface{})
	if !ok {
		return
	}
	delete(metadata, "uid")
}

// UnstructuredFromYAMLFile reads YAML and returns the unstructured map.
// This is used by the apply command.
func UnstructuredFromYAMLFile(yamlBytes []byte) (map[string]interface{}, error) {
	return FromYAML(yamlBytes)
}

// EncodeForKey encodes data in the appropriate format for the given key path.
// Built-in paths use protobuf, others use JSON.
func EncodeForKey(keyPath string, data map[string]interface{}) ([]byte, error) {
	if IsBuiltIn(keyPath) {
		return encodeProtobuf(data)
	}
	return encodeJSON(data)
}

// ConvertToUnstructuredObject wraps a map in an Unstructured object.
func ConvertToUnstructuredObject(data map[string]interface{}) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: data}
}

// GetName extracts metadata.name from an unstructured map.
func GetName(data map[string]interface{}) string {
	metadata, ok := data["metadata"].(map[string]interface{})
	if !ok {
		return ""
	}
	name, _ := metadata["name"].(string)
	return name
}

// GetNamespace extracts metadata.namespace from an unstructured map.
func GetNamespace(data map[string]interface{}) string {
	metadata, ok := data["metadata"].(map[string]interface{})
	if !ok {
		return ""
	}
	ns, _ := metadata["namespace"].(string)
	return ns
}

// clusterScopedBuiltIns are built-in resources stored without a namespace segment.
// Key format: /registry/<resource>/<name>
var clusterScopedBuiltIns = map[string]bool{
	"namespaces":                      true,
	"nodes":                           true,
	"persistentvolumes":               true,
	"clusterroles":                    true,
	"clusterrolebindings":             true,
	"storageclasses":                  true,
	"priorityclasses":                 true,
	"runtimeclasses":                  true,
	"ingressclasses":                  true,
	"flowschemas":                     true,
	"prioritylevelconfigurations":     true,
	"mutatingwebhookconfigurations":   true,
	"validatingwebhookconfigurations": true,
	"customresourcedefinitions":       true,
	"csidrivers":                      true,
	"csinodes":                        true,
	"volumeattachments":               true,
	"csistoragecapacities":            true,
}

// NameFromKey extracts the resource name from an etcd key path.
// The last segment of the key is always the resource name.
// e.g. "/registry/pods/default/nginx" -> "nginx"
func NameFromKey(keyPath string) string {
	keyPath = strings.TrimSuffix(keyPath, "/")
	if i := strings.LastIndex(keyPath, "/"); i >= 0 {
		return keyPath[i+1:]
	}
	return keyPath
}

// NamespaceFromKey attempts to extract the namespace from an etcd key path.
// For built-in namespaced resources: /registry/<resource>/<namespace>/<name> -> namespace
// For cluster-scoped resources or ambiguous keys, returns "".
func NamespaceFromKey(keyPath string) string {
	keyPath = strings.TrimSuffix(keyPath, "/")
	parts := strings.Split(strings.TrimPrefix(keyPath, "/"), "/")
	// parts[0]="registry", parts[1]=resource, parts[2]=namespace?, parts[3]=name?

	if len(parts) < 4 {
		return "" // too short to have a namespace
	}

	resource := parts[1]

	// For built-in resources, check if cluster-scoped
	if isKnownBuiltIn(resource) {
		if clusterScopedBuiltIns[resource] {
			return "" // cluster-scoped: /registry/<resource>/<name>
		}
		// Namespaced built-in: /registry/<resource>/<namespace>/<name>
		if len(parts) >= 4 {
			return parts[2]
		}
		return ""
	}

	// For CRDs: /registry/<group>/<resource>/<namespace>/<name>
	// 5+ parts with a dot in the group segment suggests namespaced CRD
	if len(parts) >= 5 && strings.Contains(parts[1], ".") {
		return parts[3]
	}

	return ""
}

// isKnownBuiltIn checks if a resource name matches a known built-in resource.
func isKnownBuiltIn(resource string) bool {
	for _, prefix := range builtInPaths {
		// Extract resource name from prefix like "/registry/pods/"
		trimmed := strings.TrimPrefix(prefix, "/registry/")
		trimmed = strings.TrimSuffix(trimmed, "/")
		if trimmed == resource {
			return true
		}
	}
	return false
}

// SetName sets metadata.name in an unstructured map.
func SetName(data map[string]interface{}, name string) {
	metadata, ok := data["metadata"].(map[string]interface{})
	if !ok {
		metadata = make(map[string]interface{})
		data["metadata"] = metadata
	}
	metadata["name"] = name
}

// HasMultipleDocuments checks if YAML bytes contain more than one document.
func HasMultipleDocuments(yamlBytes []byte) bool {
	docs := bytes.Split(yamlBytes, []byte("\n---"))
	count := 0
	for _, doc := range docs {
		if len(bytes.TrimSpace(doc)) > 0 {
			count++
		}
		if count > 1 {
			return true
		}
	}
	return false
}

// SetNamespace sets metadata.namespace in an unstructured map.
func SetNamespace(data map[string]interface{}, namespace string) {
	metadata, ok := data["metadata"].(map[string]interface{})
	if !ok {
		metadata = make(map[string]interface{})
		data["metadata"] = metadata
	}
	metadata["namespace"] = namespace
}
