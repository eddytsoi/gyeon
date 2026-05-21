<script lang="ts">
  import { goto, invalidateAll } from '$app/navigation';
  import {
    adminUpdateStockMutation,
    adminExecuteStockMutation,
    adminDeleteStockMutation,
    adminDuplicateStockMutation,
    StockMutationInsufficientStockError,
    type StockMutation,
    type StockMutationType,
    type StockMutationConflict
  } from '$lib/api/admin';
  import { notify } from '$lib/stores/notifications.svelte';
  import ProductPicker, { type ProductPickerAddPayload } from '$lib/components/admin/ProductPicker.svelte';
  import MutationItemsTable, { type MutationItemRow } from '$lib/components/admin/MutationItemsTable.svelte';
  import type { PageData } from './$types';

  let { data }: { data: PageData } = $props();
  // token is constant for the session — captured once on mount is fine.
  const token = data.token!;

  // Snapshot the server payload at mount so editable form state has a
  // stable initial value. The "live" view (badges, metadata, executed
  // flag) reads off $derived(data.mutation) so it tracks invalidateAll().
  const initial: StockMutation = data.mutation;
  const live = $derived(data.mutation);
  const isExecuted = $derived(live.status === 'executed');

  let type = $state<StockMutationType>(initial.type);
  let note = $state(initial.note ?? '');
  let items = $state<MutationItemRow[]>(
    initial.items.map((it, i) => ({
      key: `${it.variant_id}-${i}`,
      variantId: it.variant_id,
      productName: it.product_name ?? it.variant_id,
      sku: it.variant_sku ?? '',
      variantName: it.variant_name ?? null,
      primaryImageUrl: null,
      quantity: it.quantity,
      currentStock: it.current_stock ?? null
    }))
  );

  let saving = $state(false);
  let executing = $state(false);
  let conflicts = $state<StockMutationConflict[] | null>(null);

  let nextKey = items.length;
  function addItem(payload: ProductPickerAddPayload) {
    const v = payload.variant;
    if (items.some((it) => it.variantId === v.id)) {
      notify.error('呢個 variant 已經喺單入面', '同一張 mutation 唔可以重複加入。');
      return;
    }
    nextKey += 1;
    items = [
      ...items,
      {
        key: `${v.id}-${nextKey}`,
        variantId: v.id,
        productName: payload.productName,
        sku: v.sku,
        variantName: v.name,
        primaryImageUrl: payload.primaryImageUrl ?? null,
        quantity: payload.quantity,
        currentStock: v.stock_qty ?? null
      }
    ];
  }
  function changeQty(key: string, qty: number) {
    items = items.map((it) => (it.key === key ? { ...it, quantity: qty } : it));
  }
  function removeRow(key: string) {
    items = items.filter((it) => it.key !== key);
  }

  function setType(t: StockMutationType) {
    if (isExecuted) return;
    if (items.length > 0 && t !== type) {
      if (!confirm('改變方向會清除所有 line items。繼續？')) return;
      items = [];
    }
    type = t;
  }

  async function save() {
    if (isExecuted) return;
    if (items.length === 0) {
      notify.error('未有貨品', '至少要加一個 variant 先可以儲存。');
      return;
    }
    saving = true;
    try {
      await adminUpdateStockMutation(token, initial.id, {
        type,
        note: note.trim() || null,
        items: items.map((it) => ({ variant_id: it.variantId, quantity: it.quantity }))
      });
      notify.success('已儲存');
      await invalidateAll();
    } catch (e) {
      notify.error('儲存失敗', e instanceof Error ? e.message : 'unknown error');
    } finally {
      saving = false;
    }
  }

  async function execute() {
    if (isExecuted) return;
    if (!confirm(`確定執行 ${initial.mutation_number}？執行後不能撤銷或修改。`)) return;
    // Save any pending edits first, then execute.
    executing = true;
    conflicts = null;
    try {
      if (items.length === 0) {
        notify.error('未有貨品');
        return;
      }
      await adminUpdateStockMutation(token, initial.id, {
        type,
        note: note.trim() || null,
        items: items.map((it) => ({ variant_id: it.variantId, quantity: it.quantity }))
      });
      try {
        await adminExecuteStockMutation(token, initial.id);
        notify.success(`${initial.mutation_number} 已執行`);
        await invalidateAll();
      } catch (e) {
        if (e instanceof StockMutationInsufficientStockError) {
          conflicts = e.conflicts;
          notify.error('庫存不足，未執行', '請睇下面衝突列表。');
          return;
        }
        throw e;
      }
    } catch (e) {
      notify.error('執行失敗', e instanceof Error ? e.message : 'unknown error');
    } finally {
      executing = false;
    }
  }

  async function remove() {
    if (isExecuted) return;
    if (!confirm(`刪除 ${initial.mutation_number}？此操作不能撤銷。`)) return;
    try {
      await adminDeleteStockMutation(token, initial.id);
      notify.success('已刪除');
      goto('/admin/stock-mutations');
    } catch (e) {
      notify.error('刪除失敗', e instanceof Error ? e.message : 'unknown error');
    }
  }

  async function duplicate() {
    try {
      const created = await adminDuplicateStockMutation(token, initial.id);
      notify.success(`已複製為 ${created.mutation_number}`);
      goto(`/admin/stock-mutations/${created.id}`);
    } catch (e) {
      notify.error('複製失敗', e instanceof Error ? e.message : 'unknown error');
    }
  }

  function fmt(iso: string | undefined | null) {
    if (!iso) return '—';
    try { return new Date(iso).toLocaleString(); } catch { return iso; }
  }
</script>

<svelte:head><title>{live.mutation_number} · Stock Mutation</title></svelte:head>

<div class="space-y-4 max-w-4xl">
  <div class="flex items-center gap-3 flex-wrap">
    <a href="/admin/stock-mutations" class="text-sm text-gray-500 hover:text-gray-700">← Back</a>
    <h1 class="text-2xl font-semibold text-gray-900 font-mono">{live.mutation_number}</h1>
    {#if live.type === 'in'}
      <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-emerald-50 text-emerald-700">Stock In</span>
    {:else}
      <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-red-50 text-red-700">Stock Out</span>
    {/if}
    {#if live.status === 'draft'}
      <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-amber-50 text-amber-700">Draft</span>
    {:else}
      <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-50 text-blue-700">Executed</span>
    {/if}
  </div>

  <!-- Metadata -->
  <div class="bg-white border border-gray-200 rounded-xl p-4 grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs">
    <div>
      <div class="text-gray-400 uppercase tracking-wide">Created</div>
      <div class="text-gray-900 mt-0.5">{fmt(live.created_at)}</div>
      <div class="text-gray-500">{live.created_by_email ?? '—'}</div>
    </div>
    <div>
      <div class="text-gray-400 uppercase tracking-wide">Updated</div>
      <div class="text-gray-900 mt-0.5">{fmt(live.updated_at)}</div>
    </div>
    <div>
      <div class="text-gray-400 uppercase tracking-wide">Executed</div>
      <div class="text-gray-900 mt-0.5">{fmt(live.executed_at ?? null)}</div>
      <div class="text-gray-500">{live.executed_by_email ?? '—'}</div>
    </div>
    <div>
      <div class="text-gray-400 uppercase tracking-wide">Items / Qty</div>
      <div class="text-gray-900 mt-0.5">{live.items.length} items</div>
      <div class="text-gray-500">total {live.items.reduce((s, i) => s + i.quantity, 0)}</div>
    </div>
  </div>

  <!-- Type selector / readonly badge -->
  {#if !isExecuted}
    <section class="bg-white border border-gray-200 rounded-xl p-4 space-y-2">
      <h2 class="text-sm font-medium text-gray-900">Direction</h2>
      <div class="inline-flex rounded-lg border border-gray-200 overflow-hidden text-sm">
        <button class="px-4 py-2 {type === 'in' ? 'bg-emerald-600 text-white' : 'bg-white text-emerald-700 hover:bg-gray-50'}"
                onclick={() => setType('in')} type="button">＋ Stock In</button>
        <button class="px-4 py-2 border-l border-gray-200 {type === 'out' ? 'bg-red-600 text-white' : 'bg-white text-red-700 hover:bg-gray-50'}"
                onclick={() => setType('out')} type="button">− Stock Out</button>
      </div>
    </section>
  {/if}

  <!-- Items -->
  <section class="bg-white border border-gray-200 rounded-xl p-4 space-y-4">
    <h2 class="text-sm font-medium text-gray-900">Items</h2>
    {#if !isExecuted}
      <ProductPicker {token} mode="variant-only" onAdd={addItem} />
    {/if}
    <MutationItemsTable {items} {type} readonly={isExecuted}
                        onChangeQty={changeQty} onRemove={removeRow} />
  </section>

  <!-- Note -->
  <section class="bg-white border border-gray-200 rounded-xl p-4 space-y-2">
    <label for="note" class="text-sm font-medium text-gray-900">Note</label>
    {#if isExecuted}
      <p class="text-sm text-gray-600 whitespace-pre-wrap">{note || '—'}</p>
    {:else}
      <textarea id="note" bind:value={note} rows="3"
                class="w-full text-sm border border-gray-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-gray-900"></textarea>
    {/if}
  </section>

  <!-- Actions -->
  <div class="flex flex-wrap justify-end gap-2">
    {#if isExecuted}
      <button onclick={duplicate} class="px-4 py-2 text-sm rounded-lg border border-gray-200 hover:bg-gray-50">Duplicate</button>
    {:else}
      <button onclick={remove}
              class="px-4 py-2 text-sm rounded-lg border border-red-200 text-red-700 hover:bg-red-50">Delete</button>
      <button onclick={duplicate}
              class="px-4 py-2 text-sm rounded-lg border border-gray-200 hover:bg-gray-50">Duplicate</button>
      <button onclick={save} disabled={saving || executing}
              class="px-4 py-2 text-sm rounded-lg border border-gray-200 hover:bg-gray-50 disabled:opacity-50">
        {saving ? '…' : 'Save draft'}
      </button>
      <button onclick={execute} disabled={saving || executing}
              class="px-4 py-2 text-sm rounded-lg bg-gray-900 text-white hover:bg-gray-700 disabled:opacity-50">
        {executing ? '…' : 'Save & Execute'}
      </button>
    {/if}
  </div>

  {#if conflicts && conflicts.length > 0}
    <section class="bg-red-50 border border-red-200 rounded-xl p-4 space-y-2">
      <h3 class="text-sm font-medium text-red-800">庫存衝突</h3>
      <ul class="text-xs text-red-700 space-y-1">
        {#each conflicts as c}
          <li>
            • <strong>{c.product_name ?? c.variant_id}</strong>
            ({c.variant_sku ?? '—'}) — 需要 {c.requested}，現有 {c.available}
          </li>
        {/each}
      </ul>
      <p class="text-xs text-red-600">調整 quantity 或者去補貨後再 execute。</p>
    </section>
  {/if}
</div>
