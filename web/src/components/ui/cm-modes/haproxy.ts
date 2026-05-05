import { StreamLanguage } from "@codemirror/language";

// haproxyMode: dedicated StreamLanguage for haproxy.cfg.
//
// HAProxy configs have a recognisable shape:
//   global / defaults / frontend / backend / listen / resolvers / peers / …
//   are top-level section keywords.
//   Inside, a small fixed set of directives (bind, server, option, timeout,
//   mode, acl, http-check, balance, default_backend, use_backend, …) starts
//   each meaningful line.
//
// Highlighting these as keywords (vs propertyName for everything else) makes
// the file readable when an operator edits it — section boundaries pop, and
// the noisy tail (TLS suites, ACL expressions) calms down to plain text.
const SECTIONS = new Set([
  "global", "defaults", "frontend", "backend", "listen", "resolvers", "peers",
  "cache", "http-errors", "mailers", "ring", "userlist", "program",
]);
const DIRECTIVES = new Set([
  "bind", "server", "option", "timeout", "mode", "acl", "balance", "log",
  "maxconn", "default_backend", "use_backend", "use-server",
  "http-request", "http-response", "tcp-request", "tcp-response",
  "http-check", "tcp-check", "external-check", "check",
  "stick", "stick-table", "stats", "errorfile", "monitor-uri",
  "retries", "redirect", "rate-limit", "compression", "no-tls-tickets",
  "tune.ssl.default-dh-param", "ca-base", "crt-base", "ssl-default-bind-options",
  "ssl-default-bind-ciphers", "ssl-default-server-ciphers",
  "user", "group", "daemon", "pidfile", "chroot", "node",
  "description", "nbproc", "nbthread", "cpu-map", "ulimit-n",
  "frontend", "backend", "default-server", "fullconn", "source",
  "bind-process", "capture", "rspadd", "reqadd", "rspirep", "reqirep",
  "block", "redispatch", "persist", "appsession", "cookie", "dispatch",
  "errorloc", "errorloc302", "errorloc303", "force-persist", "ignore-persist",
  "max-keep-alive-queue", "monitor", "monitor-net", "monitor fail",
]);

export const haproxyMode = StreamLanguage.define<{ atLineStart: boolean }>({
  name: "haproxy",
  startState: () => ({ atLineStart: true }),
  token(stream, state) {
    if (stream.sol()) state.atLineStart = true;
    if (stream.eatSpace()) return null;

    if (stream.peek() === "#") {
      stream.skipToEnd();
      return "comment";
    }

    if (stream.match(/"(?:[^"\\]|\\.)*"/)) return "string";

    // CIDR / IPv4 (`*:5000`, `127.0.0.1`, `0.0.0.0/0`).
    if (stream.match(/\d{1,3}(?:\.\d{1,3}){3}(?:\/\d{1,3})?(?::\d+)?/)) return "number";
    if (stream.match(/\*:\d+/)) return "number";

    // Numbers with optional time/size unit.
    if (stream.match(/\d+(?:\.\d+)?(?:MB|GB|KB|kB|ms|s|min|h|d|us)?\b/)) return "number";

    if (state.atLineStart) {
      const m = stream.match(/[A-Za-z_][A-Za-z0-9_.\-]*/);
      if (m) {
        state.atLineStart = false;
        const word = (m as RegExpMatchArray)[0].toLowerCase();
        if (SECTIONS.has(word)) return "keyword";
        if (DIRECTIVES.has(word)) return "propertyName";
        return null;
      }
    }

    // ACL operators / function-style options inside a directive.
    if (stream.match(/\b(?:if|unless|or|and|not)\b/)) return "operatorKeyword";

    // Identifier or anything else.
    if (stream.match(/[A-Za-z_][A-Za-z0-9_.\-]*/)) return null;

    stream.next();
    return null;
  },
  languageData: {
    commentTokens: { line: "#" },
  },
});
