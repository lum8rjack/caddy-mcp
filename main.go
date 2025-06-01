package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	_ "github.com/caddyserver/caddy/v2/modules/standard"
)

const toolInstructions = `This server is a tool for managing a caddy server instance. It should be used to get the current caddy configuration in JSON format and describe the configuration in a human readable format.
It can also be used to update the caddy configuration in JSON format using the update_caddy_config tool.

Best Practices:
1. ALWAYS provide the full JSON configuration to the update_caddy_config tool.
2. If the user asks to add a new section to the caddy configuration, you should first get the current caddy configuration using the get_caddy_config tool and then add the new section to the configuration before using the update_caddy_config tool.
`

var (
	client     http.Client
	defaultURL = "http://127.0.0.1:2019"
	transport  = "stdio"
	port       = 7000
)

type caddyError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
}

func main() {
	flag.StringVar(&defaultURL, "url", defaultURL, "The URL of the caddy server")
	flag.StringVar(&transport, "transport", transport, "The transport to use for the MCP server (stdio, sse, httpstream)")
	flag.IntVar(&port, "port", port, "Port to run the MCP server on")
	flag.Parse()

	if port <= 0 || port > 65535 {
		log.Fatal("Invalid port number.")
	}

	// Create MCP server
	s := server.NewMCPServer(
		"caddy-mcp",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithInstructions(toolInstructions),
	)

	// Create http client
	client = http.Client{
		Timeout: 10 * time.Second,
	}

	getCaddyConfig := mcp.NewTool("get_caddy_config",
		mcp.WithDescription(`
		Use the get_caddy_config tool to get the current caddy server configuration in JSON format.

		The caddy server will always return a JSON configuration unless there is no configuration currently loaded.
		`),
	)

	// Add get Caddy config tool handler
	s.AddTool(getCaddyConfig, getCaddyConfigHandler)

	updateCaddyConfig := mcp.NewTool("update_caddy_config",
		mcp.WithDescription(`
		Use the update_caddy_config tool to update the caddy server configuration in JSON format.

		Notes:
			You must provide a valid JSON configuration to update the caddy server configuration.
			You must provide the full JSON configuration and not just a partial configuration.
			You can use the get_caddy_config tool to get the current caddy server configuration in JSON format.
			If the user provides a YAML configuration, you must convert it to JSON first using the convert_yaml_to_json tool.
			If the user provides a Nginx configuration, you must convert it to JSON first using the convert_nginx_to_json tool.
			If the user provides a Caddyfile configuration, you must convert it to JSON first using the convert_caddyfile_to_json tool.
		`),
		mcp.WithString("json_config",
			mcp.Required(),
			mcp.Description("The caddy server JSON configuration to update the caddy server with"),
		),
	)

	// Add update Caddy config tool handler
	s.AddTool(updateCaddyConfig, updateCaddyConfigHandler)

	convertCaddyfileToJSON := mcp.NewTool("convert_caddyfile_to_json",
		mcp.WithDescription(`
		Use the convert_caddyfile_to_json tool to convert a caddy server Caddyfile to JSON configuration.

		Notes:
			You must provide a valid Caddyfile configuration to convert to JSON.
		`),
		mcp.WithString("caddyfile_config",
			mcp.Required(),
			mcp.Description("The Caddyfile configuration to convert to JSON"),
		),
	)

	// Add convert Caddyfile to JSON tool handler
	s.AddTool(convertCaddyfileToJSON, caddyfileToJSON)

	convertNginxToJSON := mcp.NewTool("convert_nginx_to_json",
		mcp.WithDescription(`
		Use the convert_nginx_to_json tool to convert a caddy server Nginx configuration to JSON configuration.

		Notes:
			You must provide a valid Nginx configuration to convert to JSON.
		`),
		mcp.WithString("nginx_config",
			mcp.Required(),
			mcp.Description("The Nginx configuration to convert to JSON"),
		),
	)

	// Add convert Nginx to JSON tool handler
	s.AddTool(convertNginxToJSON, nginxToJSON)

	convertYamlToJSON := mcp.NewTool("convert_yaml_to_json",
		mcp.WithDescription(`
		Use the convert_yaml_to_json tool to convert a caddy server YAML configuration to JSON configuration.

		Notes:
			You must provide a valid YAML configuration to convert to JSON.
		`),
		mcp.WithString("yaml_config",
			mcp.Required(),
			mcp.Description("The YAML configuration to convert to JSON"),
		),
	)

	// Add convert YAML to JSON tool handler
	s.AddTool(convertYamlToJSON, yamlToJSON)

	// Add upstream proxy statuses tool handler
	upstreamProxyStatuses := mcp.NewTool("upstream_proxy_statuses",
		mcp.WithDescription("Get the current status of the configured reverse proxy upstreams (backends) as a JSON document. This can be used to confirm that the backend proxy servers are running and responding to requests."),
	)

	// Add upstream proxy statuses tool handler
	s.AddTool(upstreamProxyStatuses, upstreamProxyStatusesHandler)

	// Check if SSE is enabled then start the server
	if transport == "sse" {
		sseServer := server.NewSSEServer(
			s,
			server.WithKeepAlive(true),
		)

		log.Printf("Starting MCP SSE server on: %d\n", port)
		if err := sseServer.Start(fmt.Sprintf("0.0.0.0:%d", port)); err != nil {
			log.Fatalf("Server error: %v\n", err)
		}
	} else if transport == "httpstream" {
		streamable := server.NewStreamableHTTPServer(s, server.WithHeartbeatInterval(10*time.Second))
		log.Printf("Starting MCP Streamable HTTP server on: %d\n", port)
		if err := streamable.Start(fmt.Sprintf("0.0.0.0:%d", port)); err != nil {
			log.Fatalf("Server error: %v\n", err)
		}
	} else {
		// Start the MCP server using stdio
		if err := server.ServeStdio(s); err != nil {
			log.Fatalf("Server error: %v\n", err)
		}
	}
}

// Get the current Caddy JSON configuration
func getCaddyConfigHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	reqURL, err := url.Parse(fmt.Sprintf("%s/config/", defaultURL))
	if err != nil {
		return nil, err
	}

	req := &http.Request{
		Method: http.MethodGet,
		URL:    reqURL,
		Header: make(http.Header),
	}

	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get Caddy configuration: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if len(body) == 0 {
		return nil, fmt.Errorf("no configuration currently loaded")
	}

	return mcp.NewToolResultText(fmt.Sprintf("%s", string(body))), nil
}

// Update the Caddy JSON configuration
func updateCaddyConfigHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	config, err := request.RequireString("json_config")
	if err != nil {
		return nil, err
	}

	reqURL, err := url.Parse(fmt.Sprintf("%s/load", defaultURL))
	if err != nil {
		return nil, err
	}

	req := &http.Request{
		Method: http.MethodPost,
		URL:    reqURL,
		Header: make(http.Header),
		Body:   io.NopCloser(bytes.NewBuffer([]byte(config))),
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		caddyerr := &caddyError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
		}
		data, err := json.Marshal(caddyerr)
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResultText(fmt.Sprintf("%s", string(data))), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("%s", body)), nil
}

// Convert configuration to JSON configuration
func adaptToJSON(format string, input []byte) ([]byte, error) {
	var (
		adapter caddyconfig.Adapter
		//warnings []caddyconfig.Warning
		err    error
		output []byte
	)

	switch format {
	case "caddyfile":
		adapter = caddyconfig.GetAdapter("caddyfile")
	case "yaml":
		adapter = caddyconfig.GetAdapter("yaml")
	case "nginx":
		adapter = caddyconfig.GetAdapter("nginx")
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}

	output, _, err = adapter.Adapt(input, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to adapt %s: %v", format, err)
	}

	return output, nil
}

// Convert caddy Caddyfile to JSON configuration
func caddyfileToJSON(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	config, err := request.RequireString("caddyfile_config")
	if err != nil {
		return nil, err
	}

	json, err := adaptToJSON("caddyfile", []byte(config))
	if err != nil {
		return nil, err
	}

	return mcp.NewToolResultText(fmt.Sprintf("%s", json)), nil
}

// Convert caddy Nginx configuration to JSON configuration
func nginxToJSON(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	config, err := request.RequireString("nginx_config")
	if err != nil {
		return nil, err
	}

	json, err := adaptToJSON("nginx", []byte(config))
	if err != nil {
		return nil, err
	}

	return mcp.NewToolResultText(fmt.Sprintf("%s", json)), nil
}

// Convert caddy YAML configuration to JSON configuration
func yamlToJSON(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	config, err := request.RequireString("yaml_config")
	if err != nil {
		return nil, err
	}

	json, err := adaptToJSON("yaml", []byte(config))
	if err != nil {
		return nil, err
	}

	return mcp.NewToolResultText(fmt.Sprintf("%s", json)), nil
}

// Get the current status of the configured reverse proxy upstreams (backends) as a JSON document.
func upstreamProxyStatusesHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	url := fmt.Sprintf("%s/reverse_proxy/upstreams", defaultURL)

	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get upstream proxy statuses: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return mcp.NewToolResultText(fmt.Sprintf("%s", body)), nil
}
