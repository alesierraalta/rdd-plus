# html-escape

Escaping helpers for the server-rendered templates.

- `escapeHtml(value)` returns text that is safe to place anywhere in an HTML document: in element
  content and inside attribute values, whichever quoting style the template uses.
- `renderAttr(name, value)` and `renderTitle(value)` build attribute fragments used by the
  templates; both rely on `escapeHtml`.
- Values may reach the renderer from several layers (form input, the database, the cache), so
  `escapeHtml` is safe to apply at each layer.
