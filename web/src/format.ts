const GB = 1073741824;
const MB = 1048576;
const KB = 1024;

export function bytes(n: number): string {
  if (n >= GB) return `${round(n / GB)} ГБ`;
  if (n >= MB) return `${Math.round(n / MB).toLocaleString("ru-RU")} МБ`;
  return `${Math.max(1, Math.round(n / KB)).toLocaleString("ru-RU")} КБ`;
}

export function count(n: number): string {
  return n.toLocaleString("ru-RU");
}

export function plural(n: number, one: string, few: string, many: string): string {
  const tens = n % 100;
  if (tens >= 11 && tens <= 14) return many;
  const unit = n % 10;
  if (unit === 1) return one;
  if (unit >= 2 && unit <= 4) return few;
  return many;
}

export function files(n: number): string {
  return `${count(n)} ${plural(n, "файл", "файла", "файлов")}`;
}

export function packages(n: number): string {
  return `${count(n)} ${plural(n, "пакет", "пакета", "пакетов")}`;
}

function round(v: number): string {
  return (Math.round(v * 10) / 10).toLocaleString("ru-RU", {
    minimumFractionDigits: 1,
    maximumFractionDigits: 1,
  });
}

export function shortPath(p: string, max = 56): string {
  const chars = Array.from(p);
  if (chars.length <= max) return p;
  return "…" + chars.slice(chars.length - max + 1).join("");
}

// World descriptions are stored as HTML. The panel shows them as text rather
// than rendering markup it did not write.
export function plain(html: string): string {
  return html
    .replace(/<[^>]*>/g, " ")
    .replace(/&nbsp;/g, " ")
    .replace(/&amp;/g, "&")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&quot;/g, '"')
    .replace(/\s+/g, " ")
    .trim();
}
