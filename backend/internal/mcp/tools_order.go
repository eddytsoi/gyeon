package mcp

import (
	"context"
	"encoding/json"
	"errors"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"gyeon/backend/internal/orders"
	"gyeon/backend/internal/pricing"
)

// checkoutResult is the safe, PII-free response for the checkout tool.
// It deliberately omits: customer_id, shipping_address_id, notes, line items,
// subtotal, discount_amount, shipping_fee, and timestamps.
type checkoutResult struct {
	OrderID string  `json:"order_id"`
	Status  string  `json:"status"`
	Total   float64 `json:"total"`
}

type couponResponse struct {
	Valid          bool    `json:"valid"`
	DiscountType   string  `json:"discount_type,omitempty"`
	DiscountValue  float64 `json:"discount_value,omitempty"`
	DiscountAmount float64 `json:"discount_amount,omitempty"`
	Message        string  `json:"message,omitempty"`
}

func registerOrderTools(s *mcpserver.MCPServer, orderSvc *orders.OrderService, pricingSvc *pricing.Service) {
	s.AddTool(mcplib.NewTool("validate_coupon",
		mcplib.WithDescription("Validate a coupon code and preview the discount amount for a given subtotal"),
		mcplib.WithString("code", mcplib.Description("Coupon code to validate"), mcplib.Required()),
		mcplib.WithNumber("subtotal", mcplib.Description("Order subtotal to compute the discount against"), mcplib.Required()),
	), func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		code, err := req.RequireString("code")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		subtotal, err := req.RequireFloat("subtotal")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}

		coupon, err := pricingSvc.ValidateCoupon(ctx, code, subtotal)
		if err != nil {
			resp := couponResponse{Valid: false, Message: err.Error()}
			data, _ := json.Marshal(resp)
			return mcplib.NewToolResultText(string(data)), nil
		}

		var discountAmount float64
		switch coupon.DiscountType {
		case "percentage":
			discountAmount = subtotal * coupon.DiscountValue / 100
		case "fixed":
			discountAmount = coupon.DiscountValue
		}

		resp := couponResponse{
			Valid:          true,
			DiscountType:   string(coupon.DiscountType),
			DiscountValue:  coupon.DiscountValue,
			DiscountAmount: discountAmount,
		}
		data, _ := json.Marshal(resp)
		return mcplib.NewToolResultText(string(data)), nil
	})

	s.AddTool(mcplib.NewTool("checkout",
		mcplib.WithDescription("Place an order from a cart. Returns only order ID, status, and total — no customer or address data."),
		mcplib.WithString("cart_id", mcplib.Description("Cart UUID to check out"), mcplib.Required()),
		mcplib.WithString("coupon_code", mcplib.Description("Optional coupon code for a discount")),
		mcplib.WithNumber("shipping_fee", mcplib.Description("Shipping fee amount (default 0)"), mcplib.DefaultNumber(0)),
		mcplib.WithString("notes", mcplib.Description("Optional order notes")),
	), func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		cartID, err := req.RequireString("cart_id")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		shippingFee := req.GetFloat("shipping_fee", 0)

		checkoutReq := orders.CheckoutRequest{
			CartID:      cartID,
			ShippingFee: shippingFee,
		}
		if couponCode := req.GetString("coupon_code", ""); couponCode != "" {
			checkoutReq.CouponCode = &couponCode
		}
		if notes := req.GetString("notes", ""); notes != "" {
			checkoutReq.Notes = &notes
		}

		order, err := orderSvc.Checkout(ctx, checkoutReq)
		if err != nil {
			if errors.Is(err, orders.ErrEmptyCart) || errors.Is(err, orders.ErrCartNotFound) {
				return mcplib.NewToolResultError(err.Error()), nil
			}
			return mcplib.NewToolResultError(err.Error()), nil
		}

		// SAFETY: Project only three fields — never return the full Order struct.
		result := checkoutResult{
			OrderID: order.ID,
			Status:  string(order.Status),
			Total:   order.Total,
		}
		data, _ := json.Marshal(result)
		return mcplib.NewToolResultText(string(data)), nil
	})
}
