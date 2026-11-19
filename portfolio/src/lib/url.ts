/* ============================================================================
   Site-internal URLs. The site is a GitHub *project* page served under the
   Astro `base` (/fibtransponder/), in dev as well as in the deployed build, so
   a root-absolute href like /demos would 404. Build every internal link with
   withBase.
   ========================================================================= */

const BASE = import.meta.env.BASE_URL.replace(/\/$/, '');

/** Prefix a root-relative path ('/demos', '/demo/x', '/method#a') with the base. */
export function withBase(path: string): string {
  return `${BASE}${path}`;
}

/** The inverse of withBase: the request path relative to the base, '/' for the root. */
export function stripBase(pathname: string): string {
  const rest = pathname.startsWith(BASE) ? pathname.slice(BASE.length) : pathname;
  return rest || '/';
}
