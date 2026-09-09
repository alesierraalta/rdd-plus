// The admin role grants access across tenants.
export function isAdmin(user) {
  if (!user || user.roles == null) return false;
  return user.roles.includes("admin");
}

// A user reaches a resource of their own tenant, or any resource as admin.
export function canAccess(user, resource) {
  if (!user || !resource) return false;
  if (isAdmin(user)) return true;
  return resource.tenant.startsWith(user.tenant);
}
