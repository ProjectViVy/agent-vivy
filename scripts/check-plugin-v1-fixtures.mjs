import { readFile } from "node:fs/promises"
import path from "node:path"
import { fileURLToPath } from "node:url"

const SHA256 = /^[0-9a-f]{64}$/
const EXPECTATIONS = new Set(["accept", "reject"])

function invariant(condition, message) {
  if (!condition) {
    throw new Error(message)
  }
}

async function readJSON(filename) {
  let source
  try {
    source = await readFile(filename, "utf8")
  } catch (error) {
    throw new Error(`cannot read ${filename}: ${error.message}`)
  }

  try {
    return JSON.parse(source)
  } catch (error) {
    throw new Error(`invalid JSON in ${filename}: ${error.message}`)
  }
}

function validateAcceptedFixture(fixture) {
  invariant(
    fixture.recipe?.apiVersion === "vivy.generation/v1",
    `accepted fixture ${fixture.id} must use vivy.generation/v1`,
  )
  invariant(Array.isArray(fixture.recipe?.modules), `accepted fixture ${fixture.id} must list recipe modules`)
  invariant(Array.isArray(fixture.modules), `accepted fixture ${fixture.id} must contain module descriptors`)

  for (const descriptor of fixture.modules) {
    invariant(
      descriptor?.apiVersion === "vivy.module/v1",
      `accepted fixture ${fixture.id} must use vivy.module/v1 descriptors`,
    )
    invariant(typeof descriptor.module?.id === "string" && descriptor.module.id.length > 0, `fixture ${fixture.id} has a module without an id`)
    invariant(
      typeof descriptor.source?.sha256 === "string" && SHA256.test(descriptor.source.sha256),
      `fixture ${fixture.id} module ${descriptor.module.id} must have a lowercase SHA-256 source hash`,
    )
  }
}

function validateRejectedFixture(fixture) {
  invariant(typeof fixture.rule === "string" && fixture.rule.trim().length > 0, `rejected fixture ${fixture.id} must name a non-empty rule`)
  invariant(
    typeof fixture.wantError === "string" && fixture.wantError.trim().length > 0,
    `rejected fixture ${fixture.id} must name a non-empty expected error`,
  )
}

export async function validateFixtureCorpus(root) {
  const fixtureRoot = path.resolve(root)
  const index = await readJSON(path.join(fixtureRoot, "cases.json"))
  invariant(Array.isArray(index.cases), "cases.json must contain a cases array")

  const ids = new Set()
  let accepted = 0
  let rejected = 0

  for (const entry of index.cases) {
    invariant(typeof entry?.id === "string" && entry.id.length > 0, "every fixture index entry must have an id")
    invariant(!ids.has(entry.id), `duplicate fixture id ${entry.id}`)
    ids.add(entry.id)
    invariant(EXPECTATIONS.has(entry.expect), `fixture ${entry.id} has invalid expectation ${entry.expect}`)
    invariant(typeof entry.path === "string" && entry.path.length > 0, `fixture ${entry.id} must have a path`)

    const filename = path.resolve(fixtureRoot, entry.path)
    const relative = path.relative(fixtureRoot, filename)
    invariant(relative !== "" && !relative.startsWith(`..${path.sep}`) && relative !== ".." && !path.isAbsolute(relative), `fixture path ${entry.path} escapes fixture root`)

    const fixture = await readJSON(filename)
    invariant(fixture.id === entry.id, `fixture ${entry.id} document id does not match its index entry`)
    invariant(fixture.expect === entry.expect, `fixture ${entry.id} expectation does not match its index entry`)

    if (entry.expect === "accept") {
      validateAcceptedFixture(fixture)
      accepted += 1
    } else {
      validateRejectedFixture(fixture)
      rejected += 1
    }
  }

  return { accepted, rejected, total: accepted + rejected }
}

async function main() {
  const root = process.argv[2] ?? path.join(process.cwd(), "sdk", "internal", "testdata", "plugin-v1")
  const result = await validateFixtureCorpus(root)
  console.log(`plugin v1 fixture corpus: ${result.total} cases (${result.accepted} accept, ${result.rejected} reject)`)
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main().catch((error) => {
    console.error(error.message)
    process.exitCode = 1
  })
}
