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
const existingCspPattern =
  /<meta\b(?=[^>]*\bhttp-equiv\s*=\s*(?:"Content-Security-Policy"|'Content-Security-Policy'|Content-Security-Policy(?=\s|\/?>)))[^>]*>\s*/gi

export function replaceCustomerFacingBrand(content) {
  return content
    .replaceAll(/https?:\/\/goodbyeri\.cc/gi, 'https://meimeiapi.com')
    .replaceAll(/goodbyeri\.cc/gi, 'meimeiapi.com')
    .replaceAll('https://doc.deepkey.top', 'https://meimeiapi.com/docs')
    .replaceAll('http://doc.deepkey.top', 'https://meimeiapi.com/docs')
    .replaceAll('https://deepkey.top', 'https://meimeiapi.com')
    .replaceAll('http://deepkey.top', 'https://meimeiapi.com')
    .replaceAll('doc.deepkey.top', 'meimeiapi.com/docs')
    .replaceAll('deepkey.top', 'meimeiapi.com')
    .replaceAll('DeepKey', '莓莓 API')
    .replaceAll('deepkey', 'meimeiapi.com')
    .replaceAll(/[ \t]+$/gm, '')
}

export function stripRemoteActiveContent(content) {
  return content
    .replaceAll(/<script\b[^>]*>[\s\S]*?<\/script\s*>/gi, '')
    .replaceAll(/<script\b[^>]*\/?\s*>/gi, '')
    .replaceAll(/<(?:iframe|object)\b[^>]*>[\s\S]*?<\/(?:iframe|object)>/gi, '')
    .replaceAll(/<(?:iframe|object|embed)\b[^>]*\/?\s*>/gi, '')
    .replaceAll(
      /<meta\b(?=[^>]*\bhttp-equiv\s*=\s*(?:"refresh"|'refresh'|refresh(?=\s|\/?>)))[^>]*>\s*/gi,
      ''
    )
    .replaceAll(/\s+on[a-z]+\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+)/gi, '')
    .replaceAll(/\s+srcdoc\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+)/gi, '')
    .replaceAll(
      /\s+(href|src|xlink:href|formaction)\s*=\s*(["'])\s*(?:javascript|data:text\/html)[\s\S]*?\2/gi,
      ''
    )
}

export function addCsp(content, policy) {
  const withoutExistingCsp = content.replaceAll(existingCspPattern, '')
  if (!/<head(?:\s[^>]*)?>/i.test(withoutExistingCsp)) {
    throw new Error('Documentation page is missing the head element')
  }

  const meta = `<meta http-equiv="Content-Security-Policy" content="${policy}">`
  return withoutExistingCsp.replace(
    /<head(\s[^>]*)?>/i,
    (head) => `${head}\n  ${meta}`
  )
}

export function prepareIndexHtml(content, policy, version = '') {
  const viewerPattern =
    /<iframe\b(?=[^>]*\bid=["']viewer["'])[^>]*>[\s\S]*?<\/iframe\s*>/i
  if (!viewerPattern.test(content)) {
    throw new Error('Documentation index is missing the viewer iframe')
  }

  const viewerPlaceholder = '<!-- meimeiapi-docs-viewer -->'
  const versionQuery = version ? `?v=${version}` : ''
  const safeViewer = `<iframe id="viewer" title="文档内容" src="articles/guide-intro.html${versionQuery}" sandbox="allow-popups allow-popups-to-escape-sandbox"></iframe>`
  const sanitized = replaceCustomerFacingBrand(
    stripRemoteActiveContent(content.replace(viewerPattern, viewerPlaceholder))
  ).replace(viewerPlaceholder, safeViewer)

  if (!/<\/body\s*>/i.test(sanitized)) {
    throw new Error('Documentation index is missing the body closing tag')
  }

  const versioned = version
    ? sanitized.replaceAll(
        /((?:href|src)=["'](?:style\.css|app\.js))(["'])/g,
        `$1${versionQuery}$2`
      )
    : sanitized

  return addCsp(
    versioned.replace(
      /<\/body\s*>/i,
      `  <script src="app.js${versionQuery}" defer></script>\n</body>`
    ),
    policy
  )
}

export function prepareArticleHtml(content, policy, version = '') {
  const versionQuery = version ? `?v=${version}` : ''
  const sanitized = replaceCustomerFacingBrand(
    stripRemoteActiveContent(content)
  )
    .replaceAll(/<source\b[^>]*\/?\s*>/gi, '')
    .replaceAll(/((?:src|href)=["'])\/images\//gi, '$1../images/')
  const versioned = version
    ? sanitized.replaceAll(
        /((?:src|href)=["'])(\.\.\/(?:article\.css|images\/[^"'?]+))(["'])/g,
        `$1$2${versionQuery}$3`
      )
    : sanitized
  return addCsp(versioned, policy)
}
