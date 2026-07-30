import { lockBodyScroll, unlockBodyScroll } from './scrollLock.js'

class PopupModal {
  static openInstances = [];

  constructor(options = {}) {
    // 默认配置
    this.defaults = {
      // 基础配置
      id: `modal-${Date.now()}`,
      title: '提示',
      content: '',
      type: 'default', // default/confirm/form
      width: 'auto', // sm(300px)/md(500px)/lg(700px)/full(90%)
      showClose: true,
      mask: true,
      maskClose: true, // 默认允许遮罩关闭
      zIndex: 9999,

      // 按钮配置（默认2个按钮：取消/确认）
      buttons: [
        {
          text: '取消',
          type: 'default',
          callback: (modal) => modal.close()
        },
        {
          text: '确认',
          type: 'primary',
          callback: null
        }
      ],

      // 表单配置（type=form时生效）
      formFields: [],
      formSubmit: null,

      // 生命周期
      onOpen: null,
      onClose: null
    };

    // 合并配置（确保maskClose默认值生效）
    this.config = { ...this.defaults, ...options };
    
    // 状态管理
    this.state = {
      isOpen: false,
      formData: {}
    };
    this.previousActiveElement = null;
    this.keydownHandler = (event) => this.handleKeydown(event);

    // 初始化表单数据
    if (this.config.type === 'form' && this.config.formFields.length) {
      this.config.formFields.forEach(field => {
        this.state.formData[field.name] = field.defaultValue || '';
      });
    }

    // 创建DOM元素
    this.createElements();
    // 单独绑定遮罩事件（确保优先级）
    this.bindMaskEvent();
  }

  /**
   * 创建弹出框DOM结构
   */
  createElements() {
    // 宽度映射
    const widthMap = {
      sm: 'w-[300px]',
      md: 'w-[500px]',
      lg: 'w-[700px]',
      full: 'w-[90%]',
      auto: ['w-[calc(100%-20px)]','min-w-[320px]','max-w-[500px]']
    };

    // 1. 创建遮罩层
    this.mask = document.createElement('div');
    this.mask.id = `${this.config.id}-mask`;
    // 修复：遮罩初始状态添加pointer-events-none，避免遮挡页面
    this.mask.className = `legacy-modal-mask app-scrim fixed inset-0 bg-black/50 dark:bg-black/70 backdrop-blur-sm opacity-0 pointer-events-none`;
    this.mask.style.zIndex = this.config.zIndex - 1;
    document.body.appendChild(this.mask);

    // 2. 创建弹出框容器
    this.modal = document.createElement('div');
    this.modal.id = this.config.id;
    this.modal.className = `legacy-modal app-material fixed top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 scale-95 opacity-0 pointer-events-none rounded-xl border border-slate-200 bg-white dark:border-white/10 dark:bg-slate-900 shadow-2xl overflow-hidden`;
    this.modal.style.zIndex = this.config.zIndex;
    this.modal.setAttribute('role', 'dialog');
    this.modal.setAttribute('aria-modal', 'true');
    this.modal.setAttribute('tabindex', '-1');
    const widthClass = widthMap[this.config.width];
    this.modal.classList.add(...(Array.isArray(widthClass) ? widthClass : widthClass ? [widthClass] : widthMap.auto));
    if (!widthClass && typeof this.config.width === 'string') {
      this.modal.style.width = `min(${this.config.width}, calc(100vw - 20px))`;
      this.modal.style.maxWidth = 'calc(100vw - 20px)';
    }

    // 3. 头部（标题+关闭按钮）
    this.header = document.createElement('div');
    this.header.className = 'px-6 py-4 border-b border-light-200 dark:border-dark-100 flex justify-between items-center';
    
    // 标题
    this.titleEl = document.createElement('h3');
    this.titleEl.className = 'min-w-0 flex-1 pr-4 font-semibold text-lg text-dark-300 dark:text-light-100 truncate';
    this.titleEl.id = `${this.config.id}-title`;
    this.titleEl.innerHTML = this.config.title;
    this.modal.setAttribute('aria-labelledby', this.titleEl.id);
    this.header.appendChild(this.titleEl);

    // 关闭按钮
    if (this.config.showClose) {
      this.closeBtn = document.createElement('button');
      this.closeBtn.className = 'icon-button text-secondary hover:text-danger';
      this.closeBtn.innerHTML = '<i class="ri-close-fill font-bold text-[1.35rem]"></i>';
      this.closeBtn.setAttribute('aria-label', '关闭');
      this.closeBtn.setAttribute('title', '关闭');
      this.closeBtn.addEventListener('click', () => this.close());
      this.header.appendChild(this.closeBtn);
    }
    this.modal.appendChild(this.header);

    // 4. 内容区
    this.content = document.createElement('div');
    this.content.className = 'px-6 py-5 max-h-[60vh] overflow-y-auto';
    
    // 根据类型渲染内容
    if (this.config.type === 'form') {
      this.renderFormContent();
    } else {
      this.content.innerHTML = this.config.content;
    }
    this.modal.appendChild(this.content);

    // 5. 底部（按钮区）
    this.footer = document.createElement('div');
    this.footer.className = 'px-6 py-4 border-t border-light-200 dark:border-dark-100 flex justify-end gap-3';
    this.renderButtons();
    this.modal.appendChild(this.footer);

    // 添加到页面
    document.body.appendChild(this.modal);
  }

  /**
   * 单独绑定遮罩事件
   */
  bindMaskEvent() {
    // 确保mask和maskClose都为true时才绑定
    if (this.config.mask && this.config.maskClose) {
      // 使用箭头函数确保this指向正确
      this.maskClickHandler = () => {
        // 只有弹窗处于打开状态时才执行关闭
        if (this.state.isOpen) {
          this.close();
        }
      };
      // 绑定事件（使用addEventListener确保可移除）
      this.mask.addEventListener('click', this.maskClickHandler);
    }
  }

  /**
   * 渲染表单内容（type=form时）
   */
  renderFormContent() {
    this.content.innerHTML = '';
    const form = document.createElement('form');
    form.className = 'space-y-4';
    form.addEventListener('submit', (e) => {
      e.preventDefault();
      this.handleFormSubmit();
    });

    // 渲染表单项
    this.config.formFields.forEach((field, index) => {
      const fieldGroup = document.createElement('div');
      fieldGroup.className = 'space-y-2';

      // 标签
      const label = document.createElement('label');
      label.className = 'block text-sm font-medium text-dark-300 dark:text-light-100';
      label.textContent = field.label;
      const inputId = `${this.config.id}-${field.name || index}`;
      label.htmlFor = inputId;
      if (field.required) label.innerHTML += '<span class="text-danger ml-1">*</span>';
      fieldGroup.appendChild(label);

      // 输入框（支持多种类型）
      let input;
      switch (field.type) {
        case 'textarea':
          input = document.createElement('textarea');
          input.className = 'input-modern';
          input.rows = field.rows || 3;
          input.placeholder = field.placeholder || '';
          break;
        
        case 'select':
          input = document.createElement('select');
          input.className = 'input-modern';
          
          // 添加选项
          if (field.options && field.options.length) {
            field.options.forEach(opt => {
              const option = document.createElement('option');
              option.value = opt.value;
              option.textContent = opt.label;
              option.disabled = opt.disabled || false;
              if (opt.value === this.state.formData[field.name]) option.selected = true;
              input.appendChild(option);
            });
          }
          break;
        
        default: // input类型（text/number/email等）
          input = document.createElement('input');
          input.type = field.type || 'text';
          input.className = 'input-modern';
          input.placeholder = field.placeholder || '';
          break;
      }

      // 基础属性
      input.name = field.name;
      input.id = inputId;
      input.value = this.state.formData[field.name] || '';
      if (field.required) input.required = true;
      if (field.disabled) input.disabled = true;

      // 绑定值变化事件
      input.addEventListener('change', (e) => {
        this.state.formData[field.name] = e.target.value;
        // 返回事件
        if (field.onChange) {
          field.onChange(this, e.target.value);
        }
      });

      fieldGroup.appendChild(input);

      // 提示文本
      if (field.tip) {
        const tip = document.createElement('p');
        tip.className = 'text-xs text-secondary';
        tip.innerHTML = field.tip;
        fieldGroup.appendChild(tip);
      }

      form.appendChild(fieldGroup);
    });

    this.content.appendChild(form);
  }

  /**
   * 渲染底部按钮
   */
  renderButtons() {
    this.footer.innerHTML = '';
    this.config.buttons.forEach((btn, index) => {
      const button = document.createElement('button');
      
      // 按钮样式（根据类型）
      const btnStyles = {
        default: 'soft-button',
        primary: 'primary-button',
        danger: 'danger-button'
      };
      button.className = btnStyles[btn.type] || btnStyles.default;
      button.textContent = btn.text;
      button.type = 'button';

      // 绑定点击事件
      button.addEventListener('click', () => {
        if (typeof btn.callback === 'function') {
          btn.callback(this, this.state.formData);
        } else {
          this.close();
        }
      });

      this.footer.appendChild(button);
    });
  }

  /**
   * 处理表单提交（type=form时）
   */
  handleFormSubmit() {
    if (typeof this.config.formSubmit === 'function') {
      this.config.formSubmit(this, this.state.formData);
    } else {
      this.close();
    }
  }

  /**
   * 打开弹出框
   */
  open() {
    if (this.state.isOpen) return;
    
    this.previousActiveElement = document.activeElement;

    // 显示遮罩（先移除pointer-events-none，再显示）
    if (this.config.mask) {
      this.mask.classList.remove('pointer-events-none');
      setTimeout(() => {
        this.mask.classList.remove('opacity-0');
        this.mask.classList.add('opacity-100');
      }, 10);
    }
    
    // 显示弹窗
    this.modal.classList.remove('pointer-events-none');
    setTimeout(() => {
      this.modal.classList.remove('scale-95', 'opacity-0');
      this.modal.classList.add('scale-100', 'opacity-100');
    }, 10);
    
    // 更新状态
    this.state.isOpen = true;
    PopupModal.openInstances.push(this);
    document.addEventListener('keydown', this.keydownHandler);
    
    // 执行打开回调
    if (typeof this.config.onOpen === 'function') {
      this.config.onOpen(this);
    }

    lockBodyScroll();

    requestAnimationFrame(() => {
      const autofocus = this.modal.querySelector('[autofocus]');
      const formControl = this.modal.querySelector('input:not([disabled]), select:not([disabled]), textarea:not([disabled])');
      const fallback = this.modal.querySelector('button:not([disabled]), [href], [tabindex]:not([tabindex="-1"])');
      (autofocus || formControl || fallback || this.modal).focus({ preventScroll: true });
    });
  }

  handleKeydown(event) {
    if (!this.state.isOpen || PopupModal.openInstances.at(-1) !== this) return;
    if (event.key === 'Escape') {
      this.close();
      return;
    }
    if (event.key !== 'Tab') return;

    const focusable = [...this.modal.querySelectorAll('button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])')]
      .filter(element => element.offsetParent !== null);
    if (focusable.length === 0) {
      event.preventDefault();
      this.modal.focus();
      return;
    }
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  }

  /**
   * 关闭弹出框
   */
  close() {
    if (!this.state.isOpen) return;
    const stackIndex = PopupModal.openInstances.lastIndexOf(this);
    if (stackIndex >= 0) PopupModal.openInstances.splice(stackIndex, 1);
    
    // 隐藏遮罩（先隐藏，再添加pointer-events-none）
    if (this.config.mask) {
      this.mask.classList.remove('opacity-100');
      this.mask.classList.add('opacity-0');
      setTimeout(() => {
        this.mask.classList.add('pointer-events-none');
        this.mask.remove();
      }, 180);
    }
    
    // 隐藏弹窗
    this.modal.classList.remove('scale-100', 'opacity-100');
    this.modal.classList.add('scale-95', 'opacity-0');
    setTimeout(() => {
      this.modal.classList.add('pointer-events-none');
      this.modal.remove();
    }, 220);

    // 更新状态
    this.state.isOpen = false;
    document.removeEventListener('keydown', this.keydownHandler);
    
    // 执行关闭回调
    if (typeof this.config.onClose === 'function') {
      this.config.onClose(this);
    }

    unlockBodyScroll();
    setTimeout(() => {
      this.previousActiveElement?.focus?.({ preventScroll: true });
      this.previousActiveElement = null;
    }, 220);
  }

  /**
   * 更新弹出框内容
   * @param {Object} options - 要更新的配置（title/content/buttons等）
   */
  update(options) {
    if (options.title) {
      this.config.title = options.title;
      this.titleEl.textContent = options.title;
    }

    if (options.content) {
      this.config.content = options.content;
      if (this.config.type !== 'form') {
        this.content.innerHTML = options.content;
      }
    }

    if (options.buttons) {
      this.config.buttons = options.buttons;
      this.renderButtons();
    }

    if (options.formFields && this.config.type === 'form') {
      this.config.formFields = options.formFields;
      this.renderFormContent();
    }
  }

  /**
   * 追加表单字段
   * @param {Array} newFields - 要追加的新字段数组
   * @param {Array} keepFieldNames - 要保留的原有字段名数组
   */
  appendFormFields(newFields = [], keepFieldNames = []) {
    if (this.config.type !== 'form') {
      return;
    }
    
    const keepFields = this.config.formFields.filter(field => {
      return keepFieldNames.includes(field.name);
    });

    newFields.forEach(field => {
      if (this.state.formData[field.name] === undefined || this.state.formData[field.name] === '') {
        this.state.formData[field.name] = field.defaultValue ?? field.value ?? '';
      }
    });

    const filledNewFields = newFields.map(field => ({
      ...field,
      defaultValue: this.state.formData[field.name] ?? field.defaultValue ?? field.value ?? ''
    }));

    const finalFields = [...keepFields, ...filledNewFields];
    this.update({ formFields: finalFields });
  }

  /**
   * 销毁弹出框（从DOM中移除）
   */
  destroy() {
    // 移除遮罩事件，避免内存泄漏
    if (this.maskClickHandler) {
      this.mask.removeEventListener('click', this.maskClickHandler);
    }
    this.close();
    setTimeout(() => {
      if (this.mask && this.mask.parentNode) this.mask.parentNode.removeChild(this.mask);
      if (this.modal && this.modal.parentNode) this.modal.parentNode.removeChild(this.modal);
    }, 220);
  }
}

export default PopupModal;
