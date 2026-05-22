import { StreamLanguage } from "@codemirror/language";

// pgHbaMode: dedicated StreamLanguage for PostgreSQL pg_hba.conf.
//
// pg_hba.conf is column-based, not key=value:
//   <type> <database> <user> [<address>] <method> [<options>]
//
// Examples:
//   local   all             postgres                                peer
//   host    all             all             127.0.0.1/32            scram-sha-256
//   host    replication     replicator      0.0.0.0/0               md5
//   hostssl all             all             ::/0                    cert clientcert=verify-full
//
// Highlighting strategy:
//   - first word on the line  →  connection type   (keyword)
//   - `all` and `replication`  →  atom              (wildcards / built-in db names)
//   - IP / CIDR / IPv6         →  number
//   - known auth methods       →  atom              (trust, md5, scram-sha-256, …)
//   - options like key=value   →  propertyName + operator
const TYPES = new Set([
  "local", "host", "hostssl", "hostnossl",
  "hostgssenc", "hostnogssenc",
]);
const METHODS = new Set([
  "trust", "reject", "scram-sha-256", "md5", "password", "gss", "sspi",
  "ident", "peer", "ldap", "radius", "cert", "pam", "bsd", "krb5",
]);

export const pgHbaMode = StreamLanguage.define<{ atLineStart: boolean }>({
  name: "pg-hba",
  startState: () => ({ atLineStart: true }),
  token(stream, state) {
    if (stream.sol()) state.atLineStart = true;
    if (stream.eatSpace()) return null;

    if (stream.peek() === "#") {
      stream.skipToEnd();
      return "comment";
    }

    // IPv4 / CIDR — pg_hba uses these for the address column.
    if (stream.match(/\d{1,3}(?:\.\d{1,3}){3}(?:\/\d{1,3})?/)) return "number";
    // IPv6 — rough match (`::`, `::1`, `fe80::/10`, full forms).
    if (stream.match(/[0-9a-fA-F:]+:[0-9a-fA-F:]*(?:\/\d{1,3})?/)) return "number";
    // Hostname pattern — bare ASCII with dots (e.g. mydb.internal).
    // Skip — we let it fall through as identifier so it doesn't fight METHODS.

    // Connection type at the start of the line.
    if (state.atLineStart) {
      const m = stream.match(/[A-Za-z][A-Za-z0-9_-]*/);
      if (m) {
        state.atLineStart = false;
        const w = (m as RegExpMatchArray)[0].toLowerCase();
        if (TYPES.has(w)) return "keyword";
        // Fall through to method/atom matching for the same word.
        if (METHODS.has(w)) return "atom";
        return "propertyName";
      }
    }

    // option=value pair (e.g. clientcert=verify-full, map=usermap).
    const optMatch = stream.match(/[A-Za-z_][A-Za-z0-9_-]*(?==)/) as RegExpMatchArray | null;
    if (optMatch) return "propertyName";
    if (stream.eat("=")) return "operator";

    // Identifier-ish word — could be db name, user, or auth method.
    const word = stream.match(/[A-Za-z][A-Za-z0-9_-]*/) as RegExpMatchArray | null;
    if (word) {
      const w = word[0].toLowerCase();
      if (METHODS.has(w)) return "atom";
      if (w === "all" || w === "replication" || w === "samerole" || w === "samegroup") return "atom";
      return null;
    }

    // Quoted strings (e.g. "user with spaces").
    if (stream.match(/"(?:[^"\\]|\\.)*"/)) return "string";
    if (stream.match(/'(?:[^'\\]|\\.)*'/)) return "string";

    stream.next();
    return null;
  },
  languageData: {
    commentTokens: { line: "#" },
  },
});
