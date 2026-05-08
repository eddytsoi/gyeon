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
    settings:
      'M10.5 6h9.75M10.5 6a1.5 1.5 0 1 1-3 0m3 0a1.5 1.5 0 1 0-3 0M3.75 6H7.5m3 12h9.75m-9.75 0a1.5 1.5 0 0 1-3 0m3 0a1.5 1.5 0 0 0-3 0m-3.75 0H7.5m9-6h3.75m-3.75 0a1.5 1.5 0 0 1-3 0m3 0a1.5 1.5 0 0 0-3 0m-9.75 0h9.75'
  };

  const TABS = $derived([
    { id: 'products', label: m.admin_import_tab_products() },
    { id: 'settings', label: m.admin_import_tab_settings() }
  ] as const);
  type TabId = 'products' | 'settings';

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

  <!-- ──────────── Tab 2: Import Settings ──────────── -->
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
