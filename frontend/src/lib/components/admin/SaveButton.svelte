<script lang="ts">
  import type { Snippet } from 'svelte';
  import SaveIcon from './SaveIcon.svelte';
  import Spinner from './Spinner.svelte';

  interface Props {
    loading?: boolean;
    disabled?: boolean;
    type?: 'submit' | 'button';
    form?: string;
    class?: string;
    onclick?: (e: MouseEvent) => void;
    children?: Snippet;
  }

  let {
    loading = false,
    disabled = false,
    type = 'submit',
    form,
    class: cls = '',
    onclick,
    children,
  }: Props = $props();
</script>

<button
  {type}
  {form}
  {onclick}
  disabled={loading || disabled}
  aria-busy={loading}
  class={cls}
>
  {#if loading}
    <Spinner />
  {:else}
    <SaveIcon />
  {/if}
  {#if children}{@render children()}{/if}
</button>
