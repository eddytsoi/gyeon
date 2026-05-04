import Sortable, { type MoveEvent } from 'sortablejs';

export type SortableOptions = {
  /** Called after the user drops, with the new ID order. */
  onReorder: (orderedIds: string[]) => void | Promise<void>;
  /** CSS selector for the drag handle. Pass `false` to make the entire row draggable. Default: `[data-drag-handle]`. */
  handle?: string | false;
  /** Attribute on each row that holds its stable ID (default: `data-id`). */
  idAttr?: string;
  /** Selector for items (or descendants) that should not start a drag — e.g. pinned rows or interactive controls. */
  filter?: string;
  /** Optional move guard — return false to cancel the candidate move. */
  onMove?: (evt: MoveEvent) => boolean | void | -1 | 1;
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
    handle: options.handle === false ? undefined : (options.handle ?? '[data-drag-handle]'),
    filter: options.filter,
    preventOnFilter: false,
    animation: 180,
    easing: 'cubic-bezier(0.16, 1, 0.3, 1)',
    ghostClass: 'gy-ghost',
    chosenClass: 'gy-chosen',
    dragClass: 'gy-drag',
    forceFallback: true,
    fallbackTolerance: 4,
    onMove: options.onMove,
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
