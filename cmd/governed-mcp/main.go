package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/jonathanselby/governed-mcp/internal/mcp"
	"os"
)

const version = "0.1.0"

func main() {
	var cfg string
	var v bool
	flag.StringVar(&cfg, "config", "", "config JSON")
	flag.BoolVar(&v, "version", false, "print version")
	flag.Parse()
	if v {
		fmt.Println("governed-mcp " + version)
		return
	}
	if cfg == "" {
		flag.Usage()
		os.Exit(2)
	}
	var c mcp.Config
	b, e := os.ReadFile(cfg)
	if e != nil || json.Unmarshal(b, &c) != nil || mcp.ValidateConfig(c) != nil {
		fmt.Fprintln(os.Stderr, "invalid config")
		os.Exit(3)
	}
	s := mcp.Server{Config: c, Client: mcp.HTTPClient{Endpoint: c.Endpoint, Bearer: os.Getenv("GOVERNED_MCP_BEARER")}}
	sc := bufio.NewScanner(os.Stdin)
	enc := json.NewEncoder(os.Stdout)
	audit := json.NewEncoder(os.Stderr)
	for sc.Scan() {
		var r mcp.Request
		if e := json.Unmarshal(sc.Bytes(), &r); e != nil {
			enc.Encode(mcp.Response{JSONRPC: "2.0", Error: &mcp.RPCError{Code: -32700, Message: "parse error"}})
			audit.Encode(mcp.Audit{Outcome: "rejected", Detail: e.Error()})
			continue
		}
		resp, a := s.Handle(context.Background(), r)
		enc.Encode(resp)
		audit.Encode(a)
	}
	if e := sc.Err(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(3)
	}
}
