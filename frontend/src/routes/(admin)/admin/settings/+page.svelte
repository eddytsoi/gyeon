<script lang="ts">
  import { enhance } from '$app/forms';
  import type { PageData, ActionData } from './$types';

  let { data, form }: { data: PageData; form: ActionData } = $props();
  let saving = $state(false);
</script>

<svelte:head><title>Settings — Gyeon Admin</title></svelte:head>

<div class="max-w-2xl">
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

  <form method="POST" action="?/save"
        use:enhance={() => {
          saving = true;
          return async ({ update }) => { await update(); saving = false; };
        }}>
    <div class="bg-white rounded-2xl border border-gray-100 p-6">
      <div class="flex flex-col gap-5">
        {#each data.settings as setting}
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
        {:else}
          <p class="text-sm text-gray-400">No settings found.</p>
        {/each}
      </div>

      <div class="mt-6 pt-5 border-t border-gray-100">
        <button type="submit" disabled={saving}
                class="px-5 py-2.5 bg-gray-900 text-white text-sm font-medium rounded-xl
                       hover:bg-gray-700 transition-colors disabled:opacity-50">
          {saving ? 'Saving…' : 'Save Settings'}
        </button>
      </div>
    </div>
  </form>
</div>
