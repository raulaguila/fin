"use client";

import { useEffect, useRef, useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { Bot, Loader2, RotateCcw, Send } from "lucide-react";
import { useFinanceStatus } from "@/components/finance";
import { PageBody, PageHeader } from "@/components/bits";
import { EmptyState } from "@/components/empty-state";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { finance, type ChatMessage } from "@/lib/api/finance";
import { errorMessage } from "@/lib/errors";
import { useI18n } from "@/lib/i18n/provider";
import { cn } from "@/lib/utils";

const SUGGESTIONS = ["fin.ai.s1", "fin.ai.s2", "fin.ai.s3", "fin.ai.s4"] as const;

export default function AssistantPage() {
  const { t } = useI18n();
  const status = useFinanceStatus();
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [draft, setDraft] = useState("");
  const [error, setError] = useState("");
  const end = useRef<HTMLDivElement>(null);

  // The conversation lives in this page only; the server keeps none.
  useEffect(() => {
    end.current?.scrollIntoView({ block: "end", behavior: "smooth" });
  }, [messages]);

  const ask = useMutation({
    mutationFn: (history: ChatMessage[]) => finance.chat(history),
    onSuccess: ({ answer }) => setMessages((m) => [...m, { role: "assistant", content: answer }]),
    onError: (err) => setError(errorMessage(err, "fin.ai.failed", t)),
  });
  const send = (text: string) => {
    const content = text.trim();
    if (!content || ask.isPending) return;
    const history: ChatMessage[] = [...messages, { role: "user", content }];
    setMessages(history);
    setDraft("");
    setError("");
    ask.mutate(history);
  };

  if (status.data && !status.data.assistant) {
    return (
      <>
        <PageHeader title={t("nav.assistant")} description={t("fin.ai.subtitle")} />
        <PageBody><EmptyState icon={Bot} title={t("fin.ai.disabled_title")} description={t("fin.ai.disabled_body")} /></PageBody>
      </>
    );
  }

  return (
    <>
      <PageHeader title={t("nav.assistant")} description={t("fin.ai.subtitle")}
        action={messages.length > 0 && <Button variant="outline" onClick={() => { setMessages([]); setError(""); }}><RotateCcw className="size-4" />{t("fin.ai.clear")}</Button>} />
      <PageBody className="mx-auto flex max-w-3xl flex-col">
        <div className="space-y-3" aria-live="polite">
          {messages.length === 0 && (
            <div className="flex flex-wrap gap-2">
              {SUGGESTIONS.map((k) => (
                <Button key={k} variant="outline" size="sm" className="h-auto whitespace-normal py-1.5 text-left" onClick={() => send(t(k))}>{t(k)}</Button>
              ))}
            </div>
          )}
          {messages.map((m, i) => (
            <div key={i} className={cn("flex", m.role === "user" ? "justify-end" : "justify-start")}>
              <Card className={cn("max-w-[85%] gap-0 whitespace-pre-wrap px-4 py-2.5 text-sm leading-relaxed", m.role === "user" && "bg-primary text-primary-foreground")}>
                {m.content}
              </Card>
            </div>
          ))}
          {ask.isPending && (
            <p className="flex items-center gap-2 text-sm text-muted-foreground"><Loader2 className="size-4 animate-spin" />{t("fin.ai.thinking")}</p>
          )}
          {error && <p className="text-sm text-red-700 dark:text-red-400" role="alert">{error}</p>}
          <div ref={end} />
        </div>
        <form className="sticky bottom-0 flex items-end gap-2 bg-background py-3" onSubmit={(e) => { e.preventDefault(); send(draft); }}>
          <textarea
            value={draft}
            rows={2}
            maxLength={4000}
            aria-label={t("fin.ai.placeholder")}
            placeholder={t("fin.ai.placeholder")}
            className="min-h-[var(--control-h)] flex-1 resize-none rounded-lg border bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.shiftKey) {
                e.preventDefault();
                send(draft);
              }
            }}
          />
          <Button type="submit" disabled={!draft.trim() || ask.isPending}><Send className="size-4" />{t("fin.ai.send")}</Button>
        </form>
        <p className="text-xs text-muted-foreground">{t("fin.ai.note")}</p>
      </PageBody>
    </>
  );
}
