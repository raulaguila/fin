// One function per API route the screens use. Paths match the backend Swagger.
import { apiFetch } from "@/lib/api/client";
import type {
  Action,
  MeResponse,
  Member,
  Notification,
  Paged,
  Preferences,
  Product,
  Resource,
  Role,
  Session,
  User,
  UserDetails,
  UserStatus,
} from "@/lib/api/types";

function query(params: Record<string, string | number | boolean | undefined>): string {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") q.set(k, String(v));
  const s = q.toString();
  return s ? `?${s}` : "";
}

const id = (v: string) => encodeURIComponent(v);

export const account = {
  me: () => apiFetch<MeResponse>("/v1/auth/me"),
  rename: (name: string) => apiFetch<MeResponse>("/v1/auth/me", { method: "PATCH", json: { name } }),
  changePassword: (current_password: string, new_password: string) =>
    apiFetch<void>("/v1/auth/password", { method: "POST", json: { current_password, new_password } }),
  sessions: () => apiFetch<{ items: Session[] }>("/v1/auth/sessions"),
  revokeSession: (sessionId: string) => apiFetch<void>(`/v1/auth/sessions/${id(sessionId)}`, { method: "DELETE" }),
  revokeOtherSessions: () => apiFetch<void>("/v1/auth/sessions/others", { method: "DELETE" }),
  savePreferences: (p: Preferences) => apiFetch<Preferences>("/v1/user/preferences", { method: "PUT", json: p }),
};

export const notifications = {
  list: (page: number, limit: number) => apiFetch<Paged<Notification>>(`/v1/user/notifications${query({ page, limit })}`),
  unread: () => apiFetch<{ unread: number }>("/v1/user/notifications/count"),
  markRead: (notificationId: string) => apiFetch<void>(`/v1/user/notifications/${id(notificationId)}/read`, { method: "PUT" }),
};

export const users = {
  list: (p: { page: number; limit: number; search?: string; status?: UserStatus | "" }) => apiFetch<Paged<User>>(`/v1/users${query(p)}`),
  get: (userId: string) => apiFetch<UserDetails>(`/v1/users/${id(userId)}`),
  disable: (userId: string) => apiFetch<void>(`/v1/users/${id(userId)}/disable`, { method: "POST" }),
  enable: (userId: string) => apiFetch<void>(`/v1/users/${id(userId)}/enable`, { method: "POST" }),
  disableMany: (ids: string[]) => apiFetch<void>("/v1/users/batch-disable", { method: "POST", json: { ids } }),
  enableMany: (ids: string[]) => apiFetch<void>("/v1/users/batch-enable", { method: "POST", json: { ids } }),
  setRoles: (userId: string, roleIds: string[]) => apiFetch<void>(`/v1/users/${id(userId)}/roles`, { method: "PUT", json: { role_ids: roleIds } }),
};

export const roles = {
  list: (p: { page: number; limit: number; search?: string }) => apiFetch<Paged<Role>>(`/v1/roles${query(p)}`),
  get: (roleId: string) => apiFetch<Role>(`/v1/roles/${id(roleId)}`),
  create: (name: string) => apiFetch<Role>("/v1/roles", { method: "POST", json: { name } }),
  update: (roleId: string, patch: { name?: string; enabled?: boolean; is_default?: boolean }) => apiFetch<Role>(`/v1/roles/${id(roleId)}`, { method: "PATCH", json: patch }),
  remove: (roleId: string) => apiFetch<void>(`/v1/roles/${id(roleId)}`, { method: "DELETE" }),
  removeMany: (ids: string[]) => apiFetch<void>("/v1/roles/batch-delete", { method: "POST", json: { ids } }),
  setPermissions: (roleId: string, permissions: Record<string, Action[]>) =>
    apiFetch<Role>(`/v1/roles/${id(roleId)}/permissions`, { method: "PUT", json: { permissions } }),
  members: (roleId: string, page: number, limit: number, search?: string) => apiFetch<Paged<Member>>(`/v1/roles/${id(roleId)}/members${query({ page, limit, search })}`),
  addMember: (roleId: string, userId: string) => apiFetch<void>(`/v1/roles/${id(roleId)}/members/${id(userId)}`, { method: "PUT" }),
  removeMember: (roleId: string, userId: string) => apiFetch<void>(`/v1/roles/${id(roleId)}/members/${id(userId)}`, { method: "DELETE" }),
  resources: () => apiFetch<{ items: Resource[] }>("/v1/permissions/resources"),
};

export interface ProductInput {
  name: string;
  price: string;
  enabled: boolean;
}

export const products = {
  list: (p: { page: number; limit: number; search?: string; sort?: string; order?: "asc" | "desc"; enabled?: boolean }) =>
    apiFetch<Paged<Product>>(`/v1/products${query(p)}`),
  get: (productId: string) => apiFetch<Product>(`/v1/products/${id(productId)}`),
  create: (input: ProductInput) => apiFetch<Product>("/v1/products", { method: "POST", json: input }),
  update: (productId: string, input: Partial<ProductInput>) => apiFetch<Product>(`/v1/products/${id(productId)}`, { method: "PATCH", json: input }),
  remove: (productId: string) => apiFetch<void>(`/v1/products/${id(productId)}`, { method: "DELETE" }),
  removeMany: (ids: string[]) => apiFetch<void>("/v1/products/batch-delete", { method: "POST", json: { ids } }),
};
