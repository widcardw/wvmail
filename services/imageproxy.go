package services

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

// The image proxy keeps the sender from seeing the reader's IP. The webview
// cannot hide its own address, so remote images are rewritten to this endpoint;
// the fetch happens here instead. In server mode the endpoint runs on the
// deployed host, which is the private path. The desktop build can point at that
// same endpoint with RemoteContentConfig.ImageProxyURL.
//
// This is deliberately not a general-purpose proxy: only http(s), only images
// and fonts, private/link-local destinations are refused, redirects are capped,
// and the response body is bounded. Without those checks an exposed server
// build would be an SSRF primitive.

const (
	imageProxyMaxBytes      = 10 << 20
	imageProxyCacheMaxBytes = 64 << 20
	imageProxyCacheMaxItems = 4096
	imageProxyMaxURLLength  = 8 << 10
	imageProxyMaxRedirects  = 3
	imageProxyTimeout       = 20 * time.Second
	imageProxyDialTimeout   = 10 * time.Second
	imageProxyDefaultMaxAge = time.Hour
	imageProxyMaxMaxAge     = 24 * time.Hour
	imageProxyUserAgent     = "Mozilla/5.0 (compatible; wvmail-image-proxy/1.0; +https://github.com/widcardw/wvmail)"
	imageProxyAccept        = "image/avif,image/webp,image/apng,image/*,font/woff2,font/woff,font/ttf,font/otf,application/font-woff,application/font-woff2,application/x-font-ttf,application/x-font-opentype,application/vnd.ms-fontobject,*/*;q=0.8"
)

var (
	errImageProxyTooLarge        = errors.New("image proxy: response is too large")
	errImageProxyUnsupportedType = errors.New("image proxy: upstream is not an image or font")
)

// ImageProxy serves GET/HEAD /image-proxy?url=<absolute-url>. The handler is
// safe to mount on the Wails asset server in both desktop and server builds.
type ImageProxy struct {
	client       *http.Client
	dialer       *net.Dialer
	allowPrivate bool
	maxBytes     int64
	proxyURL     func() (*url.URL, error)
	cache        *imageProxyCache
}

// NewImageProxy returns the production proxy: private and link-local targets are
// refused, responses are cached in memory, and each fetch is bounded. When the
// app's global proxy is enabled, image fetches use it too — otherwise a mailbox
// that works only through a tunnel would still fail to load its images.
func NewImageProxy() *ImageProxy {
	return newImageProxyWithProxy(false, globalProxyURL)
}

// newImageProxy lets tests reach httptest's loopback server. Production code
// must use NewImageProxy.
func newImageProxy(allowPrivate bool) *ImageProxy {
	return newImageProxyWithProxy(allowPrivate, nil)
}

func newImageProxyWithProxy(allowPrivate bool, proxyURL func() (*url.URL, error)) *ImageProxy {
	p := &ImageProxy{
		dialer:       &net.Dialer{Timeout: imageProxyDialTimeout, KeepAlive: 30 * time.Second},
		allowPrivate: allowPrivate,
		maxBytes:     imageProxyMaxBytes,
		proxyURL:     proxyURL,
		cache:        newImageProxyCache(imageProxyCacheMaxBytes, imageProxyCacheMaxItems),
	}
	transport := &http.Transport{
		// Never inherit HTTP_PROXY: an environment proxy could bypass the IP
		// checks below, and the server's own egress is what we want to measure.
		Proxy:                 nil,
		DialContext:           p.dialContext,
		MaxIdleConns:          32,
		MaxIdleConnsPerHost:   4,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	p.client = &http.Client{
		Transport:     transport,
		Timeout:       imageProxyTimeout,
		CheckRedirect: p.checkRedirect,
	}
	return p
}

func (p *ImageProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	raw := r.URL.Query().Get("url")
	if raw == "" {
		http.Error(w, "missing url", http.StatusBadRequest)
		return
	}
	if len(raw) > imageProxyMaxURLLength {
		http.Error(w, "url is too long", http.StatusRequestURITooLong)
		return
	}

	target, err := url.Parse(raw)
	if err != nil || p.validateTarget(target) != nil {
		http.Error(w, "invalid url", http.StatusBadRequest)
		return
	}
	target.Fragment = ""
	key := target.String()

	if entry, remaining := p.cache.get(key); entry != nil {
		writeProxiedImage(w, r, entry.data, entry.contentType, remaining)
		return
	}

	res, maxAge, err := p.fetch(r.Context(), target)
	if err != nil {
		slog.Debug("image proxy: fetch failed", "host", target.Host, "error", err)
		switch {
		case errors.Is(err, errImageProxyTooLarge):
			http.Error(w, "image is too large", http.StatusRequestEntityTooLarge)
		case errors.Is(err, errImageProxyUnsupportedType):
			http.Error(w, "not an image", http.StatusUnsupportedMediaType)
		default:
			http.Error(w, "upstream fetch failed", http.StatusBadGateway)
		}
		return
	}

	if maxAge > 0 {
		p.cache.put(key, res.data, res.contentType, maxAge)
	}
	writeProxiedImage(w, r, res.data, res.contentType, maxAge)
}

func (p *ImageProxy) validateTarget(target *url.URL) error {
	if target == nil {
		return errors.New("image proxy: missing url")
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return fmt.Errorf("image proxy: unsupported scheme %q", target.Scheme)
	}
	if target.User != nil {
		return errors.New("image proxy: userinfo is not allowed")
	}
	if target.Host == "" || target.Hostname() == "" {
		return errors.New("image proxy: missing host")
	}
	if port := target.Port(); port != "" && !p.allowPrivate {
		switch port {
		case "80", "443", "8080", "8443":
		default:
			return fmt.Errorf("image proxy: port %s is not allowed", port)
		}
	}
	return nil
}

func (p *ImageProxy) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= imageProxyMaxRedirects {
		return errors.New("image proxy: too many redirects")
	}
	// Never tell the next host where the image came from.
	req.Header.Del("Referer")
	return p.validateTarget(req.URL)
}

func (p *ImageProxy) dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	host = strings.Trim(host, "[]")
	if host == "" {
		return nil, errors.New("image proxy: empty host")
	}

	if p.proxyURL != nil {
		proxyURL, err := p.proxyURL()
		if err != nil {
			return nil, err
		}
		if proxyURL != nil {
			// Let the proxy resolve the hostname: a local Clash/v2ray in fake-IP
			// mode returns 198.18.x.x from the system resolver, which is not an
			// address the proxy itself can dial. The hostname is still checked
			// below so a mail cannot tunnel to localhost through the proxy.
			if err := p.validateProxyHost(ctx, host); err != nil {
				return nil, err
			}
			proxyDialer, err := newDialer(proxyURL, imageProxyDialTimeout)
			if err != nil {
				return nil, err
			}
			return proxyDialer.DialContext(ctx, network, net.JoinHostPort(host, port))
		}
	}

	var addresses []netip.Addr
	if ip, err := netip.ParseAddr(host); err == nil {
		addresses = []netip.Addr{ip}
	} else {
		addresses, err = net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, fmt.Errorf("image proxy: resolve %s: %w", host, err)
		}
	}

	var lastErr error
	allowed := make([]netip.Addr, 0, len(addresses))
	for _, ip := range addresses {
		ip = ip.Unmap()
		if !p.allowPrivate && !isPublicIP(ip) {
			continue
		}
		allowed = append(allowed, ip)
	}
	if len(allowed) == 0 {
		return nil, fmt.Errorf("image proxy: dial %s: no public address", host)
	}

	// Happy Eyeballs: a host with both IPv6 and IPv4 must not spend the whole
	// request timeout on an unreachable IPv6 route. Start the next address after
	// a short stagger and return the first connection that succeeds.
	type dialResult struct {
		conn net.Conn
		err  error
	}
	dialCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan dialResult, len(allowed))
	for i, ip := range allowed {
		go func(index int, ip netip.Addr) {
			if index > 0 {
				select {
				case <-time.After(time.Duration(index) * 200 * time.Millisecond):
				case <-dialCtx.Done():
					results <- dialResult{err: dialCtx.Err()}
					return
				}
			}
			conn, err := p.dialer.DialContext(dialCtx, network, net.JoinHostPort(ip.String(), port))
			results <- dialResult{conn: conn, err: err}
		}(i, ip)
	}

	for remaining := len(allowed); remaining > 0; remaining-- {
		res := <-results
		if res.err == nil {
			// Close any connection that wins the race after this one.
			go func() {
				for i := 0; i < remaining-1; i++ {
					if late := <-results; late.conn != nil {
						_ = late.conn.Close()
					}
				}
			}()
			return res.conn, nil
		}
		lastErr = res.err
	}
	return nil, fmt.Errorf("image proxy: dial %s: %w", host, lastErr)
}

// validateProxyHost checks the hostname before handing it to the app's proxy.
// Fake-IP ranges are allowed because the proxy resolves them itself; everything
// private, loopback or link-local is still refused.
func (p *ImageProxy) validateProxyHost(ctx context.Context, host string) error {
	if ip, err := netip.ParseAddr(host); err == nil {
		if !p.allowPrivate && !isPublicIP(ip) {
			return fmt.Errorf("image proxy: refusing private address %s", host)
		}
		return nil
	}

	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return fmt.Errorf("image proxy: resolve %s: %w", host, err)
	}
	allowed := false
	for _, ip := range addresses {
		ip = ip.Unmap()
		if p.allowPrivate || isPublicIP(ip) {
			allowed = true
			continue
		}
		if isFakeIP(ip) {
			allowed = true
			continue
		}
		return fmt.Errorf("image proxy: refusing private address for %s", host)
	}
	if !allowed {
		return fmt.Errorf("image proxy: %s has no usable address", host)
	}
	return nil
}

// isFakeIP is the range Clash and similar tools use for their DNS fake-IP
// mode (RFC 2544 benchmarking space, repurposed locally).
func isFakeIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.Is4() {
		return false
	}
	b := ip.As4()
	return b[0] == 198 && (b[1] == 18 || b[1] == 19)
}

// isPublicIP is the SSRF gate. Everything that is not a globally routable
// unicast address is refused, including the cloud metadata range (169.254/16)
// and carrier-grade NAT.
func isPublicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() {
		return false
	}
	if ip.IsUnspecified() ||
		ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() {
		return false
	}
	if ip.Is4() {
		b := ip.As4()
		switch {
		case b[0] == 0:
			return false
		case b[0] == 100 && b[1] >= 64 && b[1] <= 127:
			return false
		case b[0] == 192 && b[1] == 0 && b[2] == 0:
			return false
		case b[0] == 198 && (b[1] == 18 || b[1] == 19):
			return false
		case b[0] >= 240:
			return false
		}
	}
	if ip.Is6() {
		b := ip.As16()
		// 2001:db8::/32 is documentation-only.
		if b[0] == 0x20 && b[1] == 0x01 && b[2] == 0x0d && b[3] == 0xb8 {
			return false
		}
	}
	return true
}

type imageProxyResponse struct {
	data        []byte
	contentType string
}

func (p *ImageProxy) fetch(ctx context.Context, target *url.URL) (imageProxyResponse, time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, imageProxyTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return imageProxyResponse{}, 0, err
	}
	req.Header.Set("User-Agent", imageProxyUserAgent)
	req.Header.Set("Accept", imageProxyAccept)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	// No cookies, no Referer, no Authorization: the request must carry nothing
	// that identifies the reader.

	resp, err := p.client.Do(req)
	if err != nil {
		return imageProxyResponse{}, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return imageProxyResponse{}, 0, fmt.Errorf("image proxy: upstream status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, p.maxBytes+1))
	if err != nil {
		return imageProxyResponse{}, 0, err
	}
	if int64(len(body)) > p.maxBytes {
		return imageProxyResponse{}, 0, errImageProxyTooLarge
	}
	if len(body) == 0 {
		return imageProxyResponse{}, 0, errImageProxyUnsupportedType
	}

	contentType, err := proxiedContentType(resp.Header.Get("Content-Type"), target, body)
	if err != nil {
		return imageProxyResponse{}, 0, err
	}

	return imageProxyResponse{data: body, contentType: contentType}, imageProxyCacheTTL(resp.Header), nil
}

func proxiedContentType(header string, target *url.URL, body []byte) (string, error) {
	mediaType, _, _ := mime.ParseMediaType(header)
	mediaType = strings.ToLower(mediaType)
	if isProxiedAssetType(mediaType) {
		return mediaType, nil
	}

	// Some CDNs serve images as application/octet-stream. Sniff the body, then
	// fall back to the URL extension before rejecting it.
	if mediaType == "" || mediaType == "application/octet-stream" || mediaType == "binary/octet-stream" {
		sniffed, _, _ := mime.ParseMediaType(http.DetectContentType(body))
		if isProxiedAssetType(sniffed) {
			return sniffed, nil
		}
		if byExt := mime.TypeByExtension(path.Ext(target.Path)); isProxiedAssetType(byExt) {
			return byExt, nil
		}
	}
	return "", errImageProxyUnsupportedType
}

func isProxiedAssetType(mediaType string) bool {
	// An SVG can reference remote images/fonts from inside the document. When
	// the browser renders it as an <img>, those references are fetched from the
	// reader, which would undo the proxy. Reject it rather than trying to
	// sanitize an XML dialect here.
	if mediaType == "image/svg+xml" || mediaType == "image/svg" {
		return false
	}
	if strings.HasPrefix(mediaType, "image/") || strings.HasPrefix(mediaType, "font/") {
		return true
	}
	switch mediaType {
	case "application/font-woff",
		"application/font-woff2",
		"application/font-sfnt",
		"application/vnd.ms-fontobject",
		"application/x-font-ttf",
		"application/x-font-opentype":
		return true
	}
	return false
}

func imageProxyCacheTTL(header http.Header) time.Duration {
	cacheControl := strings.ToLower(header.Get("Cache-Control"))
	if strings.Contains(cacheControl, "no-store") {
		return 0
	}
	for _, directive := range strings.Split(cacheControl, ",") {
		directive = strings.TrimSpace(directive)
		if !strings.HasPrefix(directive, "max-age=") {
			continue
		}
		seconds, err := strconv.Atoi(strings.TrimPrefix(directive, "max-age="))
		if err != nil || seconds <= 0 {
			return 0
		}
		ttl := time.Duration(seconds) * time.Second
		if ttl > imageProxyMaxMaxAge {
			return imageProxyMaxMaxAge
		}
		return ttl
	}
	return imageProxyDefaultMaxAge
}

func writeProxiedImage(w http.ResponseWriter, r *http.Request, data []byte, contentType string, maxAge time.Duration) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	if maxAge > 0 {
		w.Header().Set("Cache-Control", fmt.Sprintf("private, max-age=%d", int(maxAge.Seconds())))
	} else {
		w.Header().Set("Cache-Control", "no-store")
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
	// Remote fonts loaded from a cross-origin desktop proxy need CORS; it is
	// harmless for images.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	// SVG can be navigated to directly; this keeps a script in one from running
	// as a document while still letting it render as an <img>.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(data)
}

type imageProxyCacheEntry struct {
	key         string
	data        []byte
	contentType string
	expires     time.Time
}

type imageProxyCache struct {
	mu       sync.Mutex
	items    map[string]*list.Element
	lru      *list.List
	bytes    int64
	maxBytes int64
	maxItems int
}

func newImageProxyCache(maxBytes int64, maxItems int) *imageProxyCache {
	return &imageProxyCache{
		items:    make(map[string]*list.Element),
		lru:      list.New(),
		maxBytes: maxBytes,
		maxItems: maxItems,
	}
}

func (c *imageProxyCache) get(key string) (*imageProxyCacheEntry, time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	el, ok := c.items[key]
	if !ok {
		return nil, 0
	}
	entry := el.Value.(*imageProxyCacheEntry)
	remaining := time.Until(entry.expires)
	if remaining <= 0 {
		c.removeElement(el)
		return nil, 0
	}
	c.lru.MoveToFront(el)
	return entry, remaining
}

func (c *imageProxyCache) put(key string, data []byte, contentType string, ttl time.Duration) {
	if ttl <= 0 || int64(len(data)) > c.maxBytes/4 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.items[key]; ok {
		c.removeElement(el)
	}
	entry := &imageProxyCacheEntry{
		key:         key,
		data:        data,
		contentType: contentType,
		expires:     time.Now().Add(ttl),
	}
	el := c.lru.PushFront(entry)
	c.items[key] = el
	c.bytes += int64(len(data))

	for c.bytes > c.maxBytes || len(c.items) > c.maxItems {
		oldest := c.lru.Back()
		if oldest == nil {
			break
		}
		c.removeElement(oldest)
	}
}

func (c *imageProxyCache) removeElement(el *list.Element) {
	entry := el.Value.(*imageProxyCacheEntry)
	c.lru.Remove(el)
	c.bytes -= int64(len(entry.data))
	delete(c.items, entry.key)
}
