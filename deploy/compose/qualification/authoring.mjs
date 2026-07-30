import { readFile, writeFile } from 'node:fs/promises'
import process from 'node:process'
import { chromium } from 'playwright'

const baseURL = process.env.QUALIFICATION_URL || 'https://localhost'
const credentialsPath = process.env.QUALIFICATION_CREDENTIALS || '/run/secrets/credentials.json'
const evidenceRoot = process.env.QUALIFICATION_EVIDENCE_ROOT || '/evidence'
const projectID = process.env.QUALIFICATION_PROJECT_ID || 'leapview-evaluation'
const workspaceID = process.env.QUALIFICATION_WORKSPACE_ID || 'evaluation'
const screenshotPath = `${evidenceRoot}/authoring-browser-failure.png`
const credentials = JSON.parse(await readFile(credentialsPath, 'utf8'))

if (!credentials.email || !credentials.temporaryPassword || !credentials.qualificationPassword) {
  throw new Error('authoring qualification credentials are incomplete')
}

async function waitForMatch(path, pattern, description) {
  const deadline = Date.now() + 10 * 60_000
  while (Date.now() < deadline) {
    const clientFailure = await readFile(
      `${evidenceRoot}/authoring-client-failed`,
      'utf8',
    ).catch(() => '')
    if (clientFailure) {
      const developmentLog = await readFile(
        `${evidenceRoot}/authoring-dev.log`,
        'utf8',
      ).catch(() => '')
      throw new Error(
        `authoring client exited with status ${clientFailure.trim()} while waiting for ${description}\n${developmentLog}`,
      )
    }
    const contents = await readFile(path, 'utf8').catch(() => '')
    const match = contents.match(pattern)
    if (match) return match
    await new Promise((resolve) => setTimeout(resolve, 250))
  }
  throw new Error(`timed out waiting for ${description}`)
}

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

async function issueAuthoringToken(context, page, privileges) {
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
  return tokens.accessToken
}

const browser = await chromium.launch({ headless: true })
const context = await browser.newContext({ ignoreHTTPSErrors: true })
const page = await context.newPage()
let reviewerContext

try {
  await signIn(
    page,
    credentials.email,
    credentials.temporaryPassword,
    credentials.qualificationPassword,
  )

  const administratorToken = await issueAuthoringToken(
    context,
    page,
    ['MANAGE_GRANTS'],
  )
  const administratorHeaders = {
    Authorization: `Bearer ${administratorToken}`,
  }
  const reviewerEmail = `authoring-reviewer-${Date.now()}@qualification.invalid`
  const reviewer = await requireJSON(
    await context.request.post(new URL('/api/v1/principals', baseURL).href, {
      data: { email: reviewerEmail, displayName: 'Authoring Qualification Reviewer' },
      headers: {
        ...administratorHeaders,
        'Idempotency-Key': `authoring-reviewer-${Date.now()}`,
      },
    }),
    'reviewer creation',
  )
  const grantsURL = new URL(
    `/api/v1/workspaces/${encodeURIComponent(workspaceID)}/grants`,
    baseURL,
  )
  const reviewerPrivileges = [
    'VIEW_ITEM',
    'APPROVE_DEPLOYMENT',
    'ACTIVATE_DEPLOYMENT',
  ]
  const reviewerGrants = [
    { objectType: 'platform', privilege: 'MANAGE_PLATFORM' },
  ]
  for (const grant of reviewerGrants) {
    await requireJSON(
      await context.request.post(grantsURL.href, {
        data: {
          ...grant,
          subjectId: reviewer.principal.id,
          subjectType: 'principal',
        },
        headers: {
          ...administratorHeaders,
          'Idempotency-Key': `authoring-reviewer-${grant.objectType}-${grant.privilege.toLowerCase()}-${Date.now()}`,
        },
      }),
      `reviewer ${grant.objectType} ${grant.privilege} grant`,
    )
  }
  reviewerContext = await browser.newContext({ ignoreHTTPSErrors: true })
  const reviewerPage = await reviewerContext.newPage()
  await signIn(
    reviewerPage,
    reviewerEmail,
    reviewer.temporaryPassword,
    `${credentials.qualificationPassword}-reviewer`,
  )
  const reviewerToken = await issueAuthoringToken(
    reviewerContext,
    reviewerPage,
    reviewerPrivileges,
  )
  const reviewerHeaders = {
    Authorization: `Bearer ${reviewerToken}`,
  }

  const login = await waitForMatch(
    `${evidenceRoot}/authoring-login.log`,
    /Open (\S+) and enter code (\S+)/,
    'the CLI device challenge',
  )
  const deviceURL = new URL(login[1], baseURL)
  deviceURL.searchParams.set('user_code', login[2])
  await page.goto(deviceURL.href, { waitUntil: 'domcontentloaded', timeout: 60_000 })
  await page.getByRole('heading', { name: 'Authorize LeapView CLI' }).waitFor()
  await page.getByLabel('Device code').fill(login[2])
  await page.getByRole('button', { name: 'Authorize', exact: true }).click({ force: true })
  await page.getByRole('heading', { name: 'CLI authorized' }).waitFor({ timeout: 30_000 })

  const development = await waitForMatch(
    `${evidenceRoot}/authoring-dev.log`,
    /preview (\S+\/candidates\/([A-Za-z0-9_-]+))/,
    'the private candidate URL',
  )
  const previewURL = new URL(development[1], baseURL)
  if (!previewURL.pathname.startsWith('/candidates/')) {
    throw new Error(`CLI returned a non-candidate preview URL: ${previewURL.href}`)
  }
  await page.goto(previewURL.href, { waitUntil: 'domcontentloaded', timeout: 60_000 })
  await page.waitForURL(
    (url) => url.pathname.startsWith(`${previewURL.pathname}/workspaces/`),
    { timeout: 60_000 },
  )

  const dashboardURL = new URL(
    `${previewURL.pathname}/workspaces/evaluation/dashboards/sales-overview`,
    baseURL,
  )
  await page.goto(dashboardURL.href, { waitUntil: 'domcontentloaded', timeout: 60_000 })
  await page.getByText('Governed order rows', { exact: true }).waitFor({ state: 'visible', timeout: 60_000 })
  await page.getByText('24', { exact: true }).first().waitFor({ state: 'visible', timeout: 30_000 })

  await writeFile(
    `${evidenceRoot}/authoring-preview-verified`,
    `${JSON.stringify({ candidate: development[2], previewURL: previewURL.href })}\n`,
    { mode: 0o600 },
  )

  const publication = await waitForMatch(
    `${evidenceRoot}/authoring-publish.log`,
    /publish request (\S+) pending approval/,
    'the protected publication request',
  )
  const deploymentID = publication[1]
  const deploymentURL = new URL(
    `/api/v1/projects/${encodeURIComponent(projectID)}/deployments/${encodeURIComponent(deploymentID)}`,
    baseURL,
  )
  let deployment = await requireJSON(
    await reviewerContext.request.get(deploymentURL.href, {
      headers: reviewerHeaders,
    }),
    'publication lookup',
  )
  if (deployment.approval?.status !== 'pending') {
    throw new Error(`publication approval status is ${deployment.approval?.status}`)
  }
  const approvalURL = new URL(
    `${deploymentURL.pathname}/approval-requests/${encodeURIComponent(deployment.approval.id)}/approve`,
    baseURL,
  )
  const approval = await requireJSON(
    await reviewerContext.request.post(approvalURL.href, {
      data: { expectedRevision: deployment.approval.revision },
      headers: {
        ...reviewerHeaders,
        'Idempotency-Key': `authoring-approve-${deploymentID}`,
      },
    }),
    'publication approval',
  )
  if (approval.status !== 'approved') {
    throw new Error(`publication approval transitioned to ${approval.status}`)
  }
  await requireJSON(
    await reviewerContext.request.post(`${deploymentURL.href}/activate`, {
      headers: {
        ...reviewerHeaders,
        'Idempotency-Key': `authoring-activate-${deploymentID}`,
      },
    }),
    'publication activation',
  )
  const activationDeadline = Date.now() + 5 * 60_000
  while (Date.now() < activationDeadline) {
    deployment = await requireJSON(
      await reviewerContext.request.get(deploymentURL.href, {
        headers: reviewerHeaders,
      }),
      'publication activation lookup',
    )
    if (deployment.status === 'active') break
    if (['cancelled', 'failed', 'superseded'].includes(deployment.status)) {
      throw new Error(`publication activation ended in ${deployment.status}: ${deployment.error || ''}`)
    }
    await new Promise((resolve) => setTimeout(resolve, 250))
  }
  if (deployment.status !== 'active') {
    throw new Error('timed out waiting for protected publication activation')
  }
  await writeFile(
    `${evidenceRoot}/authoring-publish-verified`,
    `${JSON.stringify({
      artifactDigest: deployment.evidence.artifactDigest,
      candidate: deployment.evidence.candidateId,
      createdBy: deployment.createdBy,
      releaseDigest: deployment.evidence.releaseDigest,
      revision: deployment.evidence.candidateRevision,
      status: deployment.status,
      target: deployment.evidence.targetId,
    })}\n`,
    { mode: 0o600 },
  )
} catch (error) {
  await writeFile(
    `${evidenceRoot}/authoring-browser-failure.json`,
    `${JSON.stringify({
      error: String(error),
      title: await page.title().catch(() => ''),
      url: page.url(),
    })}\n`,
    { mode: 0o600 },
  ).catch(() => {})
  await page.screenshot({ path: screenshotPath }).catch(() => {})
  throw error
} finally {
  await reviewerContext?.close()
  await context.close()
  await browser.close()
}
