<script lang="ts">
  // Compact trust signal shown directly above the Stripe Payment Element on
  // every checkout surface. Reassures shoppers that card entry is encrypted and
  // handled by Stripe — the second line is literally true because Stripe
  // Elements tokenizes the card in the browser (the number never reaches our
  // backend). Styled with neutral design-system tokens so it reads well on the
  // white card backgrounds used by both checkout layouts and the /pay page.
  import * as m from '$lib/paraglide/messages';
</script>

<div class="flex items-start gap-2.5 rounded-xl border border-gray-100 bg-paper px-3.5 py-3">
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.7"
    stroke-linecap="round"
    stroke-linejoin="round"
    class="h-5 w-5 shrink-0 text-sky-500"
    aria-hidden="true"
    focusable="false"
  >
    <rect x="4.5" y="10.5" width="15" height="10.5" rx="2.2" />
    <path d="M8 10.5V7a4 4 0 0 1 8 0v3.5" />
    <circle cx="12" cy="15.5" r="1.3" fill="currentColor" stroke="none" />
  </svg>
  <div class="min-w-0">
    <p class="text-sm font-medium text-ink-900">{m.secure_payment_title()}</p>
    <p class="text-xs text-ink-500 mt-0.5">{m.secure_payment_subtitle()}</p>
  </div>
</div>
