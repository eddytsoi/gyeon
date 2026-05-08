<script lang="ts">
  import { onMount } from 'svelte';
  import type { PageData } from './$types';
  import { notify } from '$lib/stores/notifications.svelte';
  import * as m from '$lib/paraglide/messages';
  import PasswordInput from '$lib/components/admin/PasswordInput.svelte';

  let { data }: { data: PageData } = $props();

  type Step = 'idle' | 'confirming' | 'testing' | 'importing' | 'done' | 'error';
  type Mode = 'upsert' | 'replace';
  type ProductType = 'products' | 'bundle_products';

  interface Progress {
    total_products: number;
    processed_products: number;
    imported_products: number;
    updated_products: number;
    imported_variants: number;
    stale_deleted: number;
    failed: number;
    current_product?: string;
    done: boolean;
    errors: string[];
  }

  // ── Tabs ─────────────────────────────────────────────────────────
  const TAB_ICONS: Record<string, string> = {
    products:
      'M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5M16.5 12 12 16.5m0 0L7.5 12m4.5 4.5V3',
    orders:
      'M9 12h3.75M9 15h3.75M9 18h3.75m3 .75H18a2.25 2.25 0 0 0 2.25-2.25V6.108c0-1.135-.845-2.098-1.976-2.192a48.424 48.424 0 0 0-1.123-.08m-5.801 0c-.065.21-.1.433-.1.664 0 .414.336.75.75.75h4.5a.75.75 0 0 0 .75-.75 2.25 2.25 0 0 0-.1-.664m-5.8 0A2.251 2.251 0 0 1 13.5 2.25H15c1.012 0 1.867.668 2.15 1.586m-5.8 0c-.376.023-.75.05-1.124.08C9.095 4.01 8.25 4.973 8.25 6.108V8.25m0 0H4.875c-.621 0-1.125.504-1.125 1.125v11.25c0 .621.504 1.125 1.125 1.125h9.75c.621 0 1.125-.504 1.125-1.125V9.375c0-.621-.504-1.125-1.125-1.125H8.25ZM6.75 12h.008v.008H6.75V12Zm0 3h.008v.008H6.75V15Zm0 3h.008v.008H6.75V18Z',
    customers:
      'M15 19.128a9.38 9.38 0 0 0 2.625.372 9.337 9.337 0 0 0 4.121-.952 4.125 4.125 0 0 0-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 0 1 8.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0 1 11.964-3.07M12 6.375a3.375 3.375 0 1 1-6.75 0 3.375 3.375 0 0 1 6.75 0Zm8.25 2.25a2.625 2.625 0 1 1-5.25 0 2.625 2.625 0 0 1 5.25 0Z',
    settings:
      'M10.5 6h9.75M10.5 6a1.5 1.5 0 1 1-3 0m3 0a1.5 1.5 0 1 0-3 0M3.75 6H7.5m3 12h9.75m-9.75 0a1.5 1.5 0 0 1-3 0m3 0a1.5 1.5 0 0 0-3 0m-3.75 0H7.5m9-6h3.75m-3.75 0a1.5 1.5 0 0 1-3 0m3 0a1.5 1.5 0 0 0-3 0m-9.75 0h9.75'
  };

  const TABS = $derived([
    { id: 'products', label: m.admin_import_tab_products() },
    { id: 'orders', label: m.admin_import_tab_orders() },
    { id: 'customers', label: m.admin_import_tab_customers() },
    { id: 'settings', label: m.admin_import_tab_settings() }
  ] as const);
  type TabId = 'products' | 'orders' | 'customers' | 'settings';

  let activeTab = $state<TabId>('products');

  onMount(() => {
    const fromHash = window.location.hash.slice(1) as TabId;
    if (TABS.some((t) => t.id === fromHash)) activeTab = fromHash;
  });

  function setTab(id: TabId) {
    activeTab = id;
    history.replaceState(null, '', `#${id}`);
  }

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

  // ── Import flow state ────────────────────────────────────────────
  let step = $state<Step>('idle');
  let errorMsg = $state('');
  let progress = $state<Progress | null>(null);
  let mode = $state<Mode>('upsert');
  let productType = $state<ProductType>('products');

  let wcUrl = $state('');
  let wcKey = $state('');
  let wcSecret = $state('');
  let limit = $state<number | null>(null);

  let savingCreds = $state(false);
  let testingConn = $state(false);

  const credsConfigured = $derived(
    wcUrl.trim() !== '' && wcKey.trim() !== '' && wcSecret.trim() !== ''
  );

  async function loadCredentials() {
    try {
      const res = await fetch('/api/v1/admin/import/woocommerce/credentials', {
        headers: { Authorization: `Bearer ${data.token}` }
      });
      if (!res.ok) return;
      const j = await res.json();
      wcUrl = j.wc_url ?? '';
      wcKey = j.wc_key ?? '';
      wcSecret = j.wc_secret ?? '';
    } catch { /* silent — admin can still type creds manually */ }
  }

  async function saveCredentials() {
    savingCreds = true;
    try {
      const res = await fetch('/api/v1/admin/import/woocommerce/credentials', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${data.token}` },
        body: JSON.stringify({ wc_url: wcUrl, wc_key: wcKey, wc_secret: wcSecret })
      });
      if (!res.ok) {
        notify.error(m.admin_import_save_failed_title(), (await res.text()) || m.admin_import_save_failed_default());
      } else {
        notify.success(m.admin_import_save_creds_success());
      }
    } catch (e) {
      notify.error(m.admin_import_save_failed_title(), e instanceof Error ? e.message : m.admin_import_save_failed_default());
    } finally {
      savingCreds = false;
    }
  }

  async function testConnection() {
    if (!credsConfigured) {
      notify.error(m.admin_import_test_failed_title(), m.admin_import_test_failed_missing_creds());
      return;
    }
    testingConn = true;
    try {
      const res = await fetch('/api/v1/admin/import/woocommerce/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${data.token}` },
        body: JSON.stringify({ wc_url: wcUrl, wc_key: wcKey, wc_secret: wcSecret })
      });
      if (!res.ok) {
        const msg = (await res.text()) || m.admin_import_test_failed_check_keys();
        notify.error(m.admin_import_test_failed_title(), msg);
        return;
      }
      const j = await res.json();
      const total = typeof j.total_products === 'number' ? j.total_products : null;
      notify.success(
        m.admin_import_test_success_title(),
        total !== null ? m.admin_import_test_success_total({ total }) : undefined
      );
    } catch (e) {
      notify.error(m.admin_import_test_failed_title(), e instanceof Error ? e.message : m.admin_import_test_failed_timeout());
    } finally {
      testingConn = false;
    }
  }

  $effect(() => {
    loadCredentials();
  });

  function openConfirm() {
    if (!credsConfigured) {
      notify.error(m.admin_import_no_creds_title(), m.admin_import_no_creds_body());
      setTab('settings');
      return;
    }
    step = 'confirming';
  }

  function cancelConfirm() {
    step = 'idle';
  }

  async function runImport() {
    step = 'testing';
    errorMsg = '';
    progress = null;

    try {
      const testRes = await fetch('/api/v1/admin/import/woocommerce/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${data.token}` },
        body: JSON.stringify({ wc_url: wcUrl, wc_key: wcKey, wc_secret: wcSecret })
      });
      if (!testRes.ok) {
        errorMsg = (await testRes.text()) || m.admin_import_run_failed_default();
        step = 'error';
        return;
      }
    } catch {
      errorMsg = m.admin_import_run_timeout();
      step = 'error';
      return;
    }

    step = 'importing';
    try {
      const res = await fetch('/api/v1/admin/import/woocommerce/stream', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${data.token}` },
        body: JSON.stringify({
          wc_url: wcUrl,
          wc_key: wcKey,
          wc_secret: wcSecret,
          mode,
          product_type: productType,
          limit: limit && limit > 0 ? Math.floor(limit) : 0
        })
      });

      if (!res.ok || !res.body) {
        errorMsg = (await res.text()) || m.admin_import_stream_failed_default();
        step = 'error';
        return;
      }

      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buffer = '';

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        const messages = buffer.split('\n\n');
        buffer = messages.pop() ?? '';
        for (const msg of messages) {
          const dataLine = msg.split('\n').find(l => l.startsWith('data: '));
          if (!dataLine) continue;
          try {
            const update: Progress = JSON.parse(dataLine.slice(6));
            progress = update;
            if (update.done) { step = 'done'; }
          } catch { /* ignore malformed */ }
        }
      }

      if (step === 'importing') {
        errorMsg = m.admin_import_stream_dropped();
        step = 'error';
      }
    } catch {
      errorMsg = m.admin_import_run_error();
      step = 'error';
    }
  }

  // ── Customers import (parallel state machine) ────────────────────
  interface CustomersProgress {
    total_customers: number;
    processed_customers: number;
    imported_customers: number;
    updated_customers: number;
    imported_addresses: number;
    setup_emails_queued: number;
    failed: number;
    current_customer?: string;
    done: boolean;
    errors: string[];
  }

  type CustomersSetupEmailMode = 'skip' | 'passwordless' | 'force';

  let customersStep = $state<Step>('idle');
  let customersErrorMsg = $state('');
  let customersProgress = $state<CustomersProgress | null>(null);
  let customersLimit = $state<number | null>(null);
  let customersSetupEmailMode = $state<CustomersSetupEmailMode>('passwordless');

  const customersPct = $derived(
    customersProgress && customersProgress.total_customers > 0
      ? Math.min(100, Math.round((customersProgress.processed_customers / customersProgress.total_customers) * 100))
      : 0
  );

  function openCustomersConfirm() {
    if (!credsConfigured) {
      notify.error(m.admin_import_no_creds_title(), m.admin_import_no_creds_body());
      setTab('settings');
      return;
    }
    customersStep = 'confirming';
  }

  function cancelCustomersConfirm() {
    customersStep = 'idle';
  }

  async function runCustomersImport() {
    customersStep = 'testing';
    customersErrorMsg = '';
    customersProgress = null;

    try {
      const testRes = await fetch('/api/v1/admin/import/woocommerce/customers/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${data.token}` },
        body: JSON.stringify({ wc_url: wcUrl, wc_key: wcKey, wc_secret: wcSecret })
      });
      if (!testRes.ok) {
        customersErrorMsg = (await testRes.text()) || m.admin_import_run_failed_default();
        customersStep = 'error';
        return;
      }
    } catch {
      customersErrorMsg = m.admin_import_run_timeout();
      customersStep = 'error';
      return;
    }

    customersStep = 'importing';
    try {
      const res = await fetch('/api/v1/admin/import/woocommerce/customers/stream', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${data.token}` },
        body: JSON.stringify({
          wc_url: wcUrl,
          wc_key: wcKey,
          wc_secret: wcSecret,
          limit: customersLimit && customersLimit > 0 ? Math.floor(customersLimit) : 0,
          setup_email_mode: customersSetupEmailMode
        })
      });

      if (!res.ok || !res.body) {
        customersErrorMsg = (await res.text()) || m.admin_import_stream_failed_default();
        customersStep = 'error';
        return;
      }

      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buffer = '';

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        const messages = buffer.split('\n\n');
        buffer = messages.pop() ?? '';
        for (const msg of messages) {
          const dataLine = msg.split('\n').find(l => l.startsWith('data: '));
          if (!dataLine) continue;
          try {
            const update: CustomersProgress = JSON.parse(dataLine.slice(6));
            customersProgress = update;
            if (update.done) { customersStep = 'done'; }
          } catch { /* ignore malformed */ }
        }
      }

      if (customersStep === 'importing') {
        customersErrorMsg = m.admin_import_stream_dropped();
        customersStep = 'error';
      }
    } catch {
      customersErrorMsg = m.admin_import_run_error();
      customersStep = 'error';
    }
  }

  function resetCustomers() {
    customersStep = 'idle';
    customersErrorMsg = '';
    customersProgress = null;
  }

  // ── Orders import (parallel state machine) ───────────────────────
  interface OrdersProgress {
    total_orders: number;
    processed_orders: number;
    imported_orders: number;
    updated_orders: number;
    imported_line_items: number;
    unlinked_line_items: number;
    skipped_orders: number;
    failed: number;
    current_order?: string;
    done: boolean;
    errors: string[];
  }

  let ordersStep = $state<Step>('idle');
  let ordersErrorMsg = $state('');
  let ordersProgress = $state<OrdersProgress | null>(null);
  let ordersLimit = $state<number | null>(null);

  const ordersPct = $derived(
    ordersProgress && ordersProgress.total_orders > 0
      ? Math.min(100, Math.round((ordersProgress.processed_orders / ordersProgress.total_orders) * 100))
      : 0
  );

  function openOrdersConfirm() {
    if (!credsConfigured) {
      notify.error(m.admin_import_no_creds_title(), m.admin_import_no_creds_body());
      setTab('settings');
      return;
    }
    ordersStep = 'confirming';
  }

  function cancelOrdersConfirm() {
    ordersStep = 'idle';
  }

  async function runOrdersImport() {
    ordersStep = 'testing';
    ordersErrorMsg = '';
    ordersProgress = null;

    try {
      const testRes = await fetch('/api/v1/admin/import/woocommerce/orders/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${data.token}` },
        body: JSON.stringify({ wc_url: wcUrl, wc_key: wcKey, wc_secret: wcSecret })
      });
      if (!testRes.ok) {
        ordersErrorMsg = (await testRes.text()) || m.admin_import_run_failed_default();
        ordersStep = 'error';
        return;
      }
    } catch {
      ordersErrorMsg = m.admin_import_run_timeout();
      ordersStep = 'error';
      return;
    }

    ordersStep = 'importing';
    try {
      const res = await fetch('/api/v1/admin/import/woocommerce/orders/stream', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${data.token}` },
        body: JSON.stringify({
          wc_url: wcUrl,
          wc_key: wcKey,
          wc_secret: wcSecret,
          limit: ordersLimit && ordersLimit > 0 ? Math.floor(ordersLimit) : 0
        })
      });

      if (!res.ok || !res.body) {
        ordersErrorMsg = (await res.text()) || m.admin_import_stream_failed_default();
        ordersStep = 'error';
        return;
      }

      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buffer = '';

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        const messages = buffer.split('\n\n');
        buffer = messages.pop() ?? '';
        for (const msg of messages) {
          const dataLine = msg.split('\n').find(l => l.startsWith('data: '));
          if (!dataLine) continue;
          try {
            const update: OrdersProgress = JSON.parse(dataLine.slice(6));
            ordersProgress = update;
            if (update.done) { ordersStep = 'done'; }
          } catch { /* ignore malformed */ }
        }
      }

      if (ordersStep === 'importing') {
        ordersErrorMsg = m.admin_import_stream_dropped();
        ordersStep = 'error';
      }
    } catch {
      ordersErrorMsg = m.admin_import_run_error();
      ordersStep = 'error';
    }
  }

  function resetOrders() {
    ordersStep = 'idle';
    ordersErrorMsg = '';
    ordersProgress = null;
  }

  function reset() {
    step = 'idle';
    errorMsg = '';
    progress = null;
  }

  const pct = $derived(
    progress && progress.total_products > 0
      ? Math.min(100, Math.round((progress.processed_products / progress.total_products) * 100))
      : 0
  );
</script>

<svelte:head><title>{m.admin_import_title()}</title></svelte:head>

<!-- Confirm modal (mounted at root so it overlays regardless of active tab) -->
{#if step === 'confirming'}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4"
       role="dialog" aria-modal="true">
    <div class="bg-white rounded-2xl shadow-xl w-full max-w-sm p-6">
      <h2 class="text-base font-semibold text-gray-900 mb-3">{m.admin_import_confirm_title()}</h2>
      <p class="text-sm text-gray-600 leading-relaxed mb-6">
        {#if mode === 'upsert'}
          {limit && limit > 0
            ? m.admin_import_confirm_upsert_limited({ limit })
            : m.admin_import_confirm_upsert_full()}
        {:else}
          {limit && limit > 0
            ? m.admin_import_confirm_replace_limited({ limit })
            : m.admin_import_confirm_replace_full()}
        {/if}
      </p>
      <div class="flex gap-3 justify-end">
        <button onclick={cancelConfirm}
                class="px-4 py-2 text-sm text-gray-600 border border-gray-200 rounded-xl
                       hover:bg-gray-50 transition-colors">
          {m.admin_import_confirm_cancel()}
        </button>
        <button onclick={runImport}
                class="px-4 py-2 text-sm font-medium text-white bg-gray-900 rounded-xl
                       hover:bg-gray-700 transition-colors">
          {m.admin_import_confirm_proceed()}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Orders confirm modal -->
{#if ordersStep === 'confirming'}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4"
       role="dialog" aria-modal="true">
    <div class="bg-white rounded-2xl shadow-xl w-full max-w-sm p-6">
      <h2 class="text-base font-semibold text-gray-900 mb-3">{m.admin_import_orders_confirm_title()}</h2>
      <p class="text-sm text-gray-600 leading-relaxed mb-6">
        {ordersLimit && ordersLimit > 0
          ? m.admin_import_orders_confirm_limited({ limit: ordersLimit })
          : m.admin_import_orders_confirm_full()}
      </p>
      <div class="flex gap-3 justify-end">
        <button onclick={cancelOrdersConfirm}
                class="px-4 py-2 text-sm text-gray-600 border border-gray-200 rounded-xl
                       hover:bg-gray-50 transition-colors">
          {m.admin_import_confirm_cancel()}
        </button>
        <button onclick={runOrdersImport}
                class="px-4 py-2 text-sm font-medium text-white bg-gray-900 rounded-xl
                       hover:bg-gray-700 transition-colors">
          {m.admin_import_confirm_proceed()}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Customers confirm modal -->
{#if customersStep === 'confirming'}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4"
       role="dialog" aria-modal="true">
    <div class="bg-white rounded-2xl shadow-xl w-full max-w-sm p-6">
      <h2 class="text-base font-semibold text-gray-900 mb-3">{m.admin_import_customers_confirm_title()}</h2>
      <p class="text-sm text-gray-600 leading-relaxed mb-6">
        {customersLimit && customersLimit > 0
          ? m.admin_import_customers_confirm_limited({ limit: customersLimit })
          : m.admin_import_customers_confirm_full()}
      </p>
      <div class="flex gap-3 justify-end">
        <button onclick={cancelCustomersConfirm}
                class="px-4 py-2 text-sm text-gray-600 border border-gray-200 rounded-xl
                       hover:bg-gray-50 transition-colors">
          {m.admin_import_confirm_cancel()}
        </button>
        <button onclick={runCustomersImport}
                class="px-4 py-2 text-sm font-medium text-white bg-gray-900 rounded-xl
                       hover:bg-gray-700 transition-colors">
          {m.admin_import_confirm_proceed()}
        </button>
      </div>
    </div>
  </div>
{/if}

<div class="max-w-3xl">
  <div class="mb-8">
    <h1 class="text-2xl font-bold text-gray-900">{m.admin_import_heading()}</h1>
    <p class="text-sm text-gray-500 mt-1">{m.admin_import_subtitle()}</p>
  </div>

  <!-- Tab nav -->
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

  <!-- ──────────── Tab 1: Import Products ──────────── -->
  <div class="tab-panel" class:active={activeTab === 'products'}>
    {#if step === 'error'}
      <div class="bg-red-50 border border-red-100 text-red-600 text-sm rounded-xl px-4 py-3 mb-6">
        {errorMsg}
      </div>
    {/if}

    {#if step === 'testing' || step === 'importing' || step === 'done'}
      <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-6">

        <div class="flex flex-col gap-3 mb-6">
          <div class="flex items-center gap-3">
            {#if step === 'testing'}
              <span class="w-4 h-4 rounded-full border-2 border-gray-900 border-t-transparent animate-spin shrink-0"></span>
            {:else}
              <span class="flex items-center justify-center w-4 h-4 rounded-full bg-green-500 shrink-0">
                <svg class="w-2.5 h-2.5 text-white" viewBox="0 0 20 20" fill="currentColor">
                  <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"/>
                </svg>
              </span>
            {/if}
            <span class="text-sm {step === 'testing' ? 'font-medium text-gray-900' : 'text-gray-400'}">
              {m.admin_import_test_step_label()}
            </span>
          </div>

          <div class="flex items-center gap-3">
            {#if step === 'importing'}
              <span class="w-4 h-4 rounded-full border-2 border-gray-900 border-t-transparent animate-spin shrink-0"></span>
            {:else if step === 'done'}
              <span class="flex items-center justify-center w-4 h-4 rounded-full bg-green-500 shrink-0">
                <svg class="w-2.5 h-2.5 text-white" viewBox="0 0 20 20" fill="currentColor">
                  <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"/>
                </svg>
              </span>
            {:else}
              <span class="w-4 h-4 rounded-full border border-gray-200 bg-gray-50 shrink-0"></span>
            {/if}
            <span class="text-sm {step === 'importing' ? 'font-medium text-gray-900' : step === 'done' ? 'text-gray-400' : 'text-gray-300'}">
              {m.admin_import_import_step_label()}
            </span>
          </div>
        </div>

        {#if (step === 'importing' || step === 'done') && progress}
          <div class="mb-4">
            <div class="flex items-baseline justify-between mb-2">
              <span class="text-sm font-medium text-gray-900">
                {progress.total_products > 0
                  ? m.admin_import_progress_count({ processed: progress.processed_products, total: progress.total_products })
                  : m.admin_import_progress_count_loading({ processed: progress.processed_products })}
              </span>
              <span class="text-xs text-gray-400">
                {m.admin_import_progress_variants({ count: progress.imported_variants })}
              </span>
            </div>

            <div class="w-full bg-gray-100 rounded-full h-2 overflow-hidden">
              <div
                class="h-2 rounded-full transition-all duration-300
                       {step === 'done' ? 'bg-green-500' : 'bg-gray-900'}"
                style="width: {pct}%"
              ></div>
            </div>

            <div class="flex items-center justify-between mt-2">
              <div class="flex flex-wrap gap-x-3 gap-y-1 text-xs">
                <span class="text-green-600">{m.admin_import_progress_added({ count: progress.imported_products })}</span>
                <span class="text-blue-600">{m.admin_import_progress_updated({ count: progress.updated_products })}</span>
                {#if progress.stale_deleted > 0}
                  <span class="text-gray-500">{m.admin_import_progress_deleted({ count: progress.stale_deleted })}</span>
                {/if}
                {#if progress.failed > 0}
                  <span class="text-red-500">{m.admin_import_progress_failed({ count: progress.failed })}</span>
                {/if}
              </div>
              <span class="text-xs text-gray-400">{pct}%</span>
            </div>

            {#if progress.current_product}
              <p class="text-xs text-gray-400 mt-2 truncate">
                {m.admin_import_progress_current({ name: progress.current_product })}
              </p>
            {/if}
          </div>
        {/if}

        {#if step === 'done' && progress}
          <div class="pt-4 border-t border-gray-100">
            <p class="text-sm font-medium text-gray-700 mb-1">{m.admin_import_done_heading()}</p>
            {#if progress.errors?.length > 0}
              <details class="mt-2">
                <summary class="text-xs font-semibold text-gray-500 cursor-pointer select-none">
                  {progress.errors.length === 1
                    ? m.admin_import_done_errors_one({ count: progress.errors.length })
                    : m.admin_import_done_errors_many({ count: progress.errors.length })}
                </summary>
                <ul class="mt-2 max-h-40 overflow-y-auto flex flex-col gap-1">
                  {#each progress.errors as err}
                    <li class="text-xs text-red-500 bg-red-50 rounded-lg px-3 py-1.5">{err}</li>
                  {/each}
                </ul>
              </details>
            {/if}
            <button onclick={reset}
                    class="mt-3 text-xs text-gray-400 underline hover:text-gray-600 transition-colors">
              {m.admin_import_done_reimport()}
            </button>
          </div>
        {/if}
      </div>
    {/if}

    {#if step === 'idle' || step === 'error'}
      <div class="bg-white rounded-2xl border border-gray-100 p-6">
        {#if credsConfigured}
          <div class="flex items-center justify-between gap-3 mb-5 px-3 py-2 bg-gray-50 rounded-xl">
            <div class="text-xs text-gray-600 truncate">
              <span class="text-gray-400">{m.admin_import_creds_pill_label()}</span>
              <span class="font-medium text-gray-900">{wcUrl}</span>
            </div>
            <button type="button" onclick={() => setTab('settings')}
                    class="text-xs text-gray-500 hover:text-gray-900 underline whitespace-nowrap">
              {m.admin_import_creds_pill_edit()}
            </button>
          </div>
        {:else}
          <div class="flex items-center justify-between gap-3 mb-5 px-3 py-2 bg-amber-50 border border-amber-100 rounded-xl">
            <p class="text-xs text-amber-800">{m.admin_import_no_creds_pill()}</p>
            <button type="button" onclick={() => setTab('settings')}
                    class="text-xs font-medium text-amber-900 hover:underline whitespace-nowrap">
              {m.admin_import_no_creds_pill_link()}
            </button>
          </div>
        {/if}

        <div class="flex flex-col gap-5">
          <div class="flex flex-col gap-1.5">
            <label for="wc_product_type" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
              {m.admin_import_label_product_type()}
            </label>
            <p class="text-xs text-gray-400 -mt-0.5">{m.admin_import_product_type_hint()}</p>
            <select id="wc_product_type" bind:value={productType}
                    class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm bg-white
                           focus:outline-none focus:ring-2 focus:ring-gray-900">
              <option value="products">{m.admin_import_product_type_products()}</option>
              <option value="bundle_products">{m.admin_import_product_type_bundle_products()}</option>
            </select>
            {#if productType === 'bundle_products'}
              <p class="text-xs text-amber-600 mt-1">{m.admin_import_product_type_bundle_warning()}</p>
            {/if}
          </div>

          <div class="flex flex-col gap-1.5">
            <label for="wc_limit" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
              {m.admin_import_label_limit()}
            </label>
            <p class="text-xs text-gray-400 -mt-0.5">{m.admin_import_limit_hint()}</p>
            <input id="wc_limit" type="number" min="0" step="1" placeholder={m.admin_import_limit_placeholder()}
                   bind:value={limit}
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
            {#if limit && limit > 0}
              <p class="text-xs text-amber-600 mt-1">{m.admin_import_limit_warning()}</p>
            {/if}
          </div>

          <div class="flex flex-col gap-1.5 pt-2 border-t border-gray-100">
            <span class="text-xs font-semibold text-gray-500 uppercase tracking-wide mt-3">
              {m.admin_import_section_mode()}
            </span>
            <label class="flex items-start gap-3 p-3 rounded-xl border cursor-pointer
                          {mode === 'upsert' ? 'border-gray-900 bg-gray-50' : 'border-gray-200'}">
              <input type="radio" name="mode" value="upsert" bind:group={mode}
                     class="mt-1 accent-gray-900" />
              <span class="flex-1">
                <span class="block text-sm font-medium text-gray-900">{m.admin_import_mode_upsert_title()}</span>
                <span class="block text-xs text-gray-500 mt-0.5">
                  {m.admin_import_mode_upsert_desc()}
                </span>
              </span>
            </label>
            <label class="flex items-start gap-3 p-3 rounded-xl border cursor-pointer
                          {mode === 'replace' ? 'border-gray-900 bg-gray-50' : 'border-gray-200'}">
              <input type="radio" name="mode" value="replace" bind:group={mode}
                     class="mt-1 accent-gray-900" />
              <span class="flex-1">
                <span class="block text-sm font-medium text-gray-900">{m.admin_import_mode_replace_title()}</span>
                <span class="block text-xs text-gray-500 mt-0.5">
                  {m.admin_import_mode_replace_desc()}
                </span>
              </span>
            </label>
          </div>
        </div>

        <div class="mt-6 pt-5 border-t border-gray-100">
          <button type="button" onclick={openConfirm}
                  class="px-5 py-2.5 bg-gray-900 text-white text-sm font-medium rounded-xl
                         hover:bg-gray-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed">
            {m.admin_import_run_button()}
          </button>
          <p class="text-xs text-gray-400 mt-3">
            {m.admin_import_run_hint()}
          </p>
        </div>
      </div>
    {/if}
  </div>

  <!-- ──────────── Tab 2: Import Orders ──────────── -->
  <div class="tab-panel" class:active={activeTab === 'orders'}>
    {#if ordersStep === 'error'}
      <div class="bg-red-50 border border-red-100 text-red-600 text-sm rounded-xl px-4 py-3 mb-6">
        {ordersErrorMsg}
      </div>
    {/if}

    {#if ordersStep === 'testing' || ordersStep === 'importing' || ordersStep === 'done'}
      <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-6">

        <div class="flex flex-col gap-3 mb-6">
          <div class="flex items-center gap-3">
            {#if ordersStep === 'testing'}
              <span class="w-4 h-4 rounded-full border-2 border-gray-900 border-t-transparent animate-spin shrink-0"></span>
            {:else}
              <span class="flex items-center justify-center w-4 h-4 rounded-full bg-green-500 shrink-0">
                <svg class="w-2.5 h-2.5 text-white" viewBox="0 0 20 20" fill="currentColor">
                  <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"/>
                </svg>
              </span>
            {/if}
            <span class="text-sm {ordersStep === 'testing' ? 'font-medium text-gray-900' : 'text-gray-400'}">
              {m.admin_import_test_step_label()}
            </span>
          </div>

          <div class="flex items-center gap-3">
            {#if ordersStep === 'importing'}
              <span class="w-4 h-4 rounded-full border-2 border-gray-900 border-t-transparent animate-spin shrink-0"></span>
            {:else if ordersStep === 'done'}
              <span class="flex items-center justify-center w-4 h-4 rounded-full bg-green-500 shrink-0">
                <svg class="w-2.5 h-2.5 text-white" viewBox="0 0 20 20" fill="currentColor">
                  <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"/>
                </svg>
              </span>
            {:else}
              <span class="w-4 h-4 rounded-full border border-gray-200 bg-gray-50 shrink-0"></span>
            {/if}
            <span class="text-sm {ordersStep === 'importing' ? 'font-medium text-gray-900' : ordersStep === 'done' ? 'text-gray-400' : 'text-gray-300'}">
              {m.admin_import_orders_step_label()}
            </span>
          </div>
        </div>

        {#if (ordersStep === 'importing' || ordersStep === 'done') && ordersProgress}
          <div class="mb-4">
            <div class="flex items-baseline justify-between mb-2">
              <span class="text-sm font-medium text-gray-900">
                {ordersProgress.total_orders > 0
                  ? m.admin_import_orders_progress_count({ processed: ordersProgress.processed_orders, total: ordersProgress.total_orders })
                  : m.admin_import_orders_progress_count_loading({ processed: ordersProgress.processed_orders })}
              </span>
              <span class="text-xs text-gray-400">
                {m.admin_import_orders_progress_line_items({ count: ordersProgress.imported_line_items })}
              </span>
            </div>

            <div class="w-full bg-gray-100 rounded-full h-2 overflow-hidden">
              <div
                class="h-2 rounded-full transition-all duration-300
                       {ordersStep === 'done' ? 'bg-green-500' : 'bg-gray-900'}"
                style="width: {ordersPct}%"
              ></div>
            </div>

            <div class="flex items-center justify-between mt-2">
              <div class="flex flex-wrap gap-x-3 gap-y-1 text-xs">
                <span class="text-green-600">{m.admin_import_progress_added({ count: ordersProgress.imported_orders })}</span>
                <span class="text-blue-600">{m.admin_import_progress_updated({ count: ordersProgress.updated_orders })}</span>
                {#if ordersProgress.skipped_orders > 0}
                  <span class="text-gray-500">{m.admin_import_orders_progress_skipped({ count: ordersProgress.skipped_orders })}</span>
                {/if}
                {#if ordersProgress.unlinked_line_items > 0}
                  <span class="text-amber-600">{m.admin_import_orders_progress_unlinked({ count: ordersProgress.unlinked_line_items })}</span>
                {/if}
                {#if ordersProgress.failed > 0}
                  <span class="text-red-500">{m.admin_import_progress_failed({ count: ordersProgress.failed })}</span>
                {/if}
              </div>
              <span class="text-xs text-gray-400">{ordersPct}%</span>
            </div>

            {#if ordersProgress.current_order}
              <p class="text-xs text-gray-400 mt-2 truncate">
                {m.admin_import_orders_progress_current({ name: ordersProgress.current_order })}
              </p>
            {/if}
          </div>
        {/if}

        {#if ordersStep === 'done' && ordersProgress}
          <div class="pt-4 border-t border-gray-100">
            <p class="text-sm font-medium text-gray-700 mb-1">{m.admin_import_done_heading()}</p>
            {#if ordersProgress.errors?.length > 0}
              <details class="mt-2">
                <summary class="text-xs font-semibold text-gray-500 cursor-pointer select-none">
                  {ordersProgress.errors.length === 1
                    ? m.admin_import_done_errors_one({ count: ordersProgress.errors.length })
                    : m.admin_import_done_errors_many({ count: ordersProgress.errors.length })}
                </summary>
                <ul class="mt-2 max-h-40 overflow-y-auto flex flex-col gap-1">
                  {#each ordersProgress.errors as err}
                    <li class="text-xs text-red-500 bg-red-50 rounded-lg px-3 py-1.5">{err}</li>
                  {/each}
                </ul>
              </details>
            {/if}
            <button onclick={resetOrders}
                    class="mt-3 text-xs text-gray-400 underline hover:text-gray-600 transition-colors">
              {m.admin_import_done_reimport()}
            </button>
          </div>
        {/if}
      </div>
    {/if}

    {#if ordersStep === 'idle' || ordersStep === 'error'}
      <div class="bg-white rounded-2xl border border-gray-100 p-6">
        {#if credsConfigured}
          <div class="flex items-center justify-between gap-3 mb-5 px-3 py-2 bg-gray-50 rounded-xl">
            <div class="text-xs text-gray-600 truncate">
              <span class="text-gray-400">{m.admin_import_creds_pill_label()}</span>
              <span class="font-medium text-gray-900">{wcUrl}</span>
            </div>
            <button type="button" onclick={() => setTab('settings')}
                    class="text-xs text-gray-500 hover:text-gray-900 underline whitespace-nowrap">
              {m.admin_import_creds_pill_edit()}
            </button>
          </div>
        {:else}
          <div class="flex items-center justify-between gap-3 mb-5 px-3 py-2 bg-amber-50 border border-amber-100 rounded-xl">
            <p class="text-xs text-amber-800">{m.admin_import_no_creds_pill()}</p>
            <button type="button" onclick={() => setTab('settings')}
                    class="text-xs font-medium text-amber-900 hover:underline whitespace-nowrap">
              {m.admin_import_no_creds_pill_link()}
            </button>
          </div>
        {/if}

        <p class="text-xs text-gray-500 mb-5">{m.admin_import_orders_intro()}</p>

        <div class="flex flex-col gap-5">
          <div class="flex flex-col gap-1.5">
            <label for="wc_orders_limit" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
              {m.admin_import_label_limit()}
            </label>
            <p class="text-xs text-gray-400 -mt-0.5">{m.admin_import_limit_hint()}</p>
            <input id="wc_orders_limit" type="number" min="0" step="1" placeholder={m.admin_import_limit_placeholder()}
                   bind:value={ordersLimit}
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
            {#if ordersLimit && ordersLimit > 0}
              <p class="text-xs text-amber-600 mt-1">{m.admin_import_limit_warning()}</p>
            {/if}
          </div>
        </div>

        <div class="mt-6 pt-5 border-t border-gray-100">
          <button type="button" onclick={openOrdersConfirm}
                  class="px-5 py-2.5 bg-gray-900 text-white text-sm font-medium rounded-xl
                         hover:bg-gray-700 transition-colors">
            {m.admin_import_orders_run_button()}
          </button>
        </div>
      </div>
    {/if}
  </div>

  <!-- ──────────── Tab 3: Import Customers ──────────── -->
  <div class="tab-panel" class:active={activeTab === 'customers'}>
    {#if customersStep === 'error'}
      <div class="bg-red-50 border border-red-100 text-red-600 text-sm rounded-xl px-4 py-3 mb-6">
        {customersErrorMsg}
      </div>
    {/if}

    {#if customersStep === 'testing' || customersStep === 'importing' || customersStep === 'done'}
      <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-6">

        <div class="flex flex-col gap-3 mb-6">
          <div class="flex items-center gap-3">
            {#if customersStep === 'testing'}
              <span class="w-4 h-4 rounded-full border-2 border-gray-900 border-t-transparent animate-spin shrink-0"></span>
            {:else}
              <span class="flex items-center justify-center w-4 h-4 rounded-full bg-green-500 shrink-0">
                <svg class="w-2.5 h-2.5 text-white" viewBox="0 0 20 20" fill="currentColor">
                  <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"/>
                </svg>
              </span>
            {/if}
            <span class="text-sm {customersStep === 'testing' ? 'font-medium text-gray-900' : 'text-gray-400'}">
              {m.admin_import_test_step_label()}
            </span>
          </div>

          <div class="flex items-center gap-3">
            {#if customersStep === 'importing'}
              <span class="w-4 h-4 rounded-full border-2 border-gray-900 border-t-transparent animate-spin shrink-0"></span>
            {:else if customersStep === 'done'}
              <span class="flex items-center justify-center w-4 h-4 rounded-full bg-green-500 shrink-0">
                <svg class="w-2.5 h-2.5 text-white" viewBox="0 0 20 20" fill="currentColor">
                  <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"/>
                </svg>
              </span>
            {:else}
              <span class="w-4 h-4 rounded-full border border-gray-200 bg-gray-50 shrink-0"></span>
            {/if}
            <span class="text-sm {customersStep === 'importing' ? 'font-medium text-gray-900' : customersStep === 'done' ? 'text-gray-400' : 'text-gray-300'}">
              {m.admin_import_customers_step_label()}
            </span>
          </div>
        </div>

        {#if (customersStep === 'importing' || customersStep === 'done') && customersProgress}
          <div class="mb-4">
            <div class="flex items-baseline justify-between mb-2">
              <span class="text-sm font-medium text-gray-900">
                {customersProgress.total_customers > 0
                  ? m.admin_import_customers_progress_count({ processed: customersProgress.processed_customers, total: customersProgress.total_customers })
                  : m.admin_import_customers_progress_count_loading({ processed: customersProgress.processed_customers })}
              </span>
              <span class="text-xs text-gray-400">
                {m.admin_import_customers_progress_addresses({ count: customersProgress.imported_addresses })}
              </span>
            </div>

            <div class="w-full bg-gray-100 rounded-full h-2 overflow-hidden">
              <div
                class="h-2 rounded-full transition-all duration-300
                       {customersStep === 'done' ? 'bg-green-500' : 'bg-gray-900'}"
                style="width: {customersPct}%"
              ></div>
            </div>

            <div class="flex items-center justify-between mt-2">
              <div class="flex flex-wrap gap-x-3 gap-y-1 text-xs">
                <span class="text-green-600">{m.admin_import_progress_added({ count: customersProgress.imported_customers })}</span>
                <span class="text-blue-600">{m.admin_import_progress_updated({ count: customersProgress.updated_customers })}</span>
                {#if customersProgress.setup_emails_queued > 0}
                  <span class="text-purple-600">{m.admin_import_customers_progress_emails({ count: customersProgress.setup_emails_queued })}</span>
                {/if}
                {#if customersProgress.failed > 0}
                  <span class="text-red-500">{m.admin_import_progress_failed({ count: customersProgress.failed })}</span>
                {/if}
              </div>
              <span class="text-xs text-gray-400">{customersPct}%</span>
            </div>

            {#if customersProgress.current_customer}
              <p class="text-xs text-gray-400 mt-2 truncate">
                {m.admin_import_customers_progress_current({ name: customersProgress.current_customer })}
              </p>
            {/if}
          </div>
        {/if}

        {#if customersStep === 'done' && customersProgress}
          <div class="pt-4 border-t border-gray-100">
            <p class="text-sm font-medium text-gray-700 mb-1">{m.admin_import_done_heading()}</p>
            {#if customersProgress.errors?.length > 0}
              <details class="mt-2">
                <summary class="text-xs font-semibold text-gray-500 cursor-pointer select-none">
                  {customersProgress.errors.length === 1
                    ? m.admin_import_done_errors_one({ count: customersProgress.errors.length })
                    : m.admin_import_done_errors_many({ count: customersProgress.errors.length })}
                </summary>
                <ul class="mt-2 max-h-40 overflow-y-auto flex flex-col gap-1">
                  {#each customersProgress.errors as err}
                    <li class="text-xs text-red-500 bg-red-50 rounded-lg px-3 py-1.5">{err}</li>
                  {/each}
                </ul>
              </details>
            {/if}
            <button onclick={resetCustomers}
                    class="mt-3 text-xs text-gray-400 underline hover:text-gray-600 transition-colors">
              {m.admin_import_done_reimport()}
            </button>
          </div>
        {/if}
      </div>
    {/if}

    {#if customersStep === 'idle' || customersStep === 'error'}
      <div class="bg-white rounded-2xl border border-gray-100 p-6">
        {#if credsConfigured}
          <div class="flex items-center justify-between gap-3 mb-5 px-3 py-2 bg-gray-50 rounded-xl">
            <div class="text-xs text-gray-600 truncate">
              <span class="text-gray-400">{m.admin_import_creds_pill_label()}</span>
              <span class="font-medium text-gray-900">{wcUrl}</span>
            </div>
            <button type="button" onclick={() => setTab('settings')}
                    class="text-xs text-gray-500 hover:text-gray-900 underline whitespace-nowrap">
              {m.admin_import_creds_pill_edit()}
            </button>
          </div>
        {:else}
          <div class="flex items-center justify-between gap-3 mb-5 px-3 py-2 bg-amber-50 border border-amber-100 rounded-xl">
            <p class="text-xs text-amber-800">{m.admin_import_no_creds_pill()}</p>
            <button type="button" onclick={() => setTab('settings')}
                    class="text-xs font-medium text-amber-900 hover:underline whitespace-nowrap">
              {m.admin_import_no_creds_pill_link()}
            </button>
          </div>
        {/if}

        <p class="text-xs text-gray-500 mb-5">{m.admin_import_customers_intro()}</p>

        <div class="flex flex-col gap-5">
          <div class="flex flex-col gap-1.5">
            <label for="wc_customers_limit" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
              {m.admin_import_label_limit()}
            </label>
            <p class="text-xs text-gray-400 -mt-0.5">{m.admin_import_limit_hint()}</p>
            <input id="wc_customers_limit" type="number" min="0" step="1" placeholder={m.admin_import_limit_placeholder()}
                   bind:value={customersLimit}
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
            {#if customersLimit && customersLimit > 0}
              <p class="text-xs text-amber-600 mt-1">{m.admin_import_limit_warning()}</p>
            {/if}
          </div>

          <div class="flex flex-col gap-1.5">
            <span class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
              {m.admin_import_customers_setup_email_heading()}
            </span>
            <p class="text-xs text-gray-400 -mt-0.5">{m.admin_import_customers_setup_email_intro()}</p>
            <div class="flex flex-col gap-2 mt-1">
              {#each [
                { id: 'skip', label: m.admin_import_customers_setup_email_skip_label(), hint: m.admin_import_customers_setup_email_skip_hint() },
                { id: 'passwordless', label: m.admin_import_customers_setup_email_passwordless_label(), hint: m.admin_import_customers_setup_email_passwordless_hint() },
                { id: 'force', label: m.admin_import_customers_setup_email_force_label(), hint: m.admin_import_customers_setup_email_force_hint() }
              ] as opt}
                <label class="flex items-start gap-3 p-3 rounded-xl border cursor-pointer
                              {customersSetupEmailMode === opt.id ? 'border-gray-900 bg-gray-50' : 'border-gray-200'}">
                  <input type="radio" name="customers_setup_email_mode" value={opt.id}
                         bind:group={customersSetupEmailMode}
                         class="mt-1 accent-gray-900" />
                  <span class="flex-1">
                    <span class="block text-sm font-medium text-gray-900">{opt.label}</span>
                    <span class="block text-xs text-gray-500 mt-0.5">{opt.hint}</span>
                  </span>
                </label>
              {/each}
            </div>
          </div>
        </div>

        <div class="mt-6 pt-5 border-t border-gray-100">
          <button type="button" onclick={openCustomersConfirm}
                  class="px-5 py-2.5 bg-gray-900 text-white text-sm font-medium rounded-xl
                         hover:bg-gray-700 transition-colors">
            {m.admin_import_customers_run_button()}
          </button>
        </div>
      </div>
    {/if}
  </div>

  <!-- ──────────── Tab 4: Import Settings ──────────── -->
  <div class="tab-panel" class:active={activeTab === 'settings'}>
    <div class="bg-white rounded-2xl border border-gray-100 p-6">
      <div class="flex flex-col gap-5">
        <div class="flex flex-col gap-1.5">
          <label for="wc_url" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
            {m.admin_import_settings_label_url()}
          </label>
          <p class="text-xs text-gray-400 -mt-0.5">{m.admin_import_settings_url_hint()}</p>
          <input id="wc_url" type="url" placeholder={m.admin_import_settings_url_placeholder()}
                 bind:value={wcUrl}
                 class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                        focus:outline-none focus:ring-2 focus:ring-gray-900" />
        </div>
        <div class="flex flex-col gap-1.5">
          <label for="wc_key" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
            {m.admin_import_settings_label_key()}
          </label>
          <p class="text-xs text-gray-400 -mt-0.5">{m.admin_import_settings_key_hint()}</p>
          <input id="wc_key" type="text" placeholder={m.admin_import_settings_key_placeholder()}
                 bind:value={wcKey}
                 class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                        focus:outline-none focus:ring-2 focus:ring-gray-900" />
        </div>
        <div class="flex flex-col gap-1.5">
          <label for="wc_secret" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
            {m.admin_import_settings_label_secret()}
          </label>
          <PasswordInput id="wc_secret" placeholder={m.admin_import_settings_secret_placeholder()}
                         bind:value={wcSecret} />
        </div>

        <div class="flex flex-wrap items-center gap-2 pt-2 border-t border-gray-100">
          <button type="button" onclick={saveCredentials} disabled={savingCreds || testingConn}
                  class="px-4 py-2 text-sm font-medium text-gray-700 border border-gray-200 rounded-xl
                         hover:bg-gray-50 transition-colors disabled:opacity-50 disabled:cursor-not-allowed">
            {savingCreds ? m.admin_import_settings_saving() : m.admin_import_settings_save_button()}
          </button>
          <button type="button" onclick={testConnection} disabled={savingCreds || testingConn}
                  class="px-4 py-2 text-sm font-medium text-gray-700 border border-gray-200 rounded-xl
                         hover:bg-gray-50 transition-colors disabled:opacity-50 disabled:cursor-not-allowed">
            {testingConn ? m.admin_import_settings_testing() : m.admin_import_settings_test_button()}
          </button>
          <span class="text-xs text-gray-400">{m.admin_import_settings_test_disclaimer()}</span>
        </div>
      </div>
    </div>
  </div>
</div>

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
