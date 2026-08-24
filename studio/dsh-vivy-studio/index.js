import { readFileSync } from "node:fs"
import { dirname, join } from "node:path"
import { fileURLToPath } from "node:url"

export const name = "vivy-studio-skin"
export const inject = ["webServer"]

const here = dirname(fileURLToPath(import.meta.url))
const themeCss = readFileSync(join(here, "theme.css"), "utf8")
const brandJs = readFileSync(join(here, "brand.js"), "utf8")

function injectSkin(html) {
  const titled = html.replace(/<title>[^<]*<\/title>/i, "<title>Vivy Studio</title>")
  const snippet =
    `<style data-plugin="dsh-vivy-studio">${themeCss}</style>` +
    `<script data-plugin="dsh-vivy-studio">${brandJs}</script>`
  const body = /<body(?:\s[^>]*)?>/i.exec(titled)
  if (body === null) return titled + snippet
  const at = body.index + body[0].length
  return titled.slice(0, at) + snippet + titled.slice(at)
}

export function apply(ctx) {
  ctx.effect(
    () => ctx.webServer.tapIndex((html) => injectSkin(html)),
    "vivy-studio: fluorite theme and product identity",
  )
}
