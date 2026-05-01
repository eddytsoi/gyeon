<script lang="ts">
  import { onMount } from 'svelte';
  import { enhance } from '$app/forms';
  import type { PageData } from './$types';
  import MultiSelect from '$lib/components/MultiSelect.svelte';
  import SaveIcon from '$lib/components/admin/SaveIcon.svelte';
  import { COUNTRIES } from '$lib/data/countries';
  import { notify } from '$lib/stores/notifications.svelte';

  let { data }: { data: PageData } = $props();
  let saving = $state(false);

  // ── Tabs ────────────────────────────────────────────────────────
  // Heroicons (stroke 1.5) — matches the rest of the admin UI.
  const TAB_ICONS: Record<string, string> = {
    // cog-6-tooth
    general:
      'M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.324.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 0 1 1.37.49l1.296 2.247a1.125 1.125 0 0 1-.26 1.431l-1.003.827c-.293.241-.438.613-.43.992a7.723 7.723 0 0 1 0 .255c-.008.378.137.75.43.991l1.004.827c.424.35.534.955.26 1.43l-1.298 2.247a1.125 1.125 0 0 1-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.47 6.47 0 0 1-.22.128c-.331.183-.581.495-.644.869l-.213 1.28c-.09.543-.56.941-1.11.941h-2.594c-.55 0-1.019-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 0 1-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 0 1-1.369-.49l-1.297-2.247a1.125 1.125 0 0 1 .26-1.431l1.004-.827c.292-.24.437-.613.43-.991a6.932 6.932 0 0 1 0-.255c.007-.38-.138-.751-.43-.992l-1.004-.827a1.125 1.125 0 0 1-.26-1.43l1.297-2.247a1.125 1.125 0 0 1 1.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.087.22-.128.332-.183.582-.495.644-.869l.214-1.28Z M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z',
    // shopping-bag
    commerce:
      'M15.75 10.5V6a3.75 3.75 0 1 0-7.5 0v4.5m11.356-1.993 1.263 12c.07.665-.45 1.243-1.119 1.243H4.25a1.125 1.125 0 0 1-1.12-1.243l1.264-12A1.125 1.125 0 0 1 5.513 7.5h12.974c.576 0 1.059.435 1.119 1.007Z',
    // truck
    logistics:
      'M8.25 18.75a1.5 1.5 0 0 1-3 0m3 0a1.5 1.5 0 0 0-3 0m3 0h6m-9 0H3.375a1.125 1.125 0 0 1-1.125-1.125V14.25m17.25 4.5a1.5 1.5 0 0 1-3 0m3 0a1.5 1.5 0 0 0-3 0m3 0h1.125c.621 0 1.129-.504 1.09-1.124a17.902 17.902 0 0 0-3.213-9.193 2.056 2.056 0 0 0-1.58-.86H14.25M16.5 18.75h-2.25m0-11.177v-.958c0-.568-.422-1.048-.987-1.106a48.554 48.554 0 0 0-10.026 0 1.106 1.106 0 0 0-.987 1.106v7.635m12-6.677v6.677m0 4.5v-4.5m0 0h-12',
    // envelope
    email:
      'M21.75 6.75v10.5a2.25 2.25 0 0 1-2.25 2.25h-15a2.25 2.25 0 0 1-2.25-2.25V6.75m19.5 0A2.25 2.25 0 0 0 19.5 4.5h-15a2.25 2.25 0 0 0-2.25 2.25m19.5 0v.243a2.25 2.25 0 0 1-1.07 1.916l-7.5 4.615a2.25 2.25 0 0 1-2.36 0L3.32 8.91a2.25 2.25 0 0 1-1.07-1.916V6.75',
    // server-stack
    infrastructure:
      'M5.25 14.25h13.5m-13.5 0a3 3 0 0 1-3-3m3 3a3 3 0 1 0 0 6h13.5a3 3 0 1 0 0-6m-16.5-3a3 3 0 0 1 3-3h13.5a3 3 0 0 1 3 3m-19.5 0a4.5 4.5 0 0 1 .9-2.7L5.737 5.1a3.375 3.375 0 0 1 2.7-1.35h7.126c1.062 0 2.062.5 2.7 1.35l2.587 3.45a4.5 4.5 0 0 1 .9 2.7m0 0a3 3 0 0 1-3 3m0 3h.008v.008h-.008v-.008Zm0-6h.008v.008h-.008v-.008Zm-3 6h.008v.008h-.008v-.008Zm0-6h.008v.008h-.008v-.008Z'
  };

  const TABS = [
    { id: 'general',        label: 'General' },
    { id: 'commerce',       label: 'Commerce' },
    { id: 'logistics',      label: 'Logistics' },
    { id: 'email',          label: 'Email' },
    { id: 'infrastructure', label: 'Infrastructure' }
  ] as const;
  type TabId = (typeof TABS)[number]['id'];

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
        notify.success('Test email sent successfully', `Sent to ${testEmailAddress}`);
        showTestEmailModal = false;
      } else {
        let serverMsg = 'Check your SMTP settings and save them first.';
        try {
          const body = await res.json();
          if (typeof body?.error === 'string' && body.error) serverMsg = body.error;
        } catch { /* non-JSON body */ }
        notify.error('Failed to send test email', serverMsg);
      }
    } catch (e) {
      notify.error('Failed to send test email', e instanceof Error ? e.message : 'Network error');
    } finally {
      testEmailSending = false;
    }
  }

  const TOGGLE_KEYS = new Set(['maintenance_mode', 'mcp_enabled']);
  const SHIPPING_KEYS = new Set(['shipping_countries']);
  const CACHE_TTL_KEYS = new Set(['cache_ttl_shop', 'cache_ttl_cms', 'cache_ttl_nav']);
  const CLOUDFLARE_KEYS = new Set(['cloudflare_zone_id', 'cloudflare_api_token']);
  const MEDIA_LIMIT_KEYS = new Set(['upload_max_image_mb', 'upload_max_video_mb']);
  const ORDER_NUMBER_KEYS = new Set(['order_number_prefix']);
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

  // Stripe-supported countries (ISO 3166-1 alpha-2). Source: stripe.com/global.
  const STRIPE_COUNTRY_OPTIONS = [
    { value: 'AU', label: 'Australia' },
    { value: 'AT', label: 'Austria' },
    { value: 'BE', label: 'Belgium' },
    { value: 'BR', label: 'Brazil' },
    { value: 'BG', label: 'Bulgaria' },
    { value: 'CA', label: 'Canada' },
    { value: 'HR', label: 'Croatia' },
    { value: 'CY', label: 'Cyprus' },
    { value: 'CZ', label: 'Czech Republic' },
    { value: 'DK', label: 'Denmark' },
    { value: 'EE', label: 'Estonia' },
    { value: 'FI', label: 'Finland' },
    { value: 'FR', label: 'France' },
    { value: 'DE', label: 'Germany' },
    { value: 'GI', label: 'Gibraltar' },
    { value: 'GR', label: 'Greece' },
    { value: 'HK', label: 'Hong Kong' },
    { value: 'HU', label: 'Hungary' },
    { value: 'IN', label: 'India' },
    { value: 'ID', label: 'Indonesia' },
    { value: 'IE', label: 'Ireland' },
    { value: 'IT', label: 'Italy' },
    { value: 'JP', label: 'Japan' },
    { value: 'LV', label: 'Latvia' },
    { value: 'LI', label: 'Liechtenstein' },
    { value: 'LT', label: 'Lithuania' },
    { value: 'LU', label: 'Luxembourg' },
    { value: 'MY', label: 'Malaysia' },
    { value: 'MT', label: 'Malta' },
    { value: 'MX', label: 'Mexico' },
    { value: 'NL', label: 'Netherlands' },
    { value: 'NZ', label: 'New Zealand' },
    { value: 'NO', label: 'Norway' },
    { value: 'PL', label: 'Poland' },
    { value: 'PT', label: 'Portugal' },
    { value: 'RO', label: 'Romania' },
    { value: 'SG', label: 'Singapore' },
    { value: 'SK', label: 'Slovakia' },
    { value: 'SI', label: 'Slovenia' },
    { value: 'ES', label: 'Spain' },
    { value: 'SE', label: 'Sweden' },
    { value: 'CH', label: 'Switzerland' },
    { value: 'TH', label: 'Thailand' },
    { value: 'AE', label: 'United Arab Emirates' },
    { value: 'GB', label: 'United Kingdom' },
    { value: 'US', label: 'United States' }
  ];
  const SHIPANY_KEYS = new Set([
    'shipany_enabled',
    'shipany_user_id',
    'shipany_api_key',
    'shipany_webhook_secret',
    'shipany_region',
    'shipany_origin_name',
    'shipany_origin_phone',
    'shipany_origin_line1',
    'shipany_origin_line2',
    'shipany_origin_district',
    'shipany_origin_city',
    'shipany_origin_postal',
    'shipany_default_weight_grams',
    'shipany_default_courier',
    'shipany_default_service',
    'shipany_default_storage_type',
    'shipany_paid_by_receiver',
    'shipany_self_drop_off',
    'shipany_order_ref_suffix',
    'shipany_show_courier_tracking_number'
  ]);
  const SMTP_KEYS = new Set([
    'smtp_host',
    'smtp_port',
    'smtp_username',
    'smtp_password',
    'smtp_from_email',
    'smtp_from_name',
    'public_base_url'
  ]);

  const CACHE_TTL_LABELS: Record<string, string> = {
    cache_ttl_shop: 'Shop Cache TTL',
    cache_ttl_cms: 'CMS Cache TTL',
    cache_ttl_nav: 'Navigation Cache TTL'
  };
  const CLOUDFLARE_LABELS: Record<string, string> = {
    cloudflare_zone_id: 'Cloudflare Zone ID',
    cloudflare_api_token: 'Cloudflare API Token'
  };
  const CLOUDFLARE_PLACEHOLDERS: Record<string, string> = {
    cloudflare_zone_id: 'e.g. 5a0f426da5de...',
    cloudflare_api_token: 'cfut_...'
  };
  const MEDIA_LIMIT_LABELS: Record<string, string> = {
    upload_max_image_mb: 'Image Upload Limit (MB)',
    upload_max_video_mb: 'Video Upload Limit (MB)'
  };

  const textSettings = $derived(
    data.settings.filter(
      (s) =>
        !TOGGLE_KEYS.has(s.key) &&
        !CACHE_TTL_KEYS.has(s.key) &&
        !CLOUDFLARE_KEYS.has(s.key) &&
        !MEDIA_LIMIT_KEYS.has(s.key) &&
        !PAYMENT_KEYS.has(s.key) &&
        !SMTP_KEYS.has(s.key) &&
        !SHIPPING_KEYS.has(s.key) &&
        !SHIPANY_KEYS.has(s.key) &&
        !ORDER_NUMBER_KEYS.has(s.key)
    )
  );
  const cacheTTLSettings = $derived(data.settings.filter((s) => CACHE_TTL_KEYS.has(s.key)));
  const cloudflareSettings = $derived(data.settings.filter((s) => CLOUDFLARE_KEYS.has(s.key)));
  const mediaLimitSettings = $derived(data.settings.filter((s) => MEDIA_LIMIT_KEYS.has(s.key)));
  const maintenanceSetting = $derived(data.settings.find((s) => s.key === 'maintenance_mode'));
  let maintenanceOn = $state(maintenanceSetting?.value === 'true');

  const mcpSetting = $derived(data.settings.find((s) => s.key === 'mcp_enabled'));
  let mcpOn = $state(mcpSetting?.value === 'true');

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

  // ── Payment ─────────────────────────────────────────────────────
  function settingValue(key: string): string {
    return data.settings.find((s) => s.key === key)?.value ?? '';
  }

  let stripeLiveMode = $state(settingValue('stripe_mode') === 'live');
  let stripeSaveCards = $state(settingValue('stripe_save_cards') === 'true');

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
        // New envelope: { couriers: [...], error?: string }. Tolerate the
        // older bare-array shape too in case of stale frontend caching.
        if (Array.isArray(body)) {
          shipanyCouriers = body;
        } else {
          shipanyCouriers = Array.isArray(body?.couriers) ? body.couriers : [];
          shipanyCouriersError = body?.error ?? '';
        }
      } else {
        shipanyCouriers = [];
        shipanyCouriersError = `Server returned ${res.status}`;
      }
    } catch (e) {
      shipanyCouriers = [];
      shipanyCouriersError = e instanceof Error ? e.message : 'Network error';
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
    // Reset service plan when courier changes — the previous plan is unlikely
    // to belong to the new courier.
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
          notify.success('ShipAny connected', body.message || '');
        } else {
          notify.error('ShipAny connection failed', body.message || '');
        }
      } else {
        const msg = `Server returned ${res.status}`;
        shipanyTestResult = { ok: false, message: msg };
        notify.error('ShipAny connection failed', msg);
      }
    } catch (e) {
      const msg = e instanceof Error ? e.message : 'Network error';
      shipanyTestResult = { ok: false, message: msg };
      notify.error('ShipAny connection failed', msg);
    } finally {
      shipanyTestingConnection = false;
    }
  }

  // ── SMTP ────────────────────────────────────────────────────────
  const SMTP_FIELDS: Array<{ key: string; label: string; placeholder: string; hint?: string; password?: boolean }> = [
    { key: 'smtp_host', label: 'SMTP Host', placeholder: 'smtp.gmail.com' },
    { key: 'smtp_port', label: 'SMTP Port', placeholder: '587' },
    { key: 'smtp_username', label: 'SMTP Username', placeholder: 'you@gmail.com' },
    { key: 'smtp_password', label: 'SMTP Password', placeholder: 'Gmail App Password (16 chars)', password: true,
      hint: 'Use a Google App Password — not your account password.' },
    { key: 'smtp_from_email', label: 'From Email', placeholder: 'noreply@yourdomain.com' },
    { key: 'smtp_from_name', label: 'From Name', placeholder: 'Gyeon' },
    { key: 'public_base_url', label: 'Public Base URL', placeholder: 'https://your-storefront.com',
      hint: 'Used to build links inside transactional emails.' }
  ];
</script>

<svelte:head><title>Settings — Gyeon Admin</title></svelte:head>

<div class="max-w-3xl">
  <div class="flex items-center justify-between mb-8">
    <h1 class="text-2xl font-bold text-gray-900">Site Settings</h1>
  </div>

  <div bind:this={tabsEl}
       onmousemove={onTabsMouseMove}
       onmouseleave={onTabsMouseLeave}
       class="relative flex gap-1 mb-6 border-b border-gray-100 overflow-x-auto overflow-y-hidden">
    <!-- Magnetic spotlight: glides under the cursor and snaps to the hovered tab -->
    <div aria-hidden="true"
         class="pointer-events-none absolute z-0 rounded-lg bg-gray-100
                transition-[transform,width,opacity] duration-[80ms] ease-out
                {tabSpotlight.visible ? 'opacity-100' : 'opacity-0'}"
         style="top: 0; left: 0; transform: translate3d({tabSpotlight.left}px, 0, 0); width: {tabSpotlight.width}px; height: {tabSpotlight.height}px;">
    </div>
    {#each TABS as t}
      <button type="button"
              onclick={() => setTab(t.id)}
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
          saving = true;
          return async ({ result, update }) => {
            await update({ reset: false });
            saving = false;
            if (result.type === 'success') {
              notify.success('Settings saved');
            } else if (result.type === 'failure') {
              notify.error('Save failed', (result.data?.error as string) ?? 'Please try again.');
            } else if (result.type === 'error') {
              notify.error('Save failed', result.error?.message ?? 'Please try again.');
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
            <p class="text-sm font-semibold text-gray-900">Maintenance Mode</p>
            {#if maintenanceSetting.description}
              <p class="text-xs text-gray-400 mt-0.5">{maintenanceSetting.description}</p>
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
            ⚠ Site is in maintenance mode — non-admin visitors are redirected to the maintenance page.
          </p>
        {/if}
      </div>
    {/if}

    <!-- WebMCP -->
    {#if mcpSetting}
      <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
        <div class="flex items-center justify-between gap-4">
          <div>
            <p class="text-sm font-semibold text-gray-900">WebMCP</p>
            {#if mcpSetting.description}
              <p class="text-xs text-gray-400 mt-0.5">{mcpSetting.description}</p>
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

    </div><!-- /General tab -->

    <!-- Commerce tab -->
    <div class="tab-panel" class:active={activeTab === 'commerce'}>
    <!-- Shipping Countries -->
    <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
      <h2 class="text-sm font-semibold text-gray-900 mb-1">Shipping Countries</h2>
      <p class="text-xs text-gray-400 mb-4">
        {shippingCountriesSetting?.description ?? 'Countries available at checkout (ISO 3166-1 alpha-2 codes).'}
      </p>
      <MultiSelect
        options={countryOptions}
        selected={shippingCountries}
        placeholder="Select countries…"
        onChange={(values) => (shippingCountries = values)}
      />
      <input type="hidden" name="shipping_countries" value={JSON.stringify(shippingCountries)} />
    </div>

    <!-- Payment (Stripe) -->
    <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
      <div class="flex items-start justify-between gap-4 mb-5">
        <div>
          <h2 class="text-sm font-semibold text-gray-900">Payment (Stripe)</h2>
          <p class="text-xs text-gray-400 mt-0.5">
            Configure Stripe credentials and the runtime mode used for new orders.
          </p>
        </div>
      </div>

      <!-- Mode toggle -->
      <div class="flex items-center justify-between gap-4 pb-5 border-b border-gray-100">
        <div>
          <p class="text-sm font-semibold text-gray-900">Mode</p>
          <p class="text-xs text-gray-400 mt-0.5">
            {stripeLiveMode ? 'Live — real charges will be made.' : 'Test — no real money moves.'}
          </p>
        </div>
        <div class="flex items-center gap-3">
          <span class="text-xs font-medium {stripeLiveMode ? 'text-gray-300' : 'text-gray-700'}">Test</span>
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
          <span class="text-xs font-medium {stripeLiveMode ? 'text-indigo-600' : 'text-gray-300'}">Live</span>
        </div>
        <input type="hidden" name="stripe_mode" value={stripeLiveMode ? 'live' : 'test'} />
      </div>

      <!-- Country / Region -->
      <div class="pt-5 border-t border-gray-100 mt-5">
        <div class="flex flex-col gap-1.5">
          <label for="stripe_country" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
            Country / Region
          </label>
          <p class="text-xs text-gray-400 -mt-0.5">
            Country your Stripe account is registered in. Drives the default
            country shown in the payment address fields.
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

      <!-- Test keys -->
      <div class="pt-5 {stripeLiveMode ? 'opacity-50' : ''}">
        <p class="text-xs font-semibold text-gray-500 uppercase tracking-wide mb-3">
          Test Keys {#if stripeLiveMode}<span class="font-normal normal-case text-gray-400">— currently inactive</span>{/if}
        </p>
        <div class="flex flex-col gap-4">
          <div class="flex flex-col gap-1.5">
            <label for="stripe_test_publishable_key" class="text-xs font-medium text-gray-600">Publishable key</label>
            <input id="stripe_test_publishable_key" name="stripe_test_publishable_key"
                   type="password" value={settingValue('stripe_test_publishable_key')}
                   placeholder="pk_test_..."
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
          <div class="flex flex-col gap-1.5">
            <label for="stripe_test_secret_key" class="text-xs font-medium text-gray-600">Secret key</label>
            <input id="stripe_test_secret_key" name="stripe_test_secret_key"
                   type="password" value={settingValue('stripe_test_secret_key')}
                   placeholder="sk_test_..."
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
        </div>
      </div>

      <!-- Live keys -->
      <div class="pt-5 mt-5 border-t border-gray-100 {stripeLiveMode ? '' : 'opacity-50'}">
        <p class="text-xs font-semibold text-gray-500 uppercase tracking-wide mb-3">
          Live Keys {#if !stripeLiveMode}<span class="font-normal normal-case text-gray-400">— currently inactive</span>{/if}
        </p>
        <div class="flex flex-col gap-4">
          <div class="flex flex-col gap-1.5">
            <label for="stripe_live_publishable_key" class="text-xs font-medium text-gray-600">Publishable key</label>
            <input id="stripe_live_publishable_key" name="stripe_live_publishable_key"
                   type="password" value={settingValue('stripe_live_publishable_key')}
                   placeholder="pk_live_..."
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
          <div class="flex flex-col gap-1.5">
            <label for="stripe_live_secret_key" class="text-xs font-medium text-gray-600">Secret key</label>
            <input id="stripe_live_secret_key" name="stripe_live_secret_key"
                   type="password" value={settingValue('stripe_live_secret_key')}
                   placeholder="sk_live_..."
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
        </div>
      </div>

      <!-- Webhook secret -->
      <div class="pt-5 mt-5 border-t border-gray-100">
        <div class="flex flex-col gap-1.5">
          <label for="stripe_webhook_secret" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
            Webhook signing secret
          </label>
          <p class="text-xs text-gray-400 -mt-0.5">
            Register endpoint <code class="px-1 py-0.5 bg-gray-50 rounded text-[11px]">POST /api/v1/payments/webhook</code> in Stripe Dashboard, then paste the <code class="px-1 py-0.5 bg-gray-50 rounded text-[11px]">whsec_…</code> here.
          </p>
          <input id="stripe_webhook_secret" name="stripe_webhook_secret"
                 type="password" value={settingValue('stripe_webhook_secret')}
                 placeholder="whsec_..."
                 class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                        focus:outline-none focus:ring-2 focus:ring-gray-900" />
        </div>
      </div>

      <!-- Save cards -->
      <div class="pt-5 mt-5 border-t border-gray-100 flex items-center justify-between gap-4">
        <div>
          <p class="text-sm font-semibold text-gray-900">Save Cards</p>
          <p class="text-xs text-gray-400 mt-0.5">
            Allow logged-in customers to save cards for future purchases.
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

    </div><!-- /Commerce tab -->

    <!-- Logistics tab -->
    <div class="tab-panel" class:active={activeTab === 'logistics'}>
    <!-- Logistics (ShipAny) -->
    <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
      <div class="flex items-start justify-between gap-4 mb-5">
        <div>
          <h2 class="text-sm font-semibold text-gray-900">ShipAny</h2>
          <p class="text-xs text-gray-400 mt-0.5">
            Live shipping rates at checkout, label printing, pickup booking and tracking via
            <a href="https://www.shipany.io" target="_blank" rel="noopener" class="underline hover:text-gray-700">ShipAny</a>.
            Hong Kong only.
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
        <!-- Credentials -->
        <p class="text-xs font-semibold text-gray-500 uppercase tracking-wide mb-3">Credentials</p>
        <div class="flex flex-col gap-4">
          <div class="flex flex-col gap-1.5">
            <label for="shipany_user_id" class="text-xs font-medium text-gray-600">
              User ID <span class="text-gray-400 font-normal">(informational)</span>
            </label>
            <input id="shipany_user_id" name="shipany_user_id" type="text"
                   value={settingValue('shipany_user_id')}
                   placeholder="fac0f9cf-…"
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
          <div class="flex flex-col gap-1.5">
            <label for="shipany_api_key" class="text-xs font-medium text-gray-600">
              API Key
            </label>
            <p class="text-xs text-gray-400 -mt-0.5">
              From portal.shipany.io → Settings. Env-prefixed keys (SHIPANYDEV / SHIPANYSBX1 / SHIPANYDEMO) are auto-routed to the right subdomain.
            </p>
            <input id="shipany_api_key" name="shipany_api_key" type="password"
                   value={settingValue('shipany_api_key')}
                   placeholder="paste API token"
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
          <div class="flex flex-col gap-1.5">
            <label for="shipany_region" class="text-xs font-medium text-gray-600">Region</label>
            <select id="shipany_region" name="shipany_region"
                    value={settingValue('shipany_region')}
                    class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm bg-white
                           focus:outline-none focus:ring-2 focus:ring-gray-900">
              {#each SHIPANY_REGION_OPTIONS as opt}
                <option value={opt.value} selected={settingValue('shipany_region') === opt.value}>{opt.label}</option>
              {/each}
            </select>
          </div>
          <div class="flex flex-col gap-1.5">
            <label for="shipany_webhook_secret" class="text-xs font-medium text-gray-600">
              Webhook Signing Secret <span class="text-gray-400 font-normal">(optional, untested)</span>
            </label>
            <p class="text-xs text-gray-400 -mt-0.5">
              ShipAny mostly delivers tracking via polling. If your account supports push callbacks,
              register <code class="px-1 py-0.5 bg-gray-50 rounded text-[11px]">POST /api/v1/shipany/webhook</code>
              and paste the HMAC secret here.
            </p>
            <input id="shipany_webhook_secret" name="shipany_webhook_secret" type="password"
                   value={settingValue('shipany_webhook_secret')}
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
        </div>

        <!-- Pickup origin (warehouse address) -->
        <div class="pt-5 mt-5 border-t border-gray-100">
          <p class="text-xs font-semibold text-gray-500 uppercase tracking-wide mb-3">Pickup Origin</p>
          <p class="text-xs text-gray-400 mb-4">
            Sender address used for rate quoting and waybill generation. Falls back to merchant info from the portal when blank.
          </p>
          <div class="grid grid-cols-2 gap-4">
            <div class="flex flex-col gap-1.5">
              <label for="shipany_origin_name" class="text-xs font-medium text-gray-600">Contact name</label>
              <input id="shipany_origin_name" name="shipany_origin_name" type="text"
                     value={settingValue('shipany_origin_name')}
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </div>
            <div class="flex flex-col gap-1.5">
              <label for="shipany_origin_phone" class="text-xs font-medium text-gray-600">Contact phone</label>
              <input id="shipany_origin_phone" name="shipany_origin_phone" type="tel"
                     value={settingValue('shipany_origin_phone')}
                     placeholder="98765432"
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </div>
            <div class="flex flex-col gap-1.5 col-span-2">
              <label for="shipany_origin_line1" class="text-xs font-medium text-gray-600">Address line 1</label>
              <input id="shipany_origin_line1" name="shipany_origin_line1" type="text"
                     value={settingValue('shipany_origin_line1')}
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </div>
            <div class="flex flex-col gap-1.5 col-span-2">
              <label for="shipany_origin_line2" class="text-xs font-medium text-gray-600">Address line 2</label>
              <input id="shipany_origin_line2" name="shipany_origin_line2" type="text"
                     value={settingValue('shipany_origin_line2')}
                     placeholder="Building / floor / unit"
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </div>
            <div class="flex flex-col gap-1.5">
              <label for="shipany_origin_district" class="text-xs font-medium text-gray-600">District</label>
              <input id="shipany_origin_district" name="shipany_origin_district" type="text" list="hk-districts-origin"
                     value={settingValue('shipany_origin_district')}
                     placeholder="觀塘區"
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
              <datalist id="hk-districts-origin">
                {#each ['中西區','灣仔區','東區','南區','油尖旺區','深水埗區','九龍城區','黃大仙區','觀塘區','葵青區','荃灣區','屯門區','元朗區','北區','大埔區','沙田區','西貢區','離島區'] as d}
                  <option value={d}></option>
                {/each}
              </datalist>
            </div>
            <div class="flex flex-col gap-1.5">
              <label for="shipany_origin_city" class="text-xs font-medium text-gray-600">City</label>
              <input id="shipany_origin_city" name="shipany_origin_city" type="text"
                     value={settingValue('shipany_origin_city')}
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </div>
            <div class="flex flex-col gap-1.5">
              <label for="shipany_origin_postal" class="text-xs font-medium text-gray-600">
                Postal code <span class="text-gray-400 font-normal">(HK has none)</span>
              </label>
              <input id="shipany_origin_postal" name="shipany_origin_postal" type="text"
                     value={settingValue('shipany_origin_postal')}
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </div>
          </div>
        </div>

        <!-- Defaults -->
        <div class="pt-5 mt-5 border-t border-gray-100">
          <p class="text-xs font-semibold text-gray-500 uppercase tracking-wide mb-3">Defaults</p>
          <div class="grid grid-cols-2 gap-4">
            <div class="flex flex-col gap-1.5">
              <label for="shipany_default_weight_grams" class="text-xs font-medium text-gray-600">
                Fallback weight (grams)
              </label>
              <input id="shipany_default_weight_grams" name="shipany_default_weight_grams"
                     type="number" min="50" step="50"
                     value={settingValue('shipany_default_weight_grams')}
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </div>
            <div class="flex flex-col gap-1.5">
              <label for="shipany_default_storage_type" class="text-xs font-medium text-gray-600">
                Default storage temperature
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
                  Default courier <span class="text-gray-400 font-normal">(cour_uid)</span>
                </label>
                <button type="button" onclick={loadShipanyCouriers}
                        disabled={shipanyCouriersLoading}
                        class="text-xs text-gray-500 hover:text-gray-900 disabled:opacity-50">
                  {shipanyCouriersLoading ? 'Loading…' : 'Refresh'}
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
                  <option value="">Loading…</option>
                {:else}
                  <option value="">— None —</option>
                  {#each shipanyCouriers as c}
                    <option value={c.uid} selected={shipanyCourierUID === c.uid}>{c.name}</option>
                  {/each}
                {/if}
              </select>
              {#if shipanyCouriersLoaded && !shipanyCouriersLoading && (shipanyCouriersError || shipanyCouriers.length === 0)}
                <p class="text-xs text-gray-400">
                  {#if shipanyCouriersError}
                    Couldn't load courier list: {shipanyCouriersError}
                  {:else}
                    Couldn't load courier list — check credentials and Refresh.
                  {/if}
                </p>
              {/if}
            </div>
            <div class="flex flex-col gap-1.5">
              <label for="shipany_default_service" class="text-xs font-medium text-gray-600">
                Default service plan <span class="text-gray-400 font-normal">(cour_svc_pl)</span>
              </label>
              {#if selectedCourierSvcPlans.length > 0}
                <select id="shipany_default_service" name="shipany_default_service"
                        bind:value={shipanyServicePl}
                        class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm bg-white
                               focus:outline-none focus:ring-2 focus:ring-gray-900">
                  <option value="">— Auto —</option>
                  {#each selectedCourierSvcPlans as p}
                    <option value={p.cour_svc_pl} selected={shipanyServicePl === p.cour_svc_pl}>{p.cour_svc_pl}</option>
                  {/each}
                </select>
              {:else}
                <input id="shipany_default_service" name="shipany_default_service" type="text"
                       bind:value={shipanyServicePl}
                       placeholder={shipanyCourierUID ? 'optional' : 'pick a courier first'}
                       disabled={shipanyCouriers.length > 0 && !shipanyCourierUID}
                       class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                              focus:outline-none focus:ring-2 focus:ring-gray-900 disabled:bg-gray-50" />
              {/if}
            </div>
            <div class="flex flex-col gap-1.5 col-span-2">
              <label for="shipany_order_ref_suffix" class="text-xs font-medium text-gray-600">
                Order ref suffix <span class="text-gray-400 font-normal">(appended to ext_order_ref)</span>
              </label>
              <input id="shipany_order_ref_suffix" name="shipany_order_ref_suffix" type="text"
                     value={settingValue('shipany_order_ref_suffix')}
                     placeholder="-GYE"
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </div>
          </div>
        </div>

        <!-- Behaviour toggles -->
        <div class="pt-5 mt-5 border-t border-gray-100 flex flex-col gap-4">
          <div class="flex items-center justify-between gap-4">
            <div>
              <p class="text-sm font-semibold text-gray-900">Paid by receiver</p>
              <p class="text-xs text-gray-400 mt-0.5">Bill the recipient instead of the merchant.</p>
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
              <p class="text-sm font-semibold text-gray-900">Self drop-off</p>
              <p class="text-xs text-gray-400 mt-0.5">Drop parcels at the courier counter instead of door pickup.</p>
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
              <p class="text-sm font-semibold text-gray-900">Show courier tracking number</p>
              <p class="text-xs text-gray-400 mt-0.5">Surface the courier-side tracking number to customers.</p>
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

        <!-- Test connection -->
        <div class="pt-5 mt-5 border-t border-gray-100 flex items-center gap-3">
          <button type="button"
                  onclick={testShipanyConnection}
                  disabled={shipanyTestingConnection}
                  class="text-sm font-medium text-gray-700 border border-gray-200 rounded-xl px-4 py-2
                         hover:bg-gray-50 transition-colors disabled:opacity-50">
            {shipanyTestingConnection ? 'Testing…' : 'Test connection'}
          </button>
          {#if shipanyTestResult}
            <span class="text-xs {shipanyTestResult.ok ? 'text-green-600' : 'text-red-500'}">
              {shipanyTestResult.ok ? '✓' : '✗'} {shipanyTestResult.message || (shipanyTestResult.ok ? 'Connected.' : 'Failed.')}
            </span>
          {/if}
        </div>
      </div>
    </div>

    </div><!-- /Logistics tab -->

    <!-- Email tab -->
    <div class="tab-panel" class:active={activeTab === 'email'}>
    <!-- SMTP / Email -->
    <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
      <h2 class="text-sm font-semibold text-gray-900 mb-1">Email (SMTP)</h2>
      <p class="text-xs text-gray-400 mb-5">
        Used to send order confirmation emails. Gmail: enable 2FA → create an App Password at myaccount.google.com/apppasswords.
      </p>
      <div class="flex flex-col gap-5">
        {#each SMTP_FIELDS as field}
          <div class="flex flex-col gap-1.5">
            <label for={field.key} class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
              {field.label}
            </label>
            {#if field.hint}
              <p class="text-xs text-gray-400 -mt-0.5">{field.hint}</p>
            {/if}
            <input id={field.key} name={field.key}
                   type={field.password ? 'password' : 'text'}
                   value={settingValue(field.key)}
                   placeholder={field.placeholder}
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
        {/each}
      </div>
      <div class="pt-5 mt-5 border-t border-gray-100">
        <button type="button"
                onclick={() => { showTestEmailModal = true; testEmailAddress = ''; }}
                class="text-sm font-medium text-gray-700 border border-gray-200 rounded-xl px-4 py-2
                       hover:bg-gray-50 transition-colors">
          Test Email
        </button>
      </div>
    </div>

    </div><!-- /Email tab -->

    <!-- Infrastructure tab -->
    <div class="tab-panel" class:active={activeTab === 'infrastructure'}>
    <!-- Cache TTL Settings -->
    {#if cacheTTLSettings.length > 0}
      <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
        <h2 class="text-sm font-semibold text-gray-900 mb-5">Cache TTL</h2>
        <div class="flex flex-col gap-5">
          {#each cacheTTLSettings as setting}
            <div class="flex flex-col gap-1.5">
              <label for={setting.key}
                     class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
                {CACHE_TTL_LABELS[setting.key] ?? setting.key.replace(/_/g, ' ')}
              </label>
              {#if setting.description}
                <p class="text-xs text-gray-400 -mt-0.5">{setting.description}</p>
              {/if}
              <div class="flex items-center gap-2">
                <input id={setting.key} name={setting.key} type="number" min="60" step="30"
                       value={setting.value}
                       class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm w-32
                              focus:outline-none focus:ring-2 focus:ring-gray-900" />
                <span class="text-xs text-gray-400">seconds</span>
              </div>
            </div>
          {/each}
        </div>
      </div>
    {/if}

    <!-- Cloudflare -->
    {#if cloudflareSettings.length > 0}
      <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
        <h2 class="text-sm font-semibold text-gray-900 mb-5">Cloudflare</h2>
        <div class="flex flex-col gap-5">
          {#each cloudflareSettings as setting}
            <div class="flex flex-col gap-1.5">
              <label for={setting.key}
                     class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
                {CLOUDFLARE_LABELS[setting.key] ?? setting.key.replace(/_/g, ' ')}
              </label>
              {#if setting.description}
                <p class="text-xs text-gray-400 -mt-0.5">{setting.description}</p>
              {/if}
              <input id={setting.key} name={setting.key}
                     type={setting.key === 'cloudflare_api_token' ? 'password' : 'text'}
                     value={setting.value}
                     placeholder={CLOUDFLARE_PLACEHOLDERS[setting.key] ?? ''}
                     class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                            focus:outline-none focus:ring-2 focus:ring-gray-900" />
            </div>
          {/each}
        </div>
      </div>
    {/if}

    <!-- Media Upload Limits -->
    {#if mediaLimitSettings.length > 0}
      <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
        <h2 class="text-sm font-semibold text-gray-900 mb-5">Media</h2>
        <div class="flex flex-col gap-5">
          {#each mediaLimitSettings as setting}
            <div class="flex flex-col gap-1.5">
              <label for={setting.key}
                     class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
                {MEDIA_LIMIT_LABELS[setting.key] ?? setting.key.replace(/_/g, ' ')}
              </label>
              {#if setting.description}
                <p class="text-xs text-gray-400 -mt-0.5">{setting.description}</p>
              {/if}
              <div class="flex items-center gap-2">
                <input id={setting.key} name={setting.key} type="number" min="1"
                       value={setting.value}
                       class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm w-32
                              focus:outline-none focus:ring-2 focus:ring-gray-900" />
                <span class="text-xs text-gray-400">MB</span>
              </div>
            </div>
          {/each}
        </div>
      </div>
    {/if}

    </div><!-- /Infrastructure tab -->

    <!-- General tab (continued — catch-all text settings) -->
    {#if textSettings.length > 0}
      <div class="tab-panel" class:active={activeTab === 'general'}>
        <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
          <h2 class="text-sm font-semibold text-gray-900 mb-5">Other</h2>
          <div class="flex flex-col gap-5">
            {#each textSettings as setting}
              <div class="flex flex-col gap-1.5">
                <label for={setting.key}
                       class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
                  {setting.key.replace(/_/g, ' ')}
                </label>
                {#if setting.description}
                  <p class="text-xs text-gray-400 -mt-0.5">{setting.description}</p>
                {/if}
                <input id={setting.key} name={setting.key} value={setting.value}
                       class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                              focus:outline-none focus:ring-2 focus:ring-gray-900" />
              </div>
            {/each}
          </div>
        </div>
      </div>
    {/if}

    <!-- Save bar (always visible across tabs) -->
    <div class="bg-white rounded-2xl border border-gray-100 p-4 mt-2 flex justify-end">
      <button type="submit" disabled={saving}
              class="inline-flex items-center justify-center gap-1.5 px-5 py-2.5 bg-gray-900 text-white
                     text-sm font-medium rounded-xl hover:bg-gray-700 transition-colors disabled:opacity-50">
        <SaveIcon />
        {saving ? 'Saving…' : 'Save Settings'}
      </button>
    </div>
  </form>
</div>

{#if showTestEmailModal}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div class="absolute inset-0 bg-black/40 backdrop-blur-sm"
         onclick={() => showTestEmailModal = false}
         role="button" tabindex="-1" aria-label="Close"></div>
    <div class="relative bg-white rounded-2xl shadow-2xl p-6 w-full max-w-sm">
      <h3 class="font-semibold text-gray-900 mb-1">Send Test Email</h3>
      <p class="text-xs text-gray-400 mb-4">Sends a test email using your current saved SMTP settings.</p>

      <label class="text-xs font-semibold text-gray-500 uppercase tracking-wide" for="test-email-to">
        Recipient Email
      </label>
      <input id="test-email-to" type="email" bind:value={testEmailAddress}
             placeholder="you@example.com"
             class="mt-1.5 w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                    focus:outline-none focus:ring-2 focus:ring-gray-900" />

      <div class="flex gap-3 mt-5">
        <button type="button" disabled={testEmailSending || !testEmailAddress}
                onclick={sendTestEmail}
                class="flex-1 bg-gray-900 text-white text-sm font-medium rounded-xl py-2.5
                       disabled:opacity-50 hover:bg-gray-700 transition-colors">
          {testEmailSending ? 'Sending…' : 'Test Email'}
        </button>
        <button type="button" onclick={() => showTestEmailModal = false}
                class="flex-1 border border-gray-200 text-sm font-medium rounded-xl py-2.5
                       hover:bg-gray-50 transition-colors">
          Cancel
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  /* Tab panel transitions — Linear/Radix-style snappy fade + slide.
     Keeps panels mounted (form fields persist across tab switches), but
     hides inactive ones via display:none and animates the active panel in. */
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
