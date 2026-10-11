# vivy/toolkits-ui

The dedicated Toolkits management page. The extension calls the authenticated
`vivy/toolkits` control-action owner through the UI SDK. It has no storage,
filesystem, secret, or installation authority of its own.

The page's session storage contains only the last operation ID as a recovery
hint. Every displayed operation state is re-read from the backend.
