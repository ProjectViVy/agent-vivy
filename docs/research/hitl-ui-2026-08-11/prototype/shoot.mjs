import { chromium } from "playwright";
import { mkdirSync } from "fs";

const base = "file:///C:/Users/Administrator/Desktop/morediva/diva-go/agent-vivy/docs/research/hitl-ui-2026-08-11/prototype/vivy-hitl-prototype.html";
const out = "C:/Users/Administrator/Desktop/morediva/diva-go/agent-vivy/docs/research/hitl-ui-2026-08-11/prototype/shots";
mkdirSync(out, { recursive: true });

const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
const errors = [];
page.on("console", m => { if (m.type() === "error") errors.push(m.text()); });
page.on("pageerror", e => errors.push("PAGEERROR: " + e.message));

await page.goto(base);
await page.waitForTimeout(300);

// A. Review Center (default)
await page.screenshot({ path: out + "/A-review-center.png" });

// click a high-risk approval row -> B detail
await page.click(".review-list-item:nth-child(2)");
await page.waitForTimeout(150);
await page.screenshot({ path: out + "/B-approval-detail.png" });

// approve once -> submitting -> approved
await page.click("#approve-r2");
await page.waitForTimeout(200);
await page.screenshot({ path: out + "/B2-submitting.png" });
await page.waitForTimeout(900);
await page.screenshot({ path: out + "/B3-approved.png" });
await page.waitForTimeout(1500);
await page.screenshot({ path: out + "/B4-executing.png" });
await page.waitForTimeout(1600);
await page.screenshot({ path: out + "/B5-succeeded.png" });

// back to review, open question (r3)
await page.click(".breadcrumb a");
await page.waitForTimeout(150);
await page.click(".review-list-item:nth-child(3)");
await page.waitForTimeout(150);
await page.screenshot({ path: out + "/C-question-detail.png" });

// type answer then answer
await page.fill("#answer-r3", "blue");
await page.waitForTimeout(100);
await page.click("#submit-r3");
await page.waitForTimeout(200);
await page.screenshot({ path: out + "/C2-answering.png" });

// stale row (r4)
await page.click(".breadcrumb a");
await page.waitForTimeout(150);
await page.click(".review-list-item:nth-child(4)");
await page.waitForTimeout(150);
await page.screenshot({ path: out + "/B6-stale.png" });

// D. inspector Review tab
await page.evaluate(() => go("inspector"));
await page.waitForTimeout(200);
await page.screenshot({ path: out + "/D-inspector-review.png" });
// events tab
await page.click(".inspector-tab:nth-child(3)");
await page.waitForTimeout(150);
await page.screenshot({ path: out + "/D2-inspector-events.png" });

// E. inline conversation card
await page.evaluate(() => go("chat"));
await page.waitForTimeout(200);
await page.screenshot({ path: out + "/E-inline-card.png" });

// dark theme
await page.evaluate(() => { cycleTheme(); cycleTheme(); });
await page.evaluate(() => go("review"));
await page.waitForTimeout(200);
await page.screenshot({ path: out + "/A-dark.png" });

console.log("CONSOLE ERRORS:", errors.length ? errors : "none");
await browser.close();
