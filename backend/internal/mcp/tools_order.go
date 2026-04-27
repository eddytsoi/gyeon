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

// checkoutResult is the response for the checkout tool. It carries the
// Stripe PaymentIntent handle (client_secret + publishable_key + mode) so the
// caller's client can confirm payment via Stripe.js, and intentionally does
// NOT echo back any customer or address PII the caller already supplied.
type checkoutResult struct {
	OrderID        string  `json:"order_id"`
	Status         string  `json:"status"`
	Total          float64 `json:"total"`
	ClientSecret   string  `json:"client_secret"`
	PublishableKey string  `json:"publishable_key"`
	Mode           string  `json:"mode"`
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
		mcplib.WithDescription("Place an order from a cart. Creates a pending order plus a Stripe PaymentIntent and returns its client_secret; the caller's client (browser/device) must confirm payment with Stripe.js — the order stays in 'pending' status until Stripe webhooks confirm payment. Either customer_id (logged-in) or customer_email (+ optional name/phone for a guest) is required, and either shipping_address_id or shipping_line1+city+postal_code is required."),
		mcplib.WithString("cart_id", mcplib.Description("Cart UUID to check out"), mcplib.Required()),
		mcplib.WithString("customer_id", mcplib.Description("Existing customer UUID for a logged-in checkout. Omit for guest checkout (then customer_email is required).")),
		mcplib.WithString("customer_email", mcplib.Description("Customer email — required for guest checkout (when customer_id is omitted)")),
		mcplib.WithString("customer_first_name", mcplib.Description("Customer first name (used for guest upsert and order snapshot)")),
		mcplib.WithString("customer_last_name", mcplib.Description("Customer last name (used for guest upsert and order snapshot)")),
		mcplib.WithString("customer_phone", mcplib.Description("Customer phone number")),
		mcplib.WithString("shipping_address_id", mcplib.Description("Existing shipping address UUID. Omit to provide a new shipping_* address inline.")),
		mcplib.WithString("shipping_line1", mcplib.Description("Shipping address line 1 — required when shipping_address_id is omitted")),
		mcplib.WithString("shipping_line2", mcplib.Description("Shipping address line 2 (optional)")),
		mcplib.WithString("shipping_city", mcplib.Description("Shipping city/district — required when shipping_address_id is omitted")),
		mcplib.WithString("shipping_state", mcplib.Description("Shipping state/province (optional)")),
		mcplib.WithString("shipping_postal_code", mcplib.Description("Shipping postal code — required when shipping_address_id is omitted")),
		mcplib.WithString("shipping_country", mcplib.Description("Shipping country ISO code (default 'HK')")),
		mcplib.WithBoolean("save_address", mcplib.Description("When a new shipping address is supplied, save it to the customer's address book (default false)")),
		mcplib.WithString("coupon_code", mcplib.Description("Optional coupon code for a discount")),
		mcplib.WithNumber("shipping_fee", mcplib.Description("Shipping fee amount (default 0)"), mcplib.DefaultNumber(0)),
		mcplib.WithString("notes", mcplib.Description("Optional order notes")),
	), func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		cartID, err := req.RequireString("cart_id")
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}

		checkoutReq := orders.CheckoutRequest{
			CartID:      cartID,
			ShippingFee: req.GetFloat("shipping_fee", 0),
			SaveAddress: req.GetBool("save_address", false),
		}

		if customerID := req.GetString("customer_id", ""); customerID != "" {
			checkoutReq.CustomerID = &customerID
		}

		email := req.GetString("customer_email", "")
		firstName := req.GetString("customer_first_name", "")
		lastName := req.GetString("customer_last_name", "")
		phone := req.GetString("customer_phone", "")
		if email != "" || firstName != "" || lastName != "" || phone != "" {
			checkoutReq.CustomerInfo = &orders.CustomerInfo{
				FirstName: firstName,
				LastName:  lastName,
				Email:     email,
				Phone:     phone,
			}
		}

		if shippingAddrID := req.GetString("shipping_address_id", ""); shippingAddrID != "" {
			checkoutReq.ShippingAddressID = &shippingAddrID
		}

		line1 := req.GetString("shipping_line1", "")
		line2 := req.GetString("shipping_line2", "")
		city := req.GetString("shipping_city", "")
		state := req.GetString("shipping_state", "")
		postalCode := req.GetString("shipping_postal_code", "")
		country := req.GetString("shipping_country", "")
		if line1 != "" || line2 != "" || city != "" || state != "" || postalCode != "" || country != "" {
			checkoutReq.ShippingAddress = &orders.ShippingAddressInput{
				Line1:      line1,
				Line2:      line2,
				City:       city,
				State:      state,
				PostalCode: postalCode,
				Country:    country,
			}
		}

		if couponCode := req.GetString("coupon_code", ""); couponCode != "" {
			checkoutReq.CouponCode = &couponCode
		}
		if notes := req.GetString("notes", ""); notes != "" {
			checkoutReq.Notes = &notes
		}

		checkoutResp, err := orderSvc.Checkout(ctx, checkoutReq)
		if err != nil {
			if errors.Is(err, orders.ErrEmptyCart) || errors.Is(err, orders.ErrCartNotFound) {
				return mcplib.NewToolResultError(err.Error()), nil
			}
			return mcplib.NewToolResultError(err.Error()), nil
		}
		order := checkoutResp.Order

		// Project only the order summary + Stripe PaymentIntent handle.
		// Never echo customer/address PII the caller already supplied, and
		// never return the full Order struct.
		result := checkoutResult{
			OrderID:        order.ID,
			Status:         string(order.Status),
			Total:          order.Total,
			ClientSecret:   checkoutResp.ClientSecret,
			PublishableKey: checkoutResp.PublishableKey,
			Mode:           checkoutResp.Mode,
		}
		data, _ := json.Marshal(result)
		return mcplib.NewToolResultText(string(data)), nil
	})
}
