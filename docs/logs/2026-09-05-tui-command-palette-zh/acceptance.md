# Acceptance

1. In a real `vivy tui --live`, press `/`.
2. The list should show `/help` with a short label such as “View commands”, rather than a screen full of `/mcp [server|resources…]`.
3. When moving up and down, the current row has a background; full usage appears only below the selected row.
4. When entering `mcp` or a Chinese keyword, matching characters are underlined.
5. The footer uses Chinese labels such as “Command / Send / Approve”.
6. Enter still fills in English commands such as `/help` and `/image`; `//hello` is still sent literally.
