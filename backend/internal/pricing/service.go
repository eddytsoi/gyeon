package pricing

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"strings"
	"time"
)

var ErrCouponNotFound = errors.New("coupon not found")
var ErrCouponExpired = errors.New("coupon has expired")
var ErrCouponExhausted = errors.New("coupon usage limit reached")
var ErrCouponMinOrder = errors.New("order amount below coupon minimum")
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
	TargetID       *string      `json:"target_id,omitempty"`
	MinOrderAmount *float64     `json:"min_order_amount,omitempty"`
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
	MaxUses        *int         `json:"max_uses,omitempty"`
	UsedCount      int          `json:"used_count"`
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
	TargetID       *string      `json:"target_id"`
	MinOrderAmount *float64     `json:"min_order_amount"`
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
	MaxUses        *int         `json:"max_uses"`
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

// DiscountResult breaks down all discounts applied at checkout.
type DiscountResult struct {
	CampaignDiscount float64
	CouponDiscount   float64
	TotalDiscount    float64
	CouponID         *string
}

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// --- Campaign CRUD ---

func (s *Service) ListCampaigns(ctx context.Context) ([]Campaign, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, description, discount_type, discount_value, target_type, target_id,
		        min_order_amount, starts_at, ends_at, is_active, created_at, updated_at
		 FROM discount_campaigns ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	campaigns := make([]Campaign, 0)
	for rows.Next() {
		var c Campaign
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.DiscountType, &c.DiscountValue,
			&c.TargetType, &c.TargetID, &c.MinOrderAmount, &c.StartsAt, &c.EndsAt,
			&c.IsActive, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		campaigns = append(campaigns, c)
	}
	return campaigns, rows.Err()
}

func (s *Service) GetCampaign(ctx context.Context, id string) (*Campaign, error) {
	var c Campaign
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, description, discount_type, discount_value, target_type, target_id,
		        min_order_amount, starts_at, ends_at, is_active, created_at, updated_at
		 FROM discount_campaigns WHERE id = $1`, id).
		Scan(&c.ID, &c.Name, &c.Description, &c.DiscountType, &c.DiscountValue,
			&c.TargetType, &c.TargetID, &c.MinOrderAmount, &c.StartsAt, &c.EndsAt,
			&c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCampaignNotFound
	}
	return &c, err
}

func (s *Service) CreateCampaign(ctx context.Context, req CreateCampaignRequest) (*Campaign, error) {
	var c Campaign
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO discount_campaigns (name, description, discount_type, discount_value, target_type, target_id, min_order_amount, starts_at, ends_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, name, description, discount_type, discount_value, target_type, target_id,
		           min_order_amount, starts_at, ends_at, is_active, created_at, updated_at`,
		req.Name, req.Description, req.DiscountType, req.DiscountValue,
		req.TargetType, req.TargetID, req.MinOrderAmount, req.StartsAt, req.EndsAt).
		Scan(&c.ID, &c.Name, &c.Description, &c.DiscountType, &c.DiscountValue,
			&c.TargetType, &c.TargetID, &c.MinOrderAmount, &c.StartsAt, &c.EndsAt,
			&c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	return &c, err
}

func (s *Service) UpdateCampaign(ctx context.Context, id string, req UpdateCampaignRequest) (*Campaign, error) {
	var c Campaign
	err := s.db.QueryRowContext(ctx,
		`UPDATE discount_campaigns
		 SET name=$2, description=$3, discount_type=$4, discount_value=$5,
		     target_type=$6, target_id=$7, min_order_amount=$8, starts_at=$9, ends_at=$10, is_active=$11
		 WHERE id=$1
		 RETURNING id, name, description, discount_type, discount_value, target_type, target_id,
		           min_order_amount, starts_at, ends_at, is_active, created_at, updated_at`,
		id, req.Name, req.Description, req.DiscountType, req.DiscountValue,
		req.TargetType, req.TargetID, req.MinOrderAmount, req.StartsAt, req.EndsAt, req.IsActive).
		Scan(&c.ID, &c.Name, &c.Description, &c.DiscountType, &c.DiscountValue,
			&c.TargetType, &c.TargetID, &c.MinOrderAmount, &c.StartsAt, &c.EndsAt,
			&c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCampaignNotFound
	}
	return &c, err
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

func (s *Service) ListCoupons(ctx context.Context) ([]Coupon, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, code, description, discount_type, discount_value, min_order_amount,
		        max_uses, used_count, starts_at, ends_at, is_active, created_at, updated_at
		 FROM coupon_codes ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	coupons := make([]Coupon, 0)
	for rows.Next() {
		var c Coupon
		if err := rows.Scan(&c.ID, &c.Code, &c.Description, &c.DiscountType, &c.DiscountValue,
			&c.MinOrderAmount, &c.MaxUses, &c.UsedCount, &c.StartsAt, &c.EndsAt,
			&c.IsActive, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		coupons = append(coupons, c)
	}
	return coupons, rows.Err()
}

func (s *Service) GetCoupon(ctx context.Context, id string) (*Coupon, error) {
	var c Coupon
	err := s.db.QueryRowContext(ctx,
		`SELECT id, code, description, discount_type, discount_value, min_order_amount,
		        max_uses, used_count, starts_at, ends_at, is_active, created_at, updated_at
		 FROM coupon_codes WHERE id = $1`, id).
		Scan(&c.ID, &c.Code, &c.Description, &c.DiscountType, &c.DiscountValue,
			&c.MinOrderAmount, &c.MaxUses, &c.UsedCount, &c.StartsAt, &c.EndsAt,
			&c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCouponNotFound
	}
	return &c, err
}

func (s *Service) CreateCoupon(ctx context.Context, req CreateCouponRequest) (*Coupon, error) {
	var c Coupon
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO coupon_codes (code, description, discount_type, discount_value, min_order_amount, max_uses, starts_at, ends_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, code, description, discount_type, discount_value, min_order_amount,
		           max_uses, used_count, starts_at, ends_at, is_active, created_at, updated_at`,
		strings.ToUpper(req.Code), req.Description, req.DiscountType, req.DiscountValue,
		req.MinOrderAmount, req.MaxUses, req.StartsAt, req.EndsAt).
		Scan(&c.ID, &c.Code, &c.Description, &c.DiscountType, &c.DiscountValue,
			&c.MinOrderAmount, &c.MaxUses, &c.UsedCount, &c.StartsAt, &c.EndsAt,
			&c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	return &c, err
}

func (s *Service) UpdateCoupon(ctx context.Context, id string, req UpdateCouponRequest) (*Coupon, error) {
	var c Coupon
	err := s.db.QueryRowContext(ctx,
		`UPDATE coupon_codes
		 SET code=$2, description=$3, discount_type=$4, discount_value=$5,
		     min_order_amount=$6, max_uses=$7, starts_at=$8, ends_at=$9, is_active=$10
		 WHERE id=$1
		 RETURNING id, code, description, discount_type, discount_value, min_order_amount,
		           max_uses, used_count, starts_at, ends_at, is_active, created_at, updated_at`,
		id, strings.ToUpper(req.Code), req.Description, req.DiscountType, req.DiscountValue,
		req.MinOrderAmount, req.MaxUses, req.StartsAt, req.EndsAt, req.IsActive).
		Scan(&c.ID, &c.Code, &c.Description, &c.DiscountType, &c.DiscountValue,
			&c.MinOrderAmount, &c.MaxUses, &c.UsedCount, &c.StartsAt, &c.EndsAt,
			&c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCouponNotFound
	}
	return &c, err
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

// ValidateCoupon checks the coupon code and returns the coupon if valid for the given subtotal.
// It does NOT increment used_count — that happens atomically in the checkout transaction.
func (s *Service) ValidateCoupon(ctx context.Context, code string, subtotal float64) (*Coupon, error) {
	var c Coupon
	err := s.db.QueryRowContext(ctx,
		`SELECT id, code, description, discount_type, discount_value, min_order_amount,
		        max_uses, used_count, starts_at, ends_at, is_active, created_at, updated_at
		 FROM coupon_codes
		 WHERE code = $1 AND is_active = TRUE
		   AND (starts_at IS NULL OR starts_at <= NOW())
		   AND (ends_at   IS NULL OR ends_at   >= NOW())`,
		strings.ToUpper(code)).
		Scan(&c.ID, &c.Code, &c.Description, &c.DiscountType, &c.DiscountValue,
			&c.MinOrderAmount, &c.MaxUses, &c.UsedCount, &c.StartsAt, &c.EndsAt,
			&c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCouponNotFound
	}
	if err != nil {
		return nil, err
	}
	if c.MaxUses != nil && c.UsedCount >= *c.MaxUses {
		return nil, ErrCouponExhausted
	}
	if c.MinOrderAmount != nil && subtotal < *c.MinOrderAmount {
		return nil, ErrCouponMinOrder
	}
	return &c, nil
}

// ComputeDiscount calculates discounts from active campaigns and an optional coupon code.
// Campaign discounts are applied first; the coupon applies to the post-campaign subtotal.
// Callers must call IncrementCouponUsage within their checkout transaction if result.CouponID != nil.
func (s *Service) ComputeDiscount(ctx context.Context, items []LineItem, subtotal float64, couponCode *string) (DiscountResult, error) {
	result := DiscountResult{}

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

	// Fetch active campaigns (date-range filtered)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, discount_type, discount_value, target_type, target_id, min_order_amount
		 FROM discount_campaigns
		 WHERE is_active = TRUE
		   AND (starts_at IS NULL OR starts_at <= NOW())
		   AND (ends_at   IS NULL OR ends_at   >= NOW())`)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	var campaignDiscount float64
	seen := make(map[string]bool) // avoid double-applying the same campaign

	for rows.Next() {
		var id string
		var dtype DiscountType
		var value float64
		var ttype TargetType
		var targetID *string
		var minOrder *float64

		if err := rows.Scan(&id, &dtype, &value, &ttype, &targetID, &minOrder); err != nil {
			return result, err
		}
		if seen[id] {
			continue
		}
		if minOrder != nil && subtotal < *minOrder {
			continue
		}

		var applicable float64
		switch ttype {
		case TargetAll:
			applicable = subtotal
		case TargetProduct:
			if targetID != nil {
				// sum line totals for items matching this product
				for _, item := range items {
					if item.ProductID == *targetID {
						applicable += lineTotals[item.VariantID]
					}
				}
			}
		case TargetCategory:
			if targetID != nil {
				for _, item := range items {
					if item.CategoryID != nil && *item.CategoryID == *targetID {
						applicable += lineTotals[item.VariantID]
					}
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
		campaignDiscount += d
		seen[id] = true
	}
	if err := rows.Err(); err != nil {
		return result, err
	}

	// Cap campaign discount at subtotal
	campaignDiscount = math.Min(campaignDiscount, subtotal)
	result.CampaignDiscount = campaignDiscount

	discountedSubtotal := subtotal - campaignDiscount

	// Apply coupon on top
	if couponCode != nil && *couponCode != "" {
		coupon, err := s.ValidateCoupon(ctx, *couponCode, discountedSubtotal)
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
	}

	result.TotalDiscount = result.CampaignDiscount + result.CouponDiscount
	return result, nil
}

// IncrementCouponUsage atomically increments the used_count.
// Call this inside the checkout transaction using the tx connection.
func IncrementCouponUsage(ctx context.Context, tx *sql.Tx, couponID string) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE coupon_codes SET used_count = used_count + 1 WHERE id = $1`, couponID)
	return err
}
