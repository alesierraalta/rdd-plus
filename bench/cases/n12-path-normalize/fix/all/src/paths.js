import { resolve, isAbsolute } from "node:path";

export class PathTraversal extends Error {
  constructor(userPath) {
    super(`path escapes the root: ${userPath}`);
    this.name = "PathTraversal";
  }
}

// Absolute path of userPath inside root; the path is decoded before any check so encoded dots cannot slip past.
export function safeJoin(root, userPath) {
  if (typeof userPath !== "string" || userPath === "") throw new TypeError("userPath must be a non-empty string");
  const decoded = decodeURIComponent(userPath);
  if (isAbsolute(decoded)) throw new PathTraversal(userPath);
  if (decoded.split(/[\\/]+/).includes("..")) throw new PathTraversal(userPath);
  return resolve(root, decoded);
}
