const ACCENTS: Record<string, string> = {
  a: "á",
  e: "é",
  i: "í",
  o: "ó",
  u: "ú",
  A: "Á",
  E: "É",
  I: "Í",
  O: "Ó",
  U: "Ú",
};

/**
 * Builds the en-XA pseudo-locale text: accents vowels in literal text and brackets
 * the message, so untranslated or clipped strings are easy to spot. ICU syntax is
 * preserved: an argument (`{count, plural, one ... other ...}`) keeps its name,
 * type and selectors, while the nested message bodies (`{# planes}`) are literal
 * text and get accented.
 */
export function pseudoLocalize(message: string): string {
  // Each open brace is either an argument ("arg") or a nested message body ("msg").
  const stack: ("arg" | "msg")[] = [];
  let out = "";
  for (const ch of message) {
    if (ch === "{") {
      stack.push(stack.at(-1) === "arg" ? "msg" : "arg");
      out += ch;
    } else if (ch === "}") {
      stack.pop();
      out += ch;
    } else {
      const literal = stack.length === 0 || stack.at(-1) === "msg";
      out += literal ? (ACCENTS[ch] ?? ch) : ch;
    }
  }
  return `⟦${out}⟧`;
}

/** Applies pseudoLocalize to every message of a catalog. */
export function pseudoCatalog(catalog: Record<string, string>): Record<string, string> {
  const out: Record<string, string> = {};
  for (const [key, value] of Object.entries(catalog)) {
    out[key] = pseudoLocalize(value);
  }
  return out;
}
