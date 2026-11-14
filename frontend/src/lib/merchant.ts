// Friendlier names for bank descriptions, and a look (icon and color) per category.
import {
  ArrowLeftRight, Banknote, BookOpen, Briefcase, Car, CircleHelp, Coins, Dog, EyeOff, HeartPulse, Home, Landmark, Plane, Plug,
  Popcorn, Receipt, Repeat, Shield, ShoppingBag, ShoppingCart, TrendingUp, Undo2, Utensils, UtensilsCrossed, Wallet, type LucideIcon,
} from "lucide-react";

const SMALL = new Set(["de", "da", "do", "das", "dos", "e", "em", "a", "o"]);
// Brands whose spelling title case gets wrong.
const BRANDS: Record<string, string> = { ifood: "iFood", "99": "99", "99app": "99", paypal: "PayPal", mercadolivre: "Mercado Livre", youtube: "YouTube" };
// Prefixes banks put before the merchant.
const PREFIXES = [
  /^pagamento de pix qr code\s+/i, /^pix (enviado|recebido)( para| de)?\s+/i, /^transfer[eê]ncia (enviada|recebida)( para| de)?\s+/i,
  /^ted (enviada|recebida)\s+/i, /^compra (no )?(cart[aã]o|d[eé]bito|cr[eé]dito)\s+/i, /^pg \*/i, /^pag\*/i, /^mp \*/i, /^ec \*/i,
];

function titleCase(s: string): string {
  return s.toLowerCase().split(" ").map((w, i) => {
    if (BRANDS[w]) return BRANDS[w];
    if (i > 0 && SMALL.has(w)) return w;
    return w.charAt(0).toUpperCase() + w.slice(1);
  }).join(" ");
}

/** "UBER *TRIP" gives "Uber", "PIX ENVIADO JOAO SILVA" gives "Joao Silva", "NETFLIX.COM" gives "Netflix". */
export function merchantName(description: string): string {
  let s = description.trim();
  for (const p of PREFIXES) s = s.replace(p, "");
  const star = s.indexOf("*");
  if (star > 1) s = s.slice(0, star); // "UBER *TRIP", "IFOOD *PEDIDO": the merchant comes first
  s = s
    .replace(/\b\d{1,2}\/\d{1,2}\b/g, " ") // installment or date
    .replace(/\.(com|net)(\.br)?\b/gi, " ")
    .replace(/[*_#]+/g, " ")
    .replace(/\s+\d+\s*$/g, " ") // trailing codes
    .replace(/[,;:-]+\s*(inc|ltda|s\.?a|me|eireli)\.?\s*$/i, "")
    .replace(/\s+(inc|ltda|s\.?a|me|eireli)\.?$/i, "")
    .replace(/\s+/g, " ")
    .trim();
  return s.length >= 2 ? titleCase(s) : description;
}

const ICONS: [RegExp, LucideIcon][] = [
  [/mercado|supermerc/, ShoppingCart], [/delivery/, UtensilsCrossed], [/restaurante|alimenta/, Utensils], [/transporte|uber|combust/, Car],
  [/moradia|aluguel/, Home], [/contas da casa|energia|luz|internet/, Plug], [/sa[uú]de|farm/, HeartPulse], [/assinatura/, Repeat],
  [/compras/, ShoppingBag], [/lazer/, Popcorn], [/educa/, BookOpen], [/viagem/, Plane], [/imposto|taxa/, Receipt], [/juros|encargo/, Coins],
  [/seguro/, Shield], [/pet/, Dog], [/reembolso/, Undo2], [/sal[aá]rio/, Briefcase], [/rendimento/, TrendingUp], [/investimento/, TrendingUp],
  [/empr[eé]stimo/, Landmark], [/ignorad/, EyeOff], [/transfer|pix|pessoas|entre contas/, ArrowLeftRight], [/receita/, Banknote], [/carteira/, Wallet],
];

/** Fixed hues that read on light and dark; a category keeps its color while the list changes. */
const COLORS = ["#2a78d6", "#eb6834", "#1e9e73", "#9b5bd6", "#d9a21b", "#d6457a", "#3aa8b8", "#7a8a2e"];

export function categoryIcon(name: string): LucideIcon {
  const n = name.toLowerCase();
  return ICONS.find(([re]) => re.test(n))?.[1] ?? CircleHelp;
}

export function categoryColor(name: string): string {
  if (!name) return "var(--muted-foreground)";
  let h = 0;
  for (const ch of name.toLowerCase()) h = (h * 31 + ch.charCodeAt(0)) >>> 0;
  return COLORS[h % COLORS.length];
}

/** Money sent to people or to other accounts: worth a look, not a spending limit. */
export const TRANSFER_LIKE = /pix|transfer/i;
