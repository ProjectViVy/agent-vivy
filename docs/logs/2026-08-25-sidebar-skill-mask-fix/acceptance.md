# Acceptance

## How to confirm the fix from a product/user perspective

1. Open the app (http://127.0.0.1:3015) and inspect the left sidebar:
   - The Vivy group contains an **Evolution** item with a **Planned** badge on the right;
   - The Tools Management group still contains **MCP / Skill**, with Skill as the
     only entry to the Skills page.
2. Click Evolution in the left sidebar:
   - **There is no navigation**; a notice saying “Evolution is not implemented
     yet” appears at the bottom of the sidebar and disappears after about 1.8 seconds.
3. Click Skill to enter the Skills page:
   - **Only Skill is highlighted** in the sidebar; Evolution never becomes highlighted.
4. On the Skills page, click another navigation item (such as Chat or Dashboard):
   - Highlighting moves correctly, and Skill is no longer covered by an extra highlight.
5. After switching languages (Settings → Language):
   - In the English UI, the label is Evolution, the badge is Planned, and the
     notice says “Evolution is not implemented yet”; the corresponding zh UI
     strings remain localized in the product.
