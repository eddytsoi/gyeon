<script lang="ts">
  import type { PageData } from './$types';

  let { data }: { data: PageData } = $props();

  type Step = 'idle' | 'confirming' | 'testing' | 'importing' | 'done' | 'error';

  interface Progress {
    total_products: number;
    processed_products: number;
    imported_products: number;
    imported_variants: number;
    skipped: number;
    skipped_details: string[];
    failed: number;
    current_product?: string;
    done: boolean;
    errors: string[];
  }

  let step = $state<Step>('idle');
  let errorMsg = $state('');
  let progress = $state<Progress | null>(null);
  let showSkipped = $state(false);

  let wcUrl = $state('');
  let wcKey = $state('');
  let wcSecret = $state('');

  function openConfirm() {
    if (!wcUrl || !wcKey || !wcSecret) {
      errorMsg = '請填寫所有欄位。';
      step = 'error';
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
        body: JSON.stringify({ wc_url: wcUrl, wc_key: wcKey, wc_secret: wcSecret, clear_all: true })
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
    } catch {
      errorMsg = '匯入過程中發生錯誤，請稍後再試。';
      step = 'error';
    }
  }

  function reset() {
    step = 'idle';
    errorMsg = '';
    progress = null;
    showSkipped = false;
  }

  const pct = $derived(
    progress && progress.total_products > 0
      ? Math.min(100, Math.round((progress.processed_products / progress.total_products) * 100))
      : 0
  );
</script>

<svelte:head><title>Import Products — Gyeon Admin</title></svelte:head>

<!-- Confirm modal -->
{#if step === 'confirming'}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4"
       role="dialog" aria-modal="true">
    <div class="bg-white rounded-2xl shadow-xl w-full max-w-sm p-6">
      <h2 class="text-base font-semibold text-gray-900 mb-3">確認匯入</h2>
      <p class="text-sm text-gray-600 leading-relaxed mb-6">
        即將清除所有現有商品、變體及圖片資料，並從 WooCommerce 重新匯入。此操作無法復原，確定繼續？
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

<div class="max-w-2xl">
  <div class="mb-8">
    <h1 class="text-2xl font-bold text-gray-900">Import Products</h1>
    <p class="text-sm text-gray-500 mt-1">Import products from a WooCommerce store via REST API.</p>
  </div>

  <!-- Error -->
  {#if step === 'error'}
    <div class="bg-red-50 border border-red-100 text-red-600 text-sm rounded-xl px-4 py-3 mb-6">
      {errorMsg}
    </div>
  {/if}

  <!-- Progress panel -->
  {#if step === 'testing' || step === 'importing' || step === 'done'}
    <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-6">

      <!-- Step indicators -->
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

      <!-- Progress bar (only while importing or done) -->
      {#if (step === 'importing' || step === 'done') && progress}
        <div class="mb-4">
          <!-- Counts row -->
          <div class="flex items-baseline justify-between mb-2">
            <span class="text-sm font-medium text-gray-900">
              {progress.processed_products} / {progress.total_products > 0 ? progress.total_products : '…'} 個商品
            </span>
            <span class="text-xs text-gray-400">
              {progress.imported_variants} 個變體
            </span>
          </div>

          <!-- Bar -->
          <div class="w-full bg-gray-100 rounded-full h-2 overflow-hidden">
            <div
              class="h-2 rounded-full transition-all duration-300
                     {step === 'done' ? 'bg-green-500' : 'bg-gray-900'}"
              style="width: {pct}%"
            ></div>
          </div>

          <!-- Sub-counts + current product -->
          <div class="flex items-center justify-between mt-2">
            <div class="flex gap-3 text-xs">
              <span class="text-green-600">✓ {progress.imported_products} 已匯入</span>
              {#if progress.skipped > 0}
                <button
                  onclick={() => showSkipped = !showSkipped}
                  class="text-gray-400 hover:text-gray-600 transition-colors underline-offset-2 hover:underline">
                  ⊘ {progress.skipped} 略過
                </button>
              {:else}
                <span class="text-gray-400">⊘ 0 略過</span>
              {/if}
              {#if progress.failed > 0}
                <span class="text-red-500">✕ {progress.failed} 失敗</span>
              {/if}
            </div>
            <span class="text-xs text-gray-400">{pct}%</span>
          </div>

          {#if showSkipped && progress.skipped_details?.length > 0}
            <ul class="mt-2 max-h-32 overflow-y-auto flex flex-col gap-1">
              {#each progress.skipped_details as detail}
                <li class="text-xs text-gray-500 bg-gray-50 rounded-lg px-3 py-1.5">{detail}</li>
              {/each}
            </ul>
          {/if}

          {#if progress.current_product}
            <p class="text-xs text-gray-400 mt-2 truncate">
              正在處理：{progress.current_product}
            </p>
          {/if}
        </div>
      {/if}

      <!-- Done summary -->
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

  <!-- Credentials form -->
  {#if step === 'idle' || step === 'error'}
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
      </div>
      <div class="mt-6 pt-5 border-t border-gray-100">
        <button type="button" onclick={openConfirm}
                class="px-5 py-2.5 bg-gray-900 text-white text-sm font-medium rounded-xl
                       hover:bg-gray-700 transition-colors">
          執行匯入
        </button>
        <p class="text-xs text-gray-400 mt-3">
          系統會先驗證連線，確認成功後才開始匯入。
        </p>
      </div>
    </div>
  {/if}
</div>
