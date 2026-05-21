<script lang="ts">
  import { goto } from '$app/navigation';
  import {
    adminCreateStockMutation,
    adminExecuteStockMutation,
    StockMutationInsufficientStockError,
    type StockMutationType,
    type StockMutationConflict
  } from '$lib/api/admin';
  import { notify } from '$lib/stores/notifications.svelte';
  import ProductPicker, { type ProductPickerAddPayload } from '$lib/components/admin/ProductPicker.svelte';
  import MutationItemsTable, { type MutationItemRow } from '$lib/components/admin/MutationItemsTable.svelte';
  import type { PageData } from './$types';

  let { data }: { data: PageData } = $props();
  const token = data.token!;

  let type = $state<StockMutationType>('in');
  let note = $state('');
  let items = $state<MutationItemRow[]>([]);
  let saving = $state(false);
  let conflicts = $state<StockMutationConflict[] | null>(null);

  let nextKey = 0;
  function addItem(payload: ProductPickerAddPayload) {
    const variant = payload.variant;
    // Reject duplicate variant — backend enforces UNIQUE(mutation_id, variant_id)
    // anyway, but catching it here gives an immediate, friendlier UX.
    if (items.some((it) => it.variantId === variant.id)) {
      notify.error('呢個 variant 已經喺單入面', '同一張 mutation 唔可以重複加入。');
      return;
    }
    nextKey += 1;
    items = [
      ...items,
      {
        key: `${variant.id}-${nextKey}`,
        variantId: variant.id,
        productName: payload.productName,
        sku: variant.sku,
        variantName: variant.name,
        primaryImageUrl: payload.primaryImageUrl ?? null,
        quantity: payload.quantity,
        currentStock: variant.stock_qty ?? null
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
    if (items.length > 0 && t !== type) {
      if (!confirm('改變方向會清除所有 line items。繼續？')) return;
      items = [];
    }
    type = t;
  }

  async function save({ thenExecute }: { thenExecute: boolean }) {
    if (items.length === 0) {
      notify.error('未有貨品', '至少要加一個 variant 先可以儲存。');
      return;
    }
    saving = true;
    conflicts = null;
    try {
      const created = await adminCreateStockMutation(token, {
        type,
        note: note.trim() || null,
        items: items.map((it) => ({ variant_id: it.variantId, quantity: it.quantity }))
      });
      if (!thenExecute) {
        notify.success(`已儲存 ${created.mutation_number}`);
        goto(`/admin/stock-mutations/${created.id}`);
        return;
      }
      try {
        await adminExecuteStockMutation(token, created.id);
        notify.success(`${created.mutation_number} 已儲存並執行`);
        goto(`/admin/stock-mutations/${created.id}`);
      } catch (e) {
        if (e instanceof StockMutationInsufficientStockError) {
          conflicts = e.conflicts;
          notify.error('庫存不足，未執行', '已儲存為 draft，請睇下面衝突列表。');
          goto(`/admin/stock-mutations/${created.id}`);
          return;
        }
        throw e;
      }
    } catch (e) {
      notify.error('儲存失敗', e instanceof Error ? e.message : 'unknown error');
    } finally {
      saving = false;
    }
  }
</script>

<svelte:head><title>New Stock Mutation · Admin</title></svelte:head>

<div class="space-y-4 max-w-4xl">
  <div class="flex items-center gap-3">
    <a href="/admin/stock-mutations" class="text-sm text-gray-500 hover:text-gray-700">← Back</a>
    <h1 class="text-2xl font-semibold text-gray-900">New Stock Mutation</h1>
  </div>

  <!-- Type selector -->
  <section class="bg-white border border-gray-200 rounded-xl p-4 space-y-3">
    <h2 class="text-sm font-medium text-gray-900">Direction</h2>
    <div class="inline-flex rounded-lg border border-gray-200 overflow-hidden text-sm">
      <button class="px-4 py-2 {type === 'in' ? 'bg-emerald-600 text-white' : 'bg-white text-emerald-700 hover:bg-gray-50'}"
              onclick={() => setType('in')} type="button">＋ Stock In</button>
      <button class="px-4 py-2 border-l border-gray-200 {type === 'out' ? 'bg-red-600 text-white' : 'bg-white text-red-700 hover:bg-gray-50'}"
              onclick={() => setType('out')} type="button">− Stock Out</button>
    </div>
    <p class="text-xs text-gray-500">
      {#if type === 'in'}
        每行 quantity 都會 <strong>加入</strong>庫存。
      {:else}
        每行 quantity 都會 <strong>扣減</strong>庫存。執行時如果有 variant 不足會 reject。
      {/if}
    </p>
  </section>

  <!-- Items -->
  <section class="bg-white border border-gray-200 rounded-xl p-4 space-y-4">
    <h2 class="text-sm font-medium text-gray-900">Items</h2>
    <ProductPicker {token} mode="variant-only" onAdd={addItem} />
    <MutationItemsTable {items} {type} onChangeQty={changeQty} onRemove={removeRow} />
  </section>

  <!-- Note -->
  <section class="bg-white border border-gray-200 rounded-xl p-4 space-y-2">
    <label for="note" class="text-sm font-medium text-gray-900">Note (optional)</label>
    <textarea id="note" bind:value={note} rows="3"
              class="w-full text-sm border border-gray-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-gray-900"
              placeholder="例如：2026-05 月盤點、收 supplier #123 到貨、…"></textarea>
  </section>

  <!-- Actions -->
  <div class="flex flex-wrap justify-end gap-2">
    <a href="/admin/stock-mutations" class="px-4 py-2 text-sm rounded-lg border border-gray-200 hover:bg-gray-50">Discard</a>
    <button onclick={() => save({ thenExecute: false })} disabled={saving}
            class="px-4 py-2 text-sm rounded-lg border border-gray-200 hover:bg-gray-50 disabled:opacity-50">
      {saving ? '…' : 'Save draft'}
    </button>
    <button onclick={() => save({ thenExecute: true })} disabled={saving}
            class="px-4 py-2 text-sm rounded-lg bg-gray-900 text-white hover:bg-gray-700 disabled:opacity-50">
      {saving ? '…' : 'Save & Execute'}
    </button>
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
    </section>
  {/if}
</div>
