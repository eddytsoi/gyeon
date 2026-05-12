import * as m from '$lib/paraglide/messages';

const labels: Record<string, () => string> = {
  pending:    m.order_status_pending,
  paid:       m.order_status_paid,
  processing: m.order_status_processing,
  shipped:    m.order_status_shipped,
  delivered:  m.order_status_delivered,
  cancelled:  m.order_status_cancelled,
  refunded:   m.order_status_refunded,
};

export function orderStatusLabel(status: string): string {
  return labels[status]?.() ?? status;
}
