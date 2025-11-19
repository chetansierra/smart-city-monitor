package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// SecurityHeaders adds security-related HTTP headers
func SecurityHeaders() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Prevent MIME type sniffing
		c.Set("X-Content-Type-Options", "nosniff")

		// Enable XSS protection
		c.Set("X-XSS-Protection", "1; mode=block")

		// Prevent clickjacking
		c.Set("X-Frame-Options", "DENY")

		// Referrer policy
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Content Security Policy (CSP)
		// Strict policy - adjust based on your needs
		csp := strings.Join([]string{
			"default-src 'self'",
			"script-src 'self'",
			"style-src 'self' 'unsafe-inline'",
			"img-src 'self' data: https:",
			"font-src 'self'",
			"connect-src 'self'",
			"frame-ancestors 'none'",
			"base-uri 'self'",
			"form-action 'self'",
		}, "; ")
		c.Set("Content-Security-Policy", csp)

		// HSTS - Force HTTPS (only enable in production with HTTPS)
		// Uncomment when deployed with HTTPS
		// c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")

		// Permissions Policy (formerly Feature Policy)
		c.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		return c.Next()
	}
}

// ValidateContentType validates request content type for POST/PUT requests
func ValidateContentType() fiber.Handler {
	return func(c *fiber.Ctx) error {
		method := c.Method()

		// Only validate for methods that typically have a body
		if method == "POST" || method == "PUT" || method == "PATCH" {
			contentType := c.Get("Content-Type")

			// Allow empty content type for requests with no body
			if contentType == "" && c.Body() != nil && len(c.Body()) > 0 {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error": fiber.Map{
						"code":    "MISSING_CONTENT_TYPE",
						"message": "Content-Type header is required",
					},
				})
			}

			// Validate JSON content type
			if contentType != "" && !strings.Contains(contentType, "application/json") {
				return c.Status(fiber.StatusUnsupportedMediaType).JSON(fiber.Map{
					"success": false,
					"error": fiber.Map{
						"code":    "UNSUPPORTED_MEDIA_TYPE",
						"message": "Only application/json is supported",
					},
				})
			}
		}

		return c.Next()
	}
}

// RequestSizeLimit limits the size of incoming requests to prevent DoS
func RequestSizeLimit(maxBytes int) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Check Content-Length header
		if c.Get("Content-Length") != "" {
			contentLength := len(c.Body())
			if contentLength > maxBytes {
				return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{
					"success": false,
					"error": fiber.Map{
						"code":    "REQUEST_TOO_LARGE",
						"message": "Request body exceeds maximum allowed size",
					},
				})
			}
		}

		return c.Next()
	}
}

// SanitizeInput sanitizes common attack vectors from query parameters
func SanitizeInput() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Check query parameters for common SQL injection patterns
		queries := c.Queries()
		for key, value := range queries {
			if containsSQLInjection(value) {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error": fiber.Map{
						"code":    "INVALID_INPUT",
						"message": "Invalid characters detected in query parameter: " + key,
					},
				})
			}

			if containsXSS(value) {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error": fiber.Map{
						"code":    "INVALID_INPUT",
						"message": "Invalid characters detected in query parameter: " + key,
					},
				})
			}
		}

		return c.Next()
	}
}

// containsSQLInjection checks for common SQL injection patterns
func containsSQLInjection(input string) bool {
	lowerInput := strings.ToLower(input)

	// Common SQL injection patterns
	patterns := []string{
		"' or '1'='1",
		"\" or \"1\"=\"1",
		"'; drop table",
		"\"; drop table",
		"' union select",
		"\" union select",
		"' or 1=1--",
		"\" or 1=1--",
		"<script",
		"javascript:",
		"onerror=",
		"onload=",
	}

	for _, pattern := range patterns {
		if strings.Contains(lowerInput, pattern) {
			return true
		}
	}

	return false
}

// containsXSS checks for common XSS patterns
func containsXSS(input string) bool {
	lowerInput := strings.ToLower(input)

	// Common XSS patterns
	patterns := []string{
		"<script",
		"</script>",
		"javascript:",
		"onerror=",
		"onload=",
		"onclick=",
		"onfocus=",
		"onmouseover=",
		"<iframe",
		"<embed",
		"<object",
	}

	for _, pattern := range patterns {
		if strings.Contains(lowerInput, pattern) {
			return true
		}
	}

	return false
}

// TrustedProxies configures trusted proxy headers
func TrustedProxies(trustedProxies []string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// In production, validate that requests come from trusted proxies
		// For now, this is a placeholder that can be enhanced

		// Example: Validate X-Forwarded-For header
		forwardedFor := c.Get("X-Forwarded-For")
		if forwardedFor != "" {
			// In production, validate that the proxy is trusted
			// and properly sanitize the forwarded IP
		}

		return c.Next()
	}
}

// APIKeyAuth validates API key authentication (for future use)
func APIKeyAuth(validKeys map[string]bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiKey := c.Get("X-API-Key")

		if apiKey == "" {
			// For now, allow requests without API key
			// In production, you might want to require it
			return c.Next()
		}

		// Validate API key
		if !validKeys[apiKey] {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"error": fiber.Map{
					"code":    "INVALID_API_KEY",
					"message": "Invalid or missing API key",
				},
			})
		}

		// Store API key in context for later use
		c.Locals("api_key", apiKey)

		return c.Next()
	}
}
