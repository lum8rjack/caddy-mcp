NAME := caddy-mcp
BUILD := go build -ldflags "-s -w" -trimpath

default:
	@echo "Compiling"
	$(BUILD) -o $(NAME)

update:
	@echo "Updating MCP SDK"
	go get -u github.com/mark3labs/mcp-go
	go mod tidy

mcpinspector:
	@echo "Running MCP Inspector"
	npx @modelcontextprotocol/inspector
