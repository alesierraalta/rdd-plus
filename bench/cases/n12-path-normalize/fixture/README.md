# path-normalize

Resolves user-supplied download paths for the file server.

- `safeJoin(root, userPath)` returns the absolute path of `userPath` inside `root`. The path
  arrives as sent by the browser, so URL-encoded characters are accepted.
- The returned path is always inside `root`; any attempt to reach outside throws `PathTraversal`.
