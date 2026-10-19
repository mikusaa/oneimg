/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: 'class',
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        // 新界面设计变量（见 src/assets/tokens.css），使用 CSS 变量，不支持 /50 这类透明度写法
        oi: {
          bg: 'var(--oi-bg)',
          sheet: 'var(--oi-sheet)',
          raised: 'var(--oi-raised)',
          panel: 'var(--oi-panel)',
          hover: 'var(--oi-hover)',
          line: 'var(--oi-line)',
          'line-2': 'var(--oi-line-2)',
          ink: 'var(--oi-ink)',
          text: 'var(--oi-text)',
          muted: 'var(--oi-muted)',
          faint: 'var(--oi-faint)',
          accent: 'var(--oi-accent)',
          'accent-2': 'var(--oi-accent-2)',
          'accent-soft': 'var(--oi-accent-soft)',
          ring: 'var(--oi-ring)',
          sakura: 'var(--oi-sakura)',
          'sakura-soft': 'var(--oi-sakura-soft)',
          'chart-image': 'var(--oi-chart-image)',
          'chart-video': 'var(--oi-chart-video)',
          warn: 'var(--oi-warn)',
          'warn-soft': 'var(--oi-warn-soft)',
          danger: 'var(--oi-danger)',
          'danger-soft': 'var(--oi-danger-soft)',
          btn: 'var(--oi-btn)',
          'btn-ink': 'var(--oi-btn-ink)',
          scrim: 'var(--oi-scrim)',
        },
      },
      borderRadius: {
        'oi-sm': 'var(--oi-r-sm)',
        oi: 'var(--oi-r)',
        'oi-lg': 'var(--oi-r-lg)',
      },
      fontFamily: {
        inter: ['Inter', 'sans-serif'],
        mono: ['SF Mono', 'Monaco', 'Menlo', 'Consolas', 'monospace'],
        oi: ['var(--oi-font)'],
        'oi-mono': ['var(--oi-mono)'],
      },
      boxShadow: {
        'sm': '0 1px 3px rgba(0, 0, 0, 0.1)',
        'md': '0 4px 6px rgba(0, 0, 0, 0.1)',
        'lg': '0 10px 15px rgba(0, 0, 0, 0.1)',
        'dark-sm': '0 1px 3px rgba(0, 0, 0, 0.3)',
        'dark-md': '0 4px 6px rgba(0, 0, 0, 0.3)',
        'dark-lg': '0 10px 15px rgba(0, 0, 0, 0.3)',
        'oi-sm': 'var(--oi-shadow-sm)',
        oi: 'var(--oi-shadow)',
        'oi-lg': 'var(--oi-shadow-lg)',
      },
      animation: {
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        'oi-fade': 'oi-fade 0.15s ease-out',
        'oi-dialog-in': 'oi-dialog-in 0.2s cubic-bezier(0.3, 0.7, 0.4, 1)',
        'oi-drawer-in': 'oi-drawer-in 0.25s cubic-bezier(0.3, 0.7, 0.4, 1)',
        'oi-toast-in': 'oi-toast-in 0.22s cubic-bezier(0.3, 0.7, 0.4, 1)',
        'oi-toast-out': 'oi-toast-out 0.18s ease-in forwards',
        'oi-pop-in': 'oi-pop-in 0.14s ease-out',
      },
      keyframes: {
        'oi-fade': { from: { opacity: '0' } },
        // 弹窗用 left/top 50% 加 translate 居中，动画首尾都要保留居中位移
        'oi-dialog-in': {
          from: { opacity: '0', transform: 'translate(-50%, calc(-50% + 10px)) scale(0.98)' },
          to: { opacity: '1', transform: 'translate(-50%, -50%)' },
        },
        'oi-drawer-in': { from: { opacity: '0', transform: 'translateX(24px)' } },
        'oi-toast-in': { from: { opacity: '0', transform: 'translateY(-10px) scale(0.97)' } },
        'oi-toast-out': { to: { opacity: '0', transform: 'translateY(-10px) scale(0.97)' } },
        'oi-pop-in': { from: { opacity: '0', transform: 'translateY(4px) scale(0.98)' } },
      },
    },
  },
  plugins: [],
}

