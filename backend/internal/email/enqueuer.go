package email

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"gyeon/backend/internal/queue"
	"gyeon/backend/internal/smtplog"
)

// QueueEnqueuer wraps *Service so every transactional email goes through the
// background queue instead of dialing SMTP on the caller's goroutine. The
// QueueEnqueuer satisfies the same method shapes as *Service, so consumers
// (orders, customers, abandoned, notice, importer, forms) can be threaded
// either implementation by accepting a local interface.
//
// The worker dispatches send_email -> HandleSendEmail (template render path)
// and send_email_raw -> HandleSendEmailRaw (verbatim replay for resend).
type QueueEnqueuer struct {
	svc      *Service
	queueSvc QueueAPI
	logStore LogStore
	// Two independent burst-smoothing gates shared by both handlers. A send
	// must clear every enabled gate, so it waits the longer of the two.
	minuteBucket *tokenBucket // per-minute cap (suits Gmail SMTP)
	secondBucket *tokenBucket // per-second cap (suits Resend)
}

// QueueAPI is the slice of queue.Service used by the enqueuer.
type QueueAPI interface {
	Enqueue(ctx context.Context, jobType string, payload []byte, opts ...queue.EnqueueOptions) (string, error)
}

// LogStore is the slice of smtplog.Store the worker writes to. CountSentSince
// backs the rolling daily-limit gate.
type LogStore interface {
	Insert(ctx context.Context, in smtplog.InsertInput) (string, error)
	CountSentSince(ctx context.Context, since time.Time) (int, error)
}

func NewQueueEnqueuer(svc *Service, q QueueAPI, ls LogStore) *QueueEnqueuer {
	return &QueueEnqueuer{
		svc: svc, queueSvc: q, logStore: ls,
		minuteBucket: newTokenBucket(60),
		secondBucket: newTokenBucket(1),
	}
}

// PublicBaseURL is delegated so existing call sites don't change.
func (e *QueueEnqueuer) PublicBaseURL(ctx context.Context) string {
	return e.svc.PublicBaseURL(ctx)
}

// ── Job payloads ──────────────────────────────────────────────────────────

// SendEmailJob is the queue payload for any of the 8 templated transactional
// emails. The worker dispatches on TemplateKey to unmarshal Params into the
// matching typed struct, then renders + sends.
type SendEmailJob struct {
	TemplateKey       string          `json:"template_key"`
	Recipient         string          `json:"recipient"`
	Params            json.RawMessage `json:"params"`
	TriggerCondition  string          `json:"trigger_condition"`
	RelatedEntityType string          `json:"related_entity_type,omitempty"`
	RelatedEntityID   string          `json:"related_entity_id,omitempty"`
}

// SendContactFormJob carries the contact-form notification or auto-reply.
type SendContactFormJob struct {
	Kind             string            `json:"kind"` // "notification" | "auto_reply"
	Params           ContactFormParams `json:"params"`
	TriggerCondition string            `json:"trigger_condition"`
}

// SendEmailRawJob bypasses templates — used by the SMTP-log Resend action
// to replay a captured payload verbatim. Same shape as the smtplog handler's
// local copy; the worker decodes either since they share the JSON layout.
type SendEmailRawJob struct {
	LogID             string `json:"log_id"`
	Recipient         string `json:"recipient"`
	Subject           string `json:"subject"`
	BodyHTML          string `json:"body_html"`
	BodyText          string `json:"body_text"`
	ReplyTo           string `json:"reply_to,omitempty"`
	TriggerCondition  string `json:"trigger_condition"`
	RelatedEntityType string `json:"related_entity_type,omitempty"`
	RelatedEntityID   string `json:"related_entity_id,omitempty"`
}

// SendEmailBatchJob carries up to 100 templated emails delivered in one Resend
// /emails/batch request (see HandleSendEmailBatch). Used by the customer import
// to coalesce thousands of per-customer setup-password mails into ~N/100 calls.
// Every recipient shares TemplateKey but carries its own Params (e.g. a unique
// setup link), so the batch is 100 distinct mails, not one fanned to many.
type SendEmailBatchJob struct {
	TemplateKey      string           `json:"template_key"`
	TriggerCondition string           `json:"trigger_condition"`
	Recipients       []BatchRecipient `json:"recipients"`
}

// BatchRecipient is one mail in a SendEmailBatchJob: the address plus the typed
// template params (same JSON shape decodeParams expects for TemplateKey).
type BatchRecipient struct {
	Recipient string          `json:"recipient"`
	Params    json.RawMessage `json:"params"`
}

// ── Send methods (mirror Service signatures) ─────────────────────────────

func (e *QueueEnqueuer) SendOrderConfirmation(ctx context.Context, p OrderEmailParams) error {
	return e.enqueueTemplated(ctx, "order_confirmation", p.CustomerEmail, p, "order.paid", "order", p.OrderID)
}

// SendNewOrderAlert queues the internal "new order received" notification to the
// admin_alert_email recipient. Opt-in: when that setting is empty it silently
// no-ops (no from-email fallback) so routine orders don't email the store's own
// sender address. The recipient is resolved here so the smtp_log row records who
// the alert actually went to.
func (e *QueueEnqueuer) SendNewOrderAlert(ctx context.Context, p OrderEmailParams) error {
	to := strings.TrimSpace(e.svc.read(ctx, "admin_alert_email"))
	if to == "" {
		return nil
	}
	return e.enqueueTemplated(ctx, "new_order_alert", to, p, "order.new", "order", p.OrderID)
}

func (e *QueueEnqueuer) SendOrderShipped(ctx context.Context, p ShippedEmailParams) error {
	return e.enqueueTemplated(ctx, "order_shipped", p.CustomerEmail, p, "order.shipped", "order", p.OrderID)
}

func (e *QueueEnqueuer) SendOrderRefunded(ctx context.Context, p RefundEmailParams) error {
	return e.enqueueTemplated(ctx, "order_refunded", p.CustomerEmail, p, "order.refund", "order", p.OrderID)
}

func (e *QueueEnqueuer) SendOrderCancelledUnpaid(ctx context.Context, p OrderCancelledUnpaidParams) error {
	return e.enqueueTemplated(ctx, "order_cancelled_unpaid", p.CustomerEmail, p, "order.expired", "order", p.OrderID)
}

func (e *QueueEnqueuer) SendPaymentLink(ctx context.Context, p PaymentLinkParams) error {
	return e.enqueueTemplated(ctx, "payment_link", p.CustomerEmail, p, "checkout.payment_link", "order", p.OrderID)
}

func (e *QueueEnqueuer) SendBankTransferOnHold(ctx context.Context, p BankTransferOnHoldParams) error {
	return e.enqueueTemplated(ctx, "bank_transfer_on_hold", p.CustomerEmail, p, "order.bank_transfer_on_hold", "order", p.OrderID)
}

func (e *QueueEnqueuer) SendPasswordResetEmail(ctx context.Context, p PasswordResetParams) error {
	if p.ExpiryHours == 0 {
		p.ExpiryHours = 24
	}
	return e.enqueueTemplated(ctx, "password_reset", p.CustomerEmail, p, "auth.password_reset", "", "")
}

func (e *QueueEnqueuer) SendAccountSetupEmail(ctx context.Context, p PasswordResetParams) error {
	if p.ExpiryHours == 0 {
		p.ExpiryHours = 24
	}
	return e.enqueueTemplated(ctx, "account_setup", p.CustomerEmail, p, "auth.account_setup", "", "")
}

// EnqueueAccountSetupBatch enqueues one send_email_batch job carrying up to 100
// account-setup mails delivered via Resend's batch endpoint. The customer
// import buffers eligible customers and calls this per ≤100-recipient chunk so a
// few-thousand-customer run becomes ~N/100 HTTP requests instead of N. Empty
// recipients (no email) are dropped; an all-empty slice is a no-op.
func (e *QueueEnqueuer) EnqueueAccountSetupBatch(ctx context.Context, params []PasswordResetParams) error {
	recips := make([]BatchRecipient, 0, len(params))
	for _, p := range params {
		if p.CustomerEmail == "" {
			continue
		}
		if p.ExpiryHours == 0 {
			p.ExpiryHours = 24
		}
		raw, err := json.Marshal(p)
		if err != nil {
			return fmt.Errorf("marshal account_setup params: %w", err)
		}
		recips = append(recips, BatchRecipient{Recipient: p.CustomerEmail, Params: raw})
	}
	if len(recips) == 0 {
		return nil
	}
	job := SendEmailBatchJob{
		TemplateKey:      "account_setup",
		TriggerCondition: "auth.account_setup",
		Recipients:       recips,
	}
	b, err := json.Marshal(job)
	if err != nil {
		return err
	}
	_, err = e.queueSvc.Enqueue(ctx, queue.JobTypeSendEmailBatch, b)
	return err
}

func (e *QueueEnqueuer) SendAdminMessageNotification(ctx context.Context, p AdminMessageParams) error {
	return e.enqueueTemplated(ctx, "admin_message", p.To, p, "order.admin_message", "", "")
}

func (e *QueueEnqueuer) SendAbandonedCart(ctx context.Context, p AbandonedCartParams) error {
	return e.enqueueTemplated(ctx, "abandoned_cart", p.CustomerEmail, p, "cart.abandoned", "", "")
}

func (e *QueueEnqueuer) SendLowStockAlert(ctx context.Context, p LowStockParams) error {
	// Resolve recipient at enqueue time so the worker doesn't have to
	// re-consult settings (and so the smtp_log row reflects the actual
	// recipient at the moment the alert fired).
	to := p.To
	if to == "" {
		to = e.svc.read(ctx, "admin_alert_email")
	}
	if to == "" {
		if from, _, ferr := e.svc.FromConfig(ctx); ferr == nil {
			to = from
		}
	}
	if to == "" {
		return ErrNotConfigured
	}
	p.To = to
	return e.enqueueTemplated(ctx, "low_stock_alert", to, p, "stock.low_crossing", "", "")
}

// SendTest stays synchronous — admin clicks "Test SMTP" and expects a
// pass/fail in the response. Delegate straight to the underlying service.
func (e *QueueEnqueuer) SendTest(ctx context.Context, to string) error {
	return e.svc.SendTest(ctx, to)
}

// SendContactFormNotification queues the admin-side contact-form mail.
func (e *QueueEnqueuer) SendContactFormNotification(ctx context.Context, p ContactFormParams) error {
	return e.enqueueContactForm(ctx, "notification", p, "form.submit")
}

// SendContactFormAutoReply queues the submitter-side auto-reply mail.
func (e *QueueEnqueuer) SendContactFormAutoReply(ctx context.Context, p ContactFormParams) error {
	return e.enqueueContactForm(ctx, "auto_reply", p, "form.submit")
}

func (e *QueueEnqueuer) enqueueTemplated(ctx context.Context, key, recipient string, params any, trigger, entityType, entityID string) error {
	if recipient == "" {
		return ErrNotConfigured
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("marshal %s params: %w", key, err)
	}
	job := SendEmailJob{
		TemplateKey:       key,
		Recipient:         recipient,
		Params:            paramsJSON,
		TriggerCondition:  trigger,
		RelatedEntityType: entityType,
		RelatedEntityID:   entityID,
	}
	b, err := json.Marshal(job)
	if err != nil {
		return err
	}
	_, err = e.queueSvc.Enqueue(ctx, queue.JobTypeSendEmail, b)
	return err
}

func (e *QueueEnqueuer) enqueueContactForm(ctx context.Context, kind string, p ContactFormParams, trigger string) error {
	if p.To == "" {
		return fmt.Errorf("contact form: empty recipient")
	}
	job := SendContactFormJob{Kind: kind, Params: p, TriggerCondition: trigger}
	b, err := json.Marshal(job)
	if err != nil {
		return err
	}
	// Contact-form mails share the send_email job type so the worker can
	// run them through the same handler with a `kind` switch — fewer
	// registrations to keep in sync. The handler peeks at the payload's
	// `kind` field to decide which path to take.
	_, err = e.queueSvc.Enqueue(ctx, queue.JobTypeSendEmail, b)
	return err
}

// ── Rate limiting ────────────────────────────────────────────────────────

// emailDeferInterval is how far out a daily-capped send is pushed before the
// worker re-checks the rolling window. The 24h window keeps draining as old
// sends age out, so a fixed ~1h re-check (plus per-payload jitter) converges
// without hammering the count query.
const emailDeferInterval = time.Hour

// gateOrDefer enforces the Gmail-safe send budget before a worker handler does
// any SMTP work. It returns proceed=false when the send was deferred (the
// handler should `return err` — nil on a clean defer so the current job is
// completed without burning an attempt). It returns proceed=true to send now.
//
// Three gates, checked in order:
//  1. Daily cap (email_daily_limit): if the rolling-24h smtp_log 'sent' count
//     is at/over the limit, re-enqueue the same payload with a future RunAfter
//     and complete the current job. Sends are deferred, never dropped.
//  2/3. Per-minute cap (email_rate_per_minute, suits Gmail) and per-second cap
//     (email_rate_per_second, suits Resend): independent token buckets, each
//     0-to-disable. A send must clear both, so we reserve from every enabled
//     bucket and wait the longer of the two. A short wait is slept off inline;
//     a wait that would exceed the job's deadline defers via the queue instead
//     of blocking the worker goroutine.
func (e *QueueEnqueuer) gateOrDefer(ctx context.Context, jobType string, payload []byte) (bool, error) {
	if limit := e.intSetting(ctx, "email_daily_limit", 0); limit > 0 {
		sent, err := e.logStore.CountSentSince(ctx, time.Now().Add(-24*time.Hour))
		if err != nil {
			// Don't block delivery on a transient count failure — log and send.
			log.Printf("email rate-limit: count sent failed: %v", err)
		} else if sent >= limit {
			log.Printf("email rate-limit: daily cap reached (%d/%d sent in 24h), deferring %s to %s",
				sent, limit, jobType, recipientOf(payload))
			return false, e.deferSend(ctx, jobType, payload, emailDeferInterval)
		}
	}

	var wait time.Duration
	if perMin := e.intSetting(ctx, "email_rate_per_minute", 0); perMin > 0 {
		if w := e.minuteBucket.reserve(perMin); w > wait {
			wait = w
		}
	}
	if perSec := e.intSetting(ctx, "email_rate_per_second", 0); perSec > 0 {
		if w := e.secondBucket.reserve(perSec); w > wait {
			wait = w
		}
	}
	if wait > 0 {
		if dl, ok := ctx.Deadline(); ok && time.Now().Add(wait).After(dl) {
			// A bucket is starved harder than this job can wait — defer.
			return false, e.deferSend(ctx, jobType, payload, wait)
		}
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return false, ctx.Err() // retryable: worker backoff re-queues
		}
	}
	return true, nil
}

// deferSend re-enqueues payload to run after `base` plus a deterministic
// per-payload jitter (spread over 5 min) so a wave of deferred jobs doesn't
// all wake at once.
func (e *QueueEnqueuer) deferSend(ctx context.Context, jobType string, payload []byte, base time.Duration) error {
	runAfter := time.Now().Add(base + payloadJitter(payload, 5*time.Minute))
	_, err := e.queueSvc.Enqueue(ctx, jobType, payload, queue.EnqueueOptions{RunAfter: runAfter})
	return err
}

// intSetting reads a site setting as an int, falling back to def when unset or
// unparseable. Settings are read fresh each call so admin changes apply on the
// next send without a restart.
func (e *QueueEnqueuer) intSetting(ctx context.Context, key string, def int) int {
	raw := strings.TrimSpace(e.svc.read(ctx, key))
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return n
}

func payloadJitter(payload []byte, spread time.Duration) time.Duration {
	if spread <= 0 {
		return 0
	}
	h := fnv.New64a()
	_, _ = h.Write(payload)
	return time.Duration(h.Sum64() % uint64(spread))
}

// recipientOf best-effort extracts a recipient for the deferral log line. It
// handles both the templated/contact-form (send_email) and raw shapes.
func recipientOf(payload []byte) string {
	var p struct {
		Recipient string `json:"recipient"`
		Params    struct {
			To string `json:"to"`
		} `json:"params"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return "?"
	}
	if p.Recipient != "" {
		return p.Recipient
	}
	if p.Params.To != "" {
		return p.Params.To
	}
	return "?"
}

// ── Worker handlers ──────────────────────────────────────────────────────

// HandleSendEmail is the queue handler for send_email. It dispatches on
// the payload shape: templated transactional emails take the render+log
// path; contact-form jobs take the contact-form path.
func (e *QueueEnqueuer) HandleSendEmail(ctx context.Context, payload []byte) error {
	if proceed, err := e.gateOrDefer(ctx, queue.JobTypeSendEmail, payload); !proceed {
		return err
	}
	// Peek at the payload to decide which job shape applies.
	var probe struct {
		TemplateKey string `json:"template_key"`
		Kind        string `json:"kind"`
	}
	if err := json.Unmarshal(payload, &probe); err != nil {
		return queue.Permanent(fmt.Errorf("decode payload: %w", err))
	}
	if probe.TemplateKey != "" {
		return e.handleTemplated(ctx, payload)
	}
	if probe.Kind != "" {
		return e.handleContactForm(ctx, payload)
	}
	return queue.Permanent(errors.New("send_email payload has neither template_key nor kind"))
}

func (e *QueueEnqueuer) handleTemplated(ctx context.Context, payload []byte) error {
	var job SendEmailJob
	if err := json.Unmarshal(payload, &job); err != nil {
		return queue.Permanent(fmt.Errorf("decode send_email job: %w", err))
	}
	typed, err := decodeParams(job.TemplateKey, job.Params)
	if err != nil {
		return queue.Permanent(err)
	}
	subject, text, html, err := e.svc.RenderTemplate(ctx, job.TemplateKey, typed)
	if err != nil {
		return queue.Permanent(err)
	}
	return e.sendAndLog(ctx, sendArgs{
		TemplateKey:       &job.TemplateKey,
		TriggerCondition:  job.TriggerCondition,
		RelatedEntityType: nullable(job.RelatedEntityType),
		RelatedEntityID:   nullable(job.RelatedEntityID),
		Recipient:         job.Recipient,
		Subject:           subject,
		BodyHTML:          html,
		BodyText:          text,
	})
}

func (e *QueueEnqueuer) handleContactForm(ctx context.Context, payload []byte) error {
	var job SendContactFormJob
	if err := json.Unmarshal(payload, &job); err != nil {
		return queue.Permanent(fmt.Errorf("decode contact-form job: %w", err))
	}
	if err := e.dispatchContactForm(ctx, job); err != nil {
		// Contact-form failures are logged in form_submissions already; the
		// queue still benefits from a retry on transient SMTP errors.
		if errors.Is(err, ErrDisabled) || errors.Is(err, ErrNotConfigured) {
			return queue.Permanent(err)
		}
		return err
	}
	return nil
}

// dispatchContactForm runs the contact-form send through the existing
// Service path, which already handles CF7 placeholder substitution and HTML
// fallback rendering. We DON'T write smtp_log rows for contact-form mail —
// the existing form_submissions table already records the mail_sent flag and
// mail_error string per submission, so a second audit trail would duplicate.
func (e *QueueEnqueuer) dispatchContactForm(ctx context.Context, job SendContactFormJob) error {
	switch job.Kind {
	case "notification":
		return e.svc.SendContactFormNotification(ctx, job.Params)
	case "auto_reply":
		return e.svc.SendContactFormAutoReply(ctx, job.Params)
	}
	return queue.Permanent(fmt.Errorf("unknown contact form kind %q", job.Kind))
}

// HandleSendEmailRaw is the queue handler for send_email_raw — used by the
// SMTP-log Resend action to replay a captured payload.
func (e *QueueEnqueuer) HandleSendEmailRaw(ctx context.Context, payload []byte) error {
	if proceed, err := e.gateOrDefer(ctx, queue.JobTypeSendEmailRaw, payload); !proceed {
		return err
	}
	var job SendEmailRawJob
	if err := json.Unmarshal(payload, &job); err != nil {
		return queue.Permanent(fmt.Errorf("decode send_email_raw job: %w", err))
	}
	resentFrom := nullable(job.LogID)
	return e.sendAndLog(ctx, sendArgs{
		TemplateKey:       nil,
		TriggerCondition:  job.TriggerCondition,
		RelatedEntityType: nullable(job.RelatedEntityType),
		RelatedEntityID:   nullable(job.RelatedEntityID),
		Recipient:         job.Recipient,
		ReplyTo:           job.ReplyTo,
		Subject:           job.Subject,
		BodyHTML:          job.BodyHTML,
		BodyText:          job.BodyText,
		ResentFromID:      resentFrom,
	})
}

// emailDryRunEnabled reports whether the temporary EMAIL_DRY_RUN switch is on.
// Truthy values: 1 / true / yes / on (case-insensitive). Used to suppress
// actual delivery of import setup-mail batches during diagnosis.
func emailDryRunEnabled() bool {
	v := strings.TrimSpace(os.Getenv("EMAIL_DRY_RUN"))
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes") || strings.EqualFold(v, "on")
}

// HandleSendEmailBatch is the queue handler for send_email_batch. It renders
// every recipient's templated mail and delivers them in a single Resend
// /emails/batch request (SMTP falls back to per-message sends inside
// SendRenderedBatch), then writes one smtp_log row per recipient.
//
// Rate limiting differs from the single-send handler: a batch is ONE Resend
// request, so it reserves a single token from each enabled bucket rather than
// one per recipient. The daily cap, however, is per-email — a batch is
// all-or-nothing against it: if the rolling-24h 'sent' count leaves less
// headroom than the batch needs, the WHOLE batch is deferred (never partially
// dropped), which keeps Resend's Free-tier 100/day cap from rejecting sends.
func (e *QueueEnqueuer) HandleSendEmailBatch(ctx context.Context, payload []byte) error {
	var job SendEmailBatchJob
	if err := json.Unmarshal(payload, &job); err != nil {
		return queue.Permanent(fmt.Errorf("decode send_email_batch job: %w", err))
	}
	if len(job.Recipients) == 0 {
		return nil
	}

	// TEMP DIAGNOSTIC (EMAIL_DRY_RUN): when the env var is set, do NOT deliver
	// the batch — just log the intended recipients so customer-import setup
	// mails can be tested without sending real email. The job is marked done
	// (returns nil) and writes no smtp_log row. Unset the env (and restart the
	// backend) to restore normal delivery. Remove this block once diagnosis is
	// finished.
	if emailDryRunEnabled() {
		for _, r := range job.Recipients {
			log.Printf("EMAIL_DRY_RUN: send_email_batch SUPPRESSED template=%s trigger=%s -> recipient=%s",
				job.TemplateKey, job.TriggerCondition, r.Recipient)
		}
		log.Printf("EMAIL_DRY_RUN: send_email_batch suppressed %d recipient(s); no delivery, no smtp_log",
			len(job.Recipients))
		return nil
	}

	// Daily-cap gate (per-email, all-or-nothing for the batch).
	if limit := e.intSetting(ctx, "email_daily_limit", 0); limit > 0 {
		sent, err := e.logStore.CountSentSince(ctx, time.Now().Add(-24*time.Hour))
		if err != nil {
			// Don't block delivery on a transient count failure — log and send.
			log.Printf("email rate-limit: count sent failed: %v", err)
		} else if limit-sent < len(job.Recipients) {
			log.Printf("email rate-limit: batch of %d exceeds daily headroom (%d/%d sent in 24h), deferring",
				len(job.Recipients), sent, limit)
			return e.deferSend(ctx, queue.JobTypeSendEmailBatch, payload, emailDeferInterval)
		}
	}

	// Rate buckets: one reservation for the single batch request.
	var wait time.Duration
	if perMin := e.intSetting(ctx, "email_rate_per_minute", 0); perMin > 0 {
		if w := e.minuteBucket.reserve(perMin); w > wait {
			wait = w
		}
	}
	if perSec := e.intSetting(ctx, "email_rate_per_second", 0); perSec > 0 {
		if w := e.secondBucket.reserve(perSec); w > wait {
			wait = w
		}
	}
	if wait > 0 {
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return ctx.Err() // retryable: worker backoff re-queues
		}
	}

	// Render each recipient. A recipient whose params can't decode/render is
	// dropped with a log line rather than failing the whole batch.
	msgs := make([]RenderedEmail, 0, len(job.Recipients))
	for _, r := range job.Recipients {
		typed, derr := decodeParams(job.TemplateKey, r.Params)
		if derr != nil {
			log.Printf("send_email_batch: decode params for %s: %v", r.Recipient, derr)
			continue
		}
		subject, text, html, rerr := e.svc.RenderTemplate(ctx, job.TemplateKey, typed)
		if rerr != nil {
			log.Printf("send_email_batch: render for %s: %v", r.Recipient, rerr)
			continue
		}
		msgs = append(msgs, RenderedEmail{To: r.Recipient, Subject: subject, Text: text, HTML: html})
	}
	if len(msgs) == 0 {
		return nil
	}

	perMsg, callErr := e.svc.SendRenderedBatch(ctx, msgs)

	// One smtp_log row per recipient — mirrors the single-send audit trail.
	fromEmail, fromName, _ := e.svc.FromConfig(ctx)
	tmplKey := job.TemplateKey
	for i, m := range msgs {
		status, reason := "sent", ""
		switch {
		case callErr != nil:
			status, reason = "failed", callErr.Error()
		case i < len(perMsg) && perMsg[i] != nil:
			status, reason = "failed", perMsg[i].Error()
		}
		if _, lerr := e.logStore.Insert(ctx, smtplog.InsertInput{
			TemplateKey:      &tmplKey,
			TriggerCondition: job.TriggerCondition,
			Recipient:        m.To,
			FromEmail:        fromEmail,
			FromName:         fromName,
			Subject:          m.Subject,
			BodyHTML:         m.HTML,
			BodyText:         m.Text,
			Status:           status,
			FailureReason:    reason,
		}); lerr != nil {
			log.Printf("smtp_log insert (batch): %v", lerr)
		}
	}

	if callErr != nil {
		// Non-retryable (email disabled / not configured / bad API key) → mark
		// dead; anything else (network / 5xx / 429) → retry the whole batch.
		// Resend batch is atomic, so a failed request sent nothing — re-sending
		// is safe.
		if errors.Is(callErr, ErrDisabled) || errors.Is(callErr, ErrNotConfigured) || errors.Is(callErr, ErrAuth) {
			return queue.Permanent(callErr)
		}
		return callErr
	}
	return nil
}

type sendArgs struct {
	TemplateKey       *string
	TriggerCondition  string
	RelatedEntityType *string
	RelatedEntityID   *string
	Recipient         string
	ReplyTo           string
	Subject           string
	BodyHTML          string
	BodyText          string
	ResentFromID      *string
}

func (e *QueueEnqueuer) sendAndLog(ctx context.Context, a sendArgs) error {
	fromEmail, fromName, cfgErr := e.svc.FromConfig(ctx)
	sendErr := cfgErr
	if cfgErr == nil {
		sendErr = e.svc.SendRendered(ctx, a.Recipient, a.ReplyTo, a.Subject, a.BodyText, a.BodyHTML)
	}

	status := "sent"
	failureReason := ""
	if sendErr != nil {
		status = "failed"
		failureReason = sendErr.Error()
	}
	if _, lerr := e.logStore.Insert(ctx, smtplog.InsertInput{
		TemplateKey:       a.TemplateKey,
		TriggerCondition:  a.TriggerCondition,
		RelatedEntityType: a.RelatedEntityType,
		RelatedEntityID:   a.RelatedEntityID,
		Recipient:         a.Recipient,
		FromEmail:         fromEmail,
		FromName:          fromName,
		ReplyTo:           a.ReplyTo,
		Subject:           a.Subject,
		BodyHTML:          a.BodyHTML,
		BodyText:          a.BodyText,
		Status:            status,
		FailureReason:     failureReason,
		ResentFromID:      a.ResentFromID,
	}); lerr != nil {
		log.Printf("smtp_log insert: %v", lerr)
	}
	if sendErr != nil {
		// Non-retryable errors (email disabled / misconfigured / bad API key)
		// are classified so the queue marks the job dead instead of looping.
		if errors.Is(sendErr, ErrDisabled) || errors.Is(sendErr, ErrNotConfigured) || errors.Is(sendErr, ErrAuth) {
			return queue.Permanent(sendErr)
		}
		return sendErr
	}
	return nil
}

func decodeParams(key string, raw json.RawMessage) (any, error) {
	switch key {
	case "order_confirmation", "new_order_alert":
		var p OrderEmailParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return p, nil
	case "order_shipped":
		var p ShippedEmailParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return p, nil
	case "order_refunded":
		var p RefundEmailParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return p, nil
	case "payment_link":
		var p PaymentLinkParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return p, nil
	case "bank_transfer_on_hold":
		var p BankTransferOnHoldParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return p, nil
	case "password_reset", "account_setup":
		var p PasswordResetParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return p, nil
	case "admin_message":
		var p AdminMessageParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return p, nil
	case "abandoned_cart":
		var p AbandonedCartParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return p, nil
	case "low_stock_alert":
		var p LowStockParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return p, nil
	}
	return nil, fmt.Errorf("decode params: unknown key %q", key)
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
