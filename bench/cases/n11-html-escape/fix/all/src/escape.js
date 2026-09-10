const entities = { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" };

// An ampersand that already starts an entity is left alone, so escaping twice yields the same text.
const unsafe = /&(?![a-zA-Z][a-zA-Z0-9]*;|#\d+;|#[xX][0-9a-fA-F]+;)|[<>"']/g;

// Text safe for element content and for attribute values in either quote style.
export function escapeHtml(value) {
  return String(value).replace(unsafe, (c) => entities[c]);
}

export function renderAttr(name, value) {
  return `${name}="${escapeHtml(value)}"`;
}

// The title attribute is single-quoted in the layout templates.
export function renderTitle(value) {
  return `title='${escapeHtml(value)}'`;
}
