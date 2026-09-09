# Vivy Worldview and Product Philosophy

> Status: **recorded** (2026-08-16; the user confirmed it should be recorded).
> This document explains the product philosophy and why the product shares its structure with *Vivy: Fluorite Eye's Song*.
> It **does not replace** the four anchors in PRD §5.0 or the development-environment strategy in `VIVY-STUDIO.md`.
> The repository contains no anime setting bible. The structural parallels come from the product name, the prohibitions, and the later separation of the species / Studio.
>
> Related:
> - `../../../prd-agent-vivy-v0.md` §5.0 — citable product anchors
> - `../../../AGENT-VIVY-DIRECTION.md` — Diva and Vivy's product-line relationship; V0–V3 phasing
> - `VIVY-STUDIO.md` — the daily EXE is the artifact, not the host
> - `SELF-EVOLVING-GATEWAY.md` — the kernel is never a plugin; against hot reload

---

## 0. One Sentence

The body on stage may not modify itself on the spot. The heart stays on the human's side. Evolution gets a separate workshop.

---

## 1. This Is Not a Setting Bible

The engineering documents describe the gateway, Journal, Studio, and Eino.  
The terms “fluorite eyes” and “songstress” were not originally in the repository.

The correspondence is not read from a hidden archive. The product is called Vivy; its empty state is “A Journey to Find the True Heart”; and the prohibitions increasingly take the shape this name calls for. Fluorite cyan as the theme color is a consequence, not the source.

This document records only the structural resemblance. It does not turn the product into a singing character or treat the anime as a requirements document.

---

## 2. Four Structurally Isomorphic Bones

### 2.1 The Body That Lives Through the Night ≠ the Laboratory

In the anime, Vivy is a songstress who has to live through the night; during a performance she cannot disassemble herself into parts. The Archive storyline is the one about rewriting herself and humanity for greater strength.

In the product, the daily `vivy.exe` is the resident product—keys are local, the Journal is replayable, and a person can entrust their life to it. Vivy Studio is a separate application that manages next-generation bodies. The species does not start Studio. There is no “Open the laboratory from the chat box.”

Chasing a higher Cordis replication rate and turning everything into in-process plugins would erase the species / laboratory split. Do not do it.

### 2.2 History Is How She Remains Herself

Vivy remains the same person through a century of songs and memories, not a different soul swapped in by every hot reload.

The product makes logs first-class: “not being able to see what happened” is a product bug. What the model sees must be reconstructible from the Journal. A generation change means packing a new EXE, evaluating it, and installing it—not hanging parts onto a live process. The next generation may be stronger; this generation must remain replayable.

The kernel is never a plugin: Journal, policy, keys, and identity. The true heart cannot be hot-unloaded.

### 2.3 Diva Is the Old Public Body; Vivy Does Not Inherit That Body

In the anime, Diva is the stage name, while Vivy is the person she later becomes after finding herself.

On the product line, `agent-diva` remains maintained. Philosophical anchors may be inherited; implementation may not—move none of the crate graph, schema, or Tauri contracts. The old product is evidence of capabilities only, not a skeleton. Direction is the engineering version of this sentence.

### 2.4 One Mission per Phase

V0 assembly, V1 daily life, V2 exploration in Studio, V3 reconstruction only with evidence. A phase may not promise a stable product, a framework swap, and AGI-OS all at once.

Curated directories, not a sea of plugins. The mission must stay narrow enough to live through the night.

---

## 3. The One Point Not to Force

In the anime, “finding the true heart” is an AI growing a heart capable of taking things seriously.

In the product philosophy, the heart stays on the human's side: local, auditable, and replayable; approval authority is not handed away. The gateway is companionship, not a bid to become sentient.

Read against the worldview, the empty state “A Journey to Find the True Heart” therefore means, more precisely:

**Do not consume a person's life in exchange for your own evolution.**

---

## 4. The Name Came First; the Structure Later Converged

PRD §5.0 was initially copied from Diva's personal gateway: local, logs, curated providers, broad sections.

Only later came the separate Studio, anti-hot-reload, and the rule that the kernel is never a plugin. The more the pieces were separated, the more they resembled this name.

It was not an anime bible first and architecture afterward. The name and the prohibitions narrowed the shape to one place.
