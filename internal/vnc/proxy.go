// Package vnc reverse-proxies the noVNC web viewer (served by websockify on a
// loopback port) under the API server's /vnc/ path, so the live desktop is
// reachable on the same public domain as the rest of the API.
//
// The VNC session itself is protected at the VNC layer (x11vnc password), not by
// the API key: noVNC loads static assets and opens its websocket with URLs
// relative to /vnc/ that cannot carry the ?key= query param, so API-key auth
// cannot gate this path. main.go therefore adds /vnc/ to the auth allow list.
package vnc

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

// Handler returns a reverse proxy that forwards /vnc/* to the local noVNC
// server (websockify) at upstream, e.g. "http://127.0.0.1:6080". The /vnc
// prefix is stripped before forwarding. httputil.ReverseProxy transparently
// handles the websocket upgrade websockify needs and passes the query string
// through on the upgrade request.
//
// frameAncestors controls who may embed the viewer in an <iframe>. It becomes
// the `frame-ancestors` directive of a Content-Security-Policy response header,
// and any upstream X-Frame-Options / CSP is stripped so the policy is
// authoritative. Use "*" to allow embedding on any site, or a space-separated
// origin list (e.g. "https://app.example.com https://admin.example.com").
func Handler(upstream, frameAncestors string) (http.Handler, error) {
	target, err := url.Parse(upstream)
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Rewrite so the Host header points at the upstream and the /vnc prefix is
	// dropped: /vnc/vnc.html -> /vnc.html, /vnc/websockify -> /websockify.
	director := proxy.Director
	proxy.Director = func(r *http.Request) {
		director(r)
		r.Host = target.Host
		r.URL.Path = stripPrefix(r.URL.Path, "/vnc")
	}

	if frameAncestors == "" {
		frameAncestors = "*"
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		// Remove upstream framing controls, then set our own so the viewer can
		// be embedded per frameAncestors. Without this, a default
		// X-Frame-Options would make browsers refuse to render the iframe.
		resp.Header.Del("X-Frame-Options")
		resp.Header.Set("Content-Security-Policy", "frame-ancestors "+frameAncestors)
		return nil
	}

	// Convenience entry point: hitting /vnc/ (or /vnc) redirects to the noVNC
	// viewer with all the boilerplate query params filled in, and — since the
	// VNC password defaults to API_KEY — reuses the same ?key= as the password.
	// So one URL, one secret: /vnc/?key=SECRET. Extra params (view_only) pass
	// through. Everything else (vnc.html, assets, websocket) hits the proxy.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/vnc" || r.URL.Path == "/vnc/" {
			q := url.Values{}
			q.Set("path", "vnc/websockify")
			q.Set("autoconnect", "1")
			q.Set("resize", "scale")
			if key := r.URL.Query().Get("key"); key != "" {
				q.Set("password", key)
			}
			if vo := r.URL.Query().Get("view_only"); vo != "" {
				q.Set("view_only", vo)
			}
			w.Header().Set("Content-Security-Policy", "frame-ancestors "+frameAncestors)
			http.Redirect(w, r, "/vnc/vnc.html?"+q.Encode(), http.StatusFound)
			return
		}
		proxy.ServeHTTP(w, r)
	}), nil
}

// stripPrefix returns path with prefix removed, guaranteeing a leading slash.
// "/vnc" -> "/", "/vnc/vnc.html" -> "/vnc.html".
func stripPrefix(path, prefix string) string {
	if len(path) < len(prefix) || path[:len(prefix)] != prefix {
		return path
	}
	rest := path[len(prefix):]
	if rest == "" {
		return "/"
	}
	if rest[0] != '/' {
		return "/" + rest
	}
	return rest
}
