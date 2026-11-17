import { createServerClient, type CookieOptions } from "@supabase/ssr";
import { cookies } from "next/headers";
import { getSupabaseEnv } from "./env";

type CookieToSet = { name: string; value: string; options: CookieOptions };

export async function createClient() {
  const cookieStore = await cookies();
  const { url, anonKey } = getSupabaseEnv();
  return createServerClient(url, anonKey, {
    cookies: {
      getAll: () => cookieStore.getAll(),
      setAll(list: CookieToSet[]) {
        try {
          list.forEach(({ name, value, options }) => cookieStore.set(name, value, options));
        } catch {
          // Called from a Server Component; middleware refreshes the session instead.
        }
      },
    },
  });
}
