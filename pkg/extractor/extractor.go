package extractor

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dtnitsch/llm-web-parser/models"
)

type Strategy struct {
	MinConfidence float64
	BlockTypes    map[string]struct{}
}

func ParseStrategy(strategyStr string) (*Strategy, error) {
	if strategyStr == "" {
		return &Strategy{MinConfidence: 0.0, BlockTypes: nil}, nil // No-op strategy
	}

	strategy := &Strategy{
		MinConfidence: 0.0,
		BlockTypes:    make(map[string]struct{}),
	}

	parts := strings.Split(strategyStr, ",")
	for _, part := range parts {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid strategy part: %s", part)
		}
		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])

		switch key {
		case "conf":
			if strings.HasPrefix(value, ">=") {
				f, err := strconv.ParseFloat(strings.TrimSpace(value[2:]), 64)
				if err != nil {
					return nil, fmt.Errorf("invalid confidence value: %s", value)
				}
				strategy.MinConfidence = f
			} else {
				return nil, fmt.Errorf("unsupported confidence operator in: %s", value)
			}
		case "type":
			types := strings.Split(value, "|")
			for _, t := range types {
				strategy.BlockTypes[strings.TrimSpace(t)] = struct{}{}
			}
		default:
			return nil, fmt.Errorf("unknown strategy key: %s", key)
		}
	}

	return strategy, nil
}

func FilterPage(page *models.Page, strategy *Strategy) *models.Page {
	if strategy == nil {
		return page // No filtering
	}

	filteredPage := &models.Page{
		URL:      page.URL,
		Title:    page.Title,
		Metadata: page.Metadata, // Copy metadata
		Content:  filterSections(page.Content, strategy),
	}

	return filteredPage
}

func filterSections(sections []models.Section, strategy *Strategy) []models.Section {
	var filteredSections []models.Section

	for _, section := range sections {
		filteredSection := models.Section{
			ID:       section.ID,
			Level:    section.Level,
			Blocks:   []models.ContentBlock{},
			Children: filterSections(section.Children, strategy), // Recursively filter children
		}

		// Filter heading based on strategy
		if section.Heading != nil {
			headingPasses := true
			// Check confidence for heading
			if section.Heading.Confidence < strategy.MinConfidence {
				headingPasses = false
			}
			// Check block types for heading ONLY IF block types are specified AND it's not one of them
			if headingPasses && len(strategy.BlockTypes) > 0 { 
				if _, ok := strategy.BlockTypes[section.Heading.Type]; !ok {
					headingPasses = false
				}
			}
			if headingPasses {
				filteredSection.Heading = section.Heading
			}
		}


		// Filter blocks within the current section
		for _, block := range section.Blocks {
			// Apply filters
			if block.Confidence < strategy.MinConfidence {
				continue
			}
			if len(strategy.BlockTypes) > 0 {
				if _, ok := strategy.BlockTypes[block.Type]; !ok {
					continue
				}
			}

			// If all filters pass, add the block
			filteredSection.Blocks = append(filteredSection.Blocks, block)
		}

		// Only add the section if it has blocks, children, or a heading
		if len(filteredSection.Blocks) > 0 || len(filteredSection.Children) > 0 || filteredSection.Heading != nil {
			filteredSections = append(filteredSections, filteredSection)
		}
	}
	return filteredSections
}