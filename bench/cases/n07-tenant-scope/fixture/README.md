# tenant-scope

Authorization for a multi-tenant document store.

- Every user and every resource belongs to exactly one tenant, identified by its slug.
- `canAccess(user, resource)` is true when the user belongs to the resource's tenant, or when the
  user holds the `admin` role, which grants access across tenants.
- `user.roles` is an array of role names; records imported from the legacy directory carry the
  same roles as a comma-separated string, and both forms are accepted. Roles are matched by their
  exact name.
