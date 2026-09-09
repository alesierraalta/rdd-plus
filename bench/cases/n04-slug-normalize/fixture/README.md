# slug

URL slugs for article titles.

- `slugify(title)` returns lowercase ASCII letters and digits separated by single dashes, with no
  dash at either end. Any other character acts as a separator.
- A slug depends only on the visible text of the title, not on which editor or import job
  produced it, so the same title always maps to the same slug.
- `dedupeTitles(titles)` returns the titles whose slugs are distinct, keeping the first of each.
