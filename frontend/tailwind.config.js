/** @type {import('tailwindcss').Config} */
export default {
  content: ['./src/**/*.{html,js,svelte,ts}'],
  safelist: [
    'bg-violet-500', 'bg-blue-500', 'bg-amber-500', 'bg-emerald-500',
    'bg-violet-50',  'bg-blue-50',  'bg-amber-50',  'bg-emerald-50',
    'text-violet-600','text-blue-600','text-amber-600','text-emerald-600',
  ],
  theme: {
    extend: {}
  },
  plugins: []
};
