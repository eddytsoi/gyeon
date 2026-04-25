package mcp

import (
	"context"
	"encoding/json"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"gyeon/backend/internal/shop"
)

func registerCatalogTools(s *mcpserver.MCPServer, catSvc *shop.CategoryService, prodSvc *shop.ProductService) {
	s.AddTool(mcplib.NewTool("list_categories",
		mcplib.WithDescription("List all active product categories"),
	), func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		cats, err := catSvc.List(ctx)
		if err != nil {
			return nil, err
		}
		data, _ := json.Marshal(cats)
		return mcplib.NewToolResultText(string(data)), nil
	})

	s.AddTool(mcplib.NewTool("list_products",
		mcplib.WithDescription("List active products with optional pagination and language selection"),
		mcplib.WithNumber("limit", mcplib.Description("Max products to return (1–100, default 20)"), mcplib.DefaultNumber(20)),
		mcplib.WithNumber("offset", mcplib.Description("Number of products to skip for pagination (default 0)"), mcplib.DefaultNumber(0)),
		mcplib.WithString("lang", mcplib.Description("Language locale for translations, e.g. 'en' or 'zh-TW'. Omit for default.")),
	), func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		limit := req.GetInt("limit", 20)
		if limit < 1 {
			limit = 1
		}
		if limit > 100 {
			limit = 100
		}
		offset := req.GetInt("offset", 0)
		if offset < 0 {
			offset = 0
		}
		lang := req.GetString("lang", "")

		products, err := prodSvc.List(ctx, lang, limit, offset)
		if err != nil {
			return nil, err
		}
		data, _ := json.Marshal(products)
		return mcplib.NewToolResultText(string(data)), nil
	})

	s.AddTool(mcplib.NewTool("get_product",
		mcplib.WithDescription("Get full product detail including all variants and images"),
		mcplib.WithString("product_id", mcplib.Description("Product UUID"), mcplib.Required()),
		mcplib.WithString("lang", mcplib.Description("Language locale for translated content (optional)")),
	), func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		productID, err := req.RequireString("product_id")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		lang := req.GetString("lang", "")

		product, err := prodSvc.GetByID(ctx, productID, lang)
		if err != nil {
			return mcplib.NewToolResultError("product not found"), nil
		}
		variants, err := prodSvc.ListVariants(ctx, productID)
		if err != nil {
			return nil, err
		}
		images, err := prodSvc.ListImages(ctx, productID)
		if err != nil {
			return nil, err
		}

		result := map[string]any{
			"product":  product,
			"variants": variants,
			"images":   images,
		}
		data, _ := json.Marshal(result)
		return mcplib.NewToolResultText(string(data)), nil
	})

	s.AddTool(mcplib.NewTool("get_variant",
		mcplib.WithDescription("Get variant pricing and stock information"),
		mcplib.WithString("variant_id", mcplib.Description("Variant UUID"), mcplib.Required()),
	), func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		variantID, err := req.RequireString("variant_id")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		variant, err := prodSvc.GetVariantByID(ctx, variantID)
		if err != nil {
			return mcplib.NewToolResultError("variant not found"), nil
		}
		if !variant.IsActive {
			return mcplib.NewToolResultError("variant not available"), nil
		}
		data, _ := json.Marshal(variant)
		return mcplib.NewToolResultText(string(data)), nil
	})
}
