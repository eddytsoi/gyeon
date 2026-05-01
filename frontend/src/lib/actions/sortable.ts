import Sortable from 'sortablejs';

export type SortableOptions = {
  /** Called after the user drops, with the new ID order. */
  onReorder: (orderedIds: string[]) => void | Promise<void>;
  /** CSS selector for the drag handle (default: `[data-drag-handle]`). */
  handle?: string;
  /** Attribute on each row that holds its stable ID (default: `data-id`). */
  idAttr?: string;
};

/**
 * Svelte action that wires SortableJS on the host element. Handles cleanup on
 * destroy and calls `onReorder(ids)` with the new natural order whenever the
 * user drops a row in a new slot. Visual classes (`gy-ghost`, `gy-chosen`,
 * `gy-drag`) are styled by the consumer.
 */
export function sortable(node: HTMLElement, options: SortableOptions) {
  const idAttr = options.idAttr ?? 'data-id';

  const instance = Sortable.create(node, {
    handle: options.handle ?? '[data-drag-handle]',
    animation: 180,
    easing: 'cubic-bezier(0.16, 1, 0.3, 1)',
    ghostClass: 'gy-ghost',
    chosenClass: 'gy-chosen',
    dragClass: 'gy-drag',
    forceFallback: true,
    fallbackTolerance: 4,
    onEnd: (evt) => {
      if (evt.oldIndex === evt.newIndex) return;
      const ids = Array.from(node.children)
        .map((el) => (el as HTMLElement).getAttribute(idAttr))
        .filter((v): v is string => !!v);
      void options.onReorder(ids);
    }
  });

  return {
    destroy() {
      instance.destroy();
    }
  };
}
