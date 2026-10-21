package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fake struct {
	payload any
	path    string
	calls   int
	err     error
}

func (f *fake) Do(_ context.Context, _ string, path string, payload any) (json.RawMessage, error) {
	f.calls++
	f.payload = payload
	f.path = path
	if f.err != nil {
		return nil, f.err
	}
	return json.RawMessage(`{"ok":true}`), nil
}

func modernMeta(extra map[string]any) map[string]any {
	m := map[string]any{
		ProtocolVersionMetaKey:    ModernProtocol,
		ClientCapabilitiesMetaKey: map[string]any{},
		ClientInfoMetaKey:         map[string]any{"name": "test-host", "version": "1.0.0"},
	}
	for k, v := range extra {
		m[k] = v
	}
	return m
}

func params(v map[string]any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func discover() Request {
	return Request{JSONRPC: "2.0", ID: 1, Method: "server/discover", Params: params(map[string]any{"_meta": modernMeta(nil)})}
}

func call(name, actor, human string) Request {
	return Request{JSONRPC: "2.0", ID: 2, Method: "tools/call", Params: params(map[string]any{
		"name":      name,
		"arguments": map[string]any{"x": "y"},
		"_meta": modernMeta(map[string]any{
			ActorMetaKey: actor,
			HumanMetaKey: human,
		}),
	})}
}

func establishModern(t *testing.T, s *Server) {
	t.Helper()
	resp, _ := s.Handle(context.Background(), discover())
	if resp.Error != nil {
		t.Fatalf("modern discovery failed: %+v", resp.Error)
	}
}

func TestModernDiscoveryContract(t *testing.T) {
	s := &Server{}
	resp, audit := s.Handle(context.Background(), discover())
	if resp.Error != nil || audit.Outcome != "accepted" || s.Era != EraModern {
		t.Fatalf("unexpected discovery: %+v %+v", resp, audit)
	}
	m := resp.Result.(map[string]any)
	versions := m["supportedVersions"].([]string)
	if versions[0] != ModernProtocol || versions[1] != LegacyProtocol {
		t.Fatalf("unexpected versions: %#v", versions)
	}
	if m["ttlMs"] != 0 || m["cacheScope"] != "private" {
		t.Fatalf("missing conservative cache hints: %#v", m)
	}
	serverInfo := resp.Meta[ServerInfoMetaKey].(map[string]any)
	if serverInfo["name"] != "governed-mcp" {
		t.Fatalf("missing server identity: %#v", resp.Meta)
	}
}

func TestLegacyInitializeContract(t *testing.T) {
	s := &Server{}
	p := params(map[string]any{"protocolVersion": LegacyProtocol, "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "legacy", "version": "1"}})
	resp, audit := s.Handle(context.Background(), Request{JSONRPC: "2.0", ID: 1, Method: "initialize", Params: p})
	if resp.Error != nil || audit.Outcome != "accepted" || s.Era != EraLegacy {
		t.Fatalf("unexpected initialize: %+v %+v", resp, audit)
	}
	if resp.Result.(map[string]any)["protocolVersion"] != LegacyProtocol {
		t.Fatalf("unexpected initialize result: %#v", resp.Result)
	}
	if resp.Meta != nil {
		t.Fatalf("legacy response must not carry modern server meta: %#v", resp.Meta)
	}
}

func TestUnsupportedProtocolRejected(t *testing.T) {
	s := &Server{}
	bad := Request{JSONRPC: "2.0", ID: 1, Method: "server/discover", Params: params(map[string]any{"_meta": map[string]any{
		ProtocolVersionMetaKey:    "2025-06-18",
		ClientCapabilitiesMetaKey: map[string]any{},
	}})}
	resp, _ := s.Handle(context.Background(), bad)
	if resp.Error == nil || resp.Error.Code != unsupportedProtocolVersionCode {
		t.Fatalf("expected -32022, got %#v", resp.Error)
	}
}

func TestModernEnvelopeRequiredOnEveryRequest(t *testing.T) {
	s := &Server{Config: Config{Endpoint: "x"}}
	establishModern(t, s)
	resp, _ := s.Handle(context.Background(), Request{JSONRPC: "2.0", ID: 2, Method: "tools/list", Params: params(map[string]any{})})
	if resp.Error == nil {
		t.Fatal("expected modern envelope refusal")
	}
}

func TestToolsListDeterministicAndCacheable(t *testing.T) {
	s := &Server{Config: Config{Endpoint: "x", Tools: []Tool{{Name: "z", Mode: "read", Path: "z", AllowedActors: []string{"agent"}}, {Name: "a", Mode: "read", Path: "a", AllowedActors: []string{"agent"}}}}}
	establishModern(t, s)
	resp, _ := s.Handle(context.Background(), Request{JSONRPC: "2.0", ID: 2, Method: "tools/list", Params: params(map[string]any{"_meta": modernMeta(nil)})})
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	result := resp.Result.(map[string]any)
	tools := result["tools"].([]map[string]any)
	if tools[0]["name"] != "a" || tools[1]["name"] != "z" {
		t.Fatalf("tools not sorted: %#v", tools)
	}
	if result["ttlMs"] != 0 || result["cacheScope"] != "private" {
		t.Fatalf("bad cache hints: %#v", result)
	}
}

func TestProposalBoundary(t *testing.T) {
	f := &fake{}
	s := &Server{Config: Config{Endpoint: "http://local", Tools: []Tool{{Name: "update", Mode: "proposal", Path: "commands", AllowedActors: []string{"agent-1"}}}}, Client: f}
	establishModern(t, s)
	resp, audit := s.Handle(context.Background(), call("update", "agent-1", "major-1"))
	if resp.Error != nil || audit.Outcome != "accepted" {
		t.Fatalf("%+v %+v", resp, audit)
	}
	payload := f.payload.(map[string]any)
	if payload["command"] != "propose" {
		t.Fatalf("mutation bypassed proposal: %#v", payload)
	}
	if payload["human"] != "major-1" || payload["actor"] != "agent-1" {
		t.Fatalf("identity lost: %#v", payload)
	}
	result := resp.Result.(map[string]any)
	if result["resultType"] != "complete" || result["isError"] != false {
		t.Fatalf("bad MCP result: %#v", result)
	}
}

func TestReadToolDoesNotInventMutationCommand(t *testing.T) {
	f := &fake{}
	s := &Server{Config: Config{Endpoint: "x", Tools: []Tool{{Name: "read", Mode: "read", Path: "r", AllowedActors: []string{"agent"}}}}, Client: f}
	establishModern(t, s)
	resp, _ := s.Handle(context.Background(), call("read", "agent", "major"))
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	if _, exists := f.payload.(map[string]any)["command"]; exists {
		t.Fatalf("read tool gained mutation command: %#v", f.payload)
	}
}

func TestUnknownAndDisallowedRefusedWithoutDownstream(t *testing.T) {
	f := &fake{}
	s := &Server{Config: Config{Endpoint: "x", Tools: []Tool{{Name: "read", Mode: "read", Path: "r", AllowedActors: []string{"agent-a"}}}}, Client: f}
	establishModern(t, s)
	for _, req := range []Request{call("root", "agent-a", "major"), call("read", "agent-b", "major")} {
		resp, _ := s.Handle(context.Background(), req)
		if resp.Error == nil {
			t.Fatal("expected refusal")
		}
	}
	if f.calls != 0 {
		t.Fatalf("downstream called on refusal: %d", f.calls)
	}
}

func TestEmptyAllowListFailsClosed(t *testing.T) {
	f := &fake{}
	s := &Server{Config: Config{Endpoint: "x", Tools: []Tool{{Name: "read", Mode: "read", Path: "r"}}}, Client: f}
	establishModern(t, s)
	resp, _ := s.Handle(context.Background(), call("read", "agent", "major"))
	if resp.Error == nil || f.calls != 0 {
		t.Fatalf("empty allow-list failed open: error=%+v calls=%d", resp.Error, f.calls)
	}
}

func TestHumanMetadataRequired(t *testing.T) {
	f := &fake{}
	s := &Server{Config: Config{Endpoint: "x", Tools: []Tool{{Name: "read", Mode: "read", Path: "r", AllowedActors: []string{"agent"}}}}, Client: f}
	establishModern(t, s)
	resp, _ := s.Handle(context.Background(), call("read", "agent", ""))
	if resp.Error == nil {
		t.Fatal("expected named human requirement")
	}
	if f.calls != 0 {
		t.Fatal("downstream called without human attribution")
	}
}

func TestMalformedParamsRefusedWithoutDownstream(t *testing.T) {
	f := &fake{}
	s := &Server{Config: Config{Endpoint: "x", Tools: []Tool{{Name: "read", Mode: "read", Path: "r", AllowedActors: []string{"agent"}}}}, Client: f}
	establishModern(t, s)
	r := Request{JSONRPC: "2.0", ID: 2, Method: "tools/call", Params: json.RawMessage(`{"_meta":`)}
	resp, _ := s.Handle(context.Background(), r)
	if resp.Error == nil || f.calls != 0 {
		t.Fatalf("malformed params not safely refused: %+v calls=%d", resp.Error, f.calls)
	}
}

func TestDownstreamFailureIsToolError(t *testing.T) {
	f := &fake{err: errors.New("application unavailable")}
	s := &Server{Config: Config{Endpoint: "x", Tools: []Tool{{Name: "read", Mode: "read", Path: "r", AllowedActors: []string{"agent"}}}}, Client: f}
	establishModern(t, s)
	resp, audit := s.Handle(context.Background(), call("read", "agent", "major"))
	if resp.Error != nil || audit.Outcome != "failed" {
		t.Fatalf("%+v %+v", resp, audit)
	}
	if resp.Result.(map[string]any)["isError"] != true {
		t.Fatalf("expected tool error: %#v", resp.Result)
	}
}

func TestHTTPClientRejectsOversizedResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"payload":"` + strings.Repeat("x", MaxDownstreamResponseBytes) + `"}`))
	}))
	defer ts.Close()
	client := HTTPClient{Endpoint: ts.URL, Client: ts.Client()}
	if _, err := client.Do(context.Background(), http.MethodPost, "/", map[string]any{}); err == nil {
		t.Fatal("oversized downstream response accepted")
	}
}

func TestEraCannotSwitchMidConnection(t *testing.T) {
	s := &Server{}
	establishModern(t, s)
	p := params(map[string]any{"protocolVersion": LegacyProtocol})
	resp, _ := s.Handle(context.Background(), Request{JSONRPC: "2.0", ID: 3, Method: "initialize", Params: p})
	if resp.Error == nil || resp.Error.Code != unsupportedProtocolVersionCode {
		t.Fatalf("era switch should be refused: %#v", resp.Error)
	}
}

func TestValidateConfig(t *testing.T) {
	valid := Config{Endpoint: "http://127.0.0.1", Tools: []Tool{{Name: "read", Mode: "read", Path: "r", AllowedActors: []string{"agent-a"}}, {Name: "write", Mode: "proposal", Path: "w", AllowedActors: []string{"agent-a"}}}}
	if err := ValidateConfig(valid); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	bad := []Config{
		{},
		{Endpoint: "x", Tools: []Tool{{Name: "read", Mode: "mutate", Path: "r", AllowedActors: []string{"agent"}}}},
		{Endpoint: "x", Tools: []Tool{{Name: "same", Mode: "read", Path: "r", AllowedActors: []string{"agent"}}, {Name: "same", Mode: "read", Path: "r2", AllowedActors: []string{"agent"}}}},
		{Endpoint: "x", Tools: []Tool{{Name: "read", Mode: "read", Path: "r"}}},
		{Endpoint: "x", Tools: []Tool{{Name: "read", Mode: "read", Path: "r", AllowedActors: []string{"agent", "agent"}}}},
	}
	for i, c := range bad {
		if err := ValidateConfig(c); err == nil {
			t.Fatalf("bad config %d accepted", i)
		}
	}
}
