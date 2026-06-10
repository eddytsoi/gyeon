// Tracks whether the mobile sticky add-to-cart bar is on screen, so the
// floating WhatsApp button can lift above it instead of covering the CTA.
let _visible = $state(false);

export const stickyAddToCartBar = {
  get visible() {
    return _visible;
  },
  set visible(v: boolean) {
    _visible = v;
  }
};
