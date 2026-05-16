package email

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

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
}

// QueueAPI is the slice of queue.Service used by the enqueuer.
type QueueAPI interface {
	Enqueue(ctx context.Context, jobType string, payload []byte, opts ...queue.EnqueueOptions) (string, error)
}

// LogStore is the slice of smtplog.Store the worker writes to.
type LogStore interface {
	Insert(ctx context.Context, in smtplog.InsertInput) (string, error)
}

func NewQueueEnqueuer(svc *Service, q QueueAPI, ls LogStore) *QueueEnqueuer {
	return &QueueEnqueuer{svc: svc, queueSvc: q, logStore: ls}
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

// ── Send methods (mirror Service signatures) ─────────────────────────────

func (e *QueueEnqueuer) SendOrderConfirmation(ctx context.Context, p OrderEmailParams) error {
	return e.enqueueTemplated(ctx, "order_confirmation", p.CustomerEmail, p, "order.paid", "order", p.OrderID)
}

func (e *QueueEnqueuer) SendOrderShipped(ctx context.Context, p ShippedEmailParams) error {
	return e.enqueueTemplated(ctx, "order_shipped", p.CustomerEmail, p, "order.shipped", "order", p.OrderID)
}

func (e *QueueEnqueuer) SendOrderRefunded(ctx context.Context, p RefundEmailParams) error {
	return e.enqueueTemplated(ctx, "order_refunded", p.CustomerEmail, p, "order.refund", "order", p.OrderID)
}

func (e *QueueEnqueuer) SendPaymentLink(ctx context.Context, p PaymentLinkParams) error {
	return e.enqueueTemplated(ctx, "payment_link", p.CustomerEmail, p, "checkout.payment_link", "order", p.OrderID)
}

func (e *QueueEnqueuer) SendPasswordResetEmail(ctx context.Context, p PasswordResetParams) error {
	if p.ExpiryHours == 0 {
		p.ExpiryHours = 24
	}
	return e.enqueueTemplated(ctx, "password_reset", p.CustomerEmail, p, "auth.password_reset", "", "")
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

// ── Worker handlers ──────────────────────────────────────────────────────

// HandleSendEmail is the queue handler for send_email. It dispatches on
// the payload shape: templated transactional emails take the render+log
// path; contact-form jobs take the contact-form path.
func (e *QueueEnqueuer) HandleSendEmail(ctx context.Context, payload []byte) error {
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
		// Non-retryable errors (email disabled / SMTP misconfigured) are
		// classified so the queue marks the job dead instead of looping.
		if errors.Is(sendErr, ErrDisabled) || errors.Is(sendErr, ErrNotConfigured) {
			return queue.Permanent(sendErr)
		}
		return sendErr
	}
	return nil
}

func decodeParams(key string, raw json.RawMessage) (any, error) {
	switch key {
	case "order_confirmation":
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
	case "password_reset":
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
