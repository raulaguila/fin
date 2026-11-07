import { ApiError } from "@/lib/api/client";
import type { MessageKey } from "@/lib/i18n/messages";

const BY_CODE: Record<string, MessageKey> = {
  INVALID_CREDENTIALS: "error.invalid_credentials",
  TOO_MANY_REQUESTS: "error.too_many_requests",
  TOKEN_EXPIRED: "error.token_expired",
  FORBIDDEN: "error.forbidden",
  NOT_FOUND: "error.not_found",
  ALREADY_EXISTS: "error.already_exists",
  CONFLICT: "error.conflict",
  RESOURCE_IN_USE: "error.resource_in_use",
  UNAVAILABLE: "error.unavailable",
};

/**
 * A message the user can act on. Validation errors keep the backend text,
 * which names the field rule; anything unknown falls back to a generic line.
 */
export function errorMessage(err: unknown, fallback: MessageKey, t: (key: MessageKey) => string): string {
  if (!(err instanceof ApiError)) return t("error.network");
  if (err.code === "INVALID_INPUT" || err.code === "VALIDATION_FAILED") return err.message;
  const key = BY_CODE[err.code];
  return key ? t(key) : t(fallback);
}
