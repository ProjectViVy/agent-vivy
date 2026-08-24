import { chromium } from "playwright";
const base = "file:///C:/Users/Administrator/Desktop/morediva/diva-go/agent-vivy/docs/research/hitl-ui-2026-08-11/prototype/vivy-hitl-prototype.html";
const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
const errors = [];
page.on("console", m => { if (m.type() === "error") errors.push(m.text()); });
page.on("pageerror", e => errors.push("PAGEERR: " + e.message));
await page.goto(base);
await page.waitForTimeout(200);

const checks = [];
function ok(name, cond) { checks.push((cond ? "PASS " : "FAIL ") + name); }

// A. Review Center
ok("A: review center header", await page.locator(".review-center-header h2").count() === 1);
ok("A: 4 review rows", await page.locator(".review-list-item").count() === 4);
ok("A: risk pills present", await page.locator(".risk").count() >= 4);
ok("A: filters present", await page.locator(".chip-filter").count() === 5);
ok("A: pending count badge = 3", (await page.locator("#sidebar-count").textContent()) === "3");

// open high-risk approval (r2, row 2) -> B
await page.locator(".review-list-item").nth(1).click();
await page.waitForTimeout(100);
ok("B: approval card border-top warning", await page.locator(".review-card.is-approval").count() === 1);
ok("B: diff rendered", await page.locator(".diff .diff-line").count() > 0);
ok("B: command argv preview for execute", (await page.locator(".diff .code").first().textContent()).includes("npm run migrate"));
ok("B: high-risk warning shown", await page.locator(".review-risks").count() >= 1);
ok("B: Approve once button", (await page.locator("#approve-r2").textContent()).includes("Approve once"));

// approve -> lifecycle
await page.locator("#approve-r2").click();
await page.waitForTimeout(150);
ok("B2: submitting state", (await page.locator(".decision-recorded").textContent()).includes("Submitting"));
await page.waitForTimeout(1000);
ok("B3: approved -> executing copy", (await page.locator(".decision-recorded").textContent()).includes("Decision recorded"));
await page.waitForTimeout(1500);
await page.waitForTimeout(1700);
ok("B5: succeeded", (await page.locator(".decision-recorded").textContent()).includes("Succeeded"));

// back, open question (r3)
await page.locator(".breadcrumb a").click();
await page.waitForTimeout(100);
await page.locator(".review-list-item").nth(2).click();
await page.waitForTimeout(100);
ok("C: question card", await page.locator(".review-card.is-question").count() === 1);
ok("C: answer textarea disabled submit initially", await page.locator("#submit-r3").isDisabled());
await page.fill("#answer-r3", "blue");
await page.waitForTimeout(50);
ok("C: submit enabled after typing", !(await page.locator("#submit-r3").isDisabled()));
await page.locator("#submit-r3").click();
await page.waitForTimeout(150);
ok("C2: answering state", (await page.locator(".decision-recorded").textContent()).includes("Submitting") || (await page.locator(".decision-recorded").textContent()).includes("recorded"));

// stale (r4)
await page.locator(".breadcrumb a").click();
await page.waitForTimeout(100);
await page.locator(".review-list-item").nth(3).click();
await page.waitForTimeout(100);
ok("B6: stale card shown", await page.locator(".review-card.is-stale").count() === 1);
ok("B6: no approve button on stale", await page.locator("#approve-r4").count() === 0);
ok("B6: request-fresh action present", await page.locator(".review-card.is-stale button:has-text('Request fresh proposal')").count() === 1);

// D inspector
await page.evaluate(() => go("inspector"));
await page.waitForTimeout(150);
ok("D: inspector open", await page.locator(".app-shell.inspector-open").count() === 1);
ok("D: Review tab active", await page.locator(".inspector-tab.is-active").textContent() === "Review");
ok("D: inline review card in inspector", await page.locator(".inspector-body .review-card").count() === 1);
await page.locator(".inspector-tab").nth(2).click();
await page.waitForTimeout(100);
ok("D2: events timeline", await page.locator(".event-timeline .event-item").count() === 9);

// E inline conversation
await page.evaluate(() => go("chat"));
await page.waitForTimeout(150);
ok("E: inline card in chat", await page.locator(".inline-card-host .review-card").count() === 1);
ok("E: composer disabled while paused", await page.locator(".composer button").isDisabled());

// dark theme toggle on review
await page.evaluate(() => go("review"));
await page.evaluate(() => { cycleTheme(); cycleTheme(); });
await page.waitForTimeout(100);
ok("theme: dark dataset", await page.evaluate(() => document.documentElement.dataset.theme) === "dark");

// locale toggle
await page.evaluate(() => cycleLocale());
ok("locale: zh set", await page.evaluate(() => document.documentElement.lang) === "zh-CN");

await browser.close();
console.log(checks.join("\n"));
console.log("\nCONSOLE ERRORS: " + (errors.length ? "\n" + errors.join("\n") : "none"));
const failed = checks.filter(c => c.startsWith("FAIL"));
console.log("\n" + (failed.length ? failed.length + " FAILED" : "ALL " + checks.length + " CHECKS PASSED"));
