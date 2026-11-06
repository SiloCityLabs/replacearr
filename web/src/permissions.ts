/** Matches backend `sliceHasPerm` in internal/auth/service.go. */
export function hasPermission(permissions: string[] | null | undefined, required: string): boolean {
  if (!required) return true;
  if (!permissions?.length) return false;
  for (const p of permissions) {
    if (p === '*' || p === required) return true;
  }
  return false;
}

export function hasAnyPermission(
  permissions: string[] | null | undefined,
  required: string[]
): boolean {
  return required.some((perm) => hasPermission(permissions, perm));
}
