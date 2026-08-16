import { mkdirSync } from "node:fs"
import { dirname, join } from "node:path"
import { fileURLToPath } from "node:url"
import { chromium } from "../ui/node_modules/playwright/index.mjs"

const here = dirname(fileURLToPath(import.meta.url))
const out = join(here, "..", "data", "studio-home", "verify")
mkdirSync(out, { recursive: true })

const browser = await chromium.launch({ headless: true })
const page = await browser.newPage({ viewport: { width: 1440, height: 900 } })
const errors = []
page.on("pageerror", (err) => { errors.push(String(err) ) })

await page.goto("http://127.0.0.1:3090/", { waitUntil: "networkidle", timeout: 60000 })
await page.waitForTimeout(1500)

const afterOf = async (sel) => page.locator(sel).first().evaluate(
  (el) => getComputedStyle(el, "::after").content,
)

const title = await page.title()
const sloganAfter = await afterOf('[class*="headlineText"]')
const wordmarkAfter = await afterOf('button:has(> svg[viewBox="0 0 182 24"])')
const welcomeEl = page.locator('[class*="copy"] p').first()
const welcomeAfter = (await welcomeEl.count()) > 0 ? await afterOf('[class*="copy"] p') : "(already acknowledged)"

await page.screenshot({ path: join(out, "welcome.png"), fullPage: true })

const cont = page.getByRole("button", { name: /继续|Continue/i }).first()
if (await cont.count()) {
  await cont.click({ force: true })
  await page.waitForTimeout(800)
}

await page.screenshot({ path: join(out, "shell.png"), fullPage: true })

const settings = page.getByRole("button", { name: /设置|Settings/i }).first()
if (await settings.count()) {
  await settings.click({ force: true })
  await page.waitForTimeout(800)
  await page.screenshot({ path: join(out, "settings.png"), fullPage: true })
  const dark = page.getByText("深色", { exact: true }).first()
  if (await dark.count()) {
    await dark.click()
    await page.waitForTimeout(500)
    await page.screenshot({ path: join(out, "settings-dark.png"), fullPage: true })
  }
}

const report = {
  title,
  sloganAfter,
  welcomeAfter,
  wordmarkAfter,
  titleIsDsh: title.includes("DeepSeek") || title.includes("HARNESS"),
  pageErrors: errors,
}
console.log(JSON.stringify(report, null, 2))

const failed = report.titleIsDsh
  || !sloganAfter.includes("寻找真心之旅")
  || (welcomeAfter !== "(already acknowledged)" && !welcomeAfter.includes("独立应用"))
  || !wordmarkAfter.includes("Vivy Studio")
  || errors.length > 0
await browser.close()
process.exit(failed ? 1 : 0)
