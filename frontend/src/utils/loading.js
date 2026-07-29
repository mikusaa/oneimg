import { lockBodyScroll, unlockBodyScroll } from './scrollLock.js'

class Loading {
  static defaults = {
    text: '加载中...',
    className: 'custom-loading',
    mask: true,
    color: '#2563eb',
    fullscreen: true,
    delay: 150,
    zIndex: 9999,
    container: document.body,
    onShow: null,
    onHide: null
  }

  static instances = []

  static createLoadingDom(config) {
    const dom = document.createElement('div')
    dom.className = `app-loading ${config.fullscreen ? 'app-loading-fullscreen' : 'app-loading-local'} ${config.className}`
    dom.style.zIndex = config.zIndex
    dom.dataset.className = config.className
    dom.dataset.fullscreen = String(config.fullscreen)
    dom.setAttribute('role', 'status')
    dom.setAttribute('aria-live', 'polite')
    dom.setAttribute('aria-busy', 'true')

    if (config.mask) {
      const mask = document.createElement('div')
      mask.className = 'app-loading-mask app-material'
      dom.appendChild(mask)
    }

    const content = document.createElement('div')
    content.className = 'app-loading-content'

    const spinner = document.createElement('div')
    spinner.className = 'app-loading-spinner'
    spinner.style.setProperty('--loading-color', config.color)
    spinner.setAttribute('aria-hidden', 'true')

    const text = document.createElement('div')
    text.className = 'app-loading-text'
    text.textContent = config.text

    content.append(spinner, text)
    dom.appendChild(content)
    return dom
  }

  static show(options) {
    const config = typeof options === 'string'
      ? { ...this.defaults, text: options }
      : { ...this.defaults, ...options }

    const existing = this.instances.find(item => (
      item.config.container === config.container && item.config.fullscreen === config.fullscreen
    ))
    if (existing) existing.hide()

    const dom = this.createLoadingDom(config)
    const instance = {
      dom,
      config,
      visible: false,
      dismissed: false,
      showTimer: null,
      previousPosition: '',
      positionAdjusted: false,
      ownsScrollLock: false,
      hide: () => this.hide(dom)
    }

    instance.showTimer = setTimeout(() => {
      if (instance.dismissed) return
      if (!config.fullscreen) {
        const computedPosition = getComputedStyle(config.container).position
        if (computedPosition === 'static') {
          instance.previousPosition = config.container.style.position
          config.container.style.position = 'relative'
          instance.positionAdjusted = true
        }
      } else {
        lockBodyScroll()
        instance.ownsScrollLock = true
      }
      config.container.appendChild(dom)
      instance.visible = true
      requestAnimationFrame(() => dom.classList.add('app-loading-visible'))
      if (typeof config.onShow === 'function') config.onShow()
    }, Math.max(0, Number(config.delay) || 0))

    this.instances.push(instance)
    return instance
  }

  static hide(dom, delay = 0) {
    const instance = this.instances.find(item => item.dom === dom)
    if (!instance || instance.dismissed) return Promise.resolve()
    instance.dismissed = true
    clearTimeout(instance.showTimer)

    return new Promise(resolve => {
      setTimeout(() => {
        const finish = () => {
          dom.remove()
          if (instance.ownsScrollLock) unlockBodyScroll()
          if (instance.positionAdjusted) {
            instance.config.container.style.position = instance.previousPosition
          }
          this.instances = this.instances.filter(item => item !== instance)
          if (typeof instance.config.onHide === 'function') instance.config.onHide()
          resolve()
        }

        if (!instance.visible) {
          finish()
          return
        }
        dom.classList.remove('app-loading-visible')
        setTimeout(finish, 160)
      }, delay)
    })
  }

}

export default Loading
