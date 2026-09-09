# csv-line

Single-line CSV codec used by the export and import jobs.

- `renderLine(fields)` produces one RFC 4180 line: fields are joined with commas, and a field
  that needs protection is enclosed in double quotes.
- `parseLine(line)` is the inverse: it returns the fields of one line, with quoting removed.
- The two functions are inverses of each other for every array of strings.
