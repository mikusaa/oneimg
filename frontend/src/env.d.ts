/// <reference types="vite/client" />

declare module '*.vue' {
  const component: any
  export default component
}

declare module '*.js'
declare global {
  interface Window {
    seoStting?: Record<string, string>
    seoBus?: { callbacks: Array<(value: Record<string, string>) => void>; onUpdate: (callback: (value: Record<string, string>) => void) => void }
    refreshNavItems?: () => void
    Spotlight?: { show: (items: Array<{ src: string }>) => void }
  }
}

export {}
