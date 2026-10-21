package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	ModernProtocol = "2026-07-28"
	LegacyProtocol = "2025-11-25"

	ProtocolVersionMetaKey    = "io.modelcontextprotocol/protocolVersion"
	ClientInfoMetaKey         = "io.modelcontextprotocol/clientInfo"
	ClientCapabilitiesMetaKey = "io.modelcontextprotocol/clientCapabilities"
	ServerInfoMetaKey         = "io.modelcontextprotocol/serverInfo"

	ActorMetaKey = "jonno.garage/actor"
	HumanMetaKey = "jonno.garage/human"

	EraModern = "modern"
	EraLegacy = "legacy"

	MaxDownstreamResponseBytes = 1 << 20
)

const unsupportedProtocolVersionCode = -32022

type Tool struct {
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	Mode          string   `json:"mode"`
	Path          string   `json:"path"`
	AllowedActors []string `json:"allowed_actors,omitempty"`
}

type Config struct {
	Endpoint string `json:"endpoint"`
	Tools    []Tool `json:"tools"`
}

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      any            `json:"id,omitempty"`
	Result  any            `json:"result,omitempty"`
	Error   *RPCError      `json:"error,omitempty"`
	Meta    map[string]any `json:"_meta,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type CallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
	Meta      map[string]any `json:"_meta,omitempty"`
}

type Audit struct {
	Time     time.Time `json:"time"`
	Method   string    `json:"method,omitempty"`
	Tool     string    `json:"tool,omitempty"`
	Actor    string    `json:"actor,omitempty"`
	Human    string    `json:"human,omitempty"`
	Protocol string    `json:"protocol,omitempty"`
	Era      string    `json:"era,omitempty"`
	Outcome  string    `json:"outcome"`
	Detail   string    `json:"detail,omitempty"`
}

type CommandClient interface {
	Do(context.Context, string, string, any) (json.RawMessage, error)
}

type HTTPClient struct {
	Endpoint string
	Client   *http.Client
	Bearer   string
}

func (h HTTPClient) Do(ctx context.Context, method, path string, payload any) (json.RawMessage, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	u := strings.TrimRight(h.Endpoint, "/") + "/" + strings.TrimLeft(path, "/")
	req, err := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if h.Bearer != "" {
		req.Header.Set("Authorization", "Bearer "+h.Bearer)
	}
	client := h.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	rb, err := io.ReadAll(io.LimitReader(resp.Body, MaxDownstreamResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(rb) > MaxDownstreamResponseBytes {
		return nil, fmt.Errorf("downstream response exceeds %d bytes", MaxDownstreamResponseBytes)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("downstream status %d", resp.StatusCode)
	}
	return json.RawMessage(rb), nil
}

type Server struct {
	Config Config
	Client CommandClient
	Now    func() time.Time
	Era    string
}

func (s *Server) Handle(ctx context.Context, r Request) (Response, Audit) {
	a := Audit{Outcome: "rejected", Method: r.Method, Era: s.Era}
	if s.Now != nil {
		a.Time = s.Now()
	} else {
		a.Time = time.Now().UTC()
	}
	resp := Response{JSONRPC: "2.0", ID: r.ID}
	if r.JSONRPC != "2.0" {
		resp.Error = &RPCError{Code: -32600, Message: "invalid JSON-RPC version"}
		return resp, a
	}

	switch r.Method {
	case "server/discover":
		meta, err := requestMeta(r.Params)
		if err != nil {
			resp.Error = &RPCError{Code: -32602, Message: "invalid modern request metadata", Data: err.Error()}
			return resp, a
		}
		version, err := validateModernEnvelope(meta)
		if err != nil {
			resp.Error = protocolVersionError(version)
			return resp, a
		}
		if s.Era == EraLegacy {
			resp.Error = protocolVersionError(ModernProtocol)
			return resp, a
		}
		s.Era = EraModern
		a.Era = EraModern
		a.Protocol = ModernProtocol
		resp.Result = map[string]any{
			"supportedVersions": []string{ModernProtocol, LegacyProtocol},
			"capabilities":      map[string]any{"tools": map[string]any{}},
			"instructions":      "Governed tools only. State-changing tools are translated into proposals; identity is supplied by the client host in namespaced request metadata.",
			"ttlMs":             0,
			"cacheScope":        "private",
			"resultType":        "complete",
		}
		stampModern(&resp)
		a.Outcome = "accepted"
		return resp, a

	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		if err := json.Unmarshal(r.Params, &p); err != nil || p.ProtocolVersion == "" {
			resp.Error = &RPCError{Code: -32602, Message: "invalid initialize params"}
			return resp, a
		}
		if p.ProtocolVersion != LegacyProtocol || s.Era == EraModern {
			resp.Error = protocolVersionError(p.ProtocolVersion)
			return resp, a
		}
		s.Era = EraLegacy
		a.Era = EraLegacy
		a.Protocol = LegacyProtocol
		resp.Result = map[string]any{
			"protocolVersion": LegacyProtocol,
			"serverInfo":      map[string]any{"name": "governed-mcp", "version": "0.1.0"},
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"instructions":    "State-changing tools are proposals. The client host must attach the named actor and human in request _meta.",
		}
		a.Outcome = "accepted"
		return resp, a

	case "tools/list":
		if err := s.validateEstablishedRequest(r.Params, &a); err != nil {
			resp.Error = err
			return resp, a
		}
		tools := append([]Tool(nil), s.Config.Tools...)
		sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
		listed := make([]map[string]any, 0, len(tools))
		for _, t := range tools {
			listed = append(listed, map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"inputSchema": map[string]any{"type": "object"},
			})
		}
		result := map[string]any{"tools": listed}
		if s.Era == EraModern {
			result["ttlMs"] = 0
			result["cacheScope"] = "private"
			result["resultType"] = "complete"
			stampModern(&resp)
		}
		resp.Result = result
		a.Outcome = "accepted"
		return resp, a

	case "tools/call":
		if err := s.validateEstablishedRequest(r.Params, &a); err != nil {
			resp.Error = err
			return resp, a
		}
		var p CallParams
		if err := json.Unmarshal(r.Params, &p); err != nil || p.Name == "" {
			resp.Error = &RPCError{Code: -32602, Message: "invalid params"}
			if err != nil {
				a.Detail = err.Error()
			}
			return resp, a
		}
		a.Tool = p.Name
		a.Actor, _ = p.Meta[ActorMetaKey].(string)
		a.Human, _ = p.Meta[HumanMetaKey].(string)
		if a.Actor == "" || a.Human == "" {
			resp.Error = &RPCError{Code: -32602, Message: "client host must provide actor and named human in request _meta"}
			return resp, a
		}
		t, ok := find(s.Config.Tools, p.Name)
		if !ok || !allowed(t.AllowedActors, a.Actor) {
			resp.Error = &RPCError{Code: -32001, Message: "tool not allowed"}
			return resp, a
		}
		payload := map[string]any{
			"tool":      p.Name,
			"arguments": p.Arguments,
			"actor":     a.Actor,
			"human":     a.Human,
		}
		if t.Mode == "proposal" {
			payload["command"] = "propose"
		} else if t.Mode != "read" {
			resp.Error = &RPCError{Code: -32602, Message: "tool mode must be read or proposal"}
			return resp, a
		}
		if s.Client == nil {
			resp.Error = &RPCError{Code: -32603, Message: "downstream client unavailable"}
			return resp, a
		}
		raw, err := s.Client.Do(ctx, http.MethodPost, t.Path, payload)
		if err != nil {
			a.Detail = err.Error()
			resp.Result = callToolResult(nil, err)
			if s.Era == EraModern {
				stampModern(&resp)
			}
			a.Outcome = "failed"
			return resp, a
		}
		var structured any
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &structured); err != nil {
				structured = string(raw)
			}
		}
		resp.Result = callToolResult(structured, nil)
		if s.Era == EraModern {
			stampModern(&resp)
		}
		a.Outcome = "accepted"
		return resp, a

	default:
		resp.Error = &RPCError{Code: -32601, Message: "method not found"}
		return resp, a
	}
}

func (s *Server) validateEstablishedRequest(raw json.RawMessage, audit *Audit) *RPCError {
	if s.Era == "" {
		return &RPCError{Code: -32600, Message: "protocol era not established; call server/discover or initialize first"}
	}
	audit.Era = s.Era
	if s.Era == EraLegacy {
		audit.Protocol = LegacyProtocol
		return nil
	}
	meta, err := requestMeta(raw)
	if err != nil {
		return &RPCError{Code: -32602, Message: "invalid modern request metadata", Data: err.Error()}
	}
	version, err := validateModernEnvelope(meta)
	if err != nil {
		return protocolVersionError(version)
	}
	audit.Protocol = version
	return nil
}

func requestMeta(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 {
		return nil, errors.New("params._meta is required")
	}
	var p map[string]any
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	meta, ok := p["_meta"].(map[string]any)
	if !ok {
		return nil, errors.New("params._meta is required")
	}
	return meta, nil
}

func validateModernEnvelope(meta map[string]any) (string, error) {
	version, _ := meta[ProtocolVersionMetaKey].(string)
	if version != ModernProtocol {
		return version, errors.New("unsupported protocol version")
	}
	caps, ok := meta[ClientCapabilitiesMetaKey].(map[string]any)
	if !ok || caps == nil {
		return version, errors.New("clientCapabilities must be an object")
	}
	if info, exists := meta[ClientInfoMetaKey]; exists {
		m, ok := info.(map[string]any)
		if !ok {
			return version, errors.New("clientInfo must be an object when present")
		}
		if name, ok := m["name"].(string); !ok || name == "" {
			return version, errors.New("clientInfo.name must be a non-empty string when clientInfo is present")
		}
		if v, ok := m["version"].(string); !ok || v == "" {
			return version, errors.New("clientInfo.version must be a non-empty string when clientInfo is present")
		}
	}
	return version, nil
}

func protocolVersionError(requested string) *RPCError {
	if requested == "" {
		requested = "missing"
	}
	return &RPCError{
		Code:    unsupportedProtocolVersionCode,
		Message: "unsupported protocol version",
		Data: map[string]any{
			"supported": []string{ModernProtocol, LegacyProtocol},
			"requested": requested,
		},
	}
}

func stampModern(resp *Response) {
	resp.Meta = map[string]any{
		ServerInfoMetaKey: map[string]any{
			"name":    "governed-mcp",
			"version": "0.1.0",
		},
	}
}

func callToolResult(v any, err error) map[string]any {
	text := ""
	isError := false
	if err != nil {
		text = err.Error()
		isError = true
	} else if b, e := json.Marshal(v); e == nil {
		text = string(b)
	}
	return map[string]any{
		"content":           []map[string]any{{"type": "text", "text": text}},
		"structuredContent": v,
		"isError":           isError,
		"resultType":        "complete",
	}
}

func find(xs []Tool, name string) (Tool, bool) {
	for _, t := range xs {
		if t.Name == name {
			return t, true
		}
	}
	return Tool{}, false
}

func allowed(xs []string, actor string) bool {
	if len(xs) == 0 {
		return false
	}
	for _, x := range xs {
		if x == actor {
			return true
		}
	}
	return false
}

var ErrBadConfig = errors.New("invalid config")

func ValidateConfig(c Config) error {
	if c.Endpoint == "" {
		return ErrBadConfig
	}
	seen := map[string]bool{}
	for _, t := range c.Tools {
		if t.Name == "" || t.Path == "" || (t.Mode != "read" && t.Mode != "proposal") || seen[t.Name] || len(t.AllowedActors) == 0 {
			return ErrBadConfig
		}
		actors := map[string]bool{}
		for _, actor := range t.AllowedActors {
			if strings.TrimSpace(actor) == "" || actors[actor] {
				return ErrBadConfig
			}
			actors[actor] = true
		}
		seen[t.Name] = true
	}
	return nil
}
