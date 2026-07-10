package app

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/stroppy-io/stroppy-cloud/internal/ide"
)

// ideTicketHandler exposes the authenticated half of the browser-auth IDE
// handshake (see internal/ide/ticket.go's package doc): the SPA's "Open in
// IDE" button calls this endpoint with its existing Bearer access token
// (a normal fetch/XHR, which — unlike the subsequent browser navigation to
// /ide/<scope>/... — CAN carry a custom header), and gets back a
// short-lived single-use ticket plus the URL to navigate to. It is a plain
// net/http handler (not a connect/proto RPC): minting a ticket needs
// nothing beyond "verify the Bearer header against the target scope",
// which internal/ide.TicketIssuer already does byte-for-byte via
// Authorizer.CanAuthor — adding a proto service for this single call would
// only duplicate that check behind generated code for no benefit.
//
// GET /api/ide/ticket?target=/ide/<scope>/...[?other=query]
//
//	200 {"ticket": "...", "url": "/ide/<scope>/...?other=query&ticket=..."}
//	400 target missing/unparsable
//	403 the caller's Bearer token does not authorize target's scope
func ideTicketHandler(issuer *ide.TicketIssuer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if issuer == nil {
			http.NotFound(w, r)
			return
		}
		target := r.URL.Query().Get("target")
		if target == "" {
			http.Error(w, "missing target query parameter", http.StatusBadRequest)
			return
		}
		targetURL, err := url.Parse(target)
		if err != nil || targetURL.Path == "" {
			http.Error(w, "unparsable target query parameter", http.StatusBadRequest)
			return
		}
		ticket, _, err := issuer.Issue(r, targetURL.Path)
		if err != nil {
			http.Error(w, "not authorized to open this IDE scope", http.StatusForbidden)
			return
		}
		q := targetURL.Query()
		q.Set("ticket", ticket)
		targetURL.RawQuery = q.Encode()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			Ticket string `json:"ticket"`
			URL    string `json:"url"`
		}{Ticket: ticket, URL: targetURL.String()})
	}
}
