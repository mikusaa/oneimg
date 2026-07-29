let lockCount = 0
let previousOverflow = ''

export const lockBodyScroll = () => {
  if (typeof document === 'undefined') return
  if (lockCount === 0) {
    previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
  }
  lockCount += 1
}

export const unlockBodyScroll = () => {
  if (typeof document === 'undefined' || lockCount === 0) return
  lockCount -= 1
  if (lockCount === 0) {
    document.body.style.overflow = previousOverflow
    previousOverflow = ''
  }
}

export const resetBodyScroll = () => {
  if (typeof document === 'undefined') return
  lockCount = 0
  document.body.style.overflow = previousOverflow
  previousOverflow = ''
}
