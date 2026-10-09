package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	// CacheControlImmutable is for content-hashed build assets under /static:
	// the URL changes whenever the bytes change, so browsers and CDNs can cache
	// them indefinitely.
	CacheControlImmutable = "public, max-age=31536000, immutable"
	// CacheControlHTML lets a CDN serve the SPA shell and the build-time
	// pre-rendered pages from its edge for a short window (s-maxage) while
	// browsers always revalidate. The short window keeps a deploy from being
	// hidden behind a long-lived cached document without requiring a manual CDN
	// purge, and the CDN already holds the previous build's immutable assets.
	CacheControlHTML = "public, max-age=0, s-maxage=300, stale-while-revalidate=300"
	// CacheControlStatic is the default for other public static files
	// (robots.txt, sitemap.xml, images, ...).
	CacheControlStatic = "public, max-age=604800"
)

// Cache sets response caching headers for the web routes so that a CDN in front
// of the app can serve the whole frontend (HTML shell + static assets) from its
// edge and only forward data requests (/api, /v1) to the origin.
func Cache() func(c *gin.Context) {
	return func(c *gin.Context) {
		switch {
		case strings.HasPrefix(c.Request.RequestURI, "/static/"):
			c.Header("Cache-Control", CacheControlImmutable)
		case c.Request.RequestURI == "/":
			c.Header("Cache-Control", CacheControlHTML)
		default:
			c.Header("Cache-Control", CacheControlStatic)
		}
		c.Header("Cache-Version", "b688f2fb5be447c25e5aa3bd063087a83db32a288bf6a4f35f2d8db310e40b14")
		c.Next()
	}
}
