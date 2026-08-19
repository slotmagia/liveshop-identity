import type { RemoteModuleContext } from '@liveshops/host-sdk'
import { create, notify } from '@liveshops/design-tokens'

import { IdentityShopApi, type LoginOTP, type SMSRegion } from '../api'

const FALLBACK_REGION: SMSRegion = { dialCode: '+86', name: '中国大陆', iso2: 'CN', emoji: '🇨🇳' }

export async function renderLogin(container: HTMLElement, context: RemoteModuleContext): Promise<void> {
  const api = new IdentityShopApi(context.api)
  const shopCode = resolveShopCode(context)
  const root = create('main', 'identity-login')
  const card = create('section', 'identity-login__card')
  card.append(create('h1', 'identity-login__welcome', 'Welcome'))
  card.append(create('p', 'identity-login__shop', 'WOKFOY SHOP'))

  let mode: 'phone' | 'email' = 'phone'
  let regions: SMSRegion[] = [FALLBACK_REGION]
  let regionIndex = 0
  let regionOpen = false
  let challengeId = ''
  let verified = false
  let countdown = 0
  let countdownTimer = 0

  const modes = create('div', 'identity-login__modes')
  const phoneMode = modeButton('手机')
  const emailMode = modeButton('邮箱')
  modes.append(phoneMode, emailMode)

  const phonePill = create('div', 'identity-login__pill')
  const regionTrigger = document.createElement('button')
  regionTrigger.type = 'button'
  regionTrigger.className = 'identity-login__region'
  const regionMenu = create('div', 'identity-login__region-menu')
  regionMenu.hidden = true
  const phoneInput = document.createElement('input')
  phoneInput.type = 'tel'
  phoneInput.name = 'phone'
  phoneInput.inputMode = 'numeric'
  phoneInput.autocomplete = 'tel'
  phoneInput.placeholder = 'mobile'
  phonePill.append(regionTrigger, regionMenu, phoneInput)

  const emailPill = create('div', 'identity-login__pill')
  const emailInput = document.createElement('input')
  emailInput.type = 'email'
  emailInput.name = 'email'
  emailInput.autocomplete = 'email'
  emailInput.placeholder = 'email'
  emailPill.append(emailInput)

  const codePill = create('div', 'identity-login__pill')
  const codeInput = document.createElement('input')
  codeInput.type = 'text'
  codeInput.name = 'otp-code'
  codeInput.inputMode = 'numeric'
  codeInput.autocomplete = 'one-time-code'
  codeInput.maxLength = 6
  codeInput.placeholder = '验证码'
  const send = document.createElement('button')
  send.type = 'button'
  send.className = 'identity-login__getcode'
  send.textContent = '获取验证码'
  codePill.append(codeInput, send)

  const submit = create('button', 'identity-login__signin', '登录')
  submit.type = 'button'
  const back = create('button', 'identity-login__link', '返回个人中心')
  back.type = 'button'

  const syncMode = (): void => {
    phoneMode.classList.toggle('is-active', mode === 'phone')
    emailMode.classList.toggle('is-active', mode === 'email')
    phonePill.hidden = mode !== 'phone'
    emailPill.hidden = mode !== 'email'
    closeRegion()
  }

  const syncRegion = (): void => {
    const active = regions[regionIndex] ?? regions[0] ?? FALLBACK_REGION
    regionTrigger.replaceChildren()
    regionTrigger.append(
      create('span', undefined, active.emoji || ''),
      create('span', undefined, active.iso2 || ''),
      create('span', undefined, active.dialCode),
      create('span', 'identity-login__caret', '▾'),
    )
    regionMenu.replaceChildren()
    regions.forEach((item, index) => {
      const option = document.createElement('button')
      option.type = 'button'
      option.append(
        create('span', undefined, item.emoji || ''),
        create('span', 'identity-login__region-name', item.name || item.iso2 || item.dialCode),
        create('span', undefined, item.dialCode),
      )
      option.addEventListener('click', () => {
        regionIndex = index
        closeRegion()
        syncRegion()
      })
      regionMenu.append(option)
    })
  }

  const closeRegion = (): void => {
    regionOpen = false
    regionMenu.hidden = true
  }

  const stopCountdown = (): void => {
    if (countdownTimer) window.clearInterval(countdownTimer)
    countdownTimer = 0
    countdown = 0
    send.disabled = false
    send.textContent = '获取验证码'
  }

  const resetChallenge = (): void => {
    challengeId = ''
    verified = false
    codeInput.value = ''
    stopCountdown()
  }

  const startCountdown = (seconds: number): void => {
    stopCountdown()
    countdown = Math.max(0, Math.ceil(seconds))
    if (countdown <= 0) return
    send.disabled = true
    send.textContent = `${countdown}s`
    countdownTimer = window.setInterval(() => {
      if (!send.isConnected) {
        stopCountdown()
        return
      }
      countdown -= 1
      if (countdown <= 0) {
        stopCountdown()
        return
      }
      send.textContent = `${countdown}s`
    }, 1000)
  }

  phoneMode.addEventListener('click', () => {
    if (mode === 'phone') return
    mode = 'phone'
    resetChallenge()
    syncMode()
  })
  emailMode.addEventListener('click', () => {
    if (mode === 'email') return
    mode = 'email'
    resetChallenge()
    syncMode()
  })
  regionTrigger.addEventListener('click', () => {
    regionOpen = !regionOpen
    regionMenu.hidden = !regionOpen
  })
  root.addEventListener('click', event => {
    if (!phonePill.contains(event.target as Node)) closeRegion()
  })
  codeInput.addEventListener('input', () => {
    codeInput.value = codeInput.value.replace(/\D/g, '').slice(0, 6)
  })
  codeInput.addEventListener('keydown', event => {
    if (event.key === 'Enter') submit.click()
  })
  send.addEventListener('click', () => {
    if (!shopCode) {
      notify('缺少店铺编码', 'warning')
      return
    }
    const channel: 'SMS' | 'EMAIL' = mode === 'phone' ? 'SMS' : 'EMAIL'
    const phone = mode === 'phone' ? fullPhone(regions[regionIndex]?.dialCode || '+86', phoneInput.value) : ''
    const email = mode === 'email' ? emailInput.value.trim().toLowerCase() : ''
    if (channel === 'SMS' && !/^\+?[0-9]{8,20}$/.test(phone)) {
      notify('请输入有效手机号', 'warning')
      return
    }
    if (channel === 'EMAIL' && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      notify('请输入有效邮箱', 'warning')
      return
    }
    send.disabled = true
    const payload = channel === 'SMS'
      ? { shopCode, channel, phone }
      : { shopCode, channel, email }
    api.createLoginOTP(payload).then(data => {
      challengeId = data.challengeId
      verified = false
      startCountdown(resendWaitSeconds(data))
      notify('验证码已发送', 'success')
    }).catch(error => {
      const remaining = remainingFromCooldownError(error)
      if (remaining > 0) {
        startCountdown(remaining)
        notify(`请 ${remaining} 秒后再发送`, 'warning')
        return
      }
      send.disabled = false
      notify(error instanceof Error ? error.message : String(error), 'danger')
    })
  })
  submit.addEventListener('click', () => {
    if (!shopCode) {
      notify('缺少店铺编码', 'warning')
      return
    }
    if (!challengeId) {
      notify('请先获取验证码', 'warning')
      return
    }
    if (!/^[0-9]{6}$/.test(codeInput.value.trim())) {
      notify('请输入6位验证码', 'warning')
      return
    }
    if (!context.login) {
      notify('当前 Host 不支持会话升级', 'danger')
      return
    }
    submit.disabled = true
    const redeem = async (): Promise<void> => {
      context.navigate('/profile')
      await context.login!({ challengeId })
    }
    const verify = verified
      ? Promise.resolve()
      : api.verifyLogin({ shopCode, challengeId, code: codeInput.value.trim() }).then(() => { verified = true })
    verify.then(redeem).catch(error => {
      notify(error instanceof Error ? error.message : String(error), 'danger')
    }).finally(() => { submit.disabled = false })
  })
  back.addEventListener('click', () => context.navigate('/profile'))

  card.append(modes, phonePill, emailPill, codePill, submit, back)
  root.append(card)
  container.replaceChildren(root)
  syncMode()
  syncRegion()
  if (shopCode) {
    api.loginSMSRegions(shopCode).then(result => {
      const items = result.items ?? []
      if (!items.length) return
      regions = items
      regionIndex = 0
      syncRegion()
    }).catch(() => undefined)
  }
}

function resolveShopCode(context: RemoteModuleContext): string {
  return context.shopCode?.trim() || new URLSearchParams(window.location.search).get('shopCode')?.trim() || ''
}

function fullPhone(dial: string, local: string): string {
  const digits = local.trim()
  if (!digits) return ''
  if (digits.startsWith('+')) return digits
  const prefix = dial.trim()
  return prefix ? `${prefix}${digits.replace(/^0+/, '')}` : digits
}

function modeButton(label: string): HTMLButtonElement {
  const button = document.createElement('button')
  button.type = 'button'
  button.textContent = label
  return button
}

function resendWaitSeconds(data: LoginOTP): number {
  const fromDeadline = remainingUntil(data.nextSendAt)
  if (fromDeadline > 0) return fromDeadline
  const fromField = Number(data.resendAfterSeconds)
  if (Number.isFinite(fromField) && fromField > 0) return Math.ceil(fromField)
  return 60
}

function remainingFromCooldownError(error: unknown): number {
  const message = error instanceof Error ? error.message : String(error)
  const match = /login otp resend cooldown(?:: (\d+))?/.exec(message)
  if (!match) return 0
  const remaining = Number(match[1])
  return Number.isFinite(remaining) && remaining > 0 ? remaining : 0
}

function remainingUntil(deadline: string): number {
  const remaining = Math.ceil((Date.parse(deadline) - Date.now()) / 1000)
  return Number.isFinite(remaining) && remaining > 0 ? remaining : 0
}
