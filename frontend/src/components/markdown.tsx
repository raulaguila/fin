import { Fragment } from "react";

export type Block = { kind: "p"; text: string } | { kind: "h"; text: string } | { kind: "ul"; items: string[] } | { kind: "ol"; items: string[] };

const BULLET = /^\s*[-*•]\s+/;
const NUMBER = /^\s*\d+[.)]\s+/;

/** Splits model text into paragraphs, headings and lists; nothing else of Markdown is needed. */
export function blocks(text: string): Block[] {
  const out: Block[] = [];
  let para: string[] = [];
  const flush = () => {
    if (para.length) out.push({ kind: "p", text: para.join("\n") });
    para = [];
  };
  for (const line of text.split("\n")) {
    const list = BULLET.test(line) ? "ul" : NUMBER.test(line) ? "ol" : null;
    if (list) {
      flush();
      const item = line.replace(list === "ul" ? BULLET : NUMBER, "");
      const last = out.at(-1);
      if (last?.kind === list) last.items.push(item);
      else out.push({ kind: list, items: [item] });
    } else if (/^#{1,6}\s/.test(line)) {
      flush();
      out.push({ kind: "h", text: line.replace(/^#+\s*/, "") });
    } else if (line.trim() === "") flush();
    else para.push(line);
  }
  flush();
  return out;
}

/** **bold** and `code`; the rest stays text, so nothing from the model becomes HTML. */
function Inline({ text }: { text: string }) {
  return text.split(/(\*\*[^*]+\*\*|`[^`]+`)/g).map((part, i) =>
    part.startsWith("**") && part.endsWith("**") && part.length > 4 ? <strong key={i}>{part.slice(2, -2)}</strong>
      : part.startsWith("`") && part.endsWith("`") && part.length > 2 ? <code key={i} className="rounded bg-muted px-1 font-mono text-[0.9em]">{part.slice(1, -1)}</code>
        : <Fragment key={i}>{part}</Fragment>);
}

export function Markdown({ text }: { text: string }) {
  return (
    <div className="space-y-2">
      {blocks(text).map((b, i) => {
        if (b.kind === "h") return <p key={i} className="font-semibold"><Inline text={b.text} /></p>;
        if (b.kind === "p") return <p key={i} className="whitespace-pre-wrap"><Inline text={b.text} /></p>;
        const List = b.kind;
        return (
          <List key={i} className={b.kind === "ul" ? "list-disc space-y-1 pl-5" : "list-decimal space-y-1 pl-5"}>
            {b.items.map((item, j) => <li key={j}><Inline text={item} /></li>)}
          </List>
        );
      })}
    </div>
  );
}
