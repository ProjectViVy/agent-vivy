# Acceptance

For a human checking this in the running product.

## Look at the sidebar

Start the split pair (`just dev`) and open `http://127.0.0.1:3015`. Click
**VIVY** in the sidebar. The group lists 人格 / 面具 / 进化 / 记忆 / 记事本, and
every row has an icon on the left, drawn in the same size and style as 中控台,
工具箱, and 设置. No row is missing an icon, and no row shows a raw
`plugin.*` key.

## Open each Module page

Click 人格, 进化, 记忆, 记事本 in turn. Each page should look like the others:

- the demo banner sits at the top;
- below it, one header line with the entry's own icon and the page title
  (人格 / 进化 / 记忆 / 记事本), plus a one-line subtitle;
- the content fills the whole remaining window instead of the top part of it, and
  the sidebar stays in place.

Before this change, 记忆 and 记事本 had no title at all, the four pages used
between 269 px and 530 px of the 844 px content frame, and the rest of the
window was empty.

## Check the standards in code

- A Module names its icon with a string from `HOST_ICON_NAMES` in
  `@vivy/ui-sdk`; the host resolves it in `ui/src/plugins/host-icons.ts`. Adding
  an icon name to the SDK without teaching the host what it looks like fails
  `host-icons.test.ts`.
- A Module page declares its title, subtitle, and demo banner with
  `defineUIRoute({...})` next to its route and returns content only. Search the
  four `plugins/vivy-*/ui/*/src/page.tsx` files: none of them renders a banner,
  an `<h1>`, or a page-level `h-full` wrapper.

## Remove a Module from the Recipe

Deselect `vivy/notebook` from `recipes/default.vivy.yml`, run `just dev` again:
its sidebar entry and its page surface both disappear, and the other three pages
keep their layout. The header chrome comes from the host, so nothing else has to
change.