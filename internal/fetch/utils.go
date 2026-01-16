package fetch

import (
	"strings"

	"github.com/dtnitsch/llm-web-parser/models"
)

func ParseFeaturesFlag(features string) models.ParseMode {
	if features == "" {
		return models.ParseModeMinimal // Default: minimal (metadata only)
	}

	// Parse comma-separated features
	featureList := strings.Split(features, ",")
	for _, f := range featureList {
		f = strings.TrimSpace(strings.ToLower(f))
		switch f {
		case "full-parse":
			return models.ParseModeFull
		case "wordcount":
			// wordcount requires at least cheap parsing
			return models.ParseModeCheap
		}
	}

	// If no recognized features, default to minimal
	return models.ParseModeMinimal
}
