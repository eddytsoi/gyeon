package mcp

import (
	"context"
	"database/sql"
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
		mcplib.WithDescription("List active products (both simple and bundle products inline) with optional pagination, search and language selection. Each row carries `kind` (simple|bundle), `variant_count` (active variants; bundles always have 1), `primary_image_url` (may be null), and `default_variant_id` (the variant ready for add_to_cart — for bundles this is the auto-generated BUNDLE-* variant)."),
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

		products, err := prodSvc.ListEnriched(ctx, lang, search, limit, offset)
		if err != nil {
			return nil, err
		}
		data, _ := json.Marshal(products)
		return mcplib.NewToolResultText(string(data)), nil
	})

	s.AddTool(mcplib.NewTool("get_product",
		mcplib.WithDescription("Get full product detail: returns {product, variants, images}. "+
			"`product.kind` is one of: \"simple\" (regular product, may have multiple variants) or \"bundle\" (a fixed bundle of component variants).\n\n"+
			"For bundle products the response additionally contains:\n"+
			"  • bundle_items — component rows: {component_variant_id, quantity, component_product_name, component_sku, component_stock_qty, component_price, ...}\n"+
			"  • derived_stock — integer = min(floor(component_stock_qty / quantity)) across all bundle_items, or 0 when there are no items.\n\n"+
			"Bundle invariants:\n"+
			"  • A bundle product ALWAYS has exactly one variant; its SKU is auto-generated as \"BUNDLE-<UPPER(first 8 chars of product_id)>\".\n"+
			"  • To add a bundle to a cart, use that single variant's id with add_to_cart (the variant's reported stock_qty is automatically replaced with derived_stock).\n"+
			"  • Bundles cannot be nested (a bundle's components must be variants of simple products)."),
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
		if product.Status != "active" {
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
		mcplib.WithDescription("Replace ALL bundle items for a bundle product (full overwrite — pass an empty `items` array to clear). For incremental edits use add_bundle_item / remove_bundle_item instead. Each input item: {component_variant_id (uuid, required), quantity (int ≥ 1, required), sort_order (int, optional), display_name_override (string, optional)}."),
		mcplib.WithString("product_id", mcplib.Description("Bundle product UUID"), mcplib.Required()),
		mcplib.WithArray("items",
			mcplib.Description("Bundle component rows. Empty array clears all components."),
			mcplib.Items(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"component_variant_id":  map[string]any{"type": "string", "description": "UUID of the component variant (must belong to a simple product, not a bundle)"},
					"quantity":              map[string]any{"type": "integer", "minimum": 1, "description": "Number of this component per bundle (default 1)"},
					"sort_order":            map[string]any{"type": "integer", "description": "Display ordering, lower first (default 0)"},
					"display_name_override": map[string]any{"type": "string", "description": "Optional override of the component's display name in this bundle"},
				},
				"required": []string{"component_variant_id"},
			}),
		),
		mcplib.WithString("items_json", mcplib.Description("DEPRECATED — JSON-string fallback for `items`. Prefer the typed `items` array. If both are set, `items` wins.")),
	), func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		productID, err := req.RequireString("product_id")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		var inputs []shop.BundleItemInput
		args := req.GetArguments()
		if raw, ok := args["items"]; ok && raw != nil {
			b, _ := json.Marshal(raw)
			if err := json.Unmarshal(b, &inputs); err != nil {
				return mcplib.NewToolResultError("invalid items: " + err.Error()), nil
			}
		} else if itemsJSON := req.GetString("items_json", ""); itemsJSON != "" {
			if err := json.Unmarshal([]byte(itemsJSON), &inputs); err != nil {
				return mcplib.NewToolResultError("invalid items_json: " + err.Error()), nil
			}
		} else {
			return mcplib.NewToolResultError("provide `items` (typed array) or `items_json` (legacy string)"), nil
		}
		items, err := prodSvc.SetBundleItems(ctx, productID, inputs)
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		data, _ := json.Marshal(items)
		return mcplib.NewToolResultText(string(data)), nil
	})

	s.AddTool(mcplib.NewTool("add_bundle_item",
		mcplib.WithDescription("Add (or update) a single component on a bundle product. Idempotent: if a row already exists for the same (bundle_product_id, component_variant_id), its quantity / sort_order / display_name_override are overwritten — otherwise a new row is inserted. The component variant must belong to a simple product (no nested bundles)."),
		mcplib.WithString("bundle_product_id", mcplib.Description("Bundle product UUID"), mcplib.Required()),
		mcplib.WithString("component_variant_id", mcplib.Description("UUID of the component variant to add"), mcplib.Required()),
		mcplib.WithNumber("quantity", mcplib.Description("Number of this component per bundle (default 1)"), mcplib.DefaultNumber(1)),
		mcplib.WithNumber("sort_order", mcplib.Description("Display ordering, lower first (default 0)"), mcplib.DefaultNumber(0)),
		mcplib.WithString("display_name_override", mcplib.Description("Optional override of the component's display name in this bundle")),
	), func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		productID, err := req.RequireString("bundle_product_id")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		componentVariantID, err := req.RequireString("component_variant_id")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		input := shop.BundleItemInput{
			ComponentVariantID: componentVariantID,
			Quantity:           req.GetInt("quantity", 1),
			SortOrder:          req.GetInt("sort_order", 0),
		}
		if override := req.GetString("display_name_override", ""); override != "" {
			input.DisplayNameOverride = &override
		}
		item, err := prodSvc.AddBundleItem(ctx, productID, input)
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		data, _ := json.Marshal(item)
		return mcplib.NewToolResultText(string(data)), nil
	})

	s.AddTool(mcplib.NewTool("remove_bundle_item",
		mcplib.WithDescription("Remove a single component from a bundle product, identified by (bundle_product_id, component_variant_id). Returns 'not found' if no such row exists."),
		mcplib.WithString("bundle_product_id", mcplib.Description("Bundle product UUID"), mcplib.Required()),
		mcplib.WithString("component_variant_id", mcplib.Description("UUID of the component variant to remove"), mcplib.Required()),
	), func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		productID, err := req.RequireString("bundle_product_id")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		componentVariantID, err := req.RequireString("component_variant_id")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		if err := prodSvc.RemoveBundleItem(ctx, productID, componentVariantID); err != nil {
			if err == sql.ErrNoRows {
				return mcplib.NewToolResultError("bundle item not found"), nil
			}
			return mcplib.NewToolResultError(err.Error()), nil
		}
		return mcplib.NewToolResultText("bundle item removed"), nil
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
