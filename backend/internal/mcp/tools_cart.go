package mcp

import (
	"context"
	"encoding/json"
	"errors"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"gyeon/backend/internal/orders"
)

func registerCartTools(s *mcpserver.MCPServer, cartSvc *orders.CartService) {
	s.AddTool(mcplib.NewTool("create_cart",
		mcplib.WithDescription("Create or retrieve an anonymous cart by session token. Generate a UUID per user session and store it client-side."),
		mcplib.WithString("session_token", mcplib.Description("Unique session identifier (UUID recommended)"), mcplib.Required()),
	), func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		token, err := req.RequireString("session_token")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		cart, err := cartSvc.GetOrCreate(ctx, token, nil)
		if err != nil {
			return nil, err
		}
		data, _ := json.Marshal(cart)
		return mcplib.NewToolResultText(string(data)), nil
	})

	s.AddTool(mcplib.NewTool("get_cart",
		mcplib.WithDescription("Get current cart contents by cart ID"),
		mcplib.WithString("cart_id", mcplib.Description("Cart UUID returned from create_cart"), mcplib.Required()),
	), func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		cartID, err := req.RequireString("cart_id")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		cart, err := cartSvc.GetByID(ctx, cartID)
		if err != nil {
			if errors.Is(err, orders.ErrCartNotFound) {
				return mcplib.NewToolResultError("cart not found"), nil
			}
			return nil, err
		}
		data, _ := json.Marshal(cart)
		return mcplib.NewToolResultText(string(data)), nil
	})

	s.AddTool(mcplib.NewTool("add_to_cart",
		mcplib.WithDescription("Add a product variant to the cart"),
		mcplib.WithString("cart_id", mcplib.Description("Cart UUID"), mcplib.Required()),
		mcplib.WithString("variant_id", mcplib.Description("Product variant UUID to add"), mcplib.Required()),
		mcplib.WithNumber("quantity", mcplib.Description("Quantity to add (default 1)"), mcplib.DefaultNumber(1)),
	), func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		cartID, err := req.RequireString("cart_id")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		variantID, err := req.RequireString("variant_id")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		qty := req.GetInt("quantity", 1)
		if qty < 1 {
			qty = 1
		}

		item, err := cartSvc.AddItem(ctx, cartID, orders.AddItemRequest{
			VariantID: variantID,
			Quantity:  qty,
		})
		if err != nil {
			if errors.Is(err, orders.ErrInsufficientStock) {
				return mcplib.NewToolResultError("insufficient stock"), nil
			}
			if errors.Is(err, orders.ErrCartNotFound) {
				return mcplib.NewToolResultError("cart not found"), nil
			}
			return nil, err
		}
		data, _ := json.Marshal(item)
		return mcplib.NewToolResultText(string(data)), nil
	})

	s.AddTool(mcplib.NewTool("update_cart_item",
		mcplib.WithDescription("Update the quantity of a cart item. Set quantity to 0 to remove the item."),
		mcplib.WithString("cart_id", mcplib.Description("Cart UUID"), mcplib.Required()),
		mcplib.WithString("item_id", mcplib.Description("Cart item UUID"), mcplib.Required()),
		mcplib.WithNumber("quantity", mcplib.Description("New quantity (0 removes the item)"), mcplib.Required()),
	), func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		cartID, err := req.RequireString("cart_id")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		itemID, err := req.RequireString("item_id")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		qty := req.GetInt("quantity", 0)

		item, err := cartSvc.UpdateItem(ctx, cartID, itemID, orders.UpdateItemRequest{Quantity: qty})
		if err != nil {
			if errors.Is(err, orders.ErrCartNotFound) {
				return mcplib.NewToolResultError("cart not found"), nil
			}
			return nil, err
		}
		if item == nil {
			return mcplib.NewToolResultText("item removed from cart"), nil
		}
		data, _ := json.Marshal(item)
		return mcplib.NewToolResultText(string(data)), nil
	})

	s.AddTool(mcplib.NewTool("remove_from_cart",
		mcplib.WithDescription("Remove an item from the cart"),
		mcplib.WithString("cart_id", mcplib.Description("Cart UUID"), mcplib.Required()),
		mcplib.WithString("item_id", mcplib.Description("Cart item UUID to remove"), mcplib.Required()),
	), func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		cartID, err := req.RequireString("cart_id")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		itemID, err := req.RequireString("item_id")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		if err := cartSvc.RemoveItem(ctx, cartID, itemID); err != nil {
			if errors.Is(err, orders.ErrCartNotFound) {
				return mcplib.NewToolResultError("cart not found"), nil
			}
			return nil, err
		}
		return mcplib.NewToolResultText("item removed from cart"), nil
	})
}
