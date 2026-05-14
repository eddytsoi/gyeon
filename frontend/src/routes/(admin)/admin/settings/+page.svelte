<script lang="ts">
  import { onMount } from 'svelte';
  import { enhance } from '$app/forms';
  import type { PageData } from './$types';
  import MultiSelect from '$lib/components/MultiSelect.svelte';
  import MediaPicker from '$lib/components/admin/MediaPicker.svelte';
  import SaveButton from '$lib/components/admin/SaveButton.svelte';
  import PasswordInput from '$lib/components/admin/PasswordInput.svelte';
  import { COUNTRIES } from '$lib/data/countries';
  import { notify } from '$lib/stores/notifications.svelte';
  import * as m from '$lib/paraglide/messages';

  let { data }: { data: PageData } = $props();
  let saving = $state(false);

  // ── Tabs ────────────────────────────────────────────────────────
  const TAB_ICONS: Record<string, string> = {
    general:
      'M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.324.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 0 1 1.37.49l1.296 2.247a1.125 1.125 0 0 1-.26 1.431l-1.003.827c-.293.241-.438.613-.43.992a7.723 7.723 0 0 1 0 .255c-.008.378.137.75.43.991l1.004.827c.424.35.534.955.26 1.43l-1.298 2.247a1.125 1.125 0 0 1-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.47 6.47 0 0 1-.22.128c-.331.183-.581.495-.644.869l-.213 1.28c-.09.543-.56.941-1.11.941h-2.594c-.55 0-1.019-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 0 1-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 0 1-1.369-.49l-1.297-2.247a1.125 1.125 0 0 1 .26-1.431l1.004-.827c.292-.24.437-.613.43-.991a6.932 6.932 0 0 1 0-.255c.007-.38-.138-.751-.43-.992l-1.004-.827a1.125 1.125 0 0 1-.26-1.43l1.297-2.247a1.125 1.125 0 0 1 1.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.087.22-.128.332-.183.582-.495.644-.869l.214-1.28Z M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z',
    commerce:
      'M15.75 10.5V6a3.75 3.75 0 1 0-7.5 0v4.5m11.356-1.993 1.263 12c.07.665-.45 1.243-1.119 1.243H4.25a1.125 1.125 0 0 1-1.12-1.243l1.264-12A1.125 1.125 0 0 1 5.513 7.5h12.974c.576 0 1.059.435 1.119 1.007Z',
    logistics:
      'M8.25 18.75a1.5 1.5 0 0 1-3 0m3 0a1.5 1.5 0 0 0-3 0m3 0h6m-9 0H3.375a1.125 1.125 0 0 1-1.125-1.125V14.25m17.25 4.5a1.5 1.5 0 0 1-3 0m3 0a1.5 1.5 0 0 0-3 0m3 0h1.125c.621 0 1.129-.504 1.09-1.124a17.902 17.902 0 0 0-3.213-9.193 2.056 2.056 0 0 0-1.58-.86H14.25M16.5 18.75h-2.25m0-11.177v-.958c0-.568-.422-1.048-.987-1.106a48.554 48.554 0 0 0-10.026 0 1.106 1.106 0 0 0-.987 1.106v7.635m12-6.677v6.677m0 4.5v-4.5m0 0h-12',
    email:
      'M21.75 6.75v10.5a2.25 2.25 0 0 1-2.25 2.25h-15a2.25 2.25 0 0 1-2.25-2.25V6.75m19.5 0A2.25 2.25 0 0 0 19.5 4.5h-15a2.25 2.25 0 0 0-2.25 2.25m19.5 0v.243a2.25 2.25 0 0 1-1.07 1.916l-7.5 4.615a2.25 2.25 0 0 1-2.36 0L3.32 8.91a2.25 2.25 0 0 1-1.07-1.916V6.75',
    infrastructure:
      'M5.25 14.25h13.5m-13.5 0a3 3 0 0 1-3-3m3 3a3 3 0 1 0 0 6h13.5a3 3 0 1 0 0-6m-16.5-3a3 3 0 0 1 3-3h13.5a3 3 0 0 1 3 3m-19.5 0a4.5 4.5 0 0 1 .9-2.7L5.737 5.1a3.375 3.375 0 0 1 2.7-1.35h7.126c1.062 0 2.062.5 2.7 1.35l2.587 3.45a4.5 4.5 0 0 1 .9 2.7m0 0a3 3 0 0 1-3 3m0 3h.008v.008h-.008v-.008Zm0-6h.008v.008h-.008v-.008Zm-3 6h.008v.008h-.008v-.008Zm0-6h.008v.008h-.008v-.008Z'
  };

  const TABS = $derived([
    { id: 'general',        label: m.admin_settings_tab_general() },
    { id: 'commerce',       label: m.admin_settings_tab_commerce() },
    { id: 'logistics',      label: m.admin_settings_tab_logistics() },
    { id: 'email',          label: m.admin_settings_tab_email() },
    { id: 'infrastructure', label: m.admin_settings_tab_infrastructure() }
  ] as const);
  type TabId = 'general' | 'commerce' | 'logistics' | 'email' | 'infrastructure';

  let activeTab = $state<TabId>('general');

  onMount(() => {
    const fromHash = window.location.hash.slice(1) as TabId;
    if (TABS.some((t) => t.id === fromHash)) activeTab = fromHash;
  });

  function setTab(id: TabId) {
    activeTab = id;
    history.replaceState(null, '', `#${id}`);
  }

  // ── Tab magnetic spotlight ───────────────────────────────────────
  let tabsEl = $state<HTMLElement | undefined>();
  let tabSpotlight = $state({ visible: false, left: 0, width: 0, height: 0 });

  function moveTabSpotlightTo(btn: Element | null) {
    if (!btn || !tabsEl || !tabsEl.contains(btn)) { tabSpotlight.visible = false; return; }
    const tabsRect = tabsEl.getBoundingClientRect();
    const btnRect  = btn.getBoundingClientRect();
    tabSpotlight = {
      visible: true,
      left:    btnRect.left - tabsRect.left + tabsEl.scrollLeft,
      width:   btnRect.width,
      height:  btnRect.height,
    };
  }

  function onTabsMouseMove(e: MouseEvent) {
    moveTabSpotlightTo((e.target as HTMLElement | null)?.closest('button') ?? null);
  }

  function onTabsMouseLeave() { tabSpotlight.visible = false; }

  // ── Test Email Modal ─────────────────────────────────────────────
  let showTestEmailModal = $state(false);
  let testEmailAddress = $state('');
  let testEmailSending = $state(false);

  const token = $derived(data.token ?? '');

  async function sendTestEmail() {
    testEmailSending = true;
    try {
      const res = await fetch('/api/v1/admin/settings/test-email', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`
        },
        body: JSON.stringify({ to: testEmailAddress })
      });
      if (res.ok) {
        notify.success(m.admin_settings_test_email_success_title(), m.admin_settings_test_email_success_body({ email: testEmailAddress }));
        showTestEmailModal = false;
      } else {
        let serverMsg = m.admin_settings_test_email_failure_default();
        try {
          const body = await res.json();
          if (typeof body?.error === 'string' && body.error) serverMsg = body.error;
        } catch { /* non-JSON body */ }
        notify.error(m.admin_settings_test_email_failure_title(), serverMsg);
      }
    } catch (e) {
      notify.error(m.admin_settings_test_email_failure_title(), e instanceof Error ? e.message : m.admin_settings_test_email_network_error());
    } finally {
      testEmailSending = false;
    }
  }

  const TOGGLE_KEYS = new Set(['maintenance_mode', 'mcp_enabled']);
  const LOCALE_KEYS = new Set(['site_locale']);
  const FAVICON_KEYS = new Set(['favicon_url']);
  const SITE_NOTICE_KEYS = new Set([
    'site_notice',
    'site_notice_enabled',
    'site_notice_bg_color',
    'site_notice_text_color',
    'site_notice_text_size',
  ]);
  const HOMEPAGE_KEYS = new Set(['homepage_page_id']);
  const CURRENCY_KEYS = new Set(['currency']);
  const FREE_SHIPPING_KEYS = new Set(['free_shipping_threshold_hkd']);
  const SHIPPING_KEYS = new Set(['shipping_countries']);
  // Managed by the Hidden Products section (Commerce tab). Excluded from
  // textSettings so the generic catch-all loop doesn't render a second
  // <input name="hidden_category_ids"> that overwrites the real one on submit.
  const HIDDEN_PRODUCTS_KEYS = new Set(['hidden_category_ids']);
  const CACHE_TTL_KEYS = new Set(['cache_ttl_shop', 'cache_ttl_cms', 'cache_ttl_nav']);
  const CLOUDFLARE_KEYS = new Set(['cloudflare_zone_id', 'cloudflare_api_token']);
  const MEDIA_LIMIT_KEYS = new Set(['upload_max_image_mb', 'upload_max_video_mb']);
  const ORDER_NUMBER_KEYS = new Set(['order_number_prefix']);
  const LOW_STOCK_KEYS = new Set(['low_stock_threshold_default', 'low_stock_alert_enabled']);
  const TAX_KEYS = new Set(['tax_enabled', 'tax_rate', 'tax_label', 'tax_inclusive']);
  const LOYALTY_KEYS = new Set(['loyalty_enabled', 'loyalty_points_per_hkd', 'loyalty_redeem_rate_hkd']);
  const ABANDONED_KEYS = new Set(['abandoned_cart_enabled', 'abandoned_cart_threshold_hours']);
  const ORPHAN_KEYS = new Set(['timezone', 'site_description', 'contact_email']);
  const PAYMENT_KEYS = new Set([
    'stripe_mode',
    'stripe_country',
    'stripe_test_publishable_key',
    'stripe_test_secret_key',
    'stripe_live_publishable_key',
    'stripe_live_secret_key',
    'stripe_save_cards',
    'stripe_webhook_secret'
  ]);

  const STRIPE_COUNTRY_OPTIONS = [
    { value: 'AU', label: 'Australia' }, { value: 'AT', label: 'Austria' },
    { value: 'BE', label: 'Belgium' }, { value: 'BR', label: 'Brazil' },
    { value: 'BG', label: 'Bulgaria' }, { value: 'CA', label: 'Canada' },
    { value: 'HR', label: 'Croatia' }, { value: 'CY', label: 'Cyprus' },
    { value: 'CZ', label: 'Czech Republic' }, { value: 'DK', label: 'Denmark' },
    { value: 'EE', label: 'Estonia' }, { value: 'FI', label: 'Finland' },
    { value: 'FR', label: 'France' }, { value: 'DE', label: 'Germany' },
    { value: 'GI', label: 'Gibraltar' }, { value: 'GR', label: 'Greece' },
    { value: 'HK', label: 'Hong Kong' }, { value: 'HU', label: 'Hungary' },
    { value: 'IN', label: 'India' }, { value: 'ID', label: 'Indonesia' },
    { value: 'IE', label: 'Ireland' }, { value: 'IT', label: 'Italy' },
    { value: 'JP', label: 'Japan' }, { value: 'LV', label: 'Latvia' },
    { value: 'LI', label: 'Liechtenstein' }, { value: 'LT', label: 'Lithuania' },
    { value: 'LU', label: 'Luxembourg' }, { value: 'MY', label: 'Malaysia' },
    { value: 'MT', label: 'Malta' }, { value: 'MX', label: 'Mexico' },
    { value: 'NL', label: 'Netherlands' }, { value: 'NZ', label: 'New Zealand' },
    { value: 'NO', label: 'Norway' }, { value: 'PL', label: 'Poland' },
    { value: 'PT', label: 'Portugal' }, { value: 'RO', label: 'Romania' },
    { value: 'SG', label: 'Singapore' }, { value: 'SK', label: 'Slovakia' },
    { value: 'SI', label: 'Slovenia' }, { value: 'ES', label: 'Spain' },
    { value: 'SE', label: 'Sweden' }, { value: 'CH', label: 'Switzerland' },
    { value: 'TH', label: 'Thailand' }, { value: 'AE', label: 'United Arab Emirates' },
    { value: 'GB', label: 'United Kingdom' }, { value: 'US', label: 'United States' }
  ];
  const SHIPANY_KEYS = new Set([
    'shipany_enabled', 'shipany_user_id', 'shipany_api_key',
    'shipany_region', 'shipany_origin_name',
    'shipany_origin_phone', 'shipany_origin_line1', 'shipany_origin_line2',
    'shipany_origin_district', 'shipany_origin_city', 'shipany_origin_postal',
    'shipany_default_weight_grams', 'shipany_default_courier',
    'shipany_default_service', 'shipany_default_storage_type',
    'shipany_paid_by_receiver', 'shipany_self_drop_off',
    'shipany_order_ref_suffix', 'shipany_show_courier_tracking_number',
    'shipany_webhook_secret'
  ]);
  const RECAPTCHA_KEYS = new Set([
    'recaptcha_enabled', 'recaptcha_site_key', 'recaptcha_secret_key', 'recaptcha_min_score'
  ]);
  const SMTP_KEYS = new Set([
    'email_enabled',
    'smtp_host', 'smtp_port', 'smtp_username', 'smtp_password',
    'smtp_from_email', 'smtp_from_name', 'public_base_url',
    'admin_alert_email'
  ]);
  const WC_KEYS = new Set(['wc_consumer_key', 'wc_consumer_secret', 'wc_url']);

  const CACHE_TTL_LABELS = $derived<Record<string, string>>({
    cache_ttl_shop: m.admin_settings_cache_label_shop(),
    cache_ttl_cms: m.admin_settings_cache_label_cms(),
    cache_ttl_nav: m.admin_settings_cache_label_nav()
  });
  const CLOUDFLARE_LABELS = $derived<Record<string, string>>({
    cloudflare_zone_id: m.admin_settings_cloudflare_zone_id(),
    cloudflare_api_token: m.admin_settings_cloudflare_api_token()
  });
  const CLOUDFLARE_PLACEHOLDERS: Record<string, string> = {
    cloudflare_zone_id: 'e.g. 5a0f426da5de...',
    cloudflare_api_token: 'cfut_...'
  };
  const MEDIA_LIMIT_LABELS = $derived<Record<string, string>>({
    upload_max_image_mb: m.admin_settings_media_image_limit(),
    upload_max_video_mb: m.admin_settings_media_video_limit()
  });
  const TEXT_SETTING_LABELS = $derived<Record<string, string>>({
    currency: m.admin_settings_label_currency(),
    free_shipping_threshold_hkd: m.admin_settings_label_free_shipping_threshold_hkd(),
    ga4_measurement_id: m.admin_settings_label_ga4_measurement_id(),
    loyalty_enabled: m.admin_settings_label_loyalty_enabled(),
    loyalty_points_per_hkd: m.admin_settings_label_loyalty_points_per_hkd(),
    loyalty_redeem_rate_hkd: m.admin_settings_label_loyalty_redeem_rate_hkd(),
    meta_pixel_id: m.admin_settings_label_meta_pixel_id(),
    site_name: m.admin_settings_label_site_name()
  });
  const SETTING_DESCS = $derived<Record<string, string>>({
    maintenance_mode: m.admin_settings_desc_maintenance_mode(),
    mcp_enabled: m.admin_settings_desc_mcp_enabled(),
    cache_ttl_shop: m.admin_settings_desc_cache_ttl_shop(),
    cache_ttl_cms: m.admin_settings_desc_cache_ttl_cms(),
    cache_ttl_nav: m.admin_settings_desc_cache_ttl_nav(),
    cloudflare_zone_id: m.admin_settings_desc_cloudflare_zone_id(),
    cloudflare_api_token: m.admin_settings_desc_cloudflare_api_token(),
    upload_max_image_mb: m.admin_settings_desc_upload_max_image_mb(),
    upload_max_video_mb: m.admin_settings_desc_upload_max_video_mb(),
    currency: m.admin_settings_desc_currency(),
    free_shipping_threshold_hkd: m.admin_settings_desc_free_shipping_threshold_hkd(),
    ga4_measurement_id: m.admin_settings_desc_ga4_measurement_id(),
    loyalty_enabled: m.admin_settings_desc_loyalty_enabled(),
    loyalty_points_per_hkd: m.admin_settings_desc_loyalty_points_per_hkd(),
    loyalty_redeem_rate_hkd: m.admin_settings_desc_loyalty_redeem_rate_hkd(),
    meta_pixel_id: m.admin_settings_desc_meta_pixel_id(),
    site_name: m.admin_settings_desc_site_name()
  });

  const textSettings = $derived(
    data.settings.filter(
      (s) =>
        !TOGGLE_KEYS.has(s.key) &&
        !LOCALE_KEYS.has(s.key) &&
        !CACHE_TTL_KEYS.has(s.key) &&
        !CLOUDFLARE_KEYS.has(s.key) &&
        !MEDIA_LIMIT_KEYS.has(s.key) &&
        !PAYMENT_KEYS.has(s.key) &&
        !SMTP_KEYS.has(s.key) &&
        !SHIPPING_KEYS.has(s.key) &&
        !HIDDEN_PRODUCTS_KEYS.has(s.key) &&
        !SHIPANY_KEYS.has(s.key) &&
        !RECAPTCHA_KEYS.has(s.key) &&
        !ORDER_NUMBER_KEYS.has(s.key) &&
        !FAVICON_KEYS.has(s.key) &&
        !SITE_NOTICE_KEYS.has(s.key) &&
        !HOMEPAGE_KEYS.has(s.key) &&
        !TAX_KEYS.has(s.key) &&
        !LOYALTY_KEYS.has(s.key) &&
        !ABANDONED_KEYS.has(s.key) &&
        !LOW_STOCK_KEYS.has(s.key) &&
        !ORPHAN_KEYS.has(s.key) &&
        !CURRENCY_KEYS.has(s.key) &&
        !FREE_SHIPPING_KEYS.has(s.key) &&
        !WC_KEYS.has(s.key)
    )
  );
  const currencySetting = $derived(data.settings.find((s) => s.key === 'currency'));
  const freeShippingSetting = $derived(data.settings.find((s) => s.key === 'free_shipping_threshold_hkd'));
  const cacheTTLSettings = $derived(data.settings.filter((s) => CACHE_TTL_KEYS.has(s.key)));
  const cloudflareSettings = $derived(data.settings.filter((s) => CLOUDFLARE_KEYS.has(s.key)));
  const mediaLimitSettings = $derived(data.settings.filter((s) => MEDIA_LIMIT_KEYS.has(s.key)));
  const maintenanceSetting = $derived(data.settings.find((s) => s.key === 'maintenance_mode'));
  let maintenanceOn = $state(maintenanceSetting?.value === 'true');

  const mcpSetting = $derived(data.settings.find((s) => s.key === 'mcp_enabled'));
  let mcpOn = $state(mcpSetting?.value === 'true');

  const faviconSetting = $derived(data.settings.find((s) => s.key === 'favicon_url'));
  let faviconUrl = $state(faviconSetting?.value ?? '');

  // ── reCAPTCHA (spam protection for contact forms) ───────────────
  let recaptchaOn = $state(
    data.settings.find((s) => s.key === 'recaptcha_enabled')?.value === 'true'
  );

  // ── Site Notice (storefront announcement strip) ─────────────────
  let siteNoticeOn = $state(
    (data.settings.find((s) => s.key === 'site_notice_enabled')?.value ?? 'true') !== 'false'
  );
  let siteNoticeBgColor = $state(
    data.settings.find((s) => s.key === 'site_notice_bg_color')?.value || '#EDE9E1'
  );
  let siteNoticeTextColor = $state(
    data.settings.find((s) => s.key === 'site_notice_text_color')?.value || '#1A1A1A'
  );
  // Sanitize free-text hex input to the canonical "#RRGGBB" form, leaving
  // partial entries (e.g. "#ED") untouched so the user can keep typing.
  function normalizeHex(raw: string): string {
    const trimmed = raw.trim();
    const withHash = trimmed.startsWith('#') ? trimmed : `#${trimmed}`;
    if (/^#[0-9a-fA-F]{6}$/.test(withHash)) return withHash.toUpperCase();
    return withHash;
  }

  // ── Default Storefront Language ─────────────────────────────────
  const STOREFRONT_LANG_OPTIONS = $derived([
    { value: 'en',      label: m.admin_settings_storefront_lang_option_en() },
    { value: 'zh-Hant', label: m.admin_settings_storefront_lang_option_zh_hant() }
  ]);

  // ── Shipping Countries ──────────────────────────────────────────
  const shippingCountriesSetting = $derived(
    data.settings.find((s) => s.key === 'shipping_countries')
  );
  function parseCountryList(raw: string | undefined): string[] {
    if (!raw) return ['HK'];
    try {
      const parsed = JSON.parse(raw);
      return Array.isArray(parsed) ? parsed.filter((v) => typeof v === 'string') : ['HK'];
    } catch {
      return ['HK'];
    }
  }
  let shippingCountries = $state<string[]>(
    parseCountryList(data.settings.find((s) => s.key === 'shipping_countries')?.value)
  );
  const countryOptions = COUNTRIES.map((c) => ({ value: c.code, label: `${c.name} (${c.code})` }));

  // ── Hidden Categories ──────────────────────────────────────────
  function parseIDList(raw: string | undefined): string[] {
    if (!raw) return [];
    try {
      const parsed = JSON.parse(raw);
      return Array.isArray(parsed) ? parsed.filter((v) => typeof v === 'string') : [];
    } catch {
      return [];
    }
  }
  let hiddenCategoryIds = $state<string[]>(
    parseIDList(data.settings.find((s) => s.key === 'hidden_category_ids')?.value)
  );
  const categoryOptions = $derived(
    (data.categories ?? []).map((c) => ({ value: c.id, label: c.name }))
  );

  // ── Payment ─────────────────────────────────────────────────────
  function settingValue(key: string): string {
    return data.settings.find((s) => s.key === key)?.value ?? '';
  }

  let stripeLiveMode = $state(settingValue('stripe_mode') === 'live');
  let stripeSaveCards = $state(settingValue('stripe_save_cards') === 'true');
  let lowStockAlertEnabled = $state(settingValue('low_stock_alert_enabled') !== 'false');
  // ── Tax ─────────────────────────────────────────────────────────
  let taxEnabled = $state(settingValue('tax_enabled') === 'true');
  let taxInclusive = $state(settingValue('tax_inclusive') === 'true');
  let taxLabel = $state(settingValue('tax_label') || 'Sales Tax');
  const initialTaxRatePct = (() => {
    const raw = settingValue('tax_rate');
    if (!raw) return 0;
    const n = Number(raw);
    return Number.isFinite(n) ? n * 100 : 0;
  })();
  let taxRatePct = $state<number>(initialTaxRatePct);
  const TAX_PREVIEW_SUBTOTAL = 100;
  const taxRateValue = $derived.by(() => {
    const r = (Number(taxRatePct) || 0) / 100;
    return r.toFixed(6).replace(/\.?0+$/, '') || '0';
  });
  const taxPreviewTax = $derived.by(() => {
    if (!taxEnabled) return 0;
    const r = (Number(taxRatePct) || 0) / 100;
    if (taxInclusive) return TAX_PREVIEW_SUBTOTAL - TAX_PREVIEW_SUBTOTAL / (1 + r);
    return TAX_PREVIEW_SUBTOTAL * r;
  });
  const taxPreviewSubtotal = $derived(taxInclusive ? TAX_PREVIEW_SUBTOTAL - taxPreviewTax : TAX_PREVIEW_SUBTOTAL);
  const taxPreviewTotal = $derived(taxInclusive ? TAX_PREVIEW_SUBTOTAL : TAX_PREVIEW_SUBTOTAL + taxPreviewTax);
  function fmtHKD(n: number): string {
    return `HK$${n.toFixed(2)}`;
  }

  let abandonedEnabled = $state(settingValue('abandoned_cart_enabled') === 'true');
  let loyaltyEnabled = $state(settingValue('loyalty_enabled') === 'true');
  let emailEnabled = $state(settingValue('email_enabled') !== 'false');
  let abandonedRunPending = $state(false);
  let abandonedRunResult = $state<string | null>(null);

  async function runAbandonedNow() {
    abandonedRunPending = true;
    abandonedRunResult = null;
    try {
      const res = await fetch('/api/v1/admin/abandoned-cart/run', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${data.token}` },
        body: JSON.stringify({ force: true })
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const body = await res.json();
      abandonedRunResult = m.admin_settings_abandoned_run_result({ count: body.sent ?? 0 });
    } catch (e) {
      abandonedRunResult = e instanceof Error ? e.message : 'failed';
    } finally {
      abandonedRunPending = false;
    }
  }

  // ── ShipAny ─────────────────────────────────────────────────────
  let shipanyOn = $state(settingValue('shipany_enabled') === 'true');
  let shipanyPaidByReceiver = $state(settingValue('shipany_paid_by_receiver') === 'true');
  let shipanySelfDropOff = $state(settingValue('shipany_self_drop_off') === 'true');
  let shipanyShowCourierTracking = $state(settingValue('shipany_show_courier_tracking_number') === 'true');
  let shipanyTestingConnection = $state(false);
  let shipanyTestResult = $state<{ ok: boolean; message: string } | null>(null);

  const SHIPANY_REGION_OPTIONS = [
    { value: '',    label: 'Hong Kong (api.shipany.io)' },
    { value: '-tw', label: 'Taiwan (api-tw.shipany.io)' },
    { value: '-sg', label: 'Singapore (api-sg.shipany.io)' },
    { value: '-th', label: 'Thailand (api-th.shipany.io)' }
  ];

  const SHIPANY_STORAGE_OPTIONS = ['Normal', 'Cold', 'Frozen'];

  type ShipanyCourier = {
    uid: string;
    name: string;
    cour_svc_plans?: { cour_svc_pl: string }[];
  };

  let shipanyCouriers = $state<ShipanyCourier[]>([]);
  let shipanyCouriersLoading = $state(false);
  let shipanyCouriersLoaded = $state(false);
  let shipanyCouriersError = $state('');
  let shipanyCourierUID = $state(settingValue('shipany_default_courier'));
  let shipanyServicePl = $state(settingValue('shipany_default_service'));

  const selectedCourierSvcPlans = $derived(
    shipanyCouriers.find((c) => c.uid === shipanyCourierUID)?.cour_svc_plans ?? []
  );

  async function loadShipanyCouriers() {
    shipanyCouriersLoading = true;
    shipanyCouriersError = '';
    try {
      const res = await fetch('/api/v1/admin/shipany/couriers', {
        headers: { Authorization: `Bearer ${token}` }
      });
      if (res.ok) {
        const body = await res.json();
        if (Array.isArray(body)) {
          shipanyCouriers = body;
        } else {
          shipanyCouriers = Array.isArray(body?.couriers) ? body.couriers : [];
          shipanyCouriersError = body?.error ?? '';
        }
      } else {
        shipanyCouriers = [];
        shipanyCouriersError = m.admin_settings_shipany_server_returned({ status: res.status });
      }
    } catch (e) {
      shipanyCouriers = [];
      shipanyCouriersError = e instanceof Error ? e.message : m.admin_settings_shipany_network_error();
    } finally {
      shipanyCouriersLoading = false;
      shipanyCouriersLoaded = true;
    }
  }

  $effect(() => {
    if (activeTab === 'logistics' && !shipanyCouriersLoaded && !shipanyCouriersLoading) {
      loadShipanyCouriers();
    }
  });

  function onCourierChange(uid: string) {
    shipanyCourierUID = uid;
    const plans = shipanyCouriers.find((c) => c.uid === uid)?.cour_svc_plans ?? [];
    if (!plans.some((p) => p.cour_svc_pl === shipanyServicePl)) {
      shipanyServicePl = '';
    }
  }

  async function testShipanyConnection() {
    shipanyTestingConnection = true;
    shipanyTestResult = null;
    try {
      const res = await fetch('/api/v1/admin/shipany/test-connection', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` }
      });
      if (res.ok) {
        const body = await res.json();
        shipanyTestResult = { ok: !!body.ok, message: body.message ?? '' };
        if (body.ok) {
          notify.success(m.admin_settings_shipany_test_success_title(), body.message || '');
        } else {
          notify.error(m.admin_settings_shipany_test_failure_title(), body.message || '');
        }
      } else {
        const msg = m.admin_settings_shipany_server_returned({ status: res.status });
        shipanyTestResult = { ok: false, message: msg };
        notify.error(m.admin_settings_shipany_test_failure_title(), msg);
      }
    } catch (e) {
      const msg = e instanceof Error ? e.message : m.admin_settings_shipany_network_error();
      shipanyTestResult = { ok: false, message: msg };
      notify.error(m.admin_settings_shipany_test_failure_title(), msg);
    } finally {
      shipanyTestingConnection = false;
    }
  }

  // ── SMTP ────────────────────────────────────────────────────────
  const SMTP_FIELDS = $derived<Array<{ key: string; label: string; placeholder: string; hint?: string; password?: boolean }>>([
    { key: 'smtp_host',       label: m.admin_settings_email_smtp_host(),       placeholder: 'smtp.gmail.com' },
    { key: 'smtp_port',       label: m.admin_settings_email_smtp_port(),       placeholder: '587' },
    { key: 'smtp_username',   label: m.admin_settings_email_smtp_username(),   placeholder: 'you@gmail.com' },
    { key: 'smtp_password',   label: m.admin_settings_email_smtp_password(),   placeholder: m.admin_settings_email_smtp_password_placeholder(), password: true,
      hint: m.admin_settings_email_smtp_password_hint() },
    { key: 'smtp_from_email', label: m.admin_settings_email_from_email(),      placeholder: 'noreply@yourdomain.com' },
    { key: 'smtp_from_name',  label: m.admin_settings_email_from_name(),       placeholder: 'Gyeon' },
    { key: 'public_base_url', label: m.admin_settings_email_public_base_url(), placeholder: 'https://your-storefront.com',
      hint: m.admin_settings_email_public_base_url_hint() },
    { key: 'admin_alert_email', label: m.admin_settings_email_admin_alert(),    placeholder: 'alerts@yourdomain.com',
      hint: m.admin_settings_email_admin_alert_hint() }
  ]);
</script>

<svelte:head><title>{m.admin_settings_title()}</title></svelte:head>

<div class="max-w-3xl">
  <div class="flex items-center justify-between mb-8">
    <h1 class="text-2xl font-bold text-gray-900">{TABS.find((t) => t.id === activeTab)?.label ?? ''}</h1>
  </div>

  <div bind:this={tabsEl}
       onmousemove={onTabsMouseMove}
       onmouseleave={onTabsMouseLeave}
       class="relative flex gap-1 mb-6 border-b border-gray-100 overflow-x-auto overflow-y-hidden">
    <div aria-hidden="true"
         class="pointer-events-none absolute z-0 rounded-lg bg-gray-100
                transition-[transform,width,opacity] duration-[80ms] ease-out
                {tabSpotlight.visible ? 'opacity-100' : 'opacity-0'}"
         style="top: 0; left: 0; transform: translate3d({tabSpotlight.left}px, 0, 0); width: {tabSpotlight.width}px; height: {tabSpotlight.height}px;">
    </div>
    {#each TABS as t}
      <button type="button"
              onclick={() => setTab(t.id as TabId)}
              class="relative z-10 inline-flex items-center gap-1.5 px-4 py-2.5 text-sm font-medium
                     border-b-2 -mb-px whitespace-nowrap transition-colors
                     {activeTab === t.id
                       ? 'border-gray-900 text-gray-900'
                       : 'border-transparent text-gray-400 hover:text-gray-700'}">
        <svg class="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24"
             stroke="currentColor" stroke-width="1.5" aria-hidden="true">
          <path stroke-linecap="round" stroke-linejoin="round" d={TAB_ICONS[t.id]} />
        </svg>
        {t.label}
      </button>
    {/each}
  </div>

  <form method="POST" action="?/save"
        use:enhance={() => {
          if (saving) return;
          saving = true;
          return async ({ result, update }) => {
            await update({ reset: false });
            saving = false;
            if (result.type === 'success') {
              notify.success(m.admin_settings_save_success());
            } else if (result.type === 'failure') {
              notify.error(m.admin_settings_save_failure_title(), (result.data?.error as string) ?? m.admin_settings_save_failure_default());
            } else if (result.type === 'error') {
              notify.error(m.admin_settings_save_failure_title(), result.error?.message ?? m.admin_settings_save_failure_default());
            }
          };
        }}>

    <!-- General tab -->
    <div class="tab-panel" class:active={activeTab === 'general'}>
    <!-- Maintenance Mode -->
    {#if maintenanceSetting}
      <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
        <div class="flex items-center justify-between gap-4">
          <div>
            <p class="text-sm font-semibold text-gray-900">{m.admin_settings_maintenance_heading()}</p>
            {#if SETTING_DESCS[maintenanceSetting.key] ?? maintenanceSetting.description}
              <p class="text-xs text-gray-400 mt-0.5">{SETTING_DESCS[maintenanceSetting.key] ?? maintenanceSetting.description}</p>
            {/if}
          </div>
          <button type="button"
                  onclick={() => (maintenanceOn = !maintenanceOn)}
                  class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent
                         transition-colors duration-200 focus:outline-none focus:ring-2 focus:ring-gray-900 focus:ring-offset-2
                         {maintenanceOn ? 'bg-red-500' : 'bg-gray-200'}"
                  role="switch"
                  aria-checked={maintenanceOn}>
            <span class="pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow transform
                         transition duration-200 {maintenanceOn ? 'translate-x-5' : 'translate-x-0'}"></span>
          </button>
          <input type="hidden" name="maintenance_mode" value={maintenanceOn ? 'true' : 'false'} />
        </div>
        {#if maintenanceOn}
          <p class="mt-3 text-xs text-red-600 font-medium">
            {m.admin_settings_maintenance_warning()}
          </p>
        {/if}
      </div>
    {/if}

    <!-- WebMCP -->
    {#if mcpSetting}
      <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
        <div class="flex items-center justify-between gap-4">
          <div>
            <p class="text-sm font-semibold text-gray-900">{m.admin_settings_webmcp_heading()}</p>
            {#if SETTING_DESCS[mcpSetting.key] ?? mcpSetting.description}
              <p class="text-xs text-gray-400 mt-0.5">{SETTING_DESCS[mcpSetting.key] ?? mcpSetting.description}</p>
            {/if}
          </div>
          <button type="button"
                  onclick={() => (mcpOn = !mcpOn)}
                  class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent
                         transition-colors duration-200 focus:outline-none focus:ring-2 focus:ring-gray-900 focus:ring-offset-2
                         {mcpOn ? 'bg-green-500' : 'bg-gray-200'}"
                  role="switch"
                  aria-checked={mcpOn}>
            <span class="pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow transform
                         transition duration-200 {mcpOn ? 'translate-x-5' : 'translate-x-0'}"></span>
          </button>
          <input type="hidden" name="mcp_enabled" value={mcpOn ? 'true' : 'false'} />
        </div>
      </div>
    {/if}

    <!-- Default Storefront Language -->
    <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
      <div class="flex flex-col gap-1.5">
        <label for="site_locale" class="text-sm font-semibold text-gray-900">
          {m.admin_settings_storefront_lang_heading()}
        </label>
        <p class="text-xs text-gray-400">
          {m.admin_settings_storefront_lang_subtitle()}
        </p>
        <select id="site_locale" name="site_locale"
                class="mt-1 w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm bg-white
                       focus:outline-none focus:ring-2 focus:ring-gray-900">
          {#each STOREFRONT_LANG_OPTIONS as opt}
            <option value={opt.value}
                    selected={(settingValue('site_locale') || 'en') === opt.value}>
              {opt.label}
            </option>
          {/each}
        </select>
      </div>
    </div>

    <!-- Storefront Homepage -->
    <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
      <div class="flex flex-col gap-1.5">
        <label for="homepage_page_id" class="text-sm font-semibold text-gray-900">
          {m.admin_settings_homepage_heading()}
        </label>
        <p class="text-xs text-gray-400">
          {m.admin_settings_homepage_subtitle()}
        </p>
        <select id="homepage_page_id" name="homepage_page_id"
                class="mt-1 w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm bg-white
                       focus:outline-none focus:ring-2 focus:ring-gray-900">
          <option value="" selected={!settingValue('homepage_page_id')}>
            {m.admin_settings_homepage_default_option()}
          </option>
          {#each (data.pages ?? []) as p}
            <option value={p.id} selected={settingValue('homepage_page_id') === p.id}>
              {p.title} (/{p.slug})
            </option>
          {/each}
        </select>
      </div>
    </div>

    <!-- Site Notice (announcement strip) -->
    <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
      <div class="flex items-start justify-between gap-4">
        <div class="flex flex-col gap-0.5">
          <p class="text-sm font-semibold text-gray-900">{m.admin_settings_site_notice_heading()}</p>
          <p class="text-xs text-gray-400">{m.admin_settings_site_notice_subtitle()}</p>
        </div>
        <button type="button"
                onclick={() => (siteNoticeOn = !siteNoticeOn)}
                class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent
                       transition-colors duration-200 focus:outline-none focus:ring-2 focus:ring-gray-900 focus:ring-offset-2
                       {siteNoticeOn ? 'bg-green-500' : 'bg-gray-200'}"
                role="switch"
                aria-checked={siteNoticeOn}
                aria-label={m.admin_settings_site_notice_enabled()}>
          <span class="pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow transform
                       transition duration-200 {siteNoticeOn ? 'translate-x-5' : 'translate-x-0'}"></span>
        </button>
        <input type="hidden" name="site_notice_enabled" value={siteNoticeOn ? 'true' : 'false'} />
      </div>

      <div class="mt-5 flex flex-col gap-1.5">
        <label for="site_notice" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
          {m.admin_settings_site_notice_text_label()}
        </label>
        <textarea id="site_notice" name="site_notice" rows="2"
                  class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm bg-white
                         focus:outline-none focus:ring-2 focus:ring-gray-900"
                  value={settingValue('site_notice') || ''}></textarea>
      </div>

      <div class="mt-4 grid grid-cols-1 sm:grid-cols-3 gap-4">
        <div class="flex flex-col gap-1.5">
          <label for="site_notice_bg_color" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
            {m.admin_settings_site_notice_bg_color()}
          </label>
          <div class="flex items-stretch gap-2">
            <label for="site_notice_bg_color"
                   class="relative h-10 w-10 shrink-0 rounded-xl border border-gray-200 overflow-hidden cursor-pointer
                          shadow-inner hover:border-gray-300 transition-colors"
                   style="background-color: {siteNoticeBgColor}">
              <input id="site_notice_bg_color" name="site_notice_bg_color"
                     type="color"
                     bind:value={siteNoticeBgColor}
                     class="absolute inset-0 w-full h-full opacity-0 cursor-pointer" />
            </label>
            <input type="text"
                   aria-label={m.admin_settings_site_notice_bg_color()}
                   value={siteNoticeBgColor}
                   oninput={(e) => (siteNoticeBgColor = normalizeHex(e.currentTarget.value))}
                   class="flex-1 min-w-0 border border-gray-200 rounded-xl px-3 py-2.5 text-sm font-mono uppercase tracking-wide
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
        </div>
        <div class="flex flex-col gap-1.5">
          <label for="site_notice_text_color" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
            {m.admin_settings_site_notice_text_color()}
          </label>
          <div class="flex items-stretch gap-2">
            <label for="site_notice_text_color"
                   class="relative h-10 w-10 shrink-0 rounded-xl border border-gray-200 overflow-hidden cursor-pointer
                          shadow-inner hover:border-gray-300 transition-colors"
                   style="background-color: {siteNoticeTextColor}">
              <input id="site_notice_text_color" name="site_notice_text_color"
                     type="color"
                     bind:value={siteNoticeTextColor}
                     class="absolute inset-0 w-full h-full opacity-0 cursor-pointer" />
            </label>
            <input type="text"
                   aria-label={m.admin_settings_site_notice_text_color()}
                   value={siteNoticeTextColor}
                   oninput={(e) => (siteNoticeTextColor = normalizeHex(e.currentTarget.value))}
                   class="flex-1 min-w-0 border border-gray-200 rounded-xl px-3 py-2.5 text-sm font-mono uppercase tracking-wide
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
        </div>
        <div class="flex flex-col gap-1.5">
          <label for="site_notice_text_size" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
            {m.admin_settings_site_notice_text_size()}
          </label>
          <div class="flex items-center gap-2">
            <input id="site_notice_text_size" name="site_notice_text_size"
                   type="number" min="8" max="48" step="1"
                   value={settingValue('site_notice_text_size') || '16'}
                   class="w-24 border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
            <span class="text-xs text-gray-400">px</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Favicon -->
    {#if faviconSetting}
      <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
        <MediaPicker
          files={data.mediaFiles ?? []}
          value={faviconUrl}
          onChange={(url) => (faviconUrl = url)}
          accept="image"
          label={m.admin_settings_favicon_heading()}
          description={m.admin_settings_favicon_subtitle()}
        />
        <input type="hidden" name="favicon_url" value={faviconUrl} />
      </div>
    {/if}

    </div>

    <!-- Commerce tab -->
    <div class="tab-panel" class:active={activeTab === 'commerce'}>
    {#if currencySetting}
      <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
        <div class="flex flex-col gap-1.5">
          <label for="currency" class="text-sm font-semibold text-gray-900">
            {m.admin_settings_label_currency()}
          </label>
          {#if SETTING_DESCS['currency'] ?? currencySetting.description}
            <p class="text-xs text-gray-400">{SETTING_DESCS['currency'] ?? currencySetting.description}</p>
          {/if}
          <input id="currency" name="currency" type="text"
                 value={currencySetting.value}
                 class="mt-1 w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                        focus:outline-none focus:ring-2 focus:ring-gray-900" />
        </div>
      </div>
    {/if}

    <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
      <h2 class="text-sm font-semibold text-gray-900 mb-1">{m.admin_settings_hidden_products_heading()}</h2>
      <p class="text-xs text-gray-400 mb-4">{m.admin_settings_hidden_products_subtitle()}</p>
      <MultiSelect
        options={categoryOptions}
        selected={hiddenCategoryIds}
        placeholder={m.admin_settings_hidden_products_placeholder()}
        onChange={(values) => (hiddenCategoryIds = values)}
      />
      <input type="hidden" name="hidden_category_ids" value={JSON.stringify(hiddenCategoryIds)} />
    </div>

    <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
      <div class="flex items-start justify-between gap-4 mb-5">
        <div>
          <h2 class="text-sm font-semibold text-gray-900">{m.admin_settings_payment_heading()}</h2>
          <p class="text-xs text-gray-400 mt-0.5">
            {m.admin_settings_payment_subtitle()}
          </p>
        </div>
      </div>

      <div class="flex items-center justify-between gap-4 pb-5 border-b border-gray-100">
        <div>
          <p class="text-sm font-semibold text-gray-900">{m.admin_settings_payment_mode_heading()}</p>
          <p class="text-xs text-gray-400 mt-0.5">
            {stripeLiveMode ? m.admin_settings_payment_mode_live_hint() : m.admin_settings_payment_mode_test_hint()}
          </p>
        </div>
        <div class="flex items-center gap-3">
          <span class="text-xs font-medium {stripeLiveMode ? 'text-gray-300' : 'text-gray-700'}">{m.admin_settings_payment_mode_test_label()}</span>
          <button type="button"
                  onclick={() => (stripeLiveMode = !stripeLiveMode)}
                  class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent
                         transition-colors duration-200 focus:outline-none focus:ring-2 focus:ring-gray-900 focus:ring-offset-2
                         {stripeLiveMode ? 'bg-indigo-600' : 'bg-gray-300'}"
                  role="switch"
                  aria-checked={stripeLiveMode}>
            <span class="pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow transform
                         transition duration-200 {stripeLiveMode ? 'translate-x-5' : 'translate-x-0'}"></span>
          </button>
          <span class="text-xs font-medium {stripeLiveMode ? 'text-indigo-600' : 'text-gray-300'}">{m.admin_settings_payment_mode_live_label()}</span>
        </div>
        <input type="hidden" name="stripe_mode" value={stripeLiveMode ? 'live' : 'test'} />
      </div>

      <div class="pt-5 border-t border-gray-100 mt-5">
        <div class="flex flex-col gap-1.5">
          <label for="stripe_country" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
            {m.admin_settings_payment_country_heading()}
          </label>
          <p class="text-xs text-gray-400 -mt-0.5">
            {m.admin_settings_payment_country_hint()}
          </p>
          <select id="stripe_country" name="stripe_country"
                  class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm bg-white
                         focus:outline-none focus:ring-2 focus:ring-gray-900">
            {#each STRIPE_COUNTRY_OPTIONS as opt}
              <option value={opt.value}
                      selected={(settingValue('stripe_country') || 'HK') === opt.value}>
                {opt.label}
              </option>
            {/each}
          </select>
        </div>
      </div>

      <div class="pt-5 {stripeLiveMode ? 'opacity-50' : ''}">
        <p class="text-xs font-semibold text-gray-500 uppercase tracking-wide mb-3">
          {m.admin_settings_payment_test_keys()} {#if stripeLiveMode}<span class="font-normal normal-case text-gray-400">{m.admin_settings_payment_keys_inactive()}</span>{/if}
        </p>
        <div class="flex flex-col gap-4">
          <div class="flex flex-col gap-1.5">
            <label for="stripe_test_publishable_key" class="text-xs font-medium text-gray-600">{m.admin_settings_payment_label_publishable()}</label>
            <PasswordInput id="stripe_test_publishable_key" name="stripe_test_publishable_key"
                           value={settingValue('stripe_test_publishable_key')}
                           placeholder="pk_test_..." />
          </div>
          <div class="flex flex-col gap-1.5">
            <label for="stripe_test_secret_key" class="text-xs font-medium text-gray-600">{m.admin_settings_payment_label_secret()}</label>
            <PasswordInput id="stripe_test_secret_key" name="stripe_test_secret_key"
                           value={settingValue('stripe_test_secret_key')}
                           placeholder="sk_test_..." />
          </div>
        </div>
      </div>

      <div class="pt-5 mt-5 border-t border-gray-100 {stripeLiveMode ? '' : 'opacity-50'}">
        <p class="text-xs font-semibold text-gray-500 uppercase tracking-wide mb-3">
          {m.admin_settings_payment_live_keys()} {#if !stripeLiveMode}<span class="font-normal normal-case text-gray-400">{m.admin_settings_payment_keys_inactive()}</span>{/if}
        </p>
        <div class="flex flex-col gap-4">
          <div class="flex flex-col gap-1.5">
            <label for="stripe_live_publishable_key" class="text-xs font-medium text-gray-600">{m.admin_settings_payment_label_publishable()}</label>
            <PasswordInput id="stripe_live_publishable_key" name="stripe_live_publishable_key"
                           value={settingValue('stripe_live_publishable_key')}
                           placeholder="pk_live_..." />
          </div>
          <div class="flex flex-col gap-1.5">
            <label for="stripe_live_secret_key" class="text-xs font-medium text-gray-600">{m.admin_settings_payment_label_secret()}</label>
            <PasswordInput id="stripe_live_secret_key" name="stripe_live_secret_key"
                           value={settingValue('stripe_live_secret_key')}
                           placeholder="sk_live_..." />
          </div>
        </div>
      </div>

      <div class="pt-5 mt-5 border-t border-gray-100">
        <div class="flex flex-col gap-1.5">
          <label for="stripe_webhook_secret" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
            {m.admin_settings_payment_webhook_heading()}
          </label>
          <p class="text-xs text-gray-400 -mt-0.5">
            {m.admin_settings_payment_webhook_hint_pre()}<code class="px-1 py-0.5 bg-gray-50 rounded text-[11px]">POST /api/v1/payments/webhook</code>{m.admin_settings_payment_webhook_hint_mid()}<code class="px-1 py-0.5 bg-gray-50 rounded text-[11px]">whsec_…</code>{m.admin_settings_payment_webhook_hint_post()}
          </p>
          <PasswordInput id="stripe_webhook_secret" name="stripe_webhook_secret"
                         value={settingValue('stripe_webhook_secret')}
                         placeholder="whsec_..." />
        </div>
      </div>

      <div class="pt-5 mt-5 border-t border-gray-100 flex items-center justify-between gap-4">
        <div>
          <p class="text-sm font-semibold text-gray-900">{m.admin_settings_payment_save_cards_heading()}</p>
          <p class="text-xs text-gray-400 mt-0.5">
            {m.admin_settings_payment_save_cards_hint()}
          </p>
        </div>
        <button type="button"
                onclick={() => (stripeSaveCards = !stripeSaveCards)}
                class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent
                       transition-colors duration-200 focus:outline-none focus:ring-2 focus:ring-gray-900 focus:ring-offset-2
                       {stripeSaveCards ? 'bg-green-500' : 'bg-gray-200'}"
                role="switch"
                aria-checked={stripeSaveCards}>
          <span class="pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow transform
                       transition duration-200 {stripeSaveCards ? 'translate-x-5' : 'translate-x-0'}"></span>
        </button>
        <input type="hidden" name="stripe_save_cards" value={stripeSaveCards ? 'true' : 'false'} />
      </div>
    </div>

    <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
      <h2 class="text-sm font-semibold text-gray-900 mb-1">{m.admin_settings_low_stock_heading()}</h2>
      <p class="text-xs text-gray-400 mb-4">{m.admin_settings_low_stock_subtitle()}</p>

      <div class="flex items-center justify-between gap-4 pb-5 border-b border-gray-100">
        <div>
          <p class="text-sm font-semibold text-gray-900">{m.admin_settings_low_stock_alerts_heading()}</p>
          <p class="text-xs text-gray-400 mt-0.5">{m.admin_settings_low_stock_alerts_hint()}</p>
        </div>
        <button type="button"
                onclick={() => (lowStockAlertEnabled = !lowStockAlertEnabled)}
                class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent
                       transition-colors duration-200 focus:outline-none focus:ring-2 focus:ring-gray-900 focus:ring-offset-2
                       {lowStockAlertEnabled ? 'bg-green-500' : 'bg-gray-200'}"
                role="switch"
                aria-checked={lowStockAlertEnabled}>
          <span class="pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow transform
                       transition duration-200 {lowStockAlertEnabled ? 'translate-x-5' : 'translate-x-0'}"></span>
        </button>
        <input type="hidden" name="low_stock_alert_enabled" value={lowStockAlertEnabled ? 'true' : 'false'} />
      </div>

      <div class="pt-5 flex flex-col gap-1.5">
        <label for="low_stock_threshold_default" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
          {m.admin_settings_low_stock_default_heading()}
        </label>
        <p class="text-xs text-gray-400 -mt-0.5">{m.admin_settings_low_stock_default_hint()}</p>
        <input id="low_stock_threshold_default" name="low_stock_threshold_default"
               type="number" min="0" step="1"
               value={settingValue('low_stock_threshold_default') || '5'}
               class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                      focus:outline-none focus:ring-2 focus:ring-gray-900" />
      </div>
    </div>

    <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
      <div class="flex items-start justify-between gap-4 mb-5">
        <div>
          <h2 class="text-sm font-semibold text-gray-900">{m.admin_settings_tax_heading()}</h2>
          <p class="text-xs text-gray-400 mt-0.5">{m.admin_settings_tax_subtitle()}</p>
        </div>
        <button type="button"
                onclick={() => (taxEnabled = !taxEnabled)}
                class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent
                       transition-colors duration-200 focus:outline-none focus:ring-2 focus:ring-gray-900 focus:ring-offset-2
                       {taxEnabled ? 'bg-green-500' : 'bg-gray-200'}"
                role="switch"
                aria-checked={taxEnabled}>
          <span class="pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow transform
                       transition duration-200 {taxEnabled ? 'translate-x-5' : 'translate-x-0'}"></span>
        </button>
        <input type="hidden" name="tax_enabled" value={taxEnabled ? 'true' : 'false'} />
      </div>

      <div class="{taxEnabled ? '' : 'opacity-50 pointer-events-none'} space-y-4">
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div class="flex flex-col gap-1.5">
            <label for="tax_rate_pct" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_settings_tax_rate()}</label>
            <p class="text-xs text-gray-400 -mt-0.5">{m.admin_settings_tax_rate_hint()}</p>
            <div class="relative">
              <input id="tax_rate_pct"
                     type="number" min="0" max="100" step="0.01"
                     bind:value={taxRatePct}
                     placeholder="5"
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 pr-9 text-sm font-mono
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
              <span class="absolute inset-y-0 right-3 flex items-center text-sm text-gray-400 pointer-events-none">%</span>
            </div>
            <input type="hidden" name="tax_rate" value={taxRateValue} />
          </div>
          <div class="flex flex-col gap-1.5">
            <label for="tax_label" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_settings_tax_label()}</label>
            <p class="text-xs text-gray-400 -mt-0.5">{m.admin_settings_tax_label_hint()}</p>
            <input id="tax_label" name="tax_label"
                   type="text" bind:value={taxLabel}
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
        </div>
        <div class="flex items-center justify-between gap-4 pt-3 border-t border-gray-100">
          <div>
            <p class="text-sm font-semibold text-gray-900">{m.admin_settings_tax_inclusive_heading()}</p>
            <p class="text-xs text-gray-400 mt-0.5">{m.admin_settings_tax_inclusive_hint()}</p>
          </div>
          <button type="button"
                  onclick={() => (taxInclusive = !taxInclusive)}
                  class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent
                         transition-colors duration-200 focus:outline-none focus:ring-2 focus:ring-gray-900 focus:ring-offset-2
                         {taxInclusive ? 'bg-green-500' : 'bg-gray-200'}"
                  role="switch"
                  aria-checked={taxInclusive}>
            <span class="pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow transform
                         transition duration-200 {taxInclusive ? 'translate-x-5' : 'translate-x-0'}"></span>
          </button>
          <input type="hidden" name="tax_inclusive" value={taxInclusive ? 'true' : 'false'} />
        </div>

        <div class="pt-3 border-t border-gray-100">
          <p class="text-xs font-semibold text-gray-500 uppercase tracking-wide mb-3">{m.admin_tax_preview_heading()}</p>
          <div class="space-y-1.5 font-mono text-sm">
            <div class="flex justify-between text-gray-600">
              <span>{m.admin_tax_preview_subtotal()}</span>
              <span>{fmtHKD(taxPreviewSubtotal)}</span>
            </div>
            <div class="flex justify-between text-gray-600">
              <span>{taxLabel || 'Tax'} ({taxRatePct.toFixed(2)}%)</span>
              <span>{fmtHKD(taxPreviewTax)}</span>
            </div>
            <div class="flex justify-between pt-1.5 border-t border-gray-100 text-gray-900 font-semibold">
              <span>{m.admin_tax_preview_total()}</span>
              <span>{fmtHKD(taxPreviewTotal)}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
      <div class="flex items-start justify-between gap-4 mb-5">
        <div>
          <h2 class="text-sm font-semibold text-gray-900">{m.admin_settings_loyalty_heading()}</h2>
          <p class="text-xs text-gray-400 mt-0.5">{m.admin_settings_loyalty_subtitle()}</p>
        </div>
        <button type="button"
                onclick={() => (loyaltyEnabled = !loyaltyEnabled)}
                class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent
                       transition-colors duration-200 focus:outline-none focus:ring-2 focus:ring-gray-900 focus:ring-offset-2
                       {loyaltyEnabled ? 'bg-green-500' : 'bg-gray-200'}"
                role="switch"
                aria-checked={loyaltyEnabled}>
          <span class="pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow transform
                       transition duration-200 {loyaltyEnabled ? 'translate-x-5' : 'translate-x-0'}"></span>
        </button>
        <input type="hidden" name="loyalty_enabled" value={loyaltyEnabled ? 'true' : 'false'} />
      </div>

      <div class="{loyaltyEnabled ? '' : 'opacity-50 pointer-events-none'} grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div class="flex flex-col gap-1.5">
          <label for="loyalty_points_per_hkd" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_settings_label_loyalty_points_per_hkd()}</label>
          <p class="text-xs text-gray-400 -mt-0.5">{m.admin_settings_desc_loyalty_points_per_hkd()}</p>
          <input id="loyalty_points_per_hkd" name="loyalty_points_per_hkd"
                 type="number" min="0" step="0.1"
                 value={settingValue('loyalty_points_per_hkd') || '1'}
                 class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm font-mono
                        focus:outline-none focus:ring-2 focus:ring-gray-900" />
        </div>
        <div class="flex flex-col gap-1.5">
          <label for="loyalty_redeem_rate_hkd" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">{m.admin_settings_label_loyalty_redeem_rate_hkd()}</label>
          <p class="text-xs text-gray-400 -mt-0.5">{m.admin_settings_desc_loyalty_redeem_rate_hkd()}</p>
          <input id="loyalty_redeem_rate_hkd" name="loyalty_redeem_rate_hkd"
                 type="number" min="0" step="1"
                 value={settingValue('loyalty_redeem_rate_hkd') || '100'}
                 class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm font-mono
                        focus:outline-none focus:ring-2 focus:ring-gray-900" />
        </div>
      </div>
    </div>

    <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
      <div class="flex items-start justify-between gap-4 mb-5">
        <div>
          <h2 class="text-sm font-semibold text-gray-900">{m.admin_settings_abandoned_heading()}</h2>
          <p class="text-xs text-gray-400 mt-0.5">{m.admin_settings_abandoned_subtitle()}</p>
        </div>
        <button type="button"
                onclick={() => (abandonedEnabled = !abandonedEnabled)}
                class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent
                       transition-colors duration-200 focus:outline-none focus:ring-2 focus:ring-gray-900 focus:ring-offset-2
                       {abandonedEnabled ? 'bg-green-500' : 'bg-gray-200'}"
                role="switch"
                aria-checked={abandonedEnabled}>
          <span class="pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow transform
                       transition duration-200 {abandonedEnabled ? 'translate-x-5' : 'translate-x-0'}"></span>
        </button>
        <input type="hidden" name="abandoned_cart_enabled" value={abandonedEnabled ? 'true' : 'false'} />
      </div>

      <div class="{abandonedEnabled ? '' : 'opacity-50 pointer-events-none'} space-y-4">
        <div class="flex flex-col gap-1.5">
          <label for="abandoned_cart_threshold_hours" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
            {m.admin_settings_abandoned_threshold_heading()}
          </label>
          <p class="text-xs text-gray-400 -mt-0.5">{m.admin_settings_abandoned_threshold_hint()}</p>
          <input id="abandoned_cart_threshold_hours" name="abandoned_cart_threshold_hours"
                 type="number" min="1" step="1"
                 value={settingValue('abandoned_cart_threshold_hours') || '24'}
                 class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm font-mono
                        focus:outline-none focus:ring-2 focus:ring-gray-900" />
        </div>
      </div>

      <div class="pt-4 mt-4 border-t border-gray-100 flex items-center gap-3">
        <button type="button" onclick={runAbandonedNow} disabled={abandonedRunPending}
                class="px-4 py-2 rounded-xl border border-gray-200 text-sm font-medium
                       text-gray-700 hover:bg-gray-50 transition-colors disabled:opacity-60">
          {abandonedRunPending ? m.admin_settings_abandoned_run_pending() : m.admin_settings_abandoned_run_button()}
        </button>
        {#if abandonedRunResult}
          <span class="text-xs text-gray-500">{abandonedRunResult}</span>
        {/if}
      </div>
    </div>

    </div>

    <!-- Logistics tab -->
    <div class="tab-panel" class:active={activeTab === 'logistics'}>
    <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
      <h2 class="text-sm font-semibold text-gray-900 mb-1">{m.admin_settings_shipping_heading()}</h2>
      <p class="text-xs text-gray-400 mb-4">
        {m.admin_settings_shipping_subtitle()}
      </p>
      <MultiSelect
        options={countryOptions}
        selected={shippingCountries}
        placeholder={m.admin_settings_shipping_placeholder()}
        onChange={(values) => (shippingCountries = values)}
      />
      <input type="hidden" name="shipping_countries" value={JSON.stringify(shippingCountries)} />
    </div>

    {#if freeShippingSetting}
      <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
        <div class="flex flex-col gap-1.5">
          <label for="free_shipping_threshold_hkd" class="text-sm font-semibold text-gray-900">
            {m.admin_settings_label_free_shipping_threshold_hkd()}
          </label>
          {#if SETTING_DESCS['free_shipping_threshold_hkd'] ?? freeShippingSetting.description}
            <p class="text-xs text-gray-400">{SETTING_DESCS['free_shipping_threshold_hkd'] ?? freeShippingSetting.description}</p>
          {/if}
          <input id="free_shipping_threshold_hkd" name="free_shipping_threshold_hkd" type="text"
                 value={freeShippingSetting.value}
                 class="mt-1 w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                        focus:outline-none focus:ring-2 focus:ring-gray-900" />
        </div>
      </div>
    {/if}

    <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
      <div class="flex items-start justify-between gap-4 mb-5">
        <div>
          <h2 class="text-sm font-semibold text-gray-900">{m.admin_settings_shipany_heading()}</h2>
          <p class="text-xs text-gray-400 mt-0.5">
            {m.admin_settings_shipany_subtitle_pre()}<a href="https://www.shipany.io" target="_blank" rel="noopener" class="underline hover:text-gray-700">{m.admin_settings_shipany_subtitle_link()}</a>{m.admin_settings_shipany_subtitle_post()}
          </p>
        </div>
        <button type="button"
                onclick={() => (shipanyOn = !shipanyOn)}
                class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent
                       transition-colors duration-200 focus:outline-none focus:ring-2 focus:ring-gray-900 focus:ring-offset-2
                       {shipanyOn ? 'bg-green-500' : 'bg-gray-200'}"
                role="switch"
                aria-checked={shipanyOn}>
          <span class="pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow transform
                       transition duration-200 {shipanyOn ? 'translate-x-5' : 'translate-x-0'}"></span>
        </button>
        <input type="hidden" name="shipany_enabled" value={shipanyOn ? 'true' : 'false'} />
      </div>

      <div class="{shipanyOn ? '' : 'opacity-50 pointer-events-none'}">
        <p class="text-xs font-semibold text-gray-500 uppercase tracking-wide mb-3">{m.admin_settings_shipany_section_credentials()}</p>
        <div class="flex flex-col gap-4">
          <div class="flex flex-col gap-1.5">
            <label for="shipany_user_id" class="text-xs font-medium text-gray-600">
              {m.admin_settings_shipany_label_user_id()} <span class="text-gray-400 font-normal">{m.admin_settings_shipany_label_user_id_hint()}</span>
            </label>
            <input id="shipany_user_id" name="shipany_user_id" type="text"
                   value={settingValue('shipany_user_id')}
                   placeholder={m.admin_settings_shipany_user_id_placeholder()}
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
          <div class="flex flex-col gap-1.5">
            <label for="shipany_api_key" class="text-xs font-medium text-gray-600">
              {m.admin_settings_shipany_label_api_key()}
            </label>
            <p class="text-xs text-gray-400 -mt-0.5">
              {m.admin_settings_shipany_api_key_hint()}
            </p>
            <PasswordInput id="shipany_api_key" name="shipany_api_key"
                           value={settingValue('shipany_api_key')}
                           placeholder={m.admin_settings_shipany_api_key_placeholder()} />
          </div>
          <div class="flex flex-col gap-1.5">
            <label for="shipany_region" class="text-xs font-medium text-gray-600">{m.admin_settings_shipany_label_region()}</label>
            <select id="shipany_region" name="shipany_region"
                    value={settingValue('shipany_region')}
                    class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm bg-white
                           focus:outline-none focus:ring-2 focus:ring-gray-900">
              {#each SHIPANY_REGION_OPTIONS as opt}
                <option value={opt.value} selected={settingValue('shipany_region') === opt.value}>{opt.label}</option>
              {/each}
            </select>
          </div>
        </div>

        <div class="pt-5 mt-5 border-t border-gray-100">
          <p class="text-xs font-semibold text-gray-500 uppercase tracking-wide mb-3">{m.admin_settings_shipany_section_pickup()}</p>
          <p class="text-xs text-gray-400 mb-4">
            {m.admin_settings_shipany_pickup_subtitle()}
          </p>
          <div class="grid grid-cols-2 gap-4">
            <div class="flex flex-col gap-1.5">
              <label for="shipany_origin_name" class="text-xs font-medium text-gray-600">{m.admin_settings_shipany_pickup_contact_name()}</label>
              <input id="shipany_origin_name" name="shipany_origin_name" type="text"
                     value={settingValue('shipany_origin_name')}
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </div>
            <div class="flex flex-col gap-1.5">
              <label for="shipany_origin_phone" class="text-xs font-medium text-gray-600">{m.admin_settings_shipany_pickup_contact_phone()}</label>
              <input id="shipany_origin_phone" name="shipany_origin_phone" type="tel"
                     value={settingValue('shipany_origin_phone')}
                     placeholder={m.admin_settings_shipany_pickup_contact_phone_placeholder()}
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </div>
            <div class="flex flex-col gap-1.5 col-span-2">
              <label for="shipany_origin_line1" class="text-xs font-medium text-gray-600">{m.admin_settings_shipany_pickup_line1()}</label>
              <input id="shipany_origin_line1" name="shipany_origin_line1" type="text"
                     value={settingValue('shipany_origin_line1')}
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </div>
            <div class="flex flex-col gap-1.5 col-span-2">
              <label for="shipany_origin_line2" class="text-xs font-medium text-gray-600">{m.admin_settings_shipany_pickup_line2()}</label>
              <input id="shipany_origin_line2" name="shipany_origin_line2" type="text"
                     value={settingValue('shipany_origin_line2')}
                     placeholder={m.admin_settings_shipany_pickup_line2_placeholder()}
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </div>
            <div class="flex flex-col gap-1.5">
              <label for="shipany_origin_district" class="text-xs font-medium text-gray-600">{m.admin_settings_shipany_pickup_district()}</label>
              <input id="shipany_origin_district" name="shipany_origin_district" type="text" list="hk-districts-origin"
                     value={settingValue('shipany_origin_district')}
                     placeholder={m.admin_settings_shipany_pickup_district_placeholder()}
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
              <datalist id="hk-districts-origin">
                {#each ['中西區','灣仔區','東區','南區','油尖旺區','深水埗區','九龍城區','黃大仙區','觀塘區','葵青區','荃灣區','屯門區','元朗區','北區','大埔區','沙田區','西貢區','離島區'] as d}
                  <option value={d}></option>
                {/each}
              </datalist>
            </div>
            <div class="flex flex-col gap-1.5">
              <label for="shipany_origin_city" class="text-xs font-medium text-gray-600">{m.admin_settings_shipany_pickup_city()}</label>
              <input id="shipany_origin_city" name="shipany_origin_city" type="text"
                     value={settingValue('shipany_origin_city')}
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </div>
            <div class="flex flex-col gap-1.5">
              <label for="shipany_origin_postal" class="text-xs font-medium text-gray-600">
                {m.admin_settings_shipany_pickup_postal()} <span class="text-gray-400 font-normal">{m.admin_settings_shipany_pickup_postal_hint()}</span>
              </label>
              <input id="shipany_origin_postal" name="shipany_origin_postal" type="text"
                     value={settingValue('shipany_origin_postal')}
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </div>
          </div>
        </div>

        <div class="pt-5 mt-5 border-t border-gray-100">
          <p class="text-xs font-semibold text-gray-500 uppercase tracking-wide mb-3">{m.admin_settings_shipany_section_defaults()}</p>
          <div class="grid grid-cols-2 gap-4">
            <div class="flex flex-col gap-1.5">
              <label for="shipany_default_weight_grams" class="text-xs font-medium text-gray-600">
                {m.admin_settings_shipany_default_weight()}
              </label>
              <input id="shipany_default_weight_grams" name="shipany_default_weight_grams"
                     type="number" min="50" step="50"
                     value={settingValue('shipany_default_weight_grams')}
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </div>
            <div class="flex flex-col gap-1.5">
              <label for="shipany_default_storage_type" class="text-xs font-medium text-gray-600">
                {m.admin_settings_shipany_default_storage()}
              </label>
              <select id="shipany_default_storage_type" name="shipany_default_storage_type"
                      value={settingValue('shipany_default_storage_type')}
                      class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm bg-white
                             focus:outline-none focus:ring-2 focus:ring-gray-900">
                {#each SHIPANY_STORAGE_OPTIONS as t}
                  <option value={t} selected={settingValue('shipany_default_storage_type') === t}>{t}</option>
                {/each}
              </select>
            </div>
            <div class="flex flex-col gap-1.5">
              <div class="flex items-center justify-between gap-2">
                <label for="shipany_default_courier" class="text-xs font-medium text-gray-600">
                  {m.admin_settings_shipany_default_courier()} <span class="text-gray-400 font-normal">{m.admin_settings_shipany_default_courier_hint()}</span>
                </label>
                <button type="button" onclick={loadShipanyCouriers}
                        disabled={shipanyCouriersLoading}
                        class="text-xs text-gray-500 hover:text-gray-900 disabled:opacity-50">
                  {shipanyCouriersLoading ? m.admin_settings_shipany_courier_loading() : m.admin_settings_shipany_courier_refresh()}
                </button>
              </div>
              <select id="shipany_default_courier" name="shipany_default_courier"
                      disabled={shipanyCouriersLoading}
                      value={shipanyCourierUID}
                      onchange={(e) => onCourierChange((e.currentTarget as HTMLSelectElement).value)}
                      class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm bg-white
                             focus:outline-none focus:ring-2 focus:ring-gray-900
                             disabled:opacity-50 disabled:cursor-not-allowed">
                {#if shipanyCouriersLoading}
                  <option value="">{m.admin_settings_shipany_courier_loading()}</option>
                {:else}
                  <option value="">{m.admin_settings_shipany_courier_none()}</option>
                  {#each shipanyCouriers as c}
                    <option value={c.uid} selected={shipanyCourierUID === c.uid}>{c.name}</option>
                  {/each}
                {/if}
              </select>
              {#if shipanyCouriersLoaded && !shipanyCouriersLoading && (shipanyCouriersError || shipanyCouriers.length === 0)}
                <p class="text-xs text-gray-400">
                  {#if shipanyCouriersError}
                    {m.admin_settings_shipany_courier_load_failed_pre()}{shipanyCouriersError}
                  {:else}
                    {m.admin_settings_shipany_courier_load_failed_default()}
                  {/if}
                </p>
              {/if}
            </div>
            <div class="flex flex-col gap-1.5">
              <label for="shipany_default_service" class="text-xs font-medium text-gray-600">
                {m.admin_settings_shipany_default_service()} <span class="text-gray-400 font-normal">{m.admin_settings_shipany_default_service_hint()}</span>
              </label>
              {#if selectedCourierSvcPlans.length > 0}
                <select id="shipany_default_service" name="shipany_default_service"
                        bind:value={shipanyServicePl}
                        class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm bg-white
                               focus:outline-none focus:ring-2 focus:ring-gray-900">
                  <option value="">{m.admin_settings_shipany_service_auto()}</option>
                  {#each selectedCourierSvcPlans as p}
                    <option value={p.cour_svc_pl} selected={shipanyServicePl === p.cour_svc_pl}>{p.cour_svc_pl}</option>
                  {/each}
                </select>
              {:else}
                <input id="shipany_default_service" name="shipany_default_service" type="text"
                       bind:value={shipanyServicePl}
                       placeholder={shipanyCourierUID ? m.admin_settings_shipany_service_optional() : m.admin_settings_shipany_service_pick_courier()}
                       disabled={shipanyCouriers.length > 0 && !shipanyCourierUID}
                       class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                              focus:outline-none focus:ring-2 focus:ring-gray-900 disabled:bg-gray-50" />
              {/if}
            </div>
            <div class="flex flex-col gap-1.5 col-span-2">
              <label for="shipany_order_ref_suffix" class="text-xs font-medium text-gray-600">
                {m.admin_settings_shipany_order_ref_suffix()} <span class="text-gray-400 font-normal">{m.admin_settings_shipany_order_ref_hint()}</span>
              </label>
              <input id="shipany_order_ref_suffix" name="shipany_order_ref_suffix" type="text"
                     value={settingValue('shipany_order_ref_suffix')}
                     placeholder={m.admin_settings_shipany_order_ref_placeholder()}
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </div>
          </div>
        </div>

        <div class="pt-5 mt-5 border-t border-gray-100 flex flex-col gap-4">
          <div class="flex items-center justify-between gap-4">
            <div>
              <p class="text-sm font-semibold text-gray-900">{m.admin_settings_shipany_paid_by_receiver()}</p>
              <p class="text-xs text-gray-400 mt-0.5">{m.admin_settings_shipany_paid_by_receiver_hint()}</p>
            </div>
            <button type="button"
                    onclick={() => (shipanyPaidByReceiver = !shipanyPaidByReceiver)}
                    class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent
                           transition-colors duration-200 focus:outline-none focus:ring-2 focus:ring-gray-900 focus:ring-offset-2
                           {shipanyPaidByReceiver ? 'bg-green-500' : 'bg-gray-200'}"
                    role="switch"
                    aria-checked={shipanyPaidByReceiver}>
              <span class="pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow transform
                           transition duration-200 {shipanyPaidByReceiver ? 'translate-x-5' : 'translate-x-0'}"></span>
            </button>
            <input type="hidden" name="shipany_paid_by_receiver" value={shipanyPaidByReceiver ? 'true' : 'false'} />
          </div>
          <div class="flex items-center justify-between gap-4">
            <div>
              <p class="text-sm font-semibold text-gray-900">{m.admin_settings_shipany_self_drop_off()}</p>
              <p class="text-xs text-gray-400 mt-0.5">{m.admin_settings_shipany_self_drop_off_hint()}</p>
            </div>
            <button type="button"
                    onclick={() => (shipanySelfDropOff = !shipanySelfDropOff)}
                    class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent
                           transition-colors duration-200 focus:outline-none focus:ring-2 focus:ring-gray-900 focus:ring-offset-2
                           {shipanySelfDropOff ? 'bg-green-500' : 'bg-gray-200'}"
                    role="switch"
                    aria-checked={shipanySelfDropOff}>
              <span class="pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow transform
                           transition duration-200 {shipanySelfDropOff ? 'translate-x-5' : 'translate-x-0'}"></span>
            </button>
            <input type="hidden" name="shipany_self_drop_off" value={shipanySelfDropOff ? 'true' : 'false'} />
          </div>
          <div class="flex items-center justify-between gap-4">
            <div>
              <p class="text-sm font-semibold text-gray-900">{m.admin_settings_shipany_show_courier_tracking()}</p>
              <p class="text-xs text-gray-400 mt-0.5">{m.admin_settings_shipany_show_courier_tracking_hint()}</p>
            </div>
            <button type="button"
                    onclick={() => (shipanyShowCourierTracking = !shipanyShowCourierTracking)}
                    class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent
                           transition-colors duration-200 focus:outline-none focus:ring-2 focus:ring-gray-900 focus:ring-offset-2
                           {shipanyShowCourierTracking ? 'bg-green-500' : 'bg-gray-200'}"
                    role="switch"
                    aria-checked={shipanyShowCourierTracking}>
              <span class="pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow transform
                           transition duration-200 {shipanyShowCourierTracking ? 'translate-x-5' : 'translate-x-0'}"></span>
            </button>
            <input type="hidden" name="shipany_show_courier_tracking_number" value={shipanyShowCourierTracking ? 'true' : 'false'} />
          </div>
        </div>

        <div class="pt-5 mt-5 border-t border-gray-100 flex items-center gap-3">
          <button type="button"
                  onclick={testShipanyConnection}
                  disabled={shipanyTestingConnection}
                  class="text-sm font-medium text-gray-700 border border-gray-200 rounded-xl px-4 py-2
                         hover:bg-gray-50 transition-colors disabled:opacity-50">
            {shipanyTestingConnection ? m.admin_settings_shipany_testing() : m.admin_settings_shipany_test_button()}
          </button>
          {#if shipanyTestResult}
            <span class="text-xs {shipanyTestResult.ok ? 'text-green-600' : 'text-red-500'}">
              {shipanyTestResult.ok ? '✓' : '✗'} {shipanyTestResult.message || (shipanyTestResult.ok ? m.admin_settings_shipany_test_connected() : m.admin_settings_shipany_test_failed())}
            </span>
          {/if}
        </div>
      </div>
    </div>

    </div>

    <!-- Email tab -->
    <div class="tab-panel" class:active={activeTab === 'email'}>
    <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
      <h2 class="text-sm font-semibold text-gray-900 mb-1">{m.admin_settings_email_heading()}</h2>
      <p class="text-xs text-gray-400 mb-5">
        {m.admin_settings_email_subtitle()}
      </p>

      <div class="flex items-center justify-between gap-4 pb-5 mb-5 border-b border-gray-100">
        <div>
          <p class="text-sm font-semibold text-gray-900">{m.admin_settings_email_enabled_heading()}</p>
          <p class="text-xs text-gray-400 mt-0.5">{m.admin_settings_email_enabled_hint()}</p>
        </div>
        <button type="button"
                onclick={() => (emailEnabled = !emailEnabled)}
                class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent
                       transition-colors duration-200 focus:outline-none focus:ring-2 focus:ring-gray-900 focus:ring-offset-2
                       {emailEnabled ? 'bg-green-500' : 'bg-gray-200'}"
                role="switch"
                aria-checked={emailEnabled}>
          <span class="pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow transform
                       transition duration-200 {emailEnabled ? 'translate-x-5' : 'translate-x-0'}"></span>
        </button>
        <input type="hidden" name="email_enabled" value={emailEnabled ? 'true' : 'false'} />
      </div>

      <div class="flex flex-col gap-5">
        {#each SMTP_FIELDS as field}
          <div class="flex flex-col gap-1.5">
            <label for={field.key} class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
              {field.label}
            </label>
            {#if field.hint}
              <p class="text-xs text-gray-400 -mt-0.5">{field.hint}</p>
            {/if}
            {#if field.password}
              <PasswordInput id={field.key} name={field.key}
                             value={settingValue(field.key)}
                             placeholder={field.placeholder} />
            {:else}
              <input id={field.key} name={field.key}
                     type="text"
                     value={settingValue(field.key)}
                     placeholder={field.placeholder}
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
            {/if}
          </div>
        {/each}
      </div>
      <div class="pt-5 mt-5 border-t border-gray-100">
        <button type="button"
                onclick={() => { showTestEmailModal = true; testEmailAddress = ''; }}
                class="text-sm font-medium text-gray-700 border border-gray-200 rounded-xl px-4 py-2
                       hover:bg-gray-50 transition-colors">
          {m.admin_settings_email_test_button()}
        </button>
      </div>
    </div>

    </div>

    <!-- Infrastructure tab -->
    <div class="tab-panel" class:active={activeTab === 'infrastructure'}>
    {#if cacheTTLSettings.length > 0}
      <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
        <h2 class="text-sm font-semibold text-gray-900 mb-5">{m.admin_settings_section_cache_ttl()}</h2>
        <div class="flex flex-col gap-5">
          {#each cacheTTLSettings as setting}
            <div class="flex flex-col gap-1.5">
              <label for={setting.key}
                     class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
                {CACHE_TTL_LABELS[setting.key] ?? setting.key.replace(/_/g, ' ')}
              </label>
              {#if SETTING_DESCS[setting.key] ?? setting.description}
                <p class="text-xs text-gray-400 -mt-0.5">{SETTING_DESCS[setting.key] ?? setting.description}</p>
              {/if}
              <div class="flex items-center gap-2">
                <input id={setting.key} name={setting.key} type="number" min="60" step="30"
                       value={setting.value}
                       class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm w-32
                              focus:outline-none focus:ring-2 focus:ring-gray-900" />
                <span class="text-xs text-gray-400">{m.admin_settings_cache_ttl_seconds()}</span>
              </div>
            </div>
          {/each}
        </div>
      </div>
    {/if}

    {#if cloudflareSettings.length > 0}
      <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
        <h2 class="text-sm font-semibold text-gray-900 mb-5">{m.admin_settings_section_cloudflare()}</h2>
        <div class="flex flex-col gap-5">
          {#each cloudflareSettings as setting}
            <div class="flex flex-col gap-1.5">
              <label for={setting.key}
                     class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
                {CLOUDFLARE_LABELS[setting.key] ?? setting.key.replace(/_/g, ' ')}
              </label>
              {#if SETTING_DESCS[setting.key] ?? setting.description}
                <p class="text-xs text-gray-400 -mt-0.5">{SETTING_DESCS[setting.key] ?? setting.description}</p>
              {/if}
              {#if setting.key === 'cloudflare_api_token'}
                <PasswordInput id={setting.key} name={setting.key}
                               value={setting.value}
                               placeholder={CLOUDFLARE_PLACEHOLDERS[setting.key] ?? ''} />
              {:else}
                <input id={setting.key} name={setting.key}
                       type="text"
                       value={setting.value}
                       placeholder={CLOUDFLARE_PLACEHOLDERS[setting.key] ?? ''}
                       class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                              focus:outline-none focus:ring-2 focus:ring-gray-900" />
              {/if}
            </div>
          {/each}
        </div>
      </div>
    {/if}

    <!-- reCAPTCHA v3 (spam protection for contact forms) -->
    <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
      <div class="flex items-center justify-between gap-4">
        <div>
          <p class="text-sm font-semibold text-gray-900">{m.admin_settings_recaptcha_heading()}</p>
          <p class="text-xs text-gray-400 mt-0.5">
            {m.admin_settings_recaptcha_subtitle_pre()}<a
              href="https://www.google.com/recaptcha/admin"
              target="_blank"
              rel="noopener"
              class="underline">{m.admin_settings_recaptcha_subtitle_link()}</a>{m.admin_settings_recaptcha_subtitle_post()}
          </p>
        </div>
        <button type="button"
                onclick={() => (recaptchaOn = !recaptchaOn)}
                class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent
                       transition-colors duration-200 focus:outline-none focus:ring-2 focus:ring-gray-900 focus:ring-offset-2
                       {recaptchaOn ? 'bg-green-500' : 'bg-gray-200'}"
                role="switch"
                aria-checked={recaptchaOn}
                aria-label={m.admin_settings_recaptcha_enabled()}>
          <span class="pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow transform
                       transition duration-200 {recaptchaOn ? 'translate-x-5' : 'translate-x-0'}"></span>
        </button>
        <input type="hidden" name="recaptcha_enabled" value={recaptchaOn ? 'true' : 'false'} />
      </div>

      <div class="mt-5 flex flex-col gap-4">
        <div class="flex flex-col gap-1.5">
          <label for="recaptcha_site_key" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
            {m.admin_settings_recaptcha_site_key()}
          </label>
          <input id="recaptcha_site_key" name="recaptcha_site_key"
                 type="text"
                 value={settingValue('recaptcha_site_key')}
                 placeholder="6Lc..."
                 class="w-full border border-gray-200 rounded-xl px-3 py-2.5 font-mono text-sm
                        focus:outline-none focus:ring-2 focus:ring-gray-900" />
        </div>
        <div class="flex flex-col gap-1.5">
          <label for="recaptcha_secret_key" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
            {m.admin_settings_recaptcha_secret_key()}
          </label>
          <PasswordInput id="recaptcha_secret_key" name="recaptcha_secret_key"
                         value={settingValue('recaptcha_secret_key')}
                         placeholder="6Lc..." />
        </div>
      </div>

      <div class="mt-4 flex flex-col gap-1.5">
        <label for="recaptcha_min_score" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
          {m.admin_settings_recaptcha_min_score()}
        </label>
        <p class="text-xs text-gray-400 -mt-0.5">{m.admin_settings_recaptcha_min_score_hint()}</p>
        <input id="recaptcha_min_score" name="recaptcha_min_score"
               type="number" step="0.1" min="0" max="1"
               value={settingValue('recaptcha_min_score') || '0.5'}
               class="w-32 border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                      focus:outline-none focus:ring-2 focus:ring-gray-900" />
      </div>
    </div>

    {#if mediaLimitSettings.length > 0}
      <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
        <h2 class="text-sm font-semibold text-gray-900 mb-5">{m.admin_settings_section_media()}</h2>
        <div class="flex flex-col gap-5">
          {#each mediaLimitSettings as setting}
            <div class="flex flex-col gap-1.5">
              <label for={setting.key}
                     class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
                {MEDIA_LIMIT_LABELS[setting.key] ?? setting.key.replace(/_/g, ' ')}
              </label>
              {#if SETTING_DESCS[setting.key] ?? setting.description}
                <p class="text-xs text-gray-400 -mt-0.5">{SETTING_DESCS[setting.key] ?? setting.description}</p>
              {/if}
              <div class="flex items-center gap-2">
                <input id={setting.key} name={setting.key} type="number" min="1"
                       value={setting.value}
                       class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm w-32
                              focus:outline-none focus:ring-2 focus:ring-gray-900" />
                <span class="text-xs text-gray-400">{m.admin_settings_media_unit_mb()}</span>
              </div>
            </div>
          {/each}
        </div>
      </div>
    {/if}

    </div>

    {#if textSettings.length > 0}
      <div class="tab-panel" class:active={activeTab === 'general'}>
        <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
          <h2 class="text-sm font-semibold text-gray-900 mb-5">{m.admin_settings_section_other()}</h2>
          <div class="flex flex-col gap-5">
            {#each textSettings as setting}
              <div class="flex flex-col gap-1.5">
                <label for={setting.key}
                       class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
                  {TEXT_SETTING_LABELS[setting.key] ?? setting.key.replace(/_/g, ' ')}
                </label>
                {#if SETTING_DESCS[setting.key] ?? setting.description}
                  <p class="text-xs text-gray-400 -mt-0.5">{SETTING_DESCS[setting.key] ?? setting.description}</p>
                {/if}
                <input id={setting.key} name={setting.key} value={setting.value}
                       type={setting.key.includes('secret') || setting.key.includes('password') ? 'password' : 'text'}
                       class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                              focus:outline-none focus:ring-2 focus:ring-gray-900" />
              </div>
            {/each}
          </div>
        </div>
      </div>
    {/if}

    <div class="bg-white rounded-2xl border border-gray-100 p-4 mt-2 flex justify-end">
      <SaveButton loading={saving}
              class="inline-flex items-center justify-center gap-1.5 px-5 py-2.5 bg-gray-900 text-white
                     text-sm font-medium rounded-xl hover:bg-gray-700 transition-colors disabled:opacity-50">
        {m.admin_settings_save()}
      </SaveButton>
    </div>
  </form>
</div>

{#if showTestEmailModal}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div class="absolute inset-0 bg-black/40 backdrop-blur-sm"
         onclick={() => showTestEmailModal = false}
         role="button" tabindex="-1" aria-label={m.admin_modal_close()}></div>
    <div class="relative bg-white rounded-2xl shadow-2xl p-6 w-full max-w-sm">
      <h3 class="font-semibold text-gray-900 mb-1">{m.admin_settings_test_email_modal_title()}</h3>
      <p class="text-xs text-gray-400 mb-4">{m.admin_settings_test_email_modal_subtitle()}</p>

      <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide" for="test-email-to">
        {m.admin_settings_test_email_label()}
      </label>
      <input id="test-email-to" type="email" bind:value={testEmailAddress}
             placeholder={m.admin_settings_test_email_placeholder()}
             class="mt-1.5 w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                    focus:outline-none focus:ring-2 focus:ring-gray-900" />

      <div class="flex gap-3 mt-5">
        <button type="button" disabled={testEmailSending || !testEmailAddress}
                onclick={sendTestEmail}
                class="flex-1 bg-gray-900 text-white text-sm font-medium rounded-xl py-2.5
                       disabled:opacity-50 hover:bg-gray-700 transition-colors">
          {testEmailSending ? m.admin_settings_test_email_sending() : m.admin_settings_test_email_send()}
        </button>
        <button type="button" onclick={() => showTestEmailModal = false}
                class="flex-1 border border-gray-200 text-sm font-medium rounded-xl py-2.5
                       hover:bg-gray-50 transition-colors">
          {m.admin_settings_test_email_cancel()}
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  .tab-panel { display: none; }
  .tab-panel.active {
    display: block;
    animation: tab-in 180ms cubic-bezier(0.16, 1, 0.3, 1);
  }
  @keyframes tab-in {
    from { opacity: 0; transform: translateX(8px); }
    to   { opacity: 1; transform: translateX(0); }
  }
  @media (prefers-reduced-motion: reduce) {
    .tab-panel.active { animation: none; }
  }
</style>
