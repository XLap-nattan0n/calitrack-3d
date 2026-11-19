import Link from "next/link";
import { redirect } from "next/navigation";
import { createClient } from "@/lib/supabase/server";
import { messages } from "@/constants/messages";
import { Button } from "@/components/ui/button";

export default async function AppLayout({ children }: { children: React.ReactNode }) {
  const supabase = await createClient();
  const { data: { user } } = await supabase.auth.getUser();
  if (!user) redirect("/login"); // defense in depth; middleware already guards

  return (
    <div className="min-h-screen">
      <header className="border-b">
        <div className="mx-auto flex min-h-14 max-w-5xl flex-col items-start gap-1 px-4 py-2 sm:flex-row sm:items-center sm:justify-between sm:gap-2 sm:py-0">
          <Link href="/exercises" className="font-semibold">{messages.app.name}</Link>
          <nav className="-mx-2 flex w-full items-center gap-1 overflow-x-auto px-2 pb-1 sm:mx-0 sm:w-auto sm:overflow-visible sm:px-0 sm:pb-0 [&>*]:shrink-0">
            <Button asChild variant="ghost" size="sm"><Link href="/exercises">{messages.nav.exercises}</Link></Button>
            <Button asChild variant="ghost" size="sm"><Link href="/planner">{messages.nav.planner}</Link></Button>
            <Button asChild variant="ghost" size="sm"><Link href="/workout">{messages.nav.workout}</Link></Button>
            <Button asChild variant="ghost" size="sm"><Link href="/history">{messages.nav.history}</Link></Button>
            <Button asChild variant="ghost" size="sm"><Link href="/analytics">{messages.nav.analytics}</Link></Button>
            <Button asChild variant="ghost" size="sm"><Link href="/anatomy">{messages.nav.anatomy}</Link></Button>
            <Button asChild variant="ghost" size="sm"><Link href="/profile">{messages.nav.profile}</Link></Button>
            <form action="/auth/signout" method="post">
              <Button type="submit" variant="outline" size="sm">{messages.nav.logout}</Button>
            </form>
          </nav>
        </div>
      </header>
      <main className="mx-auto max-w-5xl p-4 sm:p-6">{children}</main>
    </div>
  );
}
