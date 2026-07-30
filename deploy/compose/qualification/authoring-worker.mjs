import { writeFile } from 'node:fs/promises'
import process from 'node:process'
import readline from 'node:readline'
import { chromium } from 'playwright'

const baseURL = process.env.QUALIFICATION_URL || 'https://localhost'
const evidenceRoot = process.env.QUALIFICATION_EVIDENCE_ROOT || '/evidence'
const projectID = process.env.QUALIFICATION_PROJECT_ID || 'leapview-evaluation'
const screenshotPath = `${evidenceRoot}/authoring-browser-failure.png`

async function requireJSON(response, description) {
  if (!response.ok()) {
    throw new Error(`${description} returned ${response.status()}: ${await response.text()}`)
  }
  return response.json()
}

async function signIn(page, email, temporaryPassword, password) {
  await page.goto(baseURL, { waitUntil: 'domcontentloaded', timeout: 60_000 })
  await page.getByLabel('Email').fill(email)
  await page.getByLabel('Password').fill(temporaryPassword)
  await page.getByLabel('Password').press('Enter')
  await page.getByLabel('Temporary password').waitFor({ state: 'visible', timeout: 30_000 })
  await page.getByLabel('Temporary password').fill(temporaryPassword)
  await page.getByLabel('New password').fill(password)
  await page.getByLabel('New password').press('Enter')
  await page.waitForURL((url) => !url.pathname.startsWith('/login'), { timeout: 30_000 })
}

async function issueToken(context, page, privileges) {
  const challenge = await requireJSON(
    await context.request.post(
      new URL('/api/v1/access/device-authorizations', baseURL).href,
      { data: { scope: { projectId: projectID, privileges } } },
    ),
    `device authorization for ${privileges.join(', ')}`,
  )
  const deviceURL = new URL(challenge.verificationUriComplete, baseURL)
  await page.goto(deviceURL.href, { waitUntil: 'domcontentloaded', timeout: 60_000 })
  await page.getByRole('heading', { name: 'Authorize LeapView CLI' }).waitFor()
  await page.getByLabel('Device code').fill(challenge.userCode)
  await page.getByRole('button', { name: 'Authorize', exact: true }).click({ force: true })
  await page.getByRole('heading', { name: 'CLI authorized' }).waitFor({ timeout: 30_000 })
  const tokens = await requireJSON(
    await context.request.post(
      new URL('/api/v1/access/device-authorizations/token', baseURL).href,
      { data: { deviceCode: challenge.deviceCode } },
    ),
    `device token exchange for ${privileges.join(', ')}`,
  )
  return { accessToken: tokens.accessToken }
}

const browser = await chromium.launch({ headless: true })
const administratorContext = await browser.newContext({ ignoreHTTPSErrors: true })
const administratorPage = await administratorContext.newPage()
let reviewerContext
let reviewerPage

const methods = {
  async signInAdministrator(params) {
    await signIn(
      administratorPage,
      params.email,
      params.temporaryPassword,
      params.password,
    )
    return { authenticated: true }
  },

  async issueAdministratorToken(params) {
    return issueToken(
      administratorContext,
      administratorPage,
      params.privileges,
    )
  },

  async signInReviewer(params) {
    reviewerContext ??= await browser.newContext({ ignoreHTTPSErrors: true })
    reviewerPage ??= await reviewerContext.newPage()
    await signIn(
      reviewerPage,
      params.email,
      params.temporaryPassword,
      params.password,
    )
    return { authenticated: true }
  },

  async issueReviewerToken(params) {
    if (!reviewerContext || !reviewerPage) {
      throw new Error('reviewer must sign in before requesting a token')
    }
    return issueToken(reviewerContext, reviewerPage, params.privileges)
  },

  async authorizeCLI(params) {
    const deviceURL = new URL(params.verificationUrl, baseURL)
    deviceURL.searchParams.set('user_code', params.userCode)
    await administratorPage.goto(
      deviceURL.href,
      { waitUntil: 'domcontentloaded', timeout: 60_000 },
    )
    await administratorPage.getByRole('heading', { name: 'Authorize LeapView CLI' }).waitFor()
    await administratorPage.getByLabel('Device code').fill(params.userCode)
    await administratorPage.getByRole('button', { name: 'Authorize', exact: true }).click({ force: true })
    await administratorPage.getByRole('heading', { name: 'CLI authorized' }).waitFor({ timeout: 30_000 })
    return { authorized: true }
  },

  async verifyPreview(params) {
    const previewURL = new URL(params.previewUrl, baseURL)
    if (!previewURL.pathname.startsWith('/candidates/')) {
      throw new Error(`CLI returned a non-candidate preview URL: ${previewURL.href}`)
    }
    await administratorPage.goto(
      previewURL.href,
      { waitUntil: 'domcontentloaded', timeout: 60_000 },
    )
    await administratorPage.waitForURL(
      (url) => url.pathname.startsWith(`${previewURL.pathname}/workspaces/`),
      { timeout: 60_000 },
    )
    const dashboardURL = new URL(
      `${previewURL.pathname}/workspaces/evaluation/dashboards/sales-overview`,
      baseURL,
    )
    await administratorPage.goto(
      dashboardURL.href,
      { waitUntil: 'domcontentloaded', timeout: 60_000 },
    )
    await administratorPage
      .getByText('Governed order rows', { exact: true })
      .waitFor({ state: 'visible', timeout: 60_000 })
    await administratorPage
      .getByText('24', { exact: true })
      .first()
      .waitFor({ state: 'visible', timeout: 30_000 })
    return {
      candidateId: params.candidateId,
      governedOrderRows: 24,
      previewUrl: previewURL.href,
    }
  },

  async close() {
    return { closed: true }
  },
}

const lines = readline.createInterface({
  input: process.stdin,
  crlfDelay: Infinity,
  terminal: false,
})

try {
  for await (const line of lines) {
    if (!line.trim()) continue
    let request
    try {
      request = JSON.parse(line)
      const method = methods[request.method]
      if (!method) {
        throw new Error(`unsupported browser worker method ${request.method}`)
      }
      const result = await method(request.params || {})
      process.stdout.write(`${JSON.stringify({ id: request.id, result })}\n`)
      if (request.method === 'close') break
    } catch (error) {
      await writeFile(
        `${evidenceRoot}/authoring-browser-failure.json`,
        `${JSON.stringify({
          error: String(error),
          method: request?.method || '',
          title: await administratorPage.title().catch(() => ''),
          url: administratorPage.url(),
        })}\n`,
        { mode: 0o600 },
      ).catch(() => {})
      await administratorPage.screenshot({ path: screenshotPath }).catch(() => {})
      process.stdout.write(`${JSON.stringify({
        id: request?.id || 0,
        error: String(error),
      })}\n`)
    }
  }
} finally {
  await reviewerContext?.close()
  await administratorContext.close()
  await browser.close()
}
