package docs

import (
	"encoding/json"
	"testing"
)

func TestOpenAPIJSONIsValid(t *testing.T) {
	var spec struct {
		OpenAPI string                    `json:"openapi"`
		Paths   map[string]map[string]any `json:"paths"`
		Info    map[string]any            `json:"info"`
		Comps   map[string]map[string]any `json:"components"`
		Sec     []map[string][]string     `json:"security"`
		Servers []map[string]string       `json:"servers"`
	}
	if err := json.Unmarshal([]byte(OpenAPIJSON), &spec); err != nil {
		t.Fatalf("OpenAPIJSON is not valid JSON: %v", err)
	}
	if spec.OpenAPI == "" {
		t.Fatal("missing openapi version")
	}
	if len(spec.Paths) == 0 {
		t.Fatal("missing paths")
	}
	if spec.Paths["/api/docs"] == nil || spec.Paths["/api/docs/openapi.json"] == nil {
		t.Fatal("docs routes must be documented")
	}
	if spec.Paths["/health"] == nil {
		t.Fatal("health route must be documented")
	}
}
