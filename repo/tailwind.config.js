/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    './internal/templates/*.templ',
  ],
  safelist: [
    // Dynamic classes from statusBadge() in base.templ
    'bg-green-100', 'text-green-800',
    'bg-yellow-100', 'text-yellow-800',
    'bg-gray-100', 'text-gray-800',
    'bg-red-100', 'text-red-800',
    'bg-purple-100', 'text-purple-800',
    'bg-blue-100', 'text-blue-800',
    // Dynamic stat card colors
    'text-yellow-600', 'text-green-600',
    // Dynamic nav item classes
    'bg-gray-900', 'text-gray-300',
    'hover:bg-gray-700', 'hover:text-white',
    // Dynamic text colors used in KPI/ticket detail
    'text-red-600', 'text-green-600',
    // Dynamic button/badge colors used in JS
    'bg-green-500', 'bg-red-500', 'bg-yellow-500', 'bg-blue-500',
  ],
  theme: {
    extend: {},
  },
  plugins: [],
}
