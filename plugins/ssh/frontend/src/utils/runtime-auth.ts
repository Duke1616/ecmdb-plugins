const SYSTEM_NAME = "ecmdb"
const ACCESS_TOKEN_KEY = `${SYSTEM_NAME}-access-token-key`
const TOKEN_CARRIER_KEY = `${SYSTEM_NAME}-token-carrier-key`
const USER_STORE_KEY = "user"

interface UserStoreState {
  currentTenantId?: number
}

export function getAccessToken(): string {
  if (typeof localStorage === "undefined") return ""
  return localStorage.getItem(ACCESS_TOKEN_KEY) || ""
}

export function getTokenCarrier(): "token" | "cookie" {
  if (typeof localStorage === "undefined") return "cookie"
  const carrier = localStorage.getItem(TOKEN_CARRIER_KEY)
  if (carrier === "token" || carrier === "cookie") return carrier
  return getAccessToken() ? "token" : "cookie"
}

export function shouldUseBearerCredential(): boolean {
  return getTokenCarrier() === "token" && Boolean(getAccessToken())
}

export function getAuthorizationHeader(): string {
  if (!shouldUseBearerCredential()) return ""
  const token = getAccessToken()
  return token ? `Bearer ${token}` : ""
}

export function getActiveTenantHeader(): string {
  try {
    const raw = localStorage.getItem(USER_STORE_KEY)
    if (!raw) return ""
    const parsed = JSON.parse(raw) as { currentTenantId?: number; state?: UserStoreState }
    const tenantId = parsed?.currentTenantId ?? parsed?.state?.currentTenantId
    return tenantId ? String(tenantId) : ""
  } catch {
    return ""
  }
}

export const getRuntimeRequestHeaders = (extra?: Record<string, string>): Record<string, string> => {
  const headers: Record<string, string> = {
    ...(extra || {})
  }

  const auth = getAuthorizationHeader()
  if (auth) {
    headers.Authorization = auth
  }

  const tenantId = getActiveTenantHeader()
  if (tenantId) {
    headers["X-Active-Tenant-ID"] = tenantId
  }

  return headers
}


