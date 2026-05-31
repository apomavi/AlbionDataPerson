export const backendBase =
  process.env.NEXT_PUBLIC_BACKEND_URL?.replace(/\/$/, "") ??
  "http://localhost:8082";

export function backendUrl(path: string) {
  if (path.startsWith("/")) {
    return `${backendBase}${path}`;
  }
  return `${backendBase}/${path}`;
}
