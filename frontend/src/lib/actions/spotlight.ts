// Magnetic spotlight: a soft highlight that glides under the cursor and snaps
// to the bounding box of whichever child element matches `selector`.
//
// Tailwind classes used (kept as literal strings here so JIT picks them up):
//   pointer-events-none absolute rounded-lg bg-gray-50
//   transition-[transform,width,height,opacity] duration-[80ms] ease-out

type Options = {
  selector: string;
};

export function spotlight(node: HTMLElement, opts: Options) {
  const sp = document.createElement('div');
  sp.setAttribute('aria-hidden', 'true');
  sp.className =
    'pointer-events-none absolute rounded-lg bg-gray-50 ' +
    'transition-[transform,width,height,opacity] duration-[80ms] ease-out';
  sp.style.cssText =
    'top: 0; left: 0; transform: translate3d(0,0,0); width: 0; height: 0; opacity: 0; z-index: -1;';

  const cs = getComputedStyle(node);
  if (cs.position === 'static') node.style.position = 'relative';
  node.style.isolation = 'isolate';
  node.prepend(sp);

  function move(item: Element | null) {
    if (!item || !node.contains(item)) {
      sp.style.opacity = '0';
      return;
    }
    const nr = node.getBoundingClientRect();
    const ir = (item as HTMLElement).getBoundingClientRect();
    sp.style.opacity = '1';
    sp.style.transform = `translate3d(${ir.left - nr.left + node.scrollLeft}px, ${ir.top - nr.top + node.scrollTop}px, 0)`;
    sp.style.width = `${ir.width}px`;
    sp.style.height = `${ir.height}px`;
  }

  function onMove(e: MouseEvent) {
    move((e.target as HTMLElement | null)?.closest(opts.selector) ?? null);
  }

  function onLeave() {
    sp.style.opacity = '0';
  }

  node.addEventListener('mousemove', onMove);
  node.addEventListener('mouseleave', onLeave);

  return {
    destroy() {
      node.removeEventListener('mousemove', onMove);
      node.removeEventListener('mouseleave', onLeave);
      sp.remove();
    }
  };
}
