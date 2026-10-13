# Acceptance

On Windows, start the split Vivy pair and use an initialized Persona and configured provider. Send a prompt and wait for its Run to finish. Reload the browser and confirm the assistant response remains in the selected session. Open Toolkits from that session and preview a local Toolkit; the preview should succeed after completed, failed, or cancelled Runs. While a Run is accepted, queued, or active, the same direct management action must remain denied.

For the Windows local-source regression, run the Toolkit Git acquisition path with an absolute local repository path such as `C:\work\toolkit`; it should record the local repository basename and preserve the existing unsafe-origin rejections. `just ci` covers the Windows-specific Hook, shell, composition, RPC, Toolkit, and browser-selector regressions.
