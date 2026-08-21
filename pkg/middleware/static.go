package middleware

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"github.com/gin-gonic/gin"
)

type StaticConfig struct {
	SkippedPathPrefixes []string
	Root                string
}

var hashedStaticAssetPattern = regexp.MustCompile(`\.[0-9a-f]{8}\.`)

func setStaticCacheHeaders(c *gin.Context, fpath string) {
	name := filepath.Base(fpath)
	if name == "index.html" {
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		return
	}

	if hashedStaticAssetPattern.MatchString(name) {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		return
	}

	c.Header("Cache-Control", "no-cache")
}

func StaticWithConfig(config StaticConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if SkippedPathPrefixes(c, config.SkippedPathPrefixes...) {
			c.Next()
			return
		}

		p := c.Request.URL.Path
		fpath := filepath.Join(config.Root, filepath.FromSlash(p))
		info, err := os.Stat(fpath)
		if err == nil && !info.IsDir() {
			setStaticCacheHeaders(c, fpath)
			c.File(fpath)
			c.Abort()
			return
		}

		// Only client-side routes fall back to the SPA entry. Missing assets must
		// return 404 so the browser never tries to parse index.html as JavaScript.
		if filepath.Ext(p) != "" {
			c.Status(http.StatusNotFound)
			c.Abort()
			return
		}

		if err != nil && !os.IsNotExist(err) {
			c.Status(http.StatusInternalServerError)
			c.Abort()
			return
		}

		if err != nil || info.IsDir() {
			fpath = filepath.Join(config.Root, "index.html")
		}
		setStaticCacheHeaders(c, fpath)
		c.File(fpath)
		c.Abort()
	}
}
