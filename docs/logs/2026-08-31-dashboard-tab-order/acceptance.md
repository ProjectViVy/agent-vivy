# Acceptance: how a person confirms the change took effect

1. Start the development pair (`just dev`) and open `http://127.0.0.1:3015`.
2. Click the left navigation item `Console`.
3. Expected: after entering, the **Token** tab is selected by default and Token statistics (total, model distribution, trend, and session details) are immediately visible.
4. The tabs from left to right are **Token, Trajectory, Sessions** (`Trajectory`, `Sessions`); there is no tab named `Overview`, and the third tab is named `Sessions`.
5. Clicking `Sessions` still shows the original overview content: runtime status (sessions/active runs/pending Review) and recent activity.
6. All three tabs render normally without errors when switched.
