package artifact_manager

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	DefaultBaseDir = "lwp-results"
	SessionsDir    = "lwp-sessions" // Separate from results
	RawHTMLDir     = "raw"           // Legacy, will be deprecated
	ParsedJSONDir  = "parsed"        // Legacy, will be deprecated
)

// GetURLDir returns the directory for a specific URL ID (URL-centric structure).
// Example: lwp-results/42/
func GetURLDir(baseDir string, urlID int64) string {
	if baseDir == "" {
		baseDir = DefaultBaseDir
	}
	return filepath.Join(baseDir, fmt.Sprintf("%d", urlID))
}

// GetURLArtifactPath returns the full path for a specific artifact.
// Example: lwp-results/42/raw.html
func GetURLArtifactPath(baseDir string, urlID int64, artifact string) string {
	return filepath.Join(GetURLDir(baseDir, urlID), artifact)
}

// Manager handles storage and retrieval of web artifacts.
type Manager struct {
	baseDir string
	maxAge  time.Duration // Max age for a stored artifact before it's considered stale
}

// NewManager creates a new Artifact Manager instance.
// It ensures the base directory and its subdirectories exist.
func NewManager(baseDir string, maxAge time.Duration) (*Manager, error) {
	if baseDir == "" {
		baseDir = DefaultBaseDir
	}
	// Ensure base directories exist
	if err := os.MkdirAll(filepath.Join(baseDir, RawHTMLDir), 0750); err != nil {
		return nil, fmt.Errorf("failed to create raw HTML directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(baseDir, ParsedJSONDir), 0750); err != nil {
		return nil, fmt.Errorf("failed to create parsed JSON directory: %w", err)
	}

	return &Manager{baseDir: baseDir, maxAge: maxAge}, nil
}

// normalizeURL creates a canonical representation of a URL for consistent hashing.
func normalizeURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	// Always use HTTPS if possible
	if u.Scheme == "http" {
		u.Scheme = "https"
	}
	u.Host = strings.ToLower(u.Host) // Lowercase host

	// Sort query parameters alphabetically
	if u.RawQuery != "" {
		params := u.Query()
		var keys []string
		for k := range params {
			keys = append(keys, k)
		}
		sort.Strings(keys) // Sort keys
		sortedQuery := url.Values{}
		for _, k := range keys {
			for _, v := range params[k] {
				sortedQuery.Add(k, v)
			}
		}
		u.RawQuery = sortedQuery.Encode()
	}

	// Strip fragment
	u.Fragment = ""

	return u.String(), nil
}

// getShortHash generates a short, stable hash from a normalized URL.
func getShortHash(normalizedURL string) string {
	hash := sha256.Sum256([]byte(normalizedURL))
	return fmt.Sprintf("%x", hash[:6]) // Use first 6 bytes for a 12-char hex string
}

// sanitizeSlug creates a filesystem-safe slug from a URL path.
var invalidFilenameChar = regexp.MustCompile(`[^a-zA-Z0-9\-_]+`)
func sanitizeSlug(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		// Fallback for invalid URLs or local files
		safe := invalidFilenameChar.ReplaceAllString(rawURL, "_")
		return strings.Trim(safe, "_")
	}

	hostPart := strings.ReplaceAll(u.Host, ".", "_")
	pathPart := strings.TrimPrefix(u.Path, "/")
	pathPart = invalidFilenameChar.ReplaceAllString(pathPart, "_")
	pathPart = strings.Trim(pathPart, "_")

	if pathPart == "" {
		return hostPart
	}
	return fmt.Sprintf("%s_%s", hostPart, pathPart)
}

// GetArtifactPath constructs a full path for an artifact based on its type.
func (m *Manager) GetArtifactPath(artifactDir, url string, ext string) (string, error) {
    normalizedURL, err := normalizeURL(url)
    if err != nil {
        return "", err
    }
	slug := sanitizeSlug(url) // Use original URL for slug for human readability
	shortHash := getShortHash(normalizedURL)

	filename := fmt.Sprintf("%s-%s%s", slug, shortHash, ext)
	return filepath.Join(m.baseDir, artifactDir, filename), nil
}

// GetRawHTML retrieves raw HTML from storage if fresh.
func (m *Manager) GetRawHTML(url string) ([]byte, bool, error) {
	filePath, err := m.GetArtifactPath(RawHTMLDir, url, ".html")
    if err != nil {
        return nil, false, err
    }
    
	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return nil, false, nil // Not found
	}
	if err != nil {
		return nil, false, fmt.Errorf("error statting raw HTML artifact: %w", err)
	}

	if m.maxAge > 0 && time.Since(info.ModTime()) > m.maxAge {
		return nil, false, nil // Stale
	}
    // If maxAge is negative, it means "never expire"
    // In this case, always fresh if exists.

	data, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return nil, false, fmt.Errorf("error reading raw HTML artifact: %w", err)
	}
	return data, true, nil // Found and fresh
}

// SetRawHTML stores raw HTML.
func (m *Manager) SetRawHTML(url string, data []byte) error {
	filePath, err := m.GetArtifactPath(RawHTMLDir, url, ".html")
    if err != nil {
        return err
    }
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write raw HTML: %w", err)
	}
	return nil
}

// GetParsedJSON retrieves parsed JSON from storage if fresh.
func (m *Manager) GetParsedJSON(url string) ([]byte, bool, error) {
    filePath, err := m.GetArtifactPath(ParsedJSONDir, url, ".json")
    if err != nil {
        return nil, false, err
    }
    
	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return nil, false, nil // Not found
	}
	if err != nil {
		return nil, false, fmt.Errorf("error statting parsed JSON artifact: %w", err)
	}

	if m.maxAge > 0 && time.Since(info.ModTime()) > m.maxAge {
		return nil, false, nil // Stale
	}
    // If maxAge is negative, it means "never expire"
    // In this case, always fresh if exists.

	data, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return nil, false, fmt.Errorf("error reading parsed JSON artifact: %w", err)
	}
	return data, true, nil // Found and fresh
}

// SetParsedJSON stores parsed JSON.
func (m *Manager) SetParsedJSON(url string, data []byte) error {
    filePath, err := m.GetArtifactPath(ParsedJSONDir, url, ".json")
    if err != nil {
        return err
    }
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write parsed JSON: %w", err)
	}
	return nil
}

// MaxAge returns the configured max age for artifacts.
func (m *Manager) MaxAge() time.Duration {
    return m.maxAge
}

// ===== NEW URL-ID-BASED METHODS =====

// EnsureURLDir ensures the directory for a URL ID exists.
// Creates lwp-results/{url_id}/ if it doesn't exist.
func (m *Manager) EnsureURLDir(urlID int64) error {
	urlDir := GetURLDir(m.baseDir, urlID)
	if err := os.MkdirAll(urlDir, 0750); err != nil {
		return fmt.Errorf("failed to create URL directory: %w", err)
	}
	return nil
}

// GetRawHTMLByID retrieves raw HTML from URL-centric storage.
// Reads from lwp-results/{url_id}/raw.html
func (m *Manager) GetRawHTMLByID(urlID int64) ([]byte, bool, error) {
	filePath := GetURLArtifactPath(m.baseDir, urlID, "raw.html")

	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return nil, false, nil // Not found
	}
	if err != nil {
		return nil, false, fmt.Errorf("error statting raw HTML: %w", err)
	}

	if m.maxAge > 0 && time.Since(info.ModTime()) > m.maxAge {
		return nil, false, nil // Stale
	}

	data, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return nil, false, fmt.Errorf("error reading raw HTML: %w", err)
	}
	return data, true, nil
}

// SetRawHTMLByID stores raw HTML in URL-centric storage.
// Writes to lwp-results/{url_id}/raw.html
func (m *Manager) SetRawHTMLByID(urlID int64, data []byte) error {
	if err := m.EnsureURLDir(urlID); err != nil {
		return err
	}

	filePath := GetURLArtifactPath(m.baseDir, urlID, "raw.html")
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write raw HTML: %w", err)
	}
	return nil
}

// GetParsedJSONByID retrieves parsed JSON from URL-centric storage.
// Reads from lwp-results/{url_id}/generic.yaml
func (m *Manager) GetParsedJSONByID(urlID int64) ([]byte, bool, error) {
	filePath := GetURLArtifactPath(m.baseDir, urlID, "generic.yaml")

	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return nil, false, nil // Not found
	}
	if err != nil {
		return nil, false, fmt.Errorf("error statting parsed YAML: %w", err)
	}

	if m.maxAge > 0 && time.Since(info.ModTime()) > m.maxAge {
		return nil, false, nil // Stale
	}

	data, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return nil, false, fmt.Errorf("error reading parsed YAML: %w", err)
	}
	return data, true, nil
}

// SetParsedYAMLByID stores parsed YAML in URL-centric storage.
// Writes to lwp-results/{url_id}/generic.yaml
func (m *Manager) SetParsedYAMLByID(urlID int64, data []byte) error {
	if err := m.EnsureURLDir(urlID); err != nil {
		return err
	}

	filePath := GetURLArtifactPath(m.baseDir, urlID, "generic.yaml")
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write parsed YAML: %w", err)
	}
	return nil
}
