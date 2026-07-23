/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { mkdir, readFile, readdir, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const sourceOrigin = 'https://doc.deepkey.top'
const publicRoot = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  '../public'
)
const outputDir = path.resolve(publicRoot, 'docs')
const appSourcePath = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  'goodbyeri-docs-app.js'
)
const relativeOutput = path.relative(publicRoot, outputDir)

const indexCsp = [
  "default-src 'none'",
  "script-src 'self'",
  "style-src 'self'",
  "img-src 'self' https: data:",
  "font-src 'self' https: data:",
  "frame-src 'self'",
  "connect-src 'none'",
  "object-src 'none'",
  "base-uri 'none'",
  "form-action 'none'",
].join('; ')

const articleCsp = [
  "default-src 'none'",
  "script-src 'none'",
  "style-src 'self' 'unsafe-inline'",
  "img-src 'self' https: data:",
  "font-src 'self' https: data:",
  "connect-src 'none'",
  "object-src 'none'",
  "base-uri 'none'",
  "form-action 'none'",
].join('; ')

if (relativeOutput !== 'docs') {
  throw new Error(`Refusing to replace unexpected output path: ${outputDir}`)
}

async function fetchResource(resourcePath) {
  const response = await fetch(new URL(resourcePath, `${sourceOrigin}/`))
  if (!response.ok) {
    throw new Error(
      `Failed to fetch ${response.url}: ${response.status} ${response.statusText}`
    )
  }
  return response
}

function replaceCustomerFacingBrand(content) {
  return content
    .replaceAll('https://doc.deepkey.top', 'https://goodbyeri.cc/docs')
    .replaceAll('http://doc.deepkey.top', 'https://goodbyeri.cc/docs')
    .replaceAll('https://deepkey.top', 'https://goodbyeri.cc')
    .replaceAll('http://deepkey.top', 'https://goodbyeri.cc')
    .replaceAll('doc.deepkey.top', 'goodbyeri.cc/docs')
    .replaceAll('deepkey.top', 'goodbyeri.cc')
    .replaceAll('DeepKey', 'Goodbyeri')
    .replaceAll('deepkey', 'goodbyeri.cc')
    .replaceAll(/[ \t]+$/gm, '')
}

function stripRemoteActiveContent(content) {
  return content
    .replaceAll(/<script\b[^>]*>[\s\S]*?<\/script\s*>/gi, '')
    .replaceAll(/<script\b[^>]*\/?\s*>/gi, '')
    .replaceAll(/<(?:iframe|object)\b[^>]*>[\s\S]*?<\/(?:iframe|object)>/gi, '')
    .replaceAll(/<(?:iframe|object|embed)\b[^>]*\/?\s*>/gi, '')
    .replaceAll(/\s+on[a-z]+\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+)/gi, '')
    .replaceAll(/\s+srcdoc\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+)/gi, '')
    .replaceAll(
      /\s+(href|src|xlink:href|formaction)\s*=\s*(["'])\s*(?:javascript|data:text\/html)[\s\S]*?\2/gi,
      ''
    )
}

function addCsp(content, policy) {
  if (/http-equiv=["']Content-Security-Policy["']/i.test(content)) {
    return content
  }
  const meta = `<meta http-equiv="Content-Security-Policy" content="${policy}">`
  return content.replace(/<head(\s[^>]*)?>/i, (head) => `${head}\n  ${meta}`)
}

function prepareIndexHtml(content) {
  const viewerPattern =
    /<iframe\b(?=[^>]*\bid=["']viewer["'])[^>]*>[\s\S]*?<\/iframe\s*>/i
  const viewer = content.match(viewerPattern)?.[0]
  if (!viewer) {
    throw new Error('Documentation index is missing the viewer iframe')
  }
  const viewerPlaceholder = '<!-- goodbyeri-docs-viewer -->'
  const sanitized = replaceCustomerFacingBrand(
    stripRemoteActiveContent(content.replace(viewerPattern, viewerPlaceholder))
  )
    .replace(viewerPlaceholder, viewer)
    .replace(/<iframe\b([^>]*\bid=["']viewer["'][^>]*)>/i, (tag, attributes) =>
      /\ssandbox\s*=/i.test(tag)
        ? tag
        : `<iframe${attributes} sandbox="allow-popups allow-popups-to-escape-sandbox">`
    )
    .replace(/<\/body>/i, '  <script src="app.js" defer></script>\n</body>')
  return addCsp(sanitized, indexCsp)
}

function prepareArticleHtml(content) {
  const sanitized = replaceCustomerFacingBrand(
    stripRemoteActiveContent(content)
  )
    .replaceAll(/<source\b[^>]*\/?\s*>/gi, '')
    .replaceAll(/((?:src|href)=["'])\/images\//gi, '$1../images/')
  return addCsp(sanitized, articleCsp)
}

if (process.argv.includes('--harden-existing')) {
  const articlesDir = path.join(outputDir, 'articles')
  const entries = await readdir(articlesDir, { withFileTypes: true })
  let hardenedCount = 0
  for (const entry of entries) {
    if (!entry.isFile() || !entry.name.endsWith('.html')) continue
    const articlePath = path.join(articlesDir, entry.name)
    const source = await readFile(articlePath, 'utf8')
    await writeFile(articlePath, prepareArticleHtml(source), 'utf8')
    hardenedCount++
  }
  const indexPath = path.join(outputDir, 'index.html')
  const indexSource = await readFile(indexPath, 'utf8')
  const docsApp = await readFile(appSourcePath, 'utf8')
  await writeFile(indexPath, prepareIndexHtml(indexSource), 'utf8')
  await writeFile(path.join(outputDir, 'app.js'), docsApp, 'utf8')
  console.log(`Hardened ${hardenedCount} existing documentation articles`)
  process.exit(0)
}

const indexSource = await (await fetchResource('/')).text()
const slugs = Array.from(
  indexSource.matchAll(/data-slug="([a-zA-Z0-9_-]+)"/g),
  (match) => match[1]
)
const uniqueSlugs = [...new Set(slugs)]

if (uniqueSlugs.length === 0) {
  throw new Error('No documentation articles were discovered')
}

const articleSources = new Map()
const assetPaths = new Set(['style.css', 'article.css'])

for (const slug of uniqueSlugs) {
  const articlePath = `articles/${slug}.html`
  const articleSource = await (await fetchResource(articlePath)).text()
  const preparedArticle = prepareArticleHtml(articleSource)
  articleSources.set(articlePath, preparedArticle)

  for (const match of preparedArticle.matchAll(
    /(?:src|href)="\.\.\/(images\/[^"]+)"/g
  )) {
    assetPaths.add(match[1])
  }
}

await rm(outputDir, { recursive: true, force: true })
await mkdir(path.join(outputDir, 'articles'), { recursive: true })
await mkdir(path.join(outputDir, 'images'), { recursive: true })

const preparedIndex = prepareIndexHtml(indexSource)
const docsApp = await readFile(appSourcePath, 'utf8')

await writeFile(path.join(outputDir, 'index.html'), preparedIndex, 'utf8')
await writeFile(path.join(outputDir, 'app.js'), docsApp, 'utf8')

for (const [articlePath, source] of articleSources) {
  await writeFile(path.join(outputDir, articlePath), source, 'utf8')
}

for (const assetPath of assetPaths) {
  const response = await fetchResource(assetPath)
  const destination = path.join(outputDir, assetPath)
  await mkdir(path.dirname(destination), { recursive: true })

  if (assetPath.endsWith('.css')) {
    const css = replaceCustomerFacingBrand(await response.text())
    await writeFile(destination, css, 'utf8')
    continue
  }

  await writeFile(destination, Buffer.from(await response.arrayBuffer()))
}

const generatedText = [preparedIndex, ...articleSources.values()].join('\n')

if (/deepkey(?:\.top)?/i.test(generatedText)) {
  throw new Error('Generated customer documentation still contains DeepKey')
}
if (
  /<script\b|\son[a-z]+\s*=|javascript:|data:text\/html/i.test(
    Array.from(articleSources.values()).join('\n')
  )
) {
  throw new Error('Generated customer articles still contain active content')
}

console.log(
  `Synced ${uniqueSlugs.length} articles and ${assetPaths.size} assets to ${outputDir}`
)
