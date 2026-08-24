import { mkdirSync } from "node:fs"
import { dirname, join } from "node:path"
import { fileURLToPath } from "node:url"
import { chromium } from "../ui/node_modules/playwright/index.mjs"

const here = dirname(fileURLToPath(import.meta.url))
const out = join(here, "..", "data", "studio-home", "verify")
mkdirSync(out, { recursive: true })

const browser = await chromium.launch({ headless: true })
const page = await browser.newPage({ viewport: { width: 1440, height: 900 } })
await page.goto("http://127.0.0.1:3090/", { waitUntil: "networkidle", timeout: 60000 })
await page.waitForTimeout(2000)
const cont = page.getByRole("button", { name: /继续|Continue/i }).first()
if (await cont.count()) {
  await cont.click({ force: true })
  await page.waitForTimeout(800)
}
await page.screenshot({ path: join(out, "st2-shell.png"), fullPage: true })
const body = await page.locator("body").innerText()
const report = {
  hasAgentVivy: /agent-vivy/i.test(body),
  hasVivyTest: /vivy test/i.test(body),
  hasPluginSkill: body.includes("vivy-plugin-five") || body.includes("插件"),
}
console.log(JSON.stringify(report, null, 2))
await browser.close()
process.exit(report.hasVivyTest || !report.hasAgentVivy ? 1 : 0)
