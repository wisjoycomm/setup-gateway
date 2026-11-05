## Web
- Never read or search `node_modules/`, `dist/`, `build/`, `.next/`, `coverage/`, or lockfiles.
- Use the package manager the lockfile shows and never mix two. Don't add a dependency without saying why in your report.
- Never read or print `.env` files, which hold real secrets. Add new variable names to `.env.example` with no values.
- Tests run once: `vitest run`, not `vitest`. Before calling a change done, run the project's typecheck and tests.
- Reuse existing components, styles and data-fetching patterns before creating new ones.
- Keep user-facing pages accessible: real buttons and links, labels on form fields, and text alternatives for images.
- Start a dev server only when the task needs it, in the background, and stop it when you finish.
