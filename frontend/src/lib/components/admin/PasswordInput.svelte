<script lang="ts">
  import * as m from '$lib/paraglide/messages';

  import type { HTMLInputAttributes } from 'svelte/elements';

  interface Props {
    id?: string;
    name?: string;
    value?: string;
    placeholder?: string;
    autocomplete?: HTMLInputAttributes['autocomplete'];
    class?: string;
  }

  let {
    id,
    name,
    value = $bindable(''),
    placeholder,
    autocomplete = 'off',
    class: extraClass = '',
  }: Props = $props();

  let copied = $state(false);
  let timer: ReturnType<typeof setTimeout> | undefined;

  async function copy() {
    const text = value ?? '';
    if (!text) return;
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      const ta = document.createElement('textarea');
      ta.value = text;
      ta.style.position = 'fixed';
      ta.style.opacity = '0';
      document.body.appendChild(ta);
      ta.select();
      try { document.execCommand('copy'); } catch { /* ignore */ }
      document.body.removeChild(ta);
    }
    copied = true;
    if (timer) clearTimeout(timer);
    timer = setTimeout(() => (copied = false), 1500);
  }
</script>

<div class="relative {extraClass}">
  <input
    {id}
    {name}
    type="password"
    bind:value
    {placeholder}
    {autocomplete}
    class="w-full border border-gray-200 rounded-xl px-3 py-2.5 pr-10 text-sm
           focus:outline-none focus:ring-2 focus:ring-gray-900" />
  <button
    type="button"
    onclick={copy}
    title={m.admin_password_copy_tip()}
    aria-label={m.admin_password_copy_tip()}
    disabled={!value}
    class="absolute inset-y-0 right-1 flex items-center justify-center w-8
           text-gray-400 hover:text-gray-700 disabled:opacity-40 disabled:cursor-not-allowed
           transition-colors">
    {#if copied}
      <svg class="w-4 h-4 text-green-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.25">
        <path stroke-linecap="round" stroke-linejoin="round" d="m4.5 12.75 6 6 9-13.5" />
      </svg>
    {:else}
      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.75">
        <path stroke-linecap="round" stroke-linejoin="round"
          d="M15.666 3.888A2.25 2.25 0 0 0 13.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 0 1-.75.75H9a.75.75 0 0 1-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 0 1-2.25 2.25H6.75A2.25 2.25 0 0 1 4.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 0 1 1.927-.184" />
      </svg>
    {/if}
  </button>
  {#if copied}
    <div class="absolute right-0 -top-7 px-2 py-0.5 rounded-md bg-gray-900 text-white text-[11px] font-medium whitespace-nowrap pointer-events-none shadow">
      {m.admin_password_copied()}
    </div>
  {/if}
</div>
