// Package tax computes a single global tax rate for orders. MVP: one rate per
// store, configured via site_settings. Multi-region / per-category tax is a
// follow-up.
package tax

import (
	"context"
	"strconv"
	"strings"

	"gyeon/backend/internal/settings"
)

type Service struct {
	settings *settings.Service
}

func NewService(s *settings.Service) *Service {
	return &Service{settings: s}
}

// Result is the computed tax for one order.
type Result struct {
	Enabled   bool
	Rate      float64 // e.g. 0.05 = 5%
	Label     string
	Inclusive bool
	// TaxAmount is in the same units as the input subtotal (HKD, not cents).
	TaxAmount float64
}

// Calculate returns the tax breakdown for the given taxable amount (post-
// discount, pre-shipping). When tax is disabled, all numeric fields are 0.
//
// Inclusive (tax-included pricing): tax = total - total / (1 + rate)
// Exclusive (tax-added pricing):    tax = taxable * rate
//
// taxableAmount is in HKD (not cents) to match the rest of the order math.
func (s *Service) Calculate(ctx context.Context, taxableAmount float64) Result {
	r := Result{
		Enabled:   strings.EqualFold(s.read(ctx, "tax_enabled"), "true"),
		Label:     s.read(ctx, "tax_label"),
		Inclusive: strings.EqualFold(s.read(ctx, "tax_inclusive"), "true"),
	}
	if r.Label == "" {
		r.Label = "Tax"
	}

	rateStr := s.read(ctx, "tax_rate")
	rate, err := strconv.ParseFloat(rateStr, 64)
	if err != nil || rate < 0 {
		rate = 0
	}
	r.Rate = rate

	if !r.Enabled || rate == 0 || taxableAmount <= 0 {
		return r
	}

	if r.Inclusive {
		// Back out the embedded tax from a tax-included subtotal.
		r.TaxAmount = taxableAmount - taxableAmount/(1+rate)
	} else {
		r.TaxAmount = taxableAmount * rate
	}
	return r
}

func (s *Service) read(ctx context.Context, key string) string {
	st, err := s.settings.Get(ctx, key)
	if err != nil {
		return ""
	}
	return st.Value
}
