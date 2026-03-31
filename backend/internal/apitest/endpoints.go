// Package apitest holds shared endpoint identifiers for API tests and coverage reports.
package apitest

// EndpointID is a short id used in subtest names (e.g. api_health|GET_ok) for report aggregation.
type EndpointID string

const (
	Health        EndpointID = "api_health"
	ListLogs      EndpointID = "api_logs"
	LogContent    EndpointID = "api_logs_content"
	OpenAPISpec   EndpointID = "openapi_yaml"
	DocsHTML      EndpointID = "api_docs"
	CORSPreflight EndpointID = "cors_options"
)

// AllEndpointIDs is the ordered list of HTTP API paths we require tests for (plus CORS).
var AllEndpointIDs = []struct {
	ID   EndpointID
	Path string
}{
	{Health, "/api/health"},
	{ListLogs, "/api/logs"},
	{LogContent, "/api/logs/content"},
	{OpenAPISpec, "/openapi.yaml"},
	{DocsHTML, "/api/docs"},
	{CORSPreflight, "(CORS OPTIONS 预检，非独立 path)"},
}

// PathByID returns the documented path for an id.
func PathByID(id EndpointID) string {
	for _, e := range AllEndpointIDs {
		if e.ID == id {
			return e.Path
		}
	}
	return ""
}
