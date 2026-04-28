<script lang="ts">
  import { invalidateAll } from '$app/navigation';
  import { deleteMySavedCard, setDefaultCard } from '$lib/api';
  import type { PageData } from './$types';

  let { data }: { data: PageData } = $props();

  let working = $state<string | null>(null); // id of card being acted on
  let errorMsg = $state('');

  async function handleDelete(id: string) {
    if (!data.token) return;
    working = id;
    errorMsg = '';
    try {
      const res = await deleteMySavedCard(data.token, id);
      if (!res.ok) throw new Error('Failed');
      await invalidateAll();
    } catch {
      errorMsg = 'Could not remove card. Please try again.';
    } finally {
      working = null;
    }
  }

  async function handleSetDefault(id: string) {
    if (!data.token) return;
    working = id;
    errorMsg = '';
    try {
      const res = await setDefaultCard(data.token, id);
      if (!res.ok) throw new Error('Failed');
      await invalidateAll();
    } catch {
      errorMsg = 'Could not update default card. Please try again.';
    } finally {
      working = null;
    }
  }
</script>

<svelte:head>
  <title>Saved Cards — Gyeon</title>
</svelte:head>

<div class="flex flex-col gap-4">
  <h1 class="text-xl font-bold text-gray-900">Saved Cards</h1>

  {#if errorMsg}
    <p class="text-sm text-red-500">{errorMsg}</p>
  {/if}

  {#if data.cards.length === 0}
    <div class="bg-white rounded-2xl border border-gray-100 p-10 text-center">
      <p class="text-gray-400 text-sm">No saved cards yet.</p>
      <p class="text-gray-400 text-xs mt-1">Cards are saved when you tick "儲存此卡以供日後使用" at checkout.</p>
    </div>
  {:else}
    <div class="flex flex-col gap-3">
      {#each data.cards as card}
        <div class="bg-white rounded-2xl border border-gray-100 p-5 flex items-center gap-4">
          <!-- Card icon placeholder -->
          <div class="w-10 h-7 rounded-md bg-gray-100 flex items-center justify-center flex-shrink-0">
            <span class="text-[10px] font-bold text-gray-500 uppercase">{card.brand.slice(0, 4)}</span>
          </div>

          <div class="flex-1 min-w-0">
            <p class="text-sm font-medium text-gray-900 capitalize">
              {card.brand} •••• {card.last4}
              {#if card.is_default}
                <span class="ml-2 px-1.5 py-0.5 bg-gray-100 text-gray-500 text-xs rounded-full font-normal">Default</span>
              {/if}
            </p>
            <p class="text-xs text-gray-400 mt-0.5">Expires {card.exp_month}/{card.exp_year}</p>
          </div>

          <div class="flex items-center gap-2 flex-shrink-0">
            {#if !card.is_default}
              <button
                type="button"
                onclick={() => handleSetDefault(card.id)}
                disabled={working === card.id}
                class="text-xs text-gray-500 hover:text-gray-900 transition-colors disabled:opacity-40"
              >
                Set default
              </button>
            {/if}
            <button
              type="button"
              onclick={() => handleDelete(card.id)}
              disabled={working === card.id}
              class="text-xs text-red-400 hover:text-red-600 transition-colors disabled:opacity-40"
            >
              {working === card.id ? '…' : 'Remove'}
            </button>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>
