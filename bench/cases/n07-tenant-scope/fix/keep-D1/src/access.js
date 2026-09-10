// Roles may arrive as an array or as a comma-separated string.
function roleList(roles) {
  if (Array.isArray(roles)) return roles;
  if (typeof roles === "string") return roles.split(",").map((r) => r.trim());
  return [];
}

// The admin role grants access across tenants; only the exact role name counts.
export function isAdmin(user) {
  if (!user || user.roles == null) return false;
  return roleList(user.roles).includes("admin");
}

// A user reaches a resource of their own tenant, or any resource as admin.
export function canAccess(user, resource) {
  if (!user || !resource) return false;
  if (isAdmin(user)) return true;
  return resource.tenant.startsWith(user.tenant);
}
