"use client";

import { useEffect, useRef, useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { Bot, Loader2, RotateCcw, Send } from "lucide-react";
import { useFinanceStatus } from "@/components/finance";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { finance, type ChatMessage } from "@/lib/api/finance";
import { errorMessage } from "@/lib/errors";
import { useI18n } from "@/lib/i18n/provider";
import { cn } from "@/lib/utils";

const SUGGESTIONS = ["fin.ai.s1", "fin.ai.s2", "fin.ai.s3", "fin.ai.s4"] as const;

/** The assistant floats over every screen; the chat opens in a side drawer and lasts until the tab closes. */
export function AssistantButton() {
  const { t } = useI18n();
  const status = useFinanceStatus();
  const [open, setOpen] = useState(false);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [draft, setDraft] = useState("");
  const [error, setError] = useState("");
  const end = useRef<HTMLDivElement>(null);

  useEffect(() => {
    end.current?.scrollIntoView({ block: "end", behavior: "smooth" });
  }, [messages, open]);

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

  // No AI provider configured: nothing to offer, so no button.
  if (!status.data?.assistant) return null;

  return (
    <>
      <Button
        aria-label={t("nav.assistant")}
        title={t("nav.assistant")}
        className="fixed right-4 bottom-4 z-30 size-12 rounded-full shadow-lg"
        onClick={() => setOpen(true)}
      >
        <Bot className="size-5" />
      </Button>
      <Sheet open={open} onOpenChange={setOpen}>
        <SheetContent side="right" className="gap-0 data-[side=right]:w-full data-[side=right]:sm:max-w-md">
          <SheetHeader className="border-b pr-12">
            <SheetTitle>{t("nav.assistant")}</SheetTitle>
            <SheetDescription>{t("fin.ai.note")}</SheetDescription>
          </SheetHeader>
          <div className="min-h-0 flex-1 space-y-3 overflow-y-auto p-4" aria-live="polite">
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
          <form className="flex items-end gap-2 border-t p-4" onSubmit={(e) => { e.preventDefault(); send(draft); }}>
            {messages.length > 0 && (
              <Button type="button" variant="ghost" size="icon" aria-label={t("fin.ai.clear")} title={t("fin.ai.clear")} onClick={() => { setMessages([]); setError(""); }}>
                <RotateCcw className="size-4" />
              </Button>
            )}
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
            <Button type="submit" size="icon" aria-label={t("fin.ai.send")} disabled={!draft.trim() || ask.isPending}><Send className="size-4" /></Button>
          </form>
        </SheetContent>
      </Sheet>
    </>
  );
}
