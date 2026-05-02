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
		mcplib.WithDescription("List active products with optional pagination, search and language selection"),
		mcplib.WithNumber("limit", mcplib.Description("Max products to return (1–100, default 20)"), mcplib.DefaultNumber(20)),
		mcplib.WithNumber("offset", mcplib.Description("Number of products to skip for pagination (default 0)"), mcplib.DefaultNumber(0)),
		mcplib.WithString("search", mcplib.Description("Optional case-insensitive substring matched against product name, slug and number. Omit to list all.")),
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
		search := req.GetString("search", "")

		products, err := prodSvc.List(ctx, lang, search, limit, offset)
		if err != nil {
			return nil, err
		}
		data, _ := json.Marshal(products)
		return mcplib.NewToolResultText(string(data)), nil
	})

	s.AddTool(mcplib.NewTool("get_product",
		mcplib.WithDescription("Get full product detail including all variants, images, and (for bundles) bundle items"),
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

		// For bundle products, include bundle items and derived stock.
		if product.Kind == "bundle" {
			bundleItems, err := prodSvc.GetBundleItems(ctx, productID)
			if err == nil {
				result["bundle_items"] = bundleItems
			}
			derived, _ := prodSvc.GetDerivedStock(ctx, productID)
			result["derived_stock"] = derived
		}

		data, _ := json.Marshal(result)
		return mcplib.NewToolResultText(string(data)), nil
	})

	s.AddTool(mcplib.NewTool("get_bundle_items",
		mcplib.WithDescription("Get the component items of a bundle product, including component product name, SKU, quantity, and derived stock"),
		mcplib.WithString("product_id", mcplib.Description("Bundle product UUID"), mcplib.Required()),
	), func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		productID, err := req.RequireString("product_id")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		items, err := prodSvc.GetBundleItems(ctx, productID)
		if err != nil {
			return nil, err
		}
		derived, _ := prodSvc.GetDerivedStock(ctx, productID)
		data, _ := json.Marshal(map[string]any{
			"items":         items,
			"derived_stock": derived,
		})
		return mcplib.NewToolResultText(string(data)), nil
	})

	s.AddTool(mcplib.NewTool("set_bundle_items",
		mcplib.WithDescription("Replace all bundle items for a bundle product. Pass an empty items array to clear the bundle contents."),
		mcplib.WithString("product_id", mcplib.Description("Bundle product UUID"), mcplib.Required()),
		mcplib.WithString("items_json", mcplib.Description(`JSON array of bundle items. Each item: {"component_variant_id":"<uuid>","quantity":<int>,"sort_order":<int>,"display_name_override":"<optional string>"}`), mcplib.Required()),
	), func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		productID, err := req.RequireString("product_id")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		itemsJSON, err := req.RequireString("items_json")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		var inputs []shop.BundleItemInput
		if err := json.Unmarshal([]byte(itemsJSON), &inputs); err != nil {
			return mcplib.NewToolResultError("invalid items_json: " + err.Error()), nil
		}
		items, err := prodSvc.SetBundleItems(ctx, productID, inputs)
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		data, _ := json.Marshal(items)
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
