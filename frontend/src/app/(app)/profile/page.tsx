import { createClient } from "@/lib/supabase/server";
import { messages } from "@/constants/messages";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export default async function ProfilePage() {
  const supabase = await createClient();
  const { data: { user } } = await supabase.auth.getUser();
  const p = messages.profile;
  const displayName = (user?.user_metadata?.display_name as string | undefined) || p.notSet;
  const since = user?.created_at ? new Date(user.created_at).toLocaleDateString("en-US", { dateStyle: "medium" }) : p.notSet;

  return (
    <Card className="max-w-xl">
      <CardHeader>
        <CardTitle>{p.title}</CardTitle>
        <CardDescription>{p.description}</CardDescription>
      </CardHeader>
      <CardContent>
        <dl className="grid grid-cols-1 gap-4 sm:grid-cols-[10rem_1fr]">
          <dt className="text-sm text-muted-foreground">{p.email}</dt>
          <dd className="break-all">{user?.email}</dd>
          <dt className="text-sm text-muted-foreground">{p.displayName}</dt>
          <dd>{displayName}</dd>
          <dt className="text-sm text-muted-foreground">{p.memberSince}</dt>
          <dd>{since}</dd>
        </dl>
      </CardContent>
    </Card>
  );
}
