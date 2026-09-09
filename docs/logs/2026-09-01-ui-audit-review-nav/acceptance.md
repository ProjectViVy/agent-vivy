# Acceptance: how to verify manually

1. Open `http://127.0.0.1:3015`; the shield entry appears in the left main
   navigation after Chat/Dashboard. Click it to open the full `/approvals` page
   (not a sheet).
2. In the Chinese/English interfaces, the source-of-record labels are
   `zh=审批中心` / `en=Approvals`, matching the Review Center name opened by the
   chat shield.
3. At the narrow 390px breakpoint, open the navigation drawer; the entry is
   still visible and clickable.
4. The review sheet opened by the chat shield behaves unchanged (this slice adds
   navigation only and does not change Review logic).
