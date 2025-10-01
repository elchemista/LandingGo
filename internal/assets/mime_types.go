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
		".mp4":   "video/mp4",
		".webm":  "video/webm",
		".ogg":   "video/ogg",
		".mp3":   "audio/mpeg",
		".wav":   "audio/wav",
		".flac":  "audio/flac",
		".aac":   "audio/aac",
		".oga":   "audio/ogg",
		".opus":  "audio/opus",
	}

	for ext, mt := range types {
		_ = mime.AddExtensionType(ext, mt)
	}
}
