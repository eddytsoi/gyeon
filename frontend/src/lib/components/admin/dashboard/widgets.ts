// Central widget registry for the admin dashboard. A single config array drives
// rendering of every KPI card, so adding a metric later is a one-line change and
// the per-admin customise feature (show/hide, reorder, presets) has something
// concrete to operate on. The four "hero" metrics (net revenue + carts abandoned
// + the two profit placeholders) are pinned in the top band and excluded from the
// customisable grid; the grid covers the Sales / Customers / Carts sections.

import * as m from '$lib/paraglide/messages';

export type SectionKey = 'sales' | 'customers' | 'carts';
export type ValueFmt = 'hk' | 'int' | 'pct' | 'num1';

export interface WidgetDef {
  key: string; // matches a key in the dashboard response `metrics` map
  title: () => string;
  section: SectionKey;
  fmt: ValueFmt;
  comparison: boolean; // show the period-over-period delta pill
  goodWhenUp?: boolean; // default true; false for refunds / abandoned / failed
  badge?: 'all_time' | 'in_period';
  accent: string; // top-border + sparkline colour
  hasSeries?: boolean; // a daily sparkline series exists for this key
}

// ── formatters ────────────────────────────────────────────────────────────────

export function fmtHK(n: number): string {
  return `HK$${n.toLocaleString('en-HK', { maximumFractionDigits: 0 })}`;
}
export function fmtInt(n: number): string {
  return n.toLocaleString('en-HK', { maximumFractionDigits: 0 });
}
export function fmtPct(n: number): string {
  return `${(n * 100).toFixed(1)}%`;
}
export function fmtNum1(n: number): string {
  return n.toLocaleString('en-HK', { minimumFractionDigits: 1, maximumFractionDigits: 1 });
}
export function formatValue(fmt: ValueFmt, n: number): string {
  switch (fmt) {
    case 'hk':
      return fmtHK(n);
    case 'pct':
      return fmtPct(n);
    case 'num1':
      return fmtNum1(n);
    default:
      return fmtInt(n);
  }
}

// ── accent palette ──────────────────────────────────────────────────────────
const C = {
  indigo: '#6366f1',
  blue: '#3b82f6',
  emerald: '#10b981',
  green: '#22c55e',
  violet: '#8b5cf6',
  amber: '#f59e0b',
  red: '#ef4444',
  rose: '#f43f5e',
  sky: '#0ea5e9',
  teal: '#14b8a6',
  slate: '#64748b'
};

// ── hero band (pinned, not customisable) ─────────────────────────────────────

export interface HeroCardDef {
  key: string;
  title: () => string;
  fmt: ValueFmt;
  accent: string;
  goodWhenUp?: boolean;
  disabled?: boolean;
  hint?: () => string;
}

// Net revenue is rendered as the large headline; these three sit beside it.
export const HERO_SECONDARY: HeroCardDef[] = [
  { key: 'carts_abandoned', title: () => m.dashboard_m_carts_abandoned(), fmt: 'int', accent: C.rose, goodWhenUp: false },
  { key: 'net_profit', title: () => m.dashboard_m_net_profit(), fmt: 'hk', accent: C.emerald, disabled: true, hint: () => m.dashboard_profit_hint() },
  { key: 'profit_margin', title: () => m.dashboard_m_profit_margin(), fmt: 'pct', accent: C.green, disabled: true, hint: () => m.dashboard_profit_hint() }
];

// ── customisable KPI grid ─────────────────────────────────────────────────────

export const GRID_SECTIONS: { key: SectionKey; title: () => string }[] = [
  { key: 'sales', title: () => m.dashboard_sec_sales() },
  { key: 'customers', title: () => m.dashboard_sec_customers() },
  { key: 'carts', title: () => m.dashboard_sec_carts() }
];

export const WIDGETS: WidgetDef[] = [
  // Sales
  { key: 'orders', title: () => m.dashboard_m_orders(), section: 'sales', fmt: 'int', comparison: true, accent: C.blue, hasSeries: true },
  { key: 'items_sold', title: () => m.dashboard_m_items_sold(), section: 'sales', fmt: 'int', comparison: true, accent: C.violet, hasSeries: true },
  { key: 'avg_order_net', title: () => m.dashboard_m_avg_order_net(), section: 'sales', fmt: 'hk', comparison: true, accent: C.emerald },
  { key: 'avg_order_items', title: () => m.dashboard_m_avg_order_items(), section: 'sales', fmt: 'num1', comparison: true, accent: C.teal },
  { key: 'refunded_amount', title: () => m.dashboard_m_refunded_amount(), section: 'sales', fmt: 'hk', comparison: true, goodWhenUp: false, accent: C.red },
  { key: 'refunds_count', title: () => m.dashboard_m_refunds_count(), section: 'sales', fmt: 'int', comparison: true, goodWhenUp: false, accent: C.red },
  { key: 'failed_orders', title: () => m.dashboard_m_failed_orders(), section: 'sales', fmt: 'int', comparison: true, goodWhenUp: false, accent: C.amber },

  // Customers
  { key: 'new_customers', title: () => m.dashboard_m_new_customers(), section: 'customers', fmt: 'int', comparison: true, accent: C.sky, hasSeries: true },
  { key: 'customers_total', title: () => m.dashboard_m_customers_total(), section: 'customers', fmt: 'int', comparison: false, badge: 'all_time', accent: C.slate },
  { key: 'customers_single', title: () => m.dashboard_m_customers_single(), section: 'customers', fmt: 'int', comparison: false, badge: 'all_time', accent: C.slate },
  { key: 'customers_repeat', title: () => m.dashboard_m_customers_repeat(), section: 'customers', fmt: 'int', comparison: false, badge: 'all_time', accent: C.indigo },
  { key: 'avg_customer_ltv', title: () => m.dashboard_m_avg_customer_ltv(), section: 'customers', fmt: 'hk', comparison: false, badge: 'all_time', accent: C.emerald },
  { key: 'avg_customer_orders', title: () => m.dashboard_m_avg_customer_orders(), section: 'customers', fmt: 'num1', comparison: false, badge: 'all_time', accent: C.teal },

  // Carts
  { key: 'carts_started', title: () => m.dashboard_m_carts_started(), section: 'carts', fmt: 'int', comparison: true, accent: C.blue, hasSeries: true },
  { key: 'cart_placed_rate', title: () => m.dashboard_m_cart_placed_rate(), section: 'carts', fmt: 'pct', comparison: true, accent: C.green },
  { key: 'cart_abandon_rate', title: () => m.dashboard_m_cart_abandon_rate(), section: 'carts', fmt: 'pct', comparison: true, goodWhenUp: false, accent: C.rose }
];

const BY_KEY = new Map(WIDGETS.map((w) => [w.key, w]));
export const widgetByKey = (key: string): WidgetDef | undefined => BY_KEY.get(key);

// ── layout model (per-admin customise state) ──────────────────────────────────

export interface WidgetState {
  key: string;
  visible: boolean;
}
export interface SectionState {
  key: SectionKey;
  collapsed: boolean;
  widgets: WidgetState[];
}
export interface DashLayout {
  sections: SectionState[];
}

// buildDefaultLayout derives the out-of-the-box layout from the registry: every
// section in registry order, every widget visible. Used when an admin has no
// saved preset yet, and as the base that saved prefs are merged over.
export function buildDefaultLayout(): DashLayout {
  return {
    sections: GRID_SECTIONS.map((s) => ({
      key: s.key,
      collapsed: false,
      widgets: WIDGETS.filter((w) => w.section === s.key).map((w) => ({ key: w.key, visible: true }))
    }))
  };
}

// mergeLayout reconciles a saved layout against the current registry so newly
// added metrics appear (appended to their section, visible) and removed metrics
// drop out — a saved preset never goes stale or hides a brand-new card forever.
export function mergeLayout(saved: DashLayout | null | undefined): DashLayout {
  const base = buildDefaultLayout();
  if (!saved?.sections?.length) return base;
  const savedByKey = new Map(saved.sections.map((s) => [s.key, s]));
  return {
    sections: base.sections.map((bs) => {
      const ss = savedByKey.get(bs.key);
      if (!ss) return bs;
      const baseKeys = new Set(bs.widgets.map((w) => w.key));
      // keep saved order/visibility for widgets that still exist…
      const kept = ss.widgets.filter((w) => baseKeys.has(w.key));
      const keptKeys = new Set(kept.map((w) => w.key));
      // …then append any registry widgets the saved layout never saw.
      const added = bs.widgets.filter((w) => !keptKeys.has(w.key));
      return { key: bs.key, collapsed: !!ss.collapsed, widgets: [...kept, ...added] };
    })
  };
}
