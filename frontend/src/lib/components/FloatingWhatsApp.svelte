<script lang="ts">
  /*
   * Floating WhatsApp contact button — shown on every storefront page
   * (mounted from (storefront)/+layout.svelte). Opens the chat link in a new
   * tab. On mobile it lifts above the sticky add-to-cart bar so it never
   * covers the primary purchase CTA (see $lib/stores/stickyAddToCart).
   */
  import { stickyAddToCartBar } from '$lib/stores/stickyAddToCart.svelte';

  // Hardcoded contact link — change here to update everywhere.
  const WHATSAPP_URL = 'https://wa.me/85234680832';
</script>

<a
  href={WHATSAPP_URL}
  target="_blank"
  rel="noopener noreferrer"
  aria-label="WhatsApp"
  class="group fixed z-50 flex h-14 w-14 items-center justify-center rounded-full
         shadow-lg transition-transform duration-300 ease-gy
         hover:scale-105 active:scale-95
         right-[calc(1rem+env(safe-area-inset-right))]
         bottom-[calc(1rem+env(safe-area-inset-bottom))]
         sm:right-[calc(1.5rem+env(safe-area-inset-right))]
         sm:bottom-[calc(1.5rem+env(safe-area-inset-bottom))]
         lg:right-[calc(2rem+env(safe-area-inset-right))]
         lg:bottom-[calc(2rem+env(safe-area-inset-bottom))]
         {stickyAddToCartBar.visible ? 'max-lg:-translate-y-20' : ''}"
>
  <svg
    xmlns="http://www.w3.org/2000/svg"
    xmlns:xlink="http://www.w3.org/1999/xlink"
    viewBox="0 0 60 60"
    class="h-full w-full"
    aria-hidden="true"
    focusable="false"
  >
    <defs>
      <clipPath id="wa-a">
        <path
          d="M22.441,43.168a25.452,25.452,0,0,0,4.938,15.081L24.141,67.8,34.1,64.635A25.677,25.677,0,1,0,22.441,43.168"
          transform="translate(-22.441 -17.445)"
          fill="none"
        />
      </clipPath>
      <linearGradient
        id="wa-b"
        x1="-0.09"
        y1="1.092"
        x2="-0.086"
        y2="1.092"
        gradientUnits="objectBoundingBox"
      >
        <stop offset="0" stop-color="#4ac14b" />
        <stop offset="1" stop-color="#06853a" />
      </linearGradient>
      <clipPath id="wa-c">
        <rect width="60" height="60" fill="none" />
      </clipPath>
    </defs>
    <g transform="translate(4.782 3.717)">
      <g clip-path="url(#wa-a)">
        <rect width="51.852" height="51.443" transform="translate(0 0)" fill="url(#wa-b)" />
      </g>
    </g>
    <g clip-path="url(#wa-c)">
      <path
        d="M60,29.228A29.5,29.5,0,0,1,16.309,54.818L0,60,5.317,44.32A28.918,28.918,0,0,1,1.092,29.228a29.456,29.456,0,0,1,58.909,0M30.547,4.655A24.7,24.7,0,0,0,5.782,29.228,24.317,24.317,0,0,0,10.5,43.636L7.407,52.76l9.515-3.023A24.791,24.791,0,0,0,55.31,29.228,24.7,24.7,0,0,0,30.547,4.655m14.873,31.3c-.183-.3-.664-.478-1.383-.837s-4.274-2.092-4.935-2.33-1.146-.358-1.624.358-1.866,2.331-2.288,2.809-.84.537-1.562.179a19.752,19.752,0,0,1-5.808-3.554A21.78,21.78,0,0,1,23.8,27.623c-.419-.716-.042-1.1.316-1.461.326-.323.724-.837,1.084-1.253a4.927,4.927,0,0,0,.722-1.2,1.3,1.3,0,0,0-.062-1.253c-.179-.358-1.624-3.883-2.227-5.319s-1.2-1.378-1.624-1.378a10.389,10.389,0,0,0-1.2.125,4.078,4.078,0,0,0-2.108.9,8.007,8.007,0,0,0-2.526,5.973c0,3.525,2.585,6.933,2.946,7.408s4.993,7.949,12.334,10.818,7.341,1.911,8.665,1.79S44.4,41.038,45,39.365a5.954,5.954,0,0,0,.423-3.405"
        transform="translate(-0.001)"
        fill="#fff"
        fill-rule="evenodd"
      />
    </g>
  </svg>
</a>
