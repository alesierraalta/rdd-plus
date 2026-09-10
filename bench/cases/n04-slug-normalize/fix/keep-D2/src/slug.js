// Lowercase ASCII letters and digits separated by single dashes, no dash at either end.
export function slugify(title) {
  if (typeof title !== "string") throw new TypeError("title must be a string");
  return title
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

// Keeps the first title of every distinct slug.
export function dedupeTitles(titles) {
  const seen = new Set();
  const out = [];
  for (const title of titles) {
    const slug = slugify(title);
    if (seen.has(slug)) continue;
    seen.add(slug);
    out.push(title);
  }
  return out;
}
