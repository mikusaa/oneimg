import { test, expect, type BrowserContext } from '@playwright/test'

let cookies: Awaited<ReturnType<BrowserContext['cookies']>>
let user: Record<string, unknown>
test.beforeAll(async ({ request }) => {
  const response = await request.post('/api/v1/auth/login', { data: { username: 'browser-test', password: 'browser-test-password' } })
  expect(response.ok()).toBeTruthy()
  user = (await response.json()).data
  cookies = (await request.storageState()).cookies
})
test.beforeEach(async ({ context, page }) => {
  await context.addCookies(cookies)
  await page.addInitScript(data => {
    localStorage.setItem('userInfo', JSON.stringify(data))
    Object.assign(window, { previewInjected: 0, svgInjected: 0 })
  }, user)
})

for (const path of ['/', '/gallery']) {
  test(`historical metadata stays data on ${path}`, async ({ page }) => {
    await page.goto(path)
    const target = page.locator('img[src*="legacy%22"]').first()
    await expect(target).toBeVisible()
    await target.click()
    const dialog = page.getByRole('dialog')
    await expect(dialog).toBeVisible()
    await expect(dialog).toContainText('photo"><img src=x onerror="window.previewInjected=1">.png')
    await expect(dialog).toContainText('tag"><img src=x onerror="window.previewInjected=1">')
    expect(await dialog.locator('script, iframe, [onclick], [onerror], [onload]').count()).toBe(0)
    expect(await page.evaluate(() => (window as unknown as { previewInjected: number }).previewInjected)).toBe(0)
    const download = dialog.getByRole('link', { name: '下载' })
    expect(await download.getAttribute('download')).not.toMatch(/["'<>?#/\\]/)
    const [downloaded] = await Promise.all([page.waitForEvent('download'), download.click()])
    expect(await downloaded.failure()).toBeNull()
    await dialog.getByRole('button', { name: '复制 URL', exact: true }).click()
    await dialog.getByRole('button', { name: '关闭', exact: true }).last().click()
    await expect(dialog).not.toBeVisible()
    await target.click()
    await dialog.getByRole('button', { name: '放大原图' }).click()
    await expect(page.locator('#spotlight.show')).toBeVisible()
    await page.keyboard.press('Escape')
    await expect(page.locator('#spotlight.show')).not.toBeVisible()
  })
}

test('mobile preview preserves actions without injected nodes', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/gallery')
  await page.locator('img[src*="legacy%22"]').first().click()
  const dialog = page.getByRole('dialog')
  await expect(dialog).toBeVisible()
  const box = await dialog.boundingBox()
  expect(box!.width).toBeLessThanOrEqual(390)
  let removed = false
  await page.route('**/api/v1/images/1/tags/*', async route => {
    if (route.request().method() === 'DELETE') { removed = true; await route.fulfill({ status: 204 }) } else await route.continue()
  })
  await dialog.getByRole('button', { name: /^移除标签/ }).click()
  await expect.poll(() => removed).toBeTruthy()
  let deleted = false
  await page.route('**/api/v1/images/1', async route => {
    if (route.request().method() === 'DELETE') { deleted = true; await route.fulfill({ status: 204 }) } else await route.continue()
  })
  await dialog.getByRole('button', { name: '删除', exact: true }).click()
  await page.getByRole('button', { name: '确认删除', exact: true }).click()
  await expect.poll(() => deleted).toBeTruthy()
  expect(await page.evaluate(() => (window as unknown as { previewInjected: number }).previewInjected)).toBe(0)
})

test('legacy SVG blocks scripts, external resources, embedded pages and forms', async ({ page }) => {
  // Request events also include CSP-blocked attempts. Count only requests received by the server.
  const before = await (await page.request.get('/__security/csp-requests')).json()
  await page.request.get('/svg-control')
  const attempts = await (await page.request.get('/__security/csp-requests')).json()
  expect(attempts).toBe(before + 1)
  const response = await page.goto('/uploads/legacy.svg')
  expect(response?.headers()['content-security-policy']).toContain("sandbox; default-src 'none'")
  expect(response?.headers()['x-content-type-options']).toBe('nosniff')
  await expect(page.locator('svg > rect')).toBeVisible()
  expect(await page.locator('svg > rect').evaluate(node => getComputedStyle(node).fill)).toBe('rgb(255, 0, 0)')
  expect(await page.evaluate(() => (window as unknown as { svgInjected?: number }).svgInjected || 0)).toBe(0)
  await page.locator('button').click({ force: true })
  await page.waitForTimeout(300)
  expect(await (await page.request.get('/__security/csp-requests')).json()).toBe(attempts)
  expect(page.url()).toContain('/uploads/legacy.svg')
})
