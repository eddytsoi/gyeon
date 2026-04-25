package mcp

import (
	"net/http"
	"os"

	mcpserver "github.com/mark3labs/mcp-go/server"

	"gyeon/backend/internal/orders"
	"gyeon/backend/internal/pricing"
	"gyeon/backend/internal/shop"
)

// Server wraps the MCP SSE server with Gyeon's public storefront tools.
type Server struct {
	sse    *mcpserver.SSEServer
	apiKey string
}

// NewServer creates the MCP server and registers all storefront tools.
// Only safe, public-facing tools are registered — no admin or customer PII endpoints.
func NewServer(
	catSvc *shop.CategoryService,
	prodSvc *shop.ProductService,
	cartSvc *orders.CartService,
	orderSvc *orders.OrderService,
	pricingSvc *pricing.Service,
) *Server {
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	apiKey := os.Getenv("MCP_API_KEY")

	s := mcpserver.NewMCPServer("Gyeon Storefront", "1.0.0")

	registerCatalogTools(s, catSvc, prodSvc)
	registerCartTools(s, cartSvc)
	registerOrderTools(s, orderSvc, pricingSvc)

	sse := mcpserver.NewSSEServer(s, mcpserver.WithBaseURL(baseURL+"/mcp"))

	return &Server{sse: sse, apiKey: apiKey}
}

// Handler returns an http.Handler to mount at /mcp in the chi router.
func (s *Server) Handler() http.Handler {
	if s.apiKey != "" {
		return apiKeyMiddleware(s.apiKey, s.sse)
	}
	return s.sse
}
