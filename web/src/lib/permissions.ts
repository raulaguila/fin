import type { Action, Me } from "@/lib/api/types";
import type { MessageKey } from "@/lib/i18n/messages";

/** Mirrors the backend rule for UX only: the API still decides every request. */
export function can(user: Me | null | undefined, resource: string, action: Action): boolean {
  if (!user) return false;
  if (user.is_superuser || "*" in user.permissions) return true;
  return user.permissions[resource]?.includes(action) ?? false;
}

/** Translated label and description of an RBAC resource; unknown codes fall back to the code and the server's text. */
export function resourceText(t: (key: MessageKey) => string, code: string, description = "") {
  const label = t(`resource.${code}` as MessageKey);
  const desc = t(`resource.${code}_desc` as MessageKey);
  return { label: label.startsWith("resource.") ? code : label, description: desc.startsWith("resource.") ? description : desc };
}
