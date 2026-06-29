package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/quarkgate/quarkgate/internal/models"
)

type DriverCapabilities struct {
	ParseResponse  bool `json:"parse_response"`
	NormalizeUsage bool `json:"normalize_usage"`
	AsyncPoll      bool `json:"async_poll"`
}

type Manifest struct {
	ID           string              `json:"id"`
	Version      string              `json:"version"`
	Category     string              `json:"category"`
	Operations   []ManifestOperation `json:"operations"`
	Pricing      json.RawMessage     `json:"pricing,omitempty"`
	Capabilities DriverCapabilities  `json:"capabilities"`
}

type ManifestOperation struct {
	ID          string   `json:"id"`
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	Streaming   bool     `json:"streaming"`
	CompatPaths []string `json:"compat_paths"`
}

type HealthResult struct {
	OK        bool   `json:"ok"`
	LatencyMs int    `json:"latency_ms"`
	Message   string `json:"message"`
}

type PollResult struct {
	RawUsage map[string]interface{} `json:"raw_usage"`
	Done     bool                   `json:"done"`
}

type Registry struct {
	manifests   map[string]Manifest
	driversPath string
	nodePath    string
	pythonPath  string
}

func Load(driversPath, nodePath string) (*Registry, error) {
	r := &Registry{
		manifests:   make(map[string]Manifest),
		driversPath: driversPath,
		nodePath:    nodePath,
		pythonPath:  "python3",
	}
	entries, err := os.ReadDir(driversPath)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), "_") || e.Name() == "fixtures" || e.Name() == "sdk" {
			continue
		}
		manifestPath := filepath.Join(driversPath, e.Name(), "manifest.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		var m Manifest
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("manifest %s: %w", e.Name(), err)
		}
		if m.ID != "" && m.ID != e.Name() {
			return nil, fmt.Errorf("manifest id %s != folder %s", m.ID, e.Name())
		}
		if m.ID == "" {
			m.ID = e.Name()
		}
		r.manifests[m.ID] = m
	}
	return r, nil
}

func (r *Registry) MatchRoute(method, path string) (*models.RouteMatch, error) {
	if method == http.MethodPost && path == "/v1/chat/completions" {
		return &models.RouteMatch{Provider: "openrouter", Operation: "chat.completions.create", Compat: true}, nil
	}
	if method == http.MethodPost && path == "/v1/quarkgate" {
		return &models.RouteMatch{Provider: "", Operation: "envelope"}, nil
	}
	for id, m := range r.manifests {
		for _, op := range m.Operations {
			for _, cp := range op.CompatPaths {
				if path == cp && strings.EqualFold(method, op.Method) {
					return &models.RouteMatch{Provider: id, Operation: op.ID, Compat: true}, nil
				}
			}
			if path == "/v1/providers/"+id+op.Path && strings.EqualFold(method, op.Method) {
				return &models.RouteMatch{Provider: id, Operation: op.ID, Compat: false}, nil
			}
		}
	}
	return nil, fmt.Errorf("unknown route")
}

func (r *Registry) GetManifest(provider string) (Manifest, bool) {
	m, ok := r.manifests[provider]
	return m, ok
}

func (r *Registry) HasCapability(provider string, cap string) bool {
	m, ok := r.manifests[provider]
	if !ok {
		return false
	}
	switch cap {
	case "parse_response":
		return m.Capabilities.ParseResponse
	case "normalize_usage":
		return m.Capabilities.NormalizeUsage
	case "async_poll":
		return m.Capabilities.AsyncPoll
	}
	return false
}

func (r *Registry) DriversPath() string { return r.driversPath }
func (r *Registry) NodePath() string    { return r.nodePath }

func (r *Registry) InvokePrepare(provider string, envelope json.RawMessage, credential string, baseURL string) (*models.DownstreamRequest, error) {
	input := map[string]interface{}{
		"ipc_version": "1",
		"action":      "prepare",
		"provider":    provider,
		"envelope":    json.RawMessage(envelope),
		"credential":  credential,
		"baseURL":     baseURL,
	}
	out, err := r.invokeJSON(input)
	if err != nil {
		return nil, err
	}
	var resp models.DownstreamRequest
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (r *Registry) InvokeEstimate(provider string, envelope json.RawMessage) (int64, error) {
	input := map[string]interface{}{
		"ipc_version": "1",
		"action":      "estimate",
		"provider":    provider,
		"envelope":    json.RawMessage(envelope),
	}
	out, err := r.invokeJSON(input)
	if err != nil {
		return 0, err
	}
	var resp struct {
		EstimateMicro int64 `json:"estimate_micro"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return 0, err
	}
	return resp.EstimateMicro, nil
}

func (r *Registry) InvokeNormalize(provider string, raw map[string]interface{}, envelope json.RawMessage) (map[string]interface{}, error) {
	if !r.HasCapability(provider, "normalize_usage") {
		return raw, nil
	}
	input := map[string]interface{}{
		"ipc_version": "1",
		"action":      "normalize",
		"provider":    provider,
		"raw_usage":   raw,
		"envelope":    json.RawMessage(envelope),
	}
	out, err := r.invokeJSON(input)
	if err != nil {
		return raw, err
	}
	var resp struct {
		RawUsage map[string]interface{} `json:"raw_usage"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return raw, err
	}
	if resp.RawUsage == nil {
		return raw, nil
	}
	return mergeUsage(raw, resp.RawUsage), nil
}

func (r *Registry) InvokeParseResponse(provider string, headers map[string]string, body string, streaming bool, envelope json.RawMessage) (map[string]interface{}, error) {
	if !r.HasCapability(provider, "parse_response") {
		return nil, nil
	}
	input := map[string]interface{}{
		"ipc_version": "1",
		"action":      "parseResponse",
		"provider":    provider,
		"headers":     headers,
		"body":        body,
		"streaming":   streaming,
		"envelope":    json.RawMessage(envelope),
	}
	out, err := r.invokeJSON(input)
	if err != nil {
		return nil, err
	}
	var resp struct {
		RawUsage map[string]interface{} `json:"raw_usage"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, err
	}
	return resp.RawUsage, nil
}

func (r *Registry) InvokeHealthCheck(provider, baseURL, credential string) (*HealthResult, error) {
	input := map[string]interface{}{
		"ipc_version": "1",
		"action":      "healthCheck",
		"provider":    provider,
		"baseURL":     baseURL,
		"credential":  credential,
	}
	out, err := r.invokeJSON(input)
	if err != nil {
		return nil, err
	}
	var h HealthResult
	if err := json.Unmarshal(out, &h); err != nil {
		return nil, err
	}
	return &h, nil
}

func (r *Registry) InvokePoll(provider, baseURL, credential string, pollContext map[string]interface{}) (*PollResult, error) {
	if !r.HasCapability(provider, "async_poll") {
		return nil, fmt.Errorf("poll not supported")
	}
	input := map[string]interface{}{
		"ipc_version":  "1",
		"action":       "poll",
		"provider":     provider,
		"baseURL":      baseURL,
		"credential":   credential,
		"poll_context": pollContext,
	}
	out, err := r.invokeJSON(input)
	if err != nil {
		return nil, err
	}
	var p PollResult
	if err := json.Unmarshal(out, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *Registry) invokeJSON(input map[string]interface{}) ([]byte, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	provider, _ := input["provider"].(string)
	bin, args, err := r.hostCommand(provider)
	if err != nil {
		return nil, err
	}
	return realExecCommand(bin, args, r.driversPath, body)
}

func (r *Registry) hostCommand(provider string) (string, []string, error) {
	driverJS := filepath.Join(r.driversPath, provider, "driver.js")
	driverPY := filepath.Join(r.driversPath, provider, "driver.py")
	sdkHostJS := filepath.Join(r.driversPath, "sdk", "host.js")
	sdkHostPY := filepath.Join(r.driversPath, "sdk", "host.py")

	if _, err := os.Stat(driverJS); err == nil {
		abs, err := filepath.Abs(sdkHostJS)
		if err != nil {
			return "", nil, err
		}
		return r.nodePath, []string{abs}, nil
	}
	if _, err := os.Stat(driverPY); err == nil {
		abs, err := filepath.Abs(sdkHostPY)
		if err != nil {
			return "", nil, err
		}
		return r.pythonPath, []string{abs}, nil
	}
	return "", nil, fmt.Errorf("no driver for %s", provider)
}

func mergeUsage(base, patch map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range patch {
		out[k] = v
	}
	return out
}

func ReadEnvelope(r io.Reader) (*models.QuarkGateEnvelope, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var e models.QuarkGateEnvelope
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// AllManifests returns loaded manifests for validation tests.
func (r *Registry) AllManifests() map[string]Manifest {
	return r.manifests
}
