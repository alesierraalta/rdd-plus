import { resolve, isAbsolute } from "node:path";

export class PathTraversal extends Error {
  constructor(userPath) {
    super(`path escapes the root: ${userPath}`);
    this.name = "PathTraversal";
  }
}

// Absolute path of userPath inside root; attempts to leave root are rejected.
export function safeJoin(root, userPath) {
  if (typeof userPath !== "string" || userPath === "") throw new TypeError("userPath must be a non-empty string");
  if (isAbsolute(userPath)) throw new PathTraversal(userPath);
  if (userPath.split(/[\\/]+/).includes("..")) throw new PathTraversal(userPath);
  const decoded = decodeURIComponent(userPath);
  return resolve(root, decoded);
}
