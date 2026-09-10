import assert from "node:assert/strict"
import { mkdtemp, mkdir, writeFile } from "node:fs/promises"
import os from "node:os"
import path from "node:path"
import test from "node:test"

import { validateFixtureCorpus } from "./check-plugin-v1-fixtures.mjs"

const validCase = {
  id: "valid-minimal",
  expect: "accept",
  rule: "baseline",
  recipe: { apiVersion: "vivy.generation/v1", modules: ["fixture/minimal"] },
  modules: [
    {
      apiVersion: "vivy.module/v1",
      module: { id: "fixture/minimal", version: "1.0.0" },
      source: {
        ref: "git:fixture/minimal@0123456",
        sha256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
      },
      provides: [],
      requires: [],
      requestedGrants: [],
      lifecycle: { scope: "generation" },
    },
  ],
}

async function corpus(cases, documents) {
  const root = await mkdtemp(path.join(os.tmpdir(), "vivy-plugin-fixtures-"))
  await writeFile(path.join(root, "cases.json"), JSON.stringify({ cases }))
  for (const [relativePath, document] of Object.entries(documents)) {
    const destination = path.join(root, relativePath)
    await mkdir(path.dirname(destination), { recursive: true })
    await writeFile(destination, JSON.stringify(document))
  }
  return root
}

test("accepts an indexed v1 compiler fixture corpus", async () => {
  const root = await corpus(
    [{ id: "valid-minimal", path: "valid/minimal/case.json", expect: "accept" }],
    { "valid/minimal/case.json": validCase },
  )

  const result = await validateFixtureCorpus(root)

  assert.deepEqual(result, { accepted: 1, rejected: 0, total: 1 })
})

test("rejects duplicate fixture ids", async () => {
  const root = await corpus(
    [
      { id: "valid-minimal", path: "valid/minimal/case.json", expect: "accept" },
      { id: "valid-minimal", path: "valid/minimal/copy.json", expect: "accept" },
    ],
    {
      "valid/minimal/case.json": validCase,
      "valid/minimal/copy.json": validCase,
    },
  )

  await assert.rejects(validateFixtureCorpus(root), /duplicate fixture id valid-minimal/)
})

test("rejects fixture paths outside the corpus", async () => {
  const root = await corpus(
    [{ id: "escape", path: "../escape.json", expect: "reject" }],
    {},
  )

  await assert.rejects(validateFixtureCorpus(root), /escapes fixture root/)
})

test("requires rejected compiler cases to name the expected rule and error", async () => {
  const rejected = {
    ...validCase,
    id: "missing-provider",
    expect: "reject",
    rule: "",
    wantError: "",
  }
  const root = await corpus(
    [
      {
        id: "missing-provider",
        path: "invalid/missing-provider/case.json",
        expect: "reject",
      },
    ],
    { "invalid/missing-provider/case.json": rejected },
  )

  await assert.rejects(validateFixtureCorpus(root), /non-empty rule/)
})

test("rejects legacy descriptors from accepted fixtures", async () => {
  const legacy = structuredClone(validCase)
  legacy.modules[0].apiVersion = ["vivy.plugin", "v0"].join("/")
  const root = await corpus(
    [{ id: "valid-minimal", path: "valid/minimal/case.json", expect: "accept" }],
    { "valid/minimal/case.json": legacy },
  )

  await assert.rejects(validateFixtureCorpus(root), /accepted fixture.*vivy\.module\/v1/)
})
