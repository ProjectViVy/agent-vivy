# Acceptance perspective: marketplace skill upgrade

1. Open `http://127.0.0.1:3015` and go to the Marketplace section of the Skills page
   (the featured list or search should show at least one uninstalled skill).
2. Click "Install" → after installation succeeds, the row becomes a "Check for updates"
   button.
3. Click "Check for updates" → an "Up to date" badge appears (the newly installed skill
   is byte-for-byte identical to the snapshot).
4. Upgrade availability: kernel tests cover (upstream snapshot changes →
   `upgrade_available` → an "Upgrade" button appears → clicking it mirrors the new
   snapshot and displays "Upgraded to marketplace version"). Live manual verification
   depends on an actual upstream release, so the httptest replay is authoritative.
5. A manually placed skill (placed under skills_root, with no `.vivy-skill.json`) shows
   "Locally placed skill — delete and reinstall to update"; calling the upgrade RPC for it
   returns 409.
6. After switching languages, the above copy is correct in both en and zh.
