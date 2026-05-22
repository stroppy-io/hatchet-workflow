import { StreamLanguage } from "@codemirror/language";

// iniLike: shared StreamLanguage for INI-style configs that show up in
// stroppy runs — postgresql.conf, pg_hba.conf, my.cnf, pgbouncer.ini.
//
// What it actually colours:
//   - `#` and `;` line comments
//   - `[section]` headers          → meta
//   - `key = value` / `key value`  → key as propertyName, `=` as operator
//   - true/false/on/off/yes/no     → atom
//   - "double" and 'single' strings
//   - numbers with optional unit (MB, GB, ms, s, min, h, d)
//   - IPv4 / IPv4 CIDR (pg_hba addresses)
//   - bare identifiers (db / user / role names in pg_hba) at column start
//
// Token tags map onto the standard @lezer/highlight palette so any CM6
// theme (including the default `dark`) styles them without extra wiring.
export const iniLike = StreamLanguage.define<{ atLineStart: boolean }>({
  name: "ini-like",
  startState: () => ({ atLineStart: true }),
  token(stream, state) {
    if (stream.sol()) state.atLineStart = true;
    if (stream.eatSpace()) return null;

    // Comments — to EOL.
    if (stream.peek() === "#" || stream.peek() === ";") {
      stream.skipToEnd();
      return "comment";
    }

    // [section]
    if (state.atLineStart && stream.match(/\[[^\]]*\]/)) {
      state.atLineStart = false;
      return "meta";
    }

    // Quoted strings (greedy, no escape handling — fine for config snippets).
    if (stream.match(/"(?:[^"\\]|\\.)*"/)) return "string";
    if (stream.match(/'(?:[^'\\]|\\.)*'/)) return "string";

    // `=` as the assignment operator.
    if (stream.eat("=")) {
      state.atLineStart = false;
      return "operator";
    }

    // CIDR / IPv4 (`127.0.0.1/32`, `0.0.0.0/0`).
    if (stream.match(/\d{1,3}(?:\.\d{1,3}){3}(?:\/\d{1,3})?/)) return "number";

    // Numbers with optional unit suffix (`4MB`, `1d`, `100ms`, `25%`).
    if (stream.match(/\d+(?:\.\d+)?(?:MB|GB|KB|kB|TB|TB|ms|s|min|h|d|B|b|%)?\b/)) return "number";

    // Booleans / on-off keywords.
    if (stream.match(/\b(?:true|false|yes|no|on|off|null|none)\b/i)) return "atom";

    // Identifier at the start of a non-empty line is the key.
    if (state.atLineStart && stream.match(/[A-Za-z_][A-Za-z0-9_.\-]*/)) {
      state.atLineStart = false;
      return "propertyName";
    }

    // Anything else: identifier-ish run.
    if (stream.match(/[A-Za-z_][A-Za-z0-9_.\-]*/)) {
      return null;
    }

    stream.next();
    return null;
  },
  languageData: {
    commentTokens: { line: "#" },
  },
});
