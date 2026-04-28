<script lang="ts">
  import { onMount } from 'svelte';
  import { enhance } from '$app/forms';
  import type { PageData, ActionData } from './$types';
  import { adminSendTestEmail } from '$lib/api/admin';
  import MultiSelect from '$lib/components/MultiSelect.svelte';
  import { COUNTRIES } from '$lib/data/countries';

  let { data, form }: { data: PageData; form: ActionData } = $props();
  let saving = $state(false);

  // ── Tabs ────────────────────────────────────────────────────────
  const TABS = [
    { id: 'general',        label: 'General' },
    { id: 'commerce',       label: 'Commerce' },
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

  // ── Test Email Modal ─────────────────────────────────────────────
  let showTestEmailModal = $state(false);
  let testEmailAddress = $state('');
  let testEmailSending = $state(false);
  let testEmailResult = $state<{ ok: boolean; msg: string } | null>(null);

  const token = $derived(data.token ?? '');

  async function sendTestEmail() {
    testEmailSending = true;
    testEmailResult = null;
    try {
      await adminSendTestEmail(token, testEmailAddress);
      testEmailResult = { ok: true, msg: 'Test email sent successfully.' };
    } catch {
      testEmailResult = { ok: false, msg: 'Failed to send. Check your SMTP settings and save them first.' };
    } finally {
      testEmailSending = false;
    }
  }

  const TOGGLE_KEYS = new Set(['maintenance_mode', 'mcp_enabled']);
  const SHIPPING_KEYS = new Set(['shipping_countries']);
  const CACHE_TTL_KEYS = new Set(['cache_ttl_shop', 'cache_ttl_cms', 'cache_ttl_nav']);
  const CLOUDFLARE_KEYS = new Set(['cloudflare_zone_id', 'cloudflare_api_token']);
  const MEDIA_LIMIT_KEYS = new Set(['upload_max_image_mb', 'upload_max_video_mb']);
  const PAYMENT_KEYS = new Set([
    'stripe_mode',
    'stripe_test_publishable_key',
    'stripe_test_secret_key',
    'stripe_live_publishable_key',
    'stripe_live_secret_key',
    'stripe_save_cards',
    'stripe_webhook_secret'
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
        !SHIPPING_KEYS.has(s.key)
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

  {#if form?.success}
    <div class="bg-green-50 border border-green-100 text-green-700 text-sm rounded-xl px-4 py-3 mb-6">
      Settings saved successfully.
    </div>
  {/if}
  {#if form?.error}
    <div class="bg-red-50 border border-red-100 text-red-600 text-sm rounded-xl px-4 py-3 mb-6">
      {form.error}
    </div>
  {/if}

  <div class="flex gap-1 mb-6 border-b border-gray-100 overflow-x-auto overflow-y-hidden">
    {#each TABS as t}
      <button type="button"
              onclick={() => setTab(t.id)}
              class="px-4 py-2.5 text-sm font-medium border-b-2 -mb-px whitespace-nowrap transition-colors
                     {activeTab === t.id
                       ? 'border-gray-900 text-gray-900'
                       : 'border-transparent text-gray-400 hover:text-gray-700'}">
        {t.label}
      </button>
    {/each}
  </div>

  <form method="POST" action="?/save"
        use:enhance={() => {
          saving = true;
          return async ({ update }) => { await update({ reset: false }); saving = false; };
        }}>

    <!-- General tab -->
    <div class:hidden={activeTab !== 'general'}>
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
    <div class:hidden={activeTab !== 'commerce'}>
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
                   class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
          <div class="flex flex-col gap-1.5">
            <label for="stripe_test_secret_key" class="text-xs font-medium text-gray-600">Secret key</label>
            <input id="stripe_test_secret_key" name="stripe_test_secret_key"
                   type="password" value={settingValue('stripe_test_secret_key')}
                   placeholder="sk_test_..."
                   class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm
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
                   class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
          <div class="flex flex-col gap-1.5">
            <label for="stripe_live_secret_key" class="text-xs font-medium text-gray-600">Secret key</label>
            <input id="stripe_live_secret_key" name="stripe_live_secret_key"
                   type="password" value={settingValue('stripe_live_secret_key')}
                   placeholder="sk_live_..."
                   class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm
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
                 class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm
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

    <!-- Email tab -->
    <div class:hidden={activeTab !== 'email'}>
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
                   class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
        {/each}
      </div>
      <div class="pt-5 mt-5 border-t border-gray-100">
        <button type="button"
                onclick={() => { showTestEmailModal = true; testEmailResult = null; testEmailAddress = ''; }}
                class="text-sm font-medium text-gray-700 border border-gray-200 rounded-xl px-4 py-2
                       hover:bg-gray-50 transition-colors">
          Test Email
        </button>
      </div>
    </div>

    </div><!-- /Email tab -->

    <!-- Infrastructure tab -->
    <div class:hidden={activeTab !== 'infrastructure'}>
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
                     class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm
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
      <div class:hidden={activeTab !== 'general'}>
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
                       class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm
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
              class="px-5 py-2.5 bg-gray-900 text-white text-sm font-medium rounded-xl
                     hover:bg-gray-700 transition-colors disabled:opacity-50">
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

      {#if testEmailResult}
        <p class="mt-3 text-sm {testEmailResult.ok ? 'text-green-600' : 'text-red-500'}">
          {testEmailResult.msg}
        </p>
      {/if}

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
