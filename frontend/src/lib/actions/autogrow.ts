// Svelte action that auto-grows a <textarea> to fit its content. The `rows`
// attribute on the element acts as the minimum height; the action only ever
// expands beyond that. Resizes on every `input` event and once on mount;
// the trailing requestAnimationFrame call covers cases where the textarea's
// font hasn't finished loading on the first measurement.

export function autogrow(node: HTMLTextAreaElement) {
  const resize = () => {
    node.style.height = 'auto';
    node.style.height = node.scrollHeight + 'px';
  };
  node.addEventListener('input', resize);
  resize();
  requestAnimationFrame(resize);
  return {
    destroy() {
      node.removeEventListener('input', resize);
    }
  };
}
