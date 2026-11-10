"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Loader2 } from "lucide-react";
import { AuthCard } from "@/components/auth-card";
import { FormError } from "@/components/bits";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ApiError, authPost } from "@/lib/api/client";
import { errorMessage } from "@/lib/errors";
import { useI18n } from "@/lib/i18n/provider";

interface RegisterResult {
  user_id: string;
  status: string;
  activation_token?: string;
}

export default function RegisterPage() {
  const { t } = useI18n();
  const router = useRouter();
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);
  const [sent, setSent] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setPending(true);
    setError(null);
    try {
      const res = await authPost<RegisterResult>("register", { name, email });
      // The token comes back only when the server delivers it by hand (no mail set up).
      if (res.activation_token) router.push(`/activate#token=${encodeURIComponent(res.activation_token)}`);
      else setSent(true);
    } catch (err) {
      // 403 here means AUTH_ALLOW_REGISTRATION is off, not a missing permission.
      setError(err instanceof ApiError && err.code === "FORBIDDEN" ? t("auth.registration_closed") : errorMessage(err, "auth.register_failed", t));
    } finally {
      setPending(false);
    }
  };

  const footer = <>{t("auth.have_account")} <Link href="/login" className="font-medium text-primary hover:underline">{t("auth.login_action")}</Link></>;

  if (sent) {
    return (
      <AuthCard title={t("auth.check_email_title")} subtitle={t("auth.check_email_activate")} footer={footer}>
        <Button className="w-full" nativeButton={false} render={<Link href="/activate" />}>{t("auth.have_token")}</Button>
      </AuthCard>
    );
  }

  return (
    <AuthCard title={t("auth.create_account")} subtitle={t("auth.register_subtitle")} footer={footer}>
      <form onSubmit={submit} className="space-y-5">
        <FormError message={error} />
        <div className="space-y-2">
          <Label htmlFor="name">{t("auth.name")}</Label>
          <Input id="name" autoComplete="name" required minLength={2} maxLength={80} autoFocus value={name} onChange={(e) => setName(e.target.value)} />
        </div>
        <div className="space-y-2">
          <Label htmlFor="email">{t("auth.email")}</Label>
          <Input id="email" type="email" autoComplete="email" required value={email} onChange={(e) => setEmail(e.target.value)} />
        </div>
        <Button type="submit" className="w-full" disabled={pending}>
          {pending && <Loader2 className="size-4 animate-spin" />}
          {t("auth.create_account")}
        </Button>
      </form>
    </AuthCard>
  );
}
