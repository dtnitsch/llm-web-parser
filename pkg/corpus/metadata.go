package corpus

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	dbpkg "github.com/dtnitsch/llm-web-parser/pkg/db"
)

// URLMetadata represents the metadata written to metadata.yaml files.
type URLMetadata struct {
	URLID               int64   `yaml:"url_id"`
	URL                 string  `yaml:"url"`
	Domain              string  `yaml:"domain"`
	ContentType         string  `yaml:"content_type,omitempty"`
	ContentSubtype      string  `yaml:"content_subtype,omitempty"`
	DetectionConfidence float64 `yaml:"detection_confidence,omitempty"`
	HasAbstract         bool    `yaml:"has_abstract"`
	HasInfobox          bool    `yaml:"has_infobox"`
	HasTOC              bool    `yaml:"has_toc"`
	HasCodeExamples     bool    `yaml:"has_code_examples"`
	SectionCount        int     `yaml:"section_count"`
	CitationCount       int     `yaml:"citation_count"`
	CodeBlockCount      int     `yaml:"code_block_count"`
}

// WriteMetadataFile writes metadata.yaml for a URL.
// Location: llm-web-parser-results/parsed/<url_id>/metadata.yaml
func WriteMetadataFile(db *dbpkg.DB, urlID int64, baseDir string) error {
	// Get URL info
	var url, domain string
	err := db.QueryRow("SELECT original_url, domain FROM urls WHERE url_id = ?", urlID).Scan(&url, &domain)
	if err != nil {
		return fmt.Errorf("failed to get URL info: %w", err)
	}

	// Get content type info
	info, err := db.GetURLContentInfo(urlID)
	if err != nil {
		return fmt.Errorf("failed to get content info: %w", err)
	}

	// Build metadata struct
	metadata := URLMetadata{
		URLID:               urlID,
		URL:                 url,
		Domain:              domain,
		HasAbstract:         info.HasAbstract,
		HasInfobox:          info.HasInfobox,
		HasTOC:              info.HasTOC,
		HasCodeExamples:     info.HasCodeExamples,
		SectionCount:        info.SectionCount,
		CitationCount:       info.CitationCount,
		CodeBlockCount:      info.CodeBlockCount,
	}

	if info.ContentType.Valid {
		metadata.ContentType = info.ContentType.String
	}
	if info.ContentSubtype.Valid {
		metadata.ContentSubtype = info.ContentSubtype.String
	}
	if info.DetectionConfidence.Valid {
		metadata.DetectionConfidence = info.DetectionConfidence.Float64
	}

	// Write to file (same location as generic.yaml)
	metadataPath := filepath.Join(baseDir, fmt.Sprintf("%d", urlID), "metadata.yaml")

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(metadataPath), 0755); err != nil {
		return fmt.Errorf("failed to create metadata directory: %w", err)
	}

	// Marshal to YAML
	yamlBytes, err := yaml.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Write file
	if err := os.WriteFile(metadataPath, yamlBytes, 0644); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	return nil
}
