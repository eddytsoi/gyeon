package pricing

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/lib/pq"

	"gyeon/backend/internal/customers"
)

var ErrCouponNotFound = errors.New("coupon not found")
var ErrCouponExpired = errors.New("coupon has expired")
var ErrCouponExhausted = errors.New("coupon usage limit reached")
var ErrCouponMinOrder = errors.New("order amount below coupon minimum")
var ErrCouponMaxOrder = errors.New("order amount above coupon maximum")
var ErrCouponWrongRole = errors.New("coupon not valid for this account type")
var ErrCampaignNotFound = errors.New("campaign not found")

type DiscountType string

const (
	DiscountPercentage DiscountType = "percentage"
	DiscountFixed      DiscountType = "fixed"
)

type TargetType string

const (
	TargetAll      TargetType = "all"
	TargetCategory TargetType = "category"
	TargetProduct  TargetType = "product"
)

type Campaign struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	Description    *string      `json:"description,omitempty"`
	DiscountType   DiscountType `json:"discount_type"`
	DiscountValue  float64      `json:"discount_value"`
	TargetType     TargetType   `json:"target_type"`
	TargetIDs      []string     `json:"target_ids"`
	MinOrderAmount *float64     `json:"min_order_amount,omitempty"`
	MaxOrderAmount *float64     `json:"max_order_amount,omitempty"`
	AllowedRoles   []string     `json:"allowed_roles"`
	AllowGuests    bool         `json:"allow_guests"`
	StartsAt       *time.Time   `json:"starts_at,omitempty"`
	EndsAt         *time.Time   `json:"ends_at,omitempty"`
	IsActive       bool         `json:"is_active"`
	CreatedAt      string       `json:"created_at"`
	UpdatedAt      string       `json:"updated_at"`
}

type Coupon struct {
	ID             string       `json:"id"`
	Code           string       `json:"code"`
	Description    *string      `json:"description,omitempty"`
	DiscountType   DiscountType `json:"discount_type"`
	DiscountValue  float64      `json:"discount_value"`
	MinOrderAmount *float64     `json:"min_order_amount,omitempty"`
	MaxOrderAmount *float64     `json:"max_order_amount,omitempty"`
	MaxUses        *int         `json:"max_uses,omitempty"`
	UsedCount      int          `json:"used_count"`
	AllowedRoles   []string     `json:"allowed_roles"`
	AllowGuests    bool         `json:"allow_guests"`
	StartsAt       *time.Time   `json:"starts_at,omitempty"`
	EndsAt         *time.Time   `json:"ends_at,omitempty"`
	IsActive       bool         `json:"is_active"`
	CreatedAt      string       `json:"created_at"`
	UpdatedAt      string       `json:"updated_at"`
}

type CreateCampaignRequest struct {
	Name           string       `json:"name"`
	Description    *string      `json:"description"`
	DiscountType   DiscountType `json:"discount_type"`
	DiscountValue  float64      `json:"discount_value"`
	TargetType     TargetType   `json:"target_type"`
	TargetIDs      []string     `json:"target_ids"`
	MinOrderAmount *float64     `json:"min_order_amount"`
	MaxOrderAmount *float64     `json:"max_order_amount"`
	AllowedRoles   []string     `json:"allowed_roles"`
	AllowGuests    bool         `json:"allow_guests"`
	StartsAt       *time.Time   `json:"starts_at"`
	EndsAt         *time.Time   `json:"ends_at"`
}

type UpdateCampaignRequest struct {
	CreateCampaignRequest
	IsActive bool `json:"is_active"`
}

type CreateCouponRequest struct {
	Code           string       `json:"code"`
	Description    *string      `json:"description"`
	DiscountType   DiscountType `json:"discount_type"`
	DiscountValue  float64      `json:"discount_value"`
	MinOrderAmount *float64     `json:"min_order_amount"`
	MaxOrderAmount *float64     `json:"max_order_amount"`
	MaxUses        *int         `json:"max_uses"`
	AllowedRoles   []string     `json:"allowed_roles"`
	AllowGuests    bool         `json:"allow_guests"`
	StartsAt       *time.Time   `json:"starts_at"`
	EndsAt         *time.Time   `json:"ends_at"`
}

type UpdateCouponRequest struct {
	CreateCouponRequest
	IsActive bool `json:"is_active"`
}

// LineItem is the minimal pricing context for a cart item at checkout.
type LineItem struct {
	VariantID  string
	ProductID  string
	CategoryID *string
	Price      float64
	Quantity   int
}

// AppliedCampaign is one campaign that actually contributed to a discount,
// hydrated with the customer-facing name + description so the storefront
// can render "why" the shopper got the discount. Amount is the post-cap
// contribution; the sum of Amounts equals DiscountResult.CampaignDiscount.
type AppliedCampaign struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	Amount      float64 `json:"amount"`
}

// AppliedCoupon mirrors AppliedCampaign for the coupon side.
type AppliedCoupon struct {
	ID          string  `json:"id"`
	Code        string  `json:"code"`
	Description *string `json:"description,omitempty"`
	Amount      float64 `json:"amount"`
}

// DiscountResult breaks down all discounts applied at checkout.
type DiscountResult struct {
	CampaignDiscount float64
	CouponDiscount   float64
	TotalDiscount    float64
	CouponID         *string
	// AppliedCampaigns lists each campaign that contributed, with the amount
	// it actually applied (after per-row cap against remaining subtotal). The
	// storefront uses these to surface promotion names + descriptions on the
	// checkout summary; the order_service persists them as a snapshot so the
	// receipt / account page can render the same later.
	AppliedCampaigns []AppliedCampaign
	AppliedCoupon    *AppliedCoupon
}

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// normalizeRoleList canonicalises an allowed_roles payload: drops blanks,
// maps legacy values via customers.NormalizeRole, dedupes, and preserves
// declaration order.
func normalizeRoleList(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, r := range in {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		canon := customers.NormalizeRole(r)
		if _, ok := seen[canon]; ok {
			continue
		}
		seen[canon] = struct{}{}
		out = append(out, canon)
	}
	return out
}

// normalizeUUIDList trims, lower-cases and dedupes a UUID list, dropping
// empties. Order is preserved.
func normalizeUUIDList(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, id := range in {
		id = strings.TrimSpace(strings.ToLower(id))
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// --- Campaign CRUD ---

const campaignSelectCols = `id, name, description, discount_type, discount_value, target_type, target_ids,
	        min_order_amount, max_order_amount, allowed_roles, allow_guests, starts_at, ends_at, is_active, created_at, updated_at`

func scanCampaign(scanner interface {
	Scan(dest ...any) error
}) (Campaign, error) {
	var c Campaign
	var roles pq.StringArray
	var targetIDs pq.StringArray
	if err := scanner.Scan(&c.ID, &c.Name, &c.Description, &c.DiscountType, &c.DiscountValue,
		&c.TargetType, &targetIDs, &c.MinOrderAmount, &c.MaxOrderAmount, &roles, &c.AllowGuests,
		&c.StartsAt, &c.EndsAt, &c.IsActive, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return c, err
	}
	c.AllowedRoles = []string(roles)
	c.TargetIDs = []string(targetIDs)
	return c, nil
}

func (s *Service) ListCampaigns(ctx context.Context, limit, offset int) ([]Campaign, int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM discount_campaigns`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT `+campaignSelectCols+`
		 FROM discount_campaigns ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	campaigns := make([]Campaign, 0)
	for rows.Next() {
		c, err := scanCampaign(rows)
		if err != nil {
			return nil, 0, err
		}
		campaigns = append(campaigns, c)
	}
	return campaigns, total, rows.Err()
}

func (s *Service) GetCampaign(ctx context.Context, id string) (*Campaign, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+campaignSelectCols+` FROM discount_campaigns WHERE id = $1`, id)
	c, err := scanCampaign(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCampaignNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Service) CreateCampaign(ctx context.Context, req CreateCampaignRequest) (*Campaign, error) {
	roles := normalizeRoleList(req.AllowedRoles)
	targets := normalizeUUIDList(req.TargetIDs)
	row := s.db.QueryRowContext(ctx,
		`INSERT INTO discount_campaigns
		   (name, description, discount_type, discount_value, target_type, target_ids,
		    min_order_amount, max_order_amount, allowed_roles, allow_guests, starts_at, ends_at)
		 VALUES ($1, $2, $3, $4, $5, $6::uuid[], $7, $8, $9::customer_role[], $10, $11, $12)
		 RETURNING `+campaignSelectCols,
		req.Name, req.Description, req.DiscountType, req.DiscountValue,
		req.TargetType, pq.Array(targets), req.MinOrderAmount, req.MaxOrderAmount, pq.Array(roles),
		req.AllowGuests, req.StartsAt, req.EndsAt)
	c, err := scanCampaign(row)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Service) UpdateCampaign(ctx context.Context, id string, req UpdateCampaignRequest) (*Campaign, error) {
	roles := normalizeRoleList(req.AllowedRoles)
	targets := normalizeUUIDList(req.TargetIDs)
	row := s.db.QueryRowContext(ctx,
		`UPDATE discount_campaigns
		   SET name=$2, description=$3, discount_type=$4, discount_value=$5,
		       target_type=$6, target_ids=$7::uuid[], min_order_amount=$8, max_order_amount=$9,
		       allowed_roles=$10::customer_role[], allow_guests=$11,
		       starts_at=$12, ends_at=$13, is_active=$14
		 WHERE id=$1
		 RETURNING `+campaignSelectCols,
		id, req.Name, req.Description, req.DiscountType, req.DiscountValue,
		req.TargetType, pq.Array(targets), req.MinOrderAmount, req.MaxOrderAmount, pq.Array(roles),
		req.AllowGuests, req.StartsAt, req.EndsAt, req.IsActive)
	c, err := scanCampaign(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCampaignNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Service) DeleteCampaign(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM discount_campaigns WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrCampaignNotFound
	}
	return nil
}

// --- Coupon CRUD ---

const couponSelectCols = `id, code, description, discount_type, discount_value, min_order_amount, max_order_amount,
	        max_uses, used_count, allowed_roles, allow_guests, starts_at, ends_at, is_active, created_at, updated_at`

func scanCoupon(scanner interface {
	Scan(dest ...any) error
}) (Coupon, error) {
	var c Coupon
	var roles pq.StringArray
	if err := scanner.Scan(&c.ID, &c.Code, &c.Description, &c.DiscountType, &c.DiscountValue,
		&c.MinOrderAmount, &c.MaxOrderAmount, &c.MaxUses, &c.UsedCount, &roles, &c.AllowGuests,
		&c.StartsAt, &c.EndsAt, &c.IsActive, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return c, err
	}
	c.AllowedRoles = []string(roles)
	return c, nil
}

func (s *Service) ListCoupons(ctx context.Context, limit, offset int) ([]Coupon, int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM coupon_codes`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT `+couponSelectCols+`
		 FROM coupon_codes ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	coupons := make([]Coupon, 0)
	for rows.Next() {
		c, err := scanCoupon(rows)
		if err != nil {
			return nil, 0, err
		}
		coupons = append(coupons, c)
	}
	return coupons, total, rows.Err()
}

func (s *Service) GetCoupon(ctx context.Context, id string) (*Coupon, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+couponSelectCols+` FROM coupon_codes WHERE id = $1`, id)
	c, err := scanCoupon(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCouponNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Service) CreateCoupon(ctx context.Context, req CreateCouponRequest) (*Coupon, error) {
	roles := normalizeRoleList(req.AllowedRoles)
	row := s.db.QueryRowContext(ctx,
		`INSERT INTO coupon_codes
		   (code, description, discount_type, discount_value, min_order_amount, max_order_amount, max_uses,
		    allowed_roles, allow_guests, starts_at, ends_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8::customer_role[], $9, $10, $11)
		 RETURNING `+couponSelectCols,
		strings.ToUpper(req.Code), req.Description, req.DiscountType, req.DiscountValue,
		req.MinOrderAmount, req.MaxOrderAmount, req.MaxUses, pq.Array(roles), req.AllowGuests,
		req.StartsAt, req.EndsAt)
	c, err := scanCoupon(row)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Service) UpdateCoupon(ctx context.Context, id string, req UpdateCouponRequest) (*Coupon, error) {
	roles := normalizeRoleList(req.AllowedRoles)
	row := s.db.QueryRowContext(ctx,
		`UPDATE coupon_codes
		   SET code=$2, description=$3, discount_type=$4, discount_value=$5,
		       min_order_amount=$6, max_order_amount=$7, max_uses=$8, allowed_roles=$9::customer_role[],
		       allow_guests=$10, starts_at=$11, ends_at=$12, is_active=$13
		 WHERE id=$1
		 RETURNING `+couponSelectCols,
		id, strings.ToUpper(req.Code), req.Description, req.DiscountType, req.DiscountValue,
		req.MinOrderAmount, req.MaxOrderAmount, req.MaxUses, pq.Array(roles), req.AllowGuests,
		req.StartsAt, req.EndsAt, req.IsActive)
	c, err := scanCoupon(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCouponNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Service) DeleteCoupon(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM coupon_codes WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrCouponNotFound
	}
	return nil
}

// --- Discount computation ---

// ValidateCoupon checks the coupon code and returns the coupon if valid for
// the given subtotal and shopper context (customerRole + isGuest).
//
// When isGuest is true the coupon must have allow_guests=true; otherwise
// customerRole (normalized) must appear in allowed_roles. customerRole is
// ignored when isGuest is true.
//
// Does NOT increment used_count — that happens atomically in the checkout
// transaction.
func (s *Service) ValidateCoupon(ctx context.Context, code string, subtotal float64, customerRole string, isGuest bool) (*Coupon, error) {
	role := customers.NormalizeRole(customerRole)
	row := s.db.QueryRowContext(ctx,
		`SELECT `+couponSelectCols+`
		 FROM coupon_codes
		 WHERE code = $1 AND is_active = TRUE
		   AND (starts_at IS NULL OR starts_at <= NOW())
		   AND (ends_at   IS NULL OR ends_at   >= NOW())`,
		strings.ToUpper(code))
	c, err := scanCoupon(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCouponNotFound
	}
	if err != nil {
		return nil, err
	}
	if !shopperAllowed(c.AllowedRoles, c.AllowGuests, role, isGuest) {
		return nil, ErrCouponWrongRole
	}
	if c.MaxUses != nil && c.UsedCount >= *c.MaxUses {
		return nil, ErrCouponExhausted
	}
	if c.MinOrderAmount != nil && subtotal < *c.MinOrderAmount {
		return nil, ErrCouponMinOrder
	}
	if c.MaxOrderAmount != nil && subtotal > *c.MaxOrderAmount {
		return nil, ErrCouponMaxOrder
	}
	return &c, nil
}

// shopperAllowed reports whether the given shopper context is eligible for a
// row with the given role + guest gating. Guests check allow_guests; logged-
// in shoppers must have their role in allowed_roles.
func shopperAllowed(allowedRoles []string, allowGuests bool, role string, isGuest bool) bool {
	if isGuest {
		return allowGuests
	}
	for _, r := range allowedRoles {
		if r == role {
			return true
		}
	}
	return false
}

// ComputeDiscount calculates discounts from active campaigns and an optional
// coupon code, scoped to the shopper. Campaign discounts are applied first;
// the coupon applies to the post-campaign subtotal. Callers must call
// IncrementCouponUsage within their checkout transaction if result.CouponID
// is non-nil.
func (s *Service) ComputeDiscount(ctx context.Context, items []LineItem, subtotal float64, couponCode *string, customerRole string, isGuest bool) (DiscountResult, error) {
	result := DiscountResult{}
	role := customers.NormalizeRole(customerRole)

	// Build sets for fast lookup
	productIDs := make(map[string]bool)
	categoryIDs := make(map[string]bool)
	lineTotals := make(map[string]float64) // variantID -> line total
	for _, item := range items {
		productIDs[item.ProductID] = true
		if item.CategoryID != nil {
			categoryIDs[*item.CategoryID] = true
		}
		lineTotals[item.VariantID] = item.Price * float64(item.Quantity)
	}

	// Fetch active campaigns scoped to the shopper. Filter happens in SQL
	// for guests/role membership so we don't pull rows we'd just discard.
	// name + description are pulled too so AppliedCampaigns can be hydrated
	// without a second round-trip.
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, description, discount_type, discount_value, target_type, target_ids, min_order_amount, max_order_amount
		 FROM discount_campaigns
		 WHERE is_active = TRUE
		   AND ($1::bool AND allow_guests
		        OR NOT $1::bool AND $2::customer_role = ANY(allowed_roles))
		   AND (starts_at IS NULL OR starts_at <= NOW())
		   AND (ends_at   IS NULL OR ends_at   >= NOW())`, isGuest, role)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	var campaignDiscount float64
	remaining := subtotal // clamp each campaign's amount so the per-row Amounts sum to CampaignDiscount
	seen := make(map[string]bool) // avoid double-applying the same campaign
	var applied []AppliedCampaign

	for rows.Next() {
		var id, name string
		var description *string
		var dtype DiscountType
		var value float64
		var ttype TargetType
		var targetIDs pq.StringArray
		var minOrder, maxOrder *float64

		if err := rows.Scan(&id, &name, &description, &dtype, &value, &ttype, &targetIDs, &minOrder, &maxOrder); err != nil {
			return result, err
		}
		if seen[id] {
			continue
		}
		if minOrder != nil && subtotal < *minOrder {
			continue
		}
		if maxOrder != nil && subtotal > *maxOrder {
			continue
		}

		var applicable float64
		switch ttype {
		case TargetAll:
			applicable = subtotal
		case TargetProduct:
			if len(targetIDs) == 0 {
				continue
			}
			idSet := uuidSet(targetIDs)
			for _, item := range items {
				if idSet[item.ProductID] {
					applicable += lineTotals[item.VariantID]
				}
			}
		case TargetCategory:
			if len(targetIDs) == 0 {
				continue
			}
			idSet := uuidSet(targetIDs)
			for _, item := range items {
				if item.CategoryID != nil && idSet[*item.CategoryID] {
					applicable += lineTotals[item.VariantID]
				}
			}
		}

		if applicable <= 0 {
			continue
		}

		var d float64
		switch dtype {
		case DiscountPercentage:
			d = applicable * (value / 100)
		case DiscountFixed:
			d = math.Min(value, applicable)
		}
		// Clamp to subtotal headroom in declaration order so multiple
		// stacked campaigns can never push the discount past the cart total
		// — and so the per-row Amount reconciles with CampaignDiscount.
		if d > remaining {
			d = remaining
		}
		if d <= 0 {
			continue
		}
		applied = append(applied, AppliedCampaign{
			ID:          id,
			Name:        name,
			Description: description,
			Amount:      d,
		})
		campaignDiscount += d
		remaining -= d
		seen[id] = true
	}
	if err := rows.Err(); err != nil {
		return result, err
	}

	result.CampaignDiscount = campaignDiscount
	result.AppliedCampaigns = applied

	discountedSubtotal := subtotal - campaignDiscount

	// Apply coupon on top
	if couponCode != nil && *couponCode != "" {
		coupon, err := s.ValidateCoupon(ctx, *couponCode, discountedSubtotal, role, isGuest)
		if err != nil {
			return result, err
		}

		var couponDiscount float64
		switch coupon.DiscountType {
		case DiscountPercentage:
			couponDiscount = discountedSubtotal * (coupon.DiscountValue / 100)
		case DiscountFixed:
			couponDiscount = math.Min(coupon.DiscountValue, discountedSubtotal)
		}
		result.CouponDiscount = couponDiscount
		result.CouponID = &coupon.ID
		if couponDiscount > 0 {
			result.AppliedCoupon = &AppliedCoupon{
				ID:          coupon.ID,
				Code:        coupon.Code,
				Description: coupon.Description,
				Amount:      couponDiscount,
			}
		}
	}

	result.TotalDiscount = result.CampaignDiscount + result.CouponDiscount
	return result, nil
}

func uuidSet(ids pq.StringArray) map[string]bool {
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out
}

// IncrementCouponUsage atomically increments the used_count.
// Call this inside the checkout transaction using the tx connection.
func IncrementCouponUsage(ctx context.Context, tx *sql.Tx, couponID string) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE coupon_codes SET used_count = used_count + 1 WHERE id = $1`, couponID)
	return err
}
