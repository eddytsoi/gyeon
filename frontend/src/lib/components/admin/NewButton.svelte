<script lang="ts">
  interface Props {
    label: string;
    href?: string;
    action?: () => void;
  }

  let { label, href, action }: Props = $props();

  function clickHandler(node: HTMLElement, fn: (() => void) | undefined) {
    if (fn) node.addEventListener('click', fn);
    return {
      destroy() { if (fn) node.removeEventListener('click', fn); },
      update(newFn: (() => void) | undefined) {
        if (fn) node.removeEventListener('click', fn);
        fn = newFn;
        if (fn) node.addEventListener('click', fn);
      }
    };
  }
</script>

{#if href}
  <a {href} class="btn" aria-label={label}>
    <span class="icon" aria-hidden="true">
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none"
           stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M12 4.5v15m7.5-7.5h-15"/>
      </svg>
    </span>
    <span class="label-grid">
      <span class="label-inner"><span class="label-text">{label}</span></span>
    </span>
  </a>
{:else}
  <button type="button" use:clickHandler={action} class="btn" aria-label={label}>
    <span class="icon" aria-hidden="true">
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none"
           stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M12 4.5v15m7.5-7.5h-15"/>
      </svg>
    </span>
    <span class="label-grid">
      <span class="label-inner"><span class="label-text">{label}</span></span>
    </span>
  </button>
{/if}

<style>
  .btn {
    display: inline-flex;
    align-items: center;
    height: 40px;
    border-radius: 9999px;
    background-color: #111827;
    color: white;
    overflow: hidden;
    transition: background-color 150ms ease;
    cursor: pointer;
    border: none;
    padding: 0;
    text-decoration: none;
    flex-shrink: 0;
  }

  .btn:hover,
  .btn:focus-visible {
    background-color: #374151;
    outline: none;
  }

  .btn:focus-visible {
    box-shadow: 0 0 0 2px #fff, 0 0 0 4px #111827;
  }

  .icon {
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    width: 40px;
    height: 40px;
  }

  .label-grid {
    display: grid;
    grid-template-columns: 0fr;
    transition: grid-template-columns 250ms cubic-bezier(0.4, 0, 0.2, 1);
  }

  .btn:hover .label-grid,
  .btn:focus-visible .label-grid {
    grid-template-columns: 1fr;
  }

  .label-inner {
    overflow: hidden;
    white-space: nowrap;
    opacity: 0;
    transition: opacity 150ms ease 80ms;
  }

  .btn:hover .label-inner,
  .btn:focus-visible .label-inner {
    opacity: 1;
  }

  .label-text {
    display: inline-block;
    font-size: 0.875rem;
    font-weight: 500;
    padding-right: 16px;
  }

  @media (prefers-reduced-motion: reduce) {
    .label-grid,
    .label-inner {
      transition: none;
    }
  }
</style>
