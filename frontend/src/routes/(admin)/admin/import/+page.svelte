<script lang="ts">
  import { onMount } from 'svelte';
  import type { PageData } from './$types';
  import { notify } from '$lib/stores/notifications.svelte';

  let { data }: { data: PageData } = $props();

  type Step = 'idle' | 'confirming' | 'testing' | 'importing' | 'done' | 'error';
  type Mode = 'upsert' | 'replace';

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
  // Heroicons (stroke 1.5) — same SVG-path approach as Site Settings
  // so the two pages share visual language without pulling in lucide.
  const TAB_ICONS: Record<string, string> = {
    // arrow-down-tray (download icon — represents "import data into Gyeon")
    products:
      'M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5M16.5 12 12 16.5m0 0L7.5 12m4.5 4.5V3',
    // adjustments-horizontal (sliders — represents "configuration / settings")
    settings:
      'M10.5 6h9.75M10.5 6a1.5 1.5 0 1 1-3 0m3 0a1.5 1.5 0 1 0-3 0M3.75 6H7.5m3 12h9.75m-9.75 0a1.5 1.5 0 0 1-3 0m3 0a1.5 1.5 0 0 0-3 0m-3.75 0H7.5m9-6h3.75m-3.75 0a1.5 1.5 0 0 1-3 0m3 0a1.5 1.5 0 0 0-3 0m-9.75 0h9.75'
  };

  const TABS = [
    { id: 'products', label: 'Import Products' },
    { id: 'settings', label: 'Import Settings' }
  ] as const;
  type TabId = (typeof TABS)[number]['id'];

  let activeTab = $state<TabId>('products');

  onMount(() => {
    const fromHash = window.location.hash.slice(1) as TabId;
    if (TABS.some((t) => t.id === fromHash)) activeTab = fromHash;
  });

  function setTab(id: TabId) {
    activeTab = id;
    history.replaceState(null, '', `#${id}`);
  }

  // Tab magnetic spotlight (copied from settings page so the hover
  // animation feels identical between the two admin pages).
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
        notify.error('儲存失敗', (await res.text()) || '請稍後再試。');
      } else {
        notify.success('已儲存憑證');
      }
    } catch (e) {
      notify.error('儲存失敗', e instanceof Error ? e.message : '請稍後再試。');
    } finally {
      savingCreds = false;
    }
  }

  async function testConnection() {
    if (!credsConfigured) {
      notify.error('Connection failed', '請先填寫 URL、Consumer Key 和 Consumer Secret。');
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
        const msg = (await res.text()) || '請檢查 URL 與 API 金鑰。';
        notify.error('Connection failed', msg);
        return;
      }
      const j = await res.json();
      const total = typeof j.total_products === 'number' ? j.total_products : null;
      notify.success(
        'Connection successful',
        total !== null ? `WooCommerce 共有 ${total} 個商品。` : undefined
      );
    } catch (e) {
      notify.error('Connection failed', e instanceof Error ? e.message : '連線逾時或網路錯誤。');
    } finally {
      testingConn = false;
    }
  }

  // Prefill saved credentials on mount; tracking-free $effect runs once
  // because nothing reactive is read inside loadCredentials.
  $effect(() => {
    loadCredentials();
  });

  function openConfirm() {
    if (!credsConfigured) {
      notify.error('未設定憑證', '請先到 Import Settings 設定 WooCommerce 憑證。');
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

    // Step 1: test connection
    try {
      const testRes = await fetch('/api/v1/admin/import/woocommerce/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${data.token}` },
        body: JSON.stringify({ wc_url: wcUrl, wc_key: wcKey, wc_secret: wcSecret })
      });
      if (!testRes.ok) {
        errorMsg = (await testRes.text()) || '無法連接至 WooCommerce，請確認網址及 API 金鑰。';
        step = 'error';
        return;
      }
    } catch {
      errorMsg = '連線逾時，請確認 WooCommerce 網址是否正確。';
      step = 'error';
      return;
    }

    // Step 2: stream import
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
          limit: limit && limit > 0 ? Math.floor(limit) : 0
        })
      });

      if (!res.ok || !res.body) {
        errorMsg = (await res.text()) || '匯入失敗，請稍後再試。';
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

      // Stream ended without a final {done:true} message — treat as a dropped
      // connection so the user sees a terminal state instead of an endless spinner.
      if (step === 'importing') {
        errorMsg = '匯入過程中連線中斷，請查看伺服器記錄並重新匯入。';
        step = 'error';
      }
    } catch {
      errorMsg = '匯入過程中發生錯誤，請稍後再試。';
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

<svelte:head><title>Import Products — Gyeon Admin</title></svelte:head>

<!-- Confirm modal (mounted at root so it overlays regardless of active tab) -->
{#if step === 'confirming'}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4"
       role="dialog" aria-modal="true">
    <div class="bg-white rounded-2xl shadow-xl w-full max-w-sm p-6">
      <h2 class="text-base font-semibold text-gray-900 mb-3">確認匯入</h2>
      <p class="text-sm text-gray-600 leading-relaxed mb-6">
        {#if mode === 'upsert'}
          將以 WooCommerce 資料更新現有商品，並新增缺少的商品。{#if limit && limit > 0}本次只處理前 {limit} 個商品；WC 端已刪除的商品<strong>不會</strong>被清除。{:else}WC 端已刪除的商品會從 Gyeon 一併移除；{/if}
          管理員手動建立的商品、翻譯、圖片不受影響。確定繼續？
        {:else}
          將先刪除所有先前由 WooCommerce 匯入的商品（含翻譯、變體、圖片），再從 WooCommerce 重新匯入{#if limit && limit > 0}前 {limit} 個商品{/if}。
          管理員手動建立的商品仍然保留。此操作無法復原，確定繼續？
        {/if}
      </p>
      <div class="flex gap-3 justify-end">
        <button onclick={cancelConfirm}
                class="px-4 py-2 text-sm text-gray-600 border border-gray-200 rounded-xl
                       hover:bg-gray-50 transition-colors">
          取消
        </button>
        <button onclick={runImport}
                class="px-4 py-2 text-sm font-medium text-white bg-gray-900 rounded-xl
                       hover:bg-gray-700 transition-colors">
          確定
        </button>
      </div>
    </div>
  </div>
{/if}

<div class="max-w-3xl">
  <div class="mb-8">
    <h1 class="text-2xl font-bold text-gray-900">Import Products</h1>
    <p class="text-sm text-gray-500 mt-1">Import products from a WooCommerce store via REST API.</p>
  </div>

  <!-- Tab nav (mirrors Site Settings page styling) -->
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

  <!-- ──────────── Tab 1: Import Products ──────────── -->
  <div class="tab-panel" class:active={activeTab === 'products'}>
    <!-- Error -->
    {#if step === 'error'}
      <div class="bg-red-50 border border-red-100 text-red-600 text-sm rounded-xl px-4 py-3 mb-6">
        {errorMsg}
      </div>
    {/if}

    <!-- Progress panel (visible during testing/importing/done) -->
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
              驗證 WooCommerce 連線
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
              匯入商品
            </span>
          </div>
        </div>

        {#if (step === 'importing' || step === 'done') && progress}
          <div class="mb-4">
            <div class="flex items-baseline justify-between mb-2">
              <span class="text-sm font-medium text-gray-900">
                {progress.processed_products} / {progress.total_products > 0 ? progress.total_products : '…'} 個商品
              </span>
              <span class="text-xs text-gray-400">
                {progress.imported_variants} 個變體
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
                <span class="text-green-600">+ {progress.imported_products} 新增</span>
                <span class="text-blue-600">↻ {progress.updated_products} 更新</span>
                {#if progress.stale_deleted > 0}
                  <span class="text-gray-500">− {progress.stale_deleted} 刪除（WC 已移除）</span>
                {/if}
                {#if progress.failed > 0}
                  <span class="text-red-500">✕ {progress.failed} 失敗</span>
                {/if}
              </div>
              <span class="text-xs text-gray-400">{pct}%</span>
            </div>

            {#if progress.current_product}
              <p class="text-xs text-gray-400 mt-2 truncate">
                正在處理：{progress.current_product}
              </p>
            {/if}
          </div>
        {/if}

        {#if step === 'done' && progress}
          <div class="pt-4 border-t border-gray-100">
            <p class="text-sm font-medium text-gray-700 mb-1">匯入完成</p>
            {#if progress.errors?.length > 0}
              <details class="mt-2">
                <summary class="text-xs font-semibold text-gray-500 cursor-pointer select-none">
                  {progress.errors.length} 個錯誤
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
              重新匯入
            </button>
          </div>
        {/if}
      </div>
    {/if}

    <!-- Run-import form (idle / error states) -->
    {#if step === 'idle' || step === 'error'}
      <div class="bg-white rounded-2xl border border-gray-100 p-6">
        <!-- Credential status pill -->
        {#if credsConfigured}
          <div class="flex items-center justify-between gap-3 mb-5 px-3 py-2 bg-gray-50 rounded-xl">
            <div class="text-xs text-gray-600 truncate">
              <span class="text-gray-400">使用憑證：</span>
              <span class="font-medium text-gray-900">{wcUrl}</span>
            </div>
            <button type="button" onclick={() => setTab('settings')}
                    class="text-xs text-gray-500 hover:text-gray-900 underline whitespace-nowrap">
              於 Settings 修改
            </button>
          </div>
        {:else}
          <div class="flex items-center justify-between gap-3 mb-5 px-3 py-2 bg-amber-50 border border-amber-100 rounded-xl">
            <p class="text-xs text-amber-800">⚠ 尚未設定 WooCommerce 憑證</p>
            <button type="button" onclick={() => setTab('settings')}
                    class="text-xs font-medium text-amber-900 hover:underline whitespace-nowrap">
              前往 Import Settings →
            </button>
          </div>
        {/if}

        <div class="flex flex-col gap-5">
          <!-- Limit input -->
          <div class="flex flex-col gap-1.5">
            <label for="wc_limit" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
              數量上限
            </label>
            <p class="text-xs text-gray-400 -mt-0.5">留空 / 0 = 匯入全部；輸入 N = 只匯入前 N 個商品（含其變體）。常用於測試。</p>
            <input id="wc_limit" type="number" min="0" step="1" placeholder="留空為全部"
                   bind:value={limit}
                   class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
            {#if limit && limit > 0}
              <p class="text-xs text-amber-600 mt-1">⚠ 限量模式下不會清除 WC 端已刪除的商品（避免誤刪未掃到的部分）。</p>
            {/if}
          </div>

          <!-- Mode selector -->
          <div class="flex flex-col gap-1.5 pt-2 border-t border-gray-100">
            <span class="text-xs font-semibold text-gray-500 uppercase tracking-wide mt-3">
              匯入模式
            </span>
            <label class="flex items-start gap-3 p-3 rounded-xl border cursor-pointer
                          {mode === 'upsert' ? 'border-gray-900 bg-gray-50' : 'border-gray-200'}">
              <input type="radio" name="mode" value="upsert" bind:group={mode}
                     class="mt-1 accent-gray-900" />
              <span class="flex-1">
                <span class="block text-sm font-medium text-gray-900">更新現有 WC 商品（建議）</span>
                <span class="block text-xs text-gray-500 mt-0.5">
                  以 WooCommerce 為來源更新庫存、價格、重量等資料；保留管理員的翻譯、手動上傳圖片與手動建立的商品。
                  WC 端已刪除的商品會一併從 Gyeon 移除。
                </span>
              </span>
            </label>
            <label class="flex items-start gap-3 p-3 rounded-xl border cursor-pointer
                          {mode === 'replace' ? 'border-gray-900 bg-gray-50' : 'border-gray-200'}">
              <input type="radio" name="mode" value="replace" bind:group={mode}
                     class="mt-1 accent-gray-900" />
              <span class="flex-1">
                <span class="block text-sm font-medium text-gray-900">重新匯入（清除舊 WC 商品後重灌）</span>
                <span class="block text-xs text-gray-500 mt-0.5">
                  先刪除所有先前由 WC 匯入的商品（含其翻譯與圖片），再重新匯入一份。管理員手動建立的商品保留。
                </span>
              </span>
            </label>
          </div>
        </div>

        <div class="mt-6 pt-5 border-t border-gray-100">
          <button type="button" onclick={openConfirm}
                  class="px-5 py-2.5 bg-gray-900 text-white text-sm font-medium rounded-xl
                         hover:bg-gray-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed">
            執行匯入
          </button>
          <p class="text-xs text-gray-400 mt-3">
            系統會先驗證連線，確認成功後才開始匯入。
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
            WooCommerce Store URL
          </label>
          <p class="text-xs text-gray-400 -mt-0.5">e.g. https://your-store.com</p>
          <input id="wc_url" type="url" placeholder="https://your-store.com"
                 bind:value={wcUrl}
                 class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                        focus:outline-none focus:ring-2 focus:ring-gray-900" />
        </div>
        <div class="flex flex-col gap-1.5">
          <label for="wc_key" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
            Consumer Key
          </label>
          <p class="text-xs text-gray-400 -mt-0.5">WooCommerce → Settings → Advanced → REST API</p>
          <input id="wc_key" type="text" placeholder="ck_..."
                 bind:value={wcKey}
                 class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                        focus:outline-none focus:ring-2 focus:ring-gray-900" />
        </div>
        <div class="flex flex-col gap-1.5">
          <label for="wc_secret" class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
            Consumer Secret
          </label>
          <input id="wc_secret" type="password" placeholder="cs_..."
                 bind:value={wcSecret}
                 class="w-full border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                        focus:outline-none focus:ring-2 focus:ring-gray-900" />
        </div>

        <div class="flex flex-wrap items-center gap-2 pt-2 border-t border-gray-100">
          <button type="button" onclick={saveCredentials} disabled={savingCreds || testingConn}
                  class="px-4 py-2 text-sm font-medium text-gray-700 border border-gray-200 rounded-xl
                         hover:bg-gray-50 transition-colors disabled:opacity-50 disabled:cursor-not-allowed">
            {savingCreds ? '儲存中…' : '儲存憑證'}
          </button>
          <button type="button" onclick={testConnection} disabled={savingCreds || testingConn}
                  class="px-4 py-2 text-sm font-medium text-gray-700 border border-gray-200 rounded-xl
                         hover:bg-gray-50 transition-colors disabled:opacity-50 disabled:cursor-not-allowed">
            {testingConn ? '測試中…' : '測試連線'}
          </button>
          <span class="text-xs text-gray-400">不會執行匯入。</span>
        </div>
      </div>
    </div>
  </div>
</div>

<style>
  /* Same panel transition as Site Settings — keeps both panels mounted
     so form fields persist across tab switches, hides inactive ones via
     display:none, and animates the active panel in. */
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
