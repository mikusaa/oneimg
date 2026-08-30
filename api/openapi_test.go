package api_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestOpenAPIContract(t *testing.T) {
	raw, err := os.ReadFile("openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	if document["openapi"] != "3.1.0" {
		t.Fatalf("openapi version = %#v", document["openapi"])
	}

	paths := object(t, document["paths"], "paths")
	expected := map[string][]string{
		"/public/config":                    {"get"},
		"/public/images/random":             {"get"},
		"/auth/login":                       {"post"},
		"/auth/register":                    {"post"},
		"/auth/logout":                      {"post"},
		"/auth/passkeys/login/options":      {"post"},
		"/auth/passkeys/login/verify":       {"post"},
		"/me":                               {"get", "patch"},
		"/me/passkeys":                      {"get"},
		"/me/passkeys/registration/options": {"post"},
		"/me/passkeys/registration/verify":  {"post"},
		"/me/passkeys/{id}":                 {"patch"},
		"/me/passkeys/{id}/revoke":          {"post"},
		"/me/tokens":                        {"get", "post"},
		"/me/tokens/{id}/revoke":            {"post"},
		"/upload-options":                   {"get"},
		"/images":                           {"get", "post"},
		"/images/{id}":                      {"get", "delete"},
		"/image-imports":                    {"post"},
		"/images/{image_id}/tags/{tag_id}":  {"put", "delete"},
		"/images/tags":                      {"patch"},
		"/tags":                             {"get", "post"},
		"/tags/{id}":                        {"patch", "delete"},
		"/storage-buckets":                  {"get", "post"},
		"/storage-buckets/{id}":             {"get", "patch", "delete"},
		"/storage-connection-tests":         {"post"},
		"/stats/dashboard":                  {"get"},
		"/stats/images":                     {"get"},
		"/users":                            {"get", "post"},
		"/users/{id}":                       {"patch", "delete"},
		"/users/{id}/permissions":           {"put"},
		"/users/{id}/password-reset":        {"post"},
		"/users/{id}/passkeys/revoke":       {"post"},
		"/settings":                         {"get", "patch"},
	}
	if len(paths) != len(expected) {
		t.Fatalf("path count = %d, want %d", len(paths), len(expected))
	}

	components := object(t, document["components"], "components")
	responseComponents := object(t, components["responses"], "components.responses")
	operationIDs := map[string]string{}
	for path, methods := range expected {
		pathItem := object(t, paths[path], path)
		for _, method := range methods {
			operation := object(t, pathItem[method], method+" "+path)
			operationID, _ := operation["operationId"].(string)
			if operationID == "" {
				t.Fatalf("%s %s has no operationId", method, path)
			}
			if previous, exists := operationIDs[operationID]; exists {
				t.Fatalf("duplicate operationId %q on %s and %s %s", operationID, previous, method, path)
			}
			operationIDs[operationID] = method + " " + path
			summary, _ := operation["summary"].(string)
			if strings.TrimSpace(summary) == "" {
				t.Fatalf("%s %s has no summary", method, path)
			}

			responses := object(t, operation["responses"], method+" "+path+" responses")
			problem := object(t, responses["default"], method+" "+path+" default response")
			if problem["$ref"] != "#/components/responses/ProblemResponse" {
				t.Fatalf("%s %s default response = %#v", method, path, problem)
			}
			for status, rawResponse := range responses {
				if status == "default" {
					continue
				}
				response := object(t, rawResponse, method+" "+path+" "+status)
				ref, _ := response["$ref"].(string)
				const prefix = "#/components/responses/"
				if !strings.HasPrefix(ref, prefix) {
					t.Fatalf("%s %s response %s must use a shared response component", method, path, status)
				}
				component := object(t, responseComponents[strings.TrimPrefix(ref, prefix)], ref)
				headers := object(t, component["headers"], ref+" headers")
				if _, exists := headers["X-Request-ID"]; !exists {
					t.Fatalf("%s does not declare X-Request-ID", ref)
				}
			}
		}
	}
}

func object(t *testing.T, value any, name string) map[string]any {
	t.Helper()
	result, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s is %T, want object: %s", name, value, fmt.Sprint(value))
	}
	return result
}
