# caddy-mcp

## Overview

**caddy-mcp** is a server that provides a Model Context Protocol (MCP) interface for managing a [Caddy](https://caddyserver.com/) server instance using the [Caddy API](https://caddyserver.com/docs/api). It exposes tools for retrieving, updating, and converting Caddy configurations (in JSON, Caddyfile, YAML, or Nginx formats) and for monitoring reverse proxy upstreams. The server can run over different transports: stdio, SSE, or HTTP stream.

## Tools

- **get_caddy_config** - Get the current Caddy server configuration in JSON format
- **update_caddy_config** - Update the Caddy server configuration by providing a full JSON configuration
- **convert_caddyfile_to_json** - Convert a Caddyfile configuration to JSON format
- **convert_nginx_to_json** - Convert an Nginx configuration to Caddy JSON format  
- **convert_yaml_to_json** - Convert a YAML configuration to Caddy JSON format
- **upstream_proxy_statuses** - Get the current status of configured reverse proxy upstreams as JSON

## Build Steps

1. **Prerequisites:**  
   - Go 1.24+ installed ([download](https://go.dev/dl/))

2. **Clone the repository:**
   ```sh
   git clone https://github.com/lum8rjack/caddy-mcp.git
   cd caddy-mcp
   ```

3. **Build the server:**
   ```sh
   make
   # or
   go build -o caddy-mcp .
   ```
   This will produce a `caddy-mcp` binary in the project root.

## Running the MCP Server

You can run the server with different transports and ports:

```sh
./caddy-mcp -h
Usage of ./caddy-mcp:
  -port int
        Port to run the MCP server on (default 7000)
  -transport string
        The transport to use for the MCP server (stdio, sse, httpstream) (default "stdio")
  -url string
        The URL of the caddy server (default "http://127.0.0.1:2019")
```

**Example MCP Settings:**

```json
{
    "mcpServers": {
        "caddy-mcp": {
            "command": "~/caddy-mcp",
            "args": ["-transport", "stdio", "-url", "http://127.0.0.1:2019"]
        }
    }
}
```

## Caddyfile Example

A sample `Caddyfile` to test with that exposed the admin API:
```caddyfile
{
    admin :2019
}

:8080 {
    respond "I am 8080"
}

:8081 {
    respond "I am 8081"
}
```



