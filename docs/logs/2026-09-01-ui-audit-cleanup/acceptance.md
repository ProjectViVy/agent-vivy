# Acceptance: UI audit cleanup

How a human can tell it worked:

1. 中控台 (`/dashboard`) shows only two tabs — Token and 轨迹. The former
   「会话」tab with the fake 12/2/1 numbers and the recent-activity list is
   gone.
2. Skill 页 (`/skills`) has no 「变更请求」 tab — only 已安装技能 and (when
   enabled) 市场.
3. In the chat input card, the upper toolbar's right side has only 历史 and
   审批中心; the Plus button next to the send button now creates a new session
   (click it: a fresh empty conversation opens). The old do-nothing 「更多」
   Plus is gone, so there is exactly one Plus in the input card.
4. Chat still works end to end; no amber preflight banner (previous iteration).
