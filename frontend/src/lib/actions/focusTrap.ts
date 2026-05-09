// Modal/dialog focus management: traps Tab cycling inside the node, focuses
// the first focusable child on mount, and returns focus to whichever element
// triggered the modal when the action is destroyed.
//
// Usage on a `role="dialog"` container:
//   <div role="dialog" use:focusTrap>...</div>
// Mount and unmount the node only when the modal is open (e.g. inside `{#if}`).
//
// Reads focusable descendants on demand — works with content that mounts after
// the action runs (e.g. async-loaded form fields). Skips disabled / hidden
// elements and any with tabindex="-1".

const FOCUSABLE = [
  'a[href]',
  'button:not([disabled])',
  'input:not([disabled]):not([type="hidden"])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])'
].join(',');

function focusable(root: HTMLElement): HTMLElement[] {
  return Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE)).filter(
    (el) => !el.hasAttribute('disabled') && el.offsetParent !== null
  );
}

export function focusTrap(node: HTMLElement) {
  const previousActive = document.activeElement as HTMLElement | null;

  // Defer initial focus by one frame so any inline transition / autofocus
  // attribute on a child input wins gracefully without competing.
  requestAnimationFrame(() => {
    const items = focusable(node);
    (items[0] ?? node).focus();
  });

  function onKey(e: KeyboardEvent) {
    if (e.key !== 'Tab') return;
    const items = focusable(node);
    if (items.length === 0) {
      e.preventDefault();
      return;
    }
    const first = items[0];
    const last = items[items.length - 1];
    const active = document.activeElement as HTMLElement | null;
    if (e.shiftKey && active === first) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && active === last) {
      e.preventDefault();
      first.focus();
    }
  }

  node.addEventListener('keydown', onKey);

  return {
    destroy() {
      node.removeEventListener('keydown', onKey);
      previousActive?.focus?.();
    }
  };
}
