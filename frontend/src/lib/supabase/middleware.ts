import { createServerClient, type CookieOptions } from "@supabase/ssr";
import { NextResponse, type NextRequest } from "next/server";
import { getSupabaseEnv } from "./env";

type CookieToSet = { name: string; value: string; options: CookieOptions };

// Routes under the (app) group. Add new top-level protected paths here.
const PROTECTED_PREFIXES = ["/profile"];
const AUTH_PAGES = ["/login", "/register"];

export async function updateSession(request: NextRequest) {
  let response = NextResponse.next({ request });
  const { url, anonKey } = getSupabaseEnv();

  const supabase = createServerClient(url, anonKey, {
    cookies: {
      getAll: () => request.cookies.getAll(),
      setAll(list: CookieToSet[]) {
        list.forEach(({ name, value }) => request.cookies.set(name, value));
        response = NextResponse.next({ request });
        list.forEach(({ name, value, options }) => response.cookies.set(name, value, options));
      },
    },
  });

  // getUser() validates the token with Supabase; do not use getSession() here.
  const { data: { user } } = await supabase.auth.getUser();
  const path = request.nextUrl.pathname;
  const isProtected = PROTECTED_PREFIXES.some((p) => path === p || path.startsWith(`${p}/`));

  const redirectTo = (target: string) => {
    const redirect = request.nextUrl.clone();
    redirect.pathname = target;
    redirect.search = "";
    const res = NextResponse.redirect(redirect);
    response.cookies.getAll().forEach((c) => res.cookies.set(c));
    return res;
  };

  if (!user && isProtected) return redirectTo("/login");
  if (user && AUTH_PAGES.includes(path)) return redirectTo("/profile");
  return response;
}
