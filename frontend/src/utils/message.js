class Message {
  static defaults = {
    type: 'info',
    message: '',
    duration: 3000,
    position: 'top-right',
    offset: 75,
    zIndex: 99999,
    showClose: false,
    onClose: null
  }

  static instances = []
  static maxVisible = 3

  static iconMap = {
    success: 'ri-check-line',
    info: 'ri-information-line',
    warning: 'ri-error-warning-line',
    error: 'ri-close-line'
  }

  static getMessageKey(config) {
    const content = typeof config.message === 'string' ? config.message : config.message?.textContent || ''
    return `${config.type}:${config.position}:${content}`
  }

  static getContainer(config) {
    const id = `message-stack-${config.position}`
    let container = document.getElementById(id)
    if (container) return container

    container = document.createElement('div')
    container.id = id
    container.className = `message-stack message-stack-${config.position}`
    container.style.setProperty('--message-offset', `${config.offset}px`)
    container.style.zIndex = config.zIndex
    container.setAttribute('aria-label', '系统通知')
    document.body.appendChild(container)
    return container
  }

  static createMessage(config) {
    const dom = document.createElement('div')
    dom.className = `app-toast app-toast-${config.type}`
    dom.dataset.position = config.position
    dom.dataset.key = this.getMessageKey(config)
    dom.setAttribute('role', config.type === 'error' ? 'alert' : 'status')
    dom.setAttribute('aria-live', config.type === 'error' ? 'assertive' : 'polite')
    dom.setAttribute('aria-atomic', 'true')

    const icon = document.createElement('span')
    icon.className = 'app-toast-icon'
    icon.setAttribute('aria-hidden', 'true')
    icon.innerHTML = `<i class="${this.iconMap[config.type] || this.iconMap.info}"></i>`

    const text = document.createElement('span')
    text.className = 'app-toast-text'
    if (config.message instanceof HTMLElement) {
      text.appendChild(config.message)
    } else {
      text.textContent = String(config.message)
    }

    const count = document.createElement('span')
    count.className = 'app-toast-count hidden'
    count.setAttribute('aria-hidden', 'true')

    dom.append(icon, text, count)

    if (config.showClose || config.duration === 0) {
      const close = document.createElement('button')
      close.type = 'button'
      close.className = 'app-toast-close pressable'
      close.setAttribute('aria-label', '关闭通知')
      close.innerHTML = '<i class="ri-close-line"></i>'
      close.addEventListener('click', () => this.close(dom))
      dom.appendChild(close)
    }

    return { dom, count }
  }

  static startTimer(instance) {
    if (instance.timer) clearTimeout(instance.timer)
    if (instance.config.duration <= 0) return
    instance.timer = setTimeout(() => this.close(instance.dom), instance.config.duration)
  }

  static show(options) {
    const config = typeof options === 'string'
      ? { ...this.defaults, message: options }
      : { ...this.defaults, ...options }

    if (!config.message) {
      console.warn('Message 通知内容不能为空')
      return null
    }

    const key = this.getMessageKey(config)
    const duplicate = this.instances.find(instance => instance.key === key)
    if (duplicate) {
      duplicate.count += 1
      duplicate.countElement.textContent = `×${duplicate.count}`
      duplicate.countElement.classList.remove('hidden')
      duplicate.dom.classList.remove('app-toast-pulse')
      requestAnimationFrame(() => duplicate.dom.classList.add('app-toast-pulse'))
      this.startTimer(duplicate)
      return duplicate
    }

    while (this.instances.length >= this.maxVisible) {
      this.close(this.instances[0].dom, true)
    }

    const { dom, count } = this.createMessage(config)
    const container = this.getContainer(config)
    const instance = {
      dom,
      config,
      key,
      count: 1,
      countElement: count,
      timer: null,
      close: () => this.close(dom)
    }

    dom.addEventListener('mouseenter', () => {
      if (instance.timer) clearTimeout(instance.timer)
    })
    dom.addEventListener('mouseleave', () => this.startTimer(instance))

    container.appendChild(dom)
    this.instances.push(instance)
    requestAnimationFrame(() => dom.classList.add('app-toast-visible'))
    this.startTimer(instance)
    return instance
  }

  static close(dom, immediate = false) {
    const index = this.instances.findIndex(instance => instance.dom === dom)
    if (index < 0) return
    const [instance] = this.instances.splice(index, 1)
    if (instance.timer) clearTimeout(instance.timer)

    const remove = () => {
      const parent = dom.parentElement
      dom.remove()
      if (parent && parent.childElementCount === 0) parent.remove()
      if (typeof instance.config.onClose === 'function') instance.config.onClose()
    }

    if (immediate) {
      remove()
      return
    }
    dom.classList.remove('app-toast-visible')
    setTimeout(remove, 180)
  }

  static closeAll() {
    ;[...this.instances].forEach(instance => this.close(instance.dom))
  }
}

Message.success = function(message, options = {}) {
  return this.show({ type: 'success', message, ...options })
}

Message.info = function(message, options = {}) {
  return this.show({ type: 'info', message, ...options })
}

Message.warning = function(message, options = {}) {
  return this.show({ type: 'warning', message, ...options })
}

Message.error = function(message, options = {}) {
  return this.show({ type: 'error', message, showClose: true, ...options })
}

window.Message = Message

if (!window.showToast) {
  window.showToast = function(message, type = 'success', duration = 2000) {
    return Message.show({ type, message, duration, position: 'top-center' })
  }
}

export default Message
