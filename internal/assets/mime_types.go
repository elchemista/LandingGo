package assets

import "mime"

func init() {
	types := map[string]string{
		".css":   "text/css; charset=utf-8",
		".js":    "application/javascript",
		".mjs":   "application/javascript",
		".json":  "application/json",
		".map":   "application/json",
		".svg":   "image/svg+xml",
		".webp":  "image/webp",
		".png":   "image/png",
		".jpg":   "image/jpeg",
		".jpeg":  "image/jpeg",
		".gif":   "image/gif",
		".ico":   "image/x-icon",
		".woff":  "font/woff",
		".woff2": "font/woff2",
		".ttf":   "font/ttf",
		".otf":   "font/otf",
	}

	for ext, mt := range types {
		_ = mime.AddExtensionType(ext, mt)
	}
}
