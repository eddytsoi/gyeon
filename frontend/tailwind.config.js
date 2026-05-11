/** @type {import('tailwindcss').Config} */
export default {
  content: ['./src/**/*.{html,js,svelte,ts}'],
  safelist: [
    'bg-violet-500', 'bg-blue-500', 'bg-amber-500', 'bg-emerald-500',
    'bg-violet-50',  'bg-blue-50',  'bg-amber-50',  'bg-emerald-50',
    'text-violet-600','text-blue-600','text-amber-600','text-emerald-600',
  ],
  theme: {
    extend: {
      // Gyeon design system — see gyeon-project-design-system.md §1.1
      colors: {
        navy: {
          900: '#19253F',
          700: '#21314D',
          500: '#334977',
          300: '#285394',
        },
        sky: {
          500: '#3692C0',
        },
        ink: {
          900: '#1A1A1A',
          500: '#8C8C8C',
          300: '#C7C7C7',
        },
        paper: '#F8F7F4',
        cream: '#EDE9E1',
        amber: {
          500: '#F8995D',
          300: '#FED022',
        },
        success: '#77A464',
        alert:   '#C0392B',
      },
      fontFamily: {
        // Barlow Condensed proxies GT America Compressed; Inter proxies Gotham.
        // Swap to the licensed faces in app.css when woff2 lands in static/fonts/.
        display: ['"Barlow Condensed"', '"GT America Compressed"', 'Helvetica', 'Arial', 'sans-serif'],
        body:    ['Inter', 'Gotham', 'Helvetica', 'Arial', 'sans-serif'],
        mono:    ['ui-monospace', 'SFMono-Regular', 'Menlo', 'monospace'],
      },
      borderRadius: {
        'sm':  '4px',
        'md':  '8px',
        'lg':  '12px',
        'xl':  '16px',
        '2xl': '24px',
      },
      boxShadow: {
        'card':       '0 1px 2px rgba(25,37,63,0.04), 0 8px 24px -8px rgba(25,37,63,0.08)',
        'card-hover': '0 4px 8px rgba(25,37,63,0.06), 0 16px 40px -8px rgba(25,37,63,0.14)',
      },
      transitionTimingFunction: {
        'gy': 'cubic-bezier(0.2, 0.7, 0.2, 1)',
      },
      aspectRatio: {
        'product': '1 / 1',
        'banner':  '16 / 9',
        'card':    '4 / 5',
      },
      // Storefront/admin shell — bumped from default 80rem (1280px) to 96rem (1536px).
      maxWidth: {
        '7xl': '96rem',
      },
    }
  },
  plugins: []
};
