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
/**
 * Build-time static page generation for the public, SEO-relevant routes.
 *
 * The app is a client-rendered SPA, so crawlers that do not execute JavaScript
 * (Baidu, GPTBot, OAI-SearchBot, PerplexityBot, ClaudeBot, ...) would only ever
 * see an empty `#root`. For each public route this script clones the built
 * `dist/index.html` and rewrites the per-route `<title>`/description/canonical/
 * Open Graph/Twitter tags, then replaces the `<noscript>` fallback with
 * route-specific content. The SPA itself is untouched: `#root` stays empty, so
 * users with JavaScript still get the normal client-rendered experience.
 *
 * Output: `dist/index.html` (home) plus `dist/<route>.html` for the rest. The
 * server serves `dist/<path>.html` for the matching path when it exists.
 */
import { readFileSync, writeFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = dirname(fileURLToPath(import.meta.url))
const distDir = resolve(__dirname, '..', 'dist')
const site = 'https://io.iioooo.com'

const template = readFileSync(join(distDir, 'index.html'), 'utf8')

// Read the home values from the build output instead of duplicating them, so
// this script keeps working when the app constants change.
const homeTitle = template.match(/<title>([^<]*)<\/title>/)?.[1] ?? ''
const homeDescription =
  template.match(/name="description"\s+content="([^"]*)"/)?.[1] ?? ''

if (!homeTitle || !homeDescription) {
  throw new Error(
    '[prerender] could not read the home title/description from dist/index.html'
  )
}

const pages = [
  {
    path: '/',
    title: homeTitle,
    description: homeDescription,
    content: `
      <h1>${homeTitle}</h1>
      <p>iioooo 提供高速稳定的 AI 大模型 API 调用与低成本接入解决方案，聚合 OpenAI、Claude、Gemini、DeepSeek 等数百款主流大模型，支持对话、图像、向量、语音等多类型接口，一个密钥即可调用全部模型。</p>
      <h2>核心优势</h2>
      <ul>
        <li>速度快：全球节点与智能路由，低延迟、高并发、稳定可靠。</li>
        <li>折扣大：按量计费，价格优惠，用多少付多少。</li>
        <li>模型多：覆盖 OpenAI、Claude、Gemini、DeepSeek 等数百款主流大模型。</li>
        <li>兼容好：兼容 OpenAI 接口，一个密钥调用全部模型。</li>
      </ul>
      <h2>支持的主要模型</h2>
      <p>OpenAI（GPT 系列）、Anthropic Claude、Google Gemini、DeepSeek、通义千问、文心一言、智谱 GLM、Kimi、豆包等。</p>
      <p><a href="${site}/pricing">模型与价格</a> · <a href="${site}/about">关于我们</a></p>
    `,
  },
  {
    path: '/about',
    title: '关于我们 - iioooo',
    description:
      '了解 iioooo：新一代 AI 大模型 API 中转平台，聚合 OpenAI、Claude、Gemini、DeepSeek 等数百款主流大模型，提供高速稳定、低成本、兼容 OpenAI 接口的一站式大模型 API 接入服务。',
    content: `
      <h1>关于我们 - iioooo</h1>
      <p>iioooo 是新一代 AI 大模型 API 中转平台，聚合 OpenAI、Claude、Gemini、DeepSeek 等数百款主流大模型，为开发者与团队提供高速稳定、低成本、兼容 OpenAI 接口的一站式大模型 API 接入服务。</p>
      <p>我们致力于让调用大模型更简单：统一接口、统一密钥、按量计费，一个 API 即可在多家上游模型之间自由切换。</p>
      <p><a href="${site}/">返回首页</a> · <a href="${site}/pricing">模型与价格</a></p>
    `,
  },
  {
    path: '/pricing',
    title: '模型与价格 - iioooo',
    description:
      'iioooo 模型与价格：OpenAI、Claude、Gemini、DeepSeek 等数百款大模型 API 按量计费、价格透明、折扣力度大，一个密钥即可调用全部模型。',
    content: `
      <h1>模型与价格 - iioooo</h1>
      <p>iioooo 提供 OpenAI、Claude、Gemini、DeepSeek 等数百款主流大模型的 API 调用，全部按量计费、价格透明，折扣力度大，一个密钥即可调用全部模型。</p>
      <h2>支持的模型</h2>
      <p>OpenAI（GPT 系列）、Anthropic Claude、Google Gemini、DeepSeek、通义千问、文心一言、智谱 GLM、Kimi、豆包等。</p>
      <p><a href="${site}/">返回首页</a> · <a href="${site}/rankings">模型排行榜</a></p>
    `,
  },
  {
    path: '/rankings',
    title: '模型排行榜 - iioooo',
    description:
      'iioooo 模型排行榜：实时展示各大 AI 大模型的调用量与热度排行，帮助开发者了解热门模型并选择高性价比方案。',
    content: `
      <h1>模型排行榜 - iioooo</h1>
      <p>iioooo 模型排行榜实时展示各大 AI 大模型的调用量与热度，帮助开发者了解热门模型并选择高性价比方案。</p>
      <p><a href="${site}/">返回首页</a> · <a href="${site}/pricing">模型与价格</a></p>
    `,
  },
  {
    path: '/privacy-policy',
    title: '隐私政策 - iioooo',
    description:
      'iioooo 隐私政策：说明我们在提供 AI 大模型 API 中转服务时如何收集、使用、存储和保护你的信息，以及你享有的相关权利。',
    content: `
      <h1>隐私政策 - iioooo</h1>
      <p>本页说明 iioooo 在提供 AI 大模型 API 中转服务时如何收集、使用、存储和保护你的信息，以及你享有的相关权利。</p>
      <p><a href="${site}/">返回首页</a> · <a href="${site}/user-agreement">用户协议</a></p>
    `,
  },
  {
    path: '/user-agreement',
    title: '用户协议 - iioooo',
    description:
      'iioooo 用户协议：使用本站 AI 大模型 API 中转服务前，请阅读并同意相关条款。',
    content: `
      <h1>用户协议 - iioooo</h1>
      <p>本页为 iioooo AI 大模型 API 中转服务的用户协议，使用本站服务前请阅读并同意相关条款。</p>
      <p><a href="${site}/">返回首页</a> · <a href="${site}/privacy-policy">隐私政策</a></p>
    `,
  },
]

function render(page) {
  let html = template
  html = html.split(homeTitle).join(page.title)
  html = html.split(homeDescription).join(page.description)
  html = html.split(`content="${site}/"`).join(`content="${site}${page.path}"`)
  html = html.replace(
    '</head>',
    `<link rel="canonical" href="${site}${page.path}" /></head>`
  )
  html = html.replace(
    /<noscript>[\s\S]*?<\/noscript>/,
    `<noscript>${page.content}</noscript>`
  )
  return html
}

for (const page of pages) {
  const target =
    page.path === '/'
      ? join(distDir, 'index.html')
      : join(distDir, `${page.path}.html`)
  writeFileSync(target, render(page))
}

console.log(`[prerender] wrote ${pages.length} static page(s) into dist/`)
