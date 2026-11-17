"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import { createClient } from "@/lib/supabase/client";
import { messages } from "@/constants/messages";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

type Mode = "login" | "register";

export function AuthForm({ mode }: { mode: Mode }) {
  const router = useRouter();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const m = messages.auth;
  const isRegister = mode === "register";

  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError(null);
    setNotice(null);
    const form = new FormData(e.currentTarget);
    const email = String(form.get("email") ?? "").trim();
    const password = String(form.get("password") ?? "");
    const displayName = String(form.get("displayName") ?? "").trim();

    if (!email || !password) return setError(m.errors.missingFields);
    if (isRegister && password.length < 8) return setError(m.errors.passwordTooShort);

    setPending(true);
    const supabase = createClient();
    try {
      if (isRegister) {
        const { data, error } = await supabase.auth.signUp({
          email,
          password,
          options: displayName ? { data: { display_name: displayName } } : undefined,
        });
        if (error) return setError(error.message);
        if (!data.session) return setNotice(m.checkEmail); // email confirmation enabled
      } else {
        const { error } = await supabase.auth.signInWithPassword({ email, password });
        if (error) return setError(error.message);
      }
      router.replace("/profile");
      router.refresh();
    } catch {
      setError(m.errors.generic);
    } finally {
      setPending(false);
    }
  }

  return (
    <Card className="w-full max-w-md">
      <CardHeader>
        <CardTitle>{isRegister ? m.registerTitle : m.loginTitle}</CardTitle>
        <CardDescription>{isRegister ? m.registerDescription : m.loginDescription}</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={onSubmit} className="space-y-4" noValidate>
          {isRegister && (
            <div className="space-y-2">
              <Label htmlFor="displayName">{m.displayName}</Label>
              <Input id="displayName" name="displayName" autoComplete="nickname" />
            </div>
          )}
          <div className="space-y-2">
            <Label htmlFor="email">{m.email}</Label>
            <Input id="email" name="email" type="email" autoComplete="email" required />
          </div>
          <div className="space-y-2">
            <Label htmlFor="password">{m.password}</Label>
            <Input
              id="password"
              name="password"
              type="password"
              autoComplete={isRegister ? "new-password" : "current-password"}
              required
            />
          </div>
          <p role="alert" className="min-h-5 text-sm text-destructive">{error}</p>
          {notice && <p role="status" className="text-sm text-muted-foreground">{notice}</p>}
          <Button type="submit" className="w-full" disabled={pending}>
            {pending ? m.submitting : isRegister ? m.submitRegister : m.submitLogin}
          </Button>
        </form>
        <p className="mt-4 text-center text-sm text-muted-foreground">
          {isRegister ? m.haveAccount : m.noAccount}{" "}
          <Link href={isRegister ? "/login" : "/register"} className="font-medium text-foreground underline underline-offset-4">
            {isRegister ? messages.nav.login : messages.nav.register}
          </Link>
        </p>
      </CardContent>
    </Card>
  );
}
