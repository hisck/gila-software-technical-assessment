# Verification retries

| Check | Finding | Resolution |
| --- | --- | --- |
| Local Next.js build | Turbopack could not open its worker socket in the workspace sandbox. The Webpack build exposed a missing Node type dependency. | Used the supported Next.js Webpack build option and installed Node types. The strict static build passed locally and in Docker. |
| Independent checkout review | After a committed payment with a lost response, the old error could remain visible beside the confirmed state. | Restricted submit errors to the pending choice and kept status errors separate. A browser check with an interrupted response confirmed that the message cleared when the durable status advanced. |
| Browser deep links on a mapped port | Nginx redirected a route without its trailing slash to the container port. | Switched to relative redirects and added origin and route assertions to the runtime smoke. |
| Order history with more than nine orders | The sort resolved the text output alias and placed single-digit references ahead of newer orders. | Qualified the underlying numeric column and added an assertion across ten consecutive pending orders. |
