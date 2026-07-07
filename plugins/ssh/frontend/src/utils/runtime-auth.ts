const TOKEN_COOKIE_KEY = "ecmdb-token-key"
const USER_STORE_KEY = "user"

interface UserStoreState {
  currentTenantId?: number
}

const readCookie = (name: string) => {
  const escaped = name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")
  const match = document.cookie.match(new RegExp(`(?:^|; )${escaped}=([^;]*)`))
  return match ? decodeURIComponent(match[1]) : ""
}

export const getAuthorizationHeader = () => {
  const token = readCookie(TOKEN_COOKIE_KEY)
  if (!token) return ""
  return token.startsWith("Bearer ") ? token : `Bearer ${token}`
}

export const getActiveTenantHeader = () => {
  try {
    const raw = localStorage.getItem(USER_STORE_KEY)
    if (!raw) return ""

    const parsed = JSON.parse(raw) as { currentTenantId?: number; state?: UserStoreState }
    const tenantId = parsed?.currentTenantId ?? parsed?.state?.currentTenantId
    if (!tenantId) return ""
    return String(tenantId)
  } catch {
    return ""
  }
}

export const getRuntimeRequestHeaders = (extra?: Record<string, string>) => {
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
