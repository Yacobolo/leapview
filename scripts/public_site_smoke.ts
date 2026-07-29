import { isDeepStrictEqual } from 'node:util'

export interface PublicReleaseArtifact {
  os: string
  architecture: string
  archiveUrl: string
  checksumUrl: string
}

export interface PublicReleaseManifest {
  schemaVersion: number
  version: string
  tag: string
  revision: string
  image: string
  releaseUrl: string
  artifacts: PublicReleaseArtifact[]
}

export interface PublicSiteSmokeOptions {
  baseURL: string
  expectedRelease: PublicReleaseManifest
  aliases?: string[]
  allowHTTP?: boolean
  verifyArtifacts?: boolean
  fetch?: typeof fetch
}

function normalizedOrigin(raw: string): string {
  const url = new URL(raw)
  if (url.pathname !== '/' || url.search !== '' || url.hash !== '') {
    throw new Error(`public site origin must not contain a path, query, or fragment: ${raw}`)
  }
  return url.origin
}

async function successfulResponse(fetcher: typeof fetch, url: string, init?: RequestInit): Promise<Response> {
  let response: Response
  try {
    response = await fetcher(url, { redirect: 'follow', ...init })
  } catch (error) {
    throw new Error(`request failed for ${url}: ${error instanceof Error ? error.message : String(error)}`)
  }
  if (!response.ok) {
    await response.body?.cancel()
    throw new Error(`request failed for ${url}: HTTP ${response.status}`)
  }
  return response
}

export async function verifyPublicSite(options: PublicSiteSmokeOptions): Promise<void> {
  const fetcher = options.fetch ?? fetch
  const baseURL = normalizedOrigin(options.baseURL)
  if (!options.allowHTTP && new URL(baseURL).protocol !== 'https:') {
    throw new Error(`public site must use HTTPS: ${baseURL}`)
  }

  for (const alias of options.aliases ?? []) {
    const response = await successfulResponse(fetcher, alias)
    const finalURL = new URL(response.url)
    await response.body?.cancel()
    if (finalURL.origin !== baseURL || finalURL.pathname !== '/') {
      throw new Error(`public alias ${alias} resolved to ${response.url}, want ${baseURL}/`)
    }
  }

  for (const path of ['/healthz', '/readyz']) {
    const response = await successfulResponse(fetcher, baseURL + path)
    const body = (await response.text()).trim()
    if (body !== 'ok') {
      throw new Error(`${path} returned ${JSON.stringify(body)}, want "ok"`)
    }
  }

  const manifestResponse = await successfulResponse(fetcher, baseURL + '/release.json')
  const deployedRelease = (await manifestResponse.json()) as PublicReleaseManifest
  if (!isDeepStrictEqual(deployedRelease, options.expectedRelease)) {
    throw new Error('deployed /release.json does not match docs/public-release.json')
  }

  const installationResponse = await successfulResponse(fetcher, baseURL + '/docs/installation')
  const installation = await installationResponse.text()
  const requiredValues = [
    options.expectedRelease.version,
    options.expectedRelease.tag,
    options.expectedRelease.revision,
    options.expectedRelease.image,
    options.expectedRelease.releaseUrl,
    ...options.expectedRelease.artifacts.flatMap((artifact) => [artifact.archiveUrl, artifact.checksumUrl]),
  ]
  for (const value of requiredValues) {
    if (!installation.includes(value)) {
      throw new Error(`installation page does not contain ${value}`)
    }
  }

  if (options.verifyArtifacts !== false) {
    for (const artifact of options.expectedRelease.artifacts) {
      for (const url of [artifact.archiveUrl, artifact.checksumUrl]) {
        const response = await successfulResponse(fetcher, url, { headers: { Range: 'bytes=0-0' } })
        await response.body?.cancel()
      }
    }
  }
}

async function main(): Promise<void> {
  const manifestPath = process.env.LEAPVIEW_PUBLIC_RELEASE_MANIFEST ?? 'docs/public-release.json'
  const expectedRelease = (await Bun.file(manifestPath).json()) as PublicReleaseManifest
  const baseURL = process.env.LEAPVIEW_PUBLIC_SITE_URL ?? 'https://leapview.dev'
  const aliases = (process.env.LEAPVIEW_PUBLIC_SITE_ALIASES ?? 'http://leapview.dev,https://www.leapview.dev')
    .split(',')
    .map((value) => value.trim())
    .filter(Boolean)
  await verifyPublicSite({ baseURL, expectedRelease, aliases })
  console.log(`public adoption smoke passed for ${baseURL} and ${expectedRelease.image}`)
}

if (import.meta.main) {
  await main()
}
