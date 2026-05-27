package orders

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"gyeon/backend/internal/customers"
	"gyeon/backend/internal/pricing"
)

// QuoteRequest is the storefront's ask for a pre-payment price breakdown of
// a cart. Mirrors the slice of CheckoutRequest needed for pricing only — no
// shipping address, no customer info, no Stripe.
type QuoteRequest struct {
	CartID     string  `json:"cart_id"`
	CouponCode *string `json:"coupon_code,omitempty"`
	// CustomerID is optional and mirrors CheckoutRequest.CustomerID: when
	// set, role-based promotion eligibility (allowed_roles vs allow_guests)
	// applies as if this customer were checking out. The /orders/checkout
	// path already trusts the body for this field, so /quote does the same.
	CustomerID *string `json:"customer_id,omitempty"`
}

// QuoteAppliedCampaign is the response-side projection of
// pricing.AppliedCampaign — same fields, JSON-tagged.
type QuoteAppliedCampaign struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	Amount      float64 `json:"amount"`
}

// QuoteAppliedCoupon mirrors QuoteAppliedCampaign for the coupon side.
type QuoteAppliedCoupon struct {
	ID          string  `json:"id"`
	Code        string  `json:"code"`
	Description *string `json:"description,omitempty"`
	Amount      float64 `json:"amount"`
}

// QuoteResult is the full price breakdown the storefront uses to render the
// checkout summary. Source of truth: server runs the exact same pricing /
// tax / free-shipping rules as Checkout, so the displayed pre-payment
// total matches what'll be charged.
//
// Note: this is a *quote*. Checkout re-runs ComputeDiscount independently;
// a campaign expiring between quote and pay will silently drop off the
// final order without surfacing here.
type QuoteResult struct {
	Subtotal         float64                `json:"subtotal"`
	AppliedCampaigns []QuoteAppliedCampaign `json:"applied_campaigns"`
	AppliedCoupon    *QuoteAppliedCoupon    `json:"applied_coupon,omitempty"`
	TotalDiscount    float64                `json:"total_discount"`
	TaxAmount        float64                `json:"tax_amount"`
	TaxInclusive     bool                   `json:"tax_inclusive"`
	ShippingFree     bool                   `json:"shipping_free"`
	// Total is subtotal − discount + (exclusive) tax. Shipping is NOT
	// included; the storefront knows the SF Express label from ShippingFree
	// and adds any non-zero fee on top. Matches the order's `total` minus
	// shipping_fee for a checkout that hasn't picked a paid carrier.
	Total float64 `json:"total"`
	// CouponError and CouponErrorCode are populated when the supplied
	// coupon_code failed validation. Campaigns are still computed and
	// returned in that case, so the storefront can show "discount applied"
	// alongside "coupon invalid".
	CouponError     string `json:"coupon_error,omitempty"`
	CouponErrorCode string `json:"coupon_error_code,omitempty"`
}

// Quote returns the pre-payment price breakdown for a cart. Reuses every
// pricing primitive that Checkout uses (ComputeDiscount, taxSvc.Calculate,
// shippingFreeFor) so the storefront's displayed total can't drift from
// the actual charge.
func (s *OrderService) Quote(ctx context.Context, req QuoteRequest) (*QuoteResult, error) {
	cart, err := s.cartSvc.GetByID(ctx, req.CartID)
	if err != nil {
		return nil, err
	}

	customerRole := customers.RoleCustomer
	isGuest := true
	if req.CustomerID != nil && *req.CustomerID != "" && s.customerSvc != nil {
		c, err := s.customerSvc.GetByID(ctx, *req.CustomerID)
		if err == nil {
			customerRole = customers.NormalizeRole(c.Role)
			isGuest = false
		}
	}

	if len(cart.Items) == 0 {
		return &QuoteResult{AppliedCampaigns: []QuoteAppliedCampaign{}}, nil
	}

	pricingItems, subtotal, err := s.loadPricingLines(ctx, cart.Items)
	if err != nil {
		return nil, err
	}

	var (
		discountResult pricing.DiscountResult
		couponErr      string
		couponErrCode  string
	)
	if s.pricingSvc != nil {
		discountResult, err = s.pricingSvc.ComputeDiscount(ctx, pricingItems, subtotal, req.CouponCode, customerRole, isGuest)
		if err != nil {
			// Coupon-specific failures: retry without the coupon so any
			// auto-applied campaigns still render. The frontend learns the
			// coupon was rejected via CouponError / CouponErrorCode and
			// keeps the input field showing the user-facing reason.
			switch {
			case errors.Is(err, pricing.ErrCouponNotFound):
				couponErr = "invalid coupon"
			case errors.Is(err, pricing.ErrCouponExpired):
				couponErr = "coupon has expired"
			case errors.Is(err, pricing.ErrCouponExhausted):
				couponErr = "coupon usage limit reached"
			case errors.Is(err, pricing.ErrCouponMinOrder):
				couponErr = "order amount below coupon minimum"
			case errors.Is(err, pricing.ErrCouponWrongRole):
				couponErr = "This coupon is not valid for your account"
				couponErrCode = "wrong_role"
			default:
				return nil, err
			}
			discountResult, err = s.pricingSvc.ComputeDiscount(ctx, pricingItems, subtotal, nil, customerRole, isGuest)
			if err != nil {
				return nil, err
			}
		}
	}

	discountAmount := discountResult.TotalDiscount
	taxableAmount := subtotal - discountAmount
	if taxableAmount < 0 {
		taxableAmount = 0
	}

	var taxAmount float64
	var taxInclusive bool
	if s.taxSvc != nil {
		taxRes := s.taxSvc.Calculate(ctx, taxableAmount)
		taxAmount = taxRes.TaxAmount
		taxInclusive = taxRes.Inclusive
		if !taxRes.Inclusive {
			taxableAmount += taxAmount
		}
	}

	// Mirror Checkout: free shipping is decided against the post-discount
	// subtotal, server-side, so the storefront and order math agree even
	// when a campaign tips the threshold.
	shippingFree := s.shippingFreeFor(ctx, customerRole, subtotal-discountAmount)

	total := taxableAmount
	if total < 0 {
		total = 0
	}

	campaigns := make([]QuoteAppliedCampaign, 0, len(discountResult.AppliedCampaigns))
	for _, c := range discountResult.AppliedCampaigns {
		campaigns = append(campaigns, QuoteAppliedCampaign{
			ID:          c.ID,
			Name:        c.Name,
			Description: c.Description,
			Amount:      c.Amount,
		})
	}
	var coupon *QuoteAppliedCoupon
	if discountResult.AppliedCoupon != nil {
		coupon = &QuoteAppliedCoupon{
			ID:          discountResult.AppliedCoupon.ID,
			Code:        discountResult.AppliedCoupon.Code,
			Description: discountResult.AppliedCoupon.Description,
			Amount:      discountResult.AppliedCoupon.Amount,
		}
	}

	return &QuoteResult{
		Subtotal:         subtotal,
		AppliedCampaigns: campaigns,
		AppliedCoupon:    coupon,
		TotalDiscount:    discountAmount,
		TaxAmount:        taxAmount,
		TaxInclusive:     taxInclusive,
		ShippingFree:     shippingFree,
		Total:            total,
		CouponError:      couponErr,
		CouponErrorCode:  couponErrCode,
	}, nil
}

// loadPricingLines builds pricing.LineItem rows from cart items for the
// Quote path. Checkout has its own equivalent loop because it also reads
// SKU / product name / bundle components for the order_items insert and
// stock decrement; Quote only needs the discount inputs, so this slimmer
// helper avoids touching the working Checkout loop.
func (s *OrderService) loadPricingLines(ctx context.Context, items []CartItem) ([]pricing.LineItem, float64, error) {
	out := make([]pricing.LineItem, 0, len(items))
	var subtotal float64
	for _, item := range items {
		var (
			price       float64
			productID   string
			categoryID  *string
			productName string
			variantName sql.NullString
			kind        string
			sku         string
		)
		err := s.db.QueryRowContext(ctx,
			`SELECT pv.sku, pv.price, pv.product_id, p.category_id, p.name, pv.name, p.kind
			 FROM product_variants pv
			 JOIN products p ON p.id = pv.product_id
			 WHERE pv.id = $1`, item.VariantID).
			Scan(&sku, &price, &productID, &categoryID, &productName, &variantName, &kind)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, 0, fmt.Errorf("variant %s not found", item.VariantID)
		}
		if err != nil {
			return nil, 0, err
		}
		out = append(out, pricing.LineItem{
			VariantID:  item.VariantID,
			ProductID:  productID,
			CategoryID: categoryID,
			Price:      price,
			Quantity:   item.Quantity,
		})
		subtotal += price * float64(item.Quantity)
	}
	return out, subtotal, nil
}
