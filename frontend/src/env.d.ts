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
    copyPreviewImageLink?: (...args: any[]) => void
    downloadPreviewImage?: (...args: any[]) => void
    deletePreviewImage?: (...args: any[]) => void
    closePreviewModal?: (...args: any[]) => void
    toggleHomePreviewTags?: (...args: any[]) => void
    togglePreviewTags?: (...args: any[]) => void
    togglePreviewCopyMenu?: (...args: any[]) => void
    addImageTag?: (...args: any[]) => void
    deleteImageTag?: (...args: any[]) => void
  }
}

export {}
