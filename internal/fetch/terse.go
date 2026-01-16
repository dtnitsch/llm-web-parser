package fetch


func ToTerseStatus(status string) int {
	if status == "success" {
		return 0
	}
	return 1
}

// toTerseContentType converts content_type to single char (l=landing, a=article, d=docs, u=unknown).
func ToTerseContentType(ct string) string {
	switch ct {
	case "landing":
		return "l"
	case "article":
		return "a"
	case "documentation":
		return "d"
	default:
		return "u"
	}
}

// toTerseQuality converts extraction_quality to int (1=ok, 0=low, -1=degraded).
func ToTerseQuality(q string) int {
	switch q {
	case "ok":
		return 1
	case "low":
		return 0
	case "degraded":
		return -1
	default:
		return 0
	}
}

// toTerseResult converts ResultSummary to ResultSummaryTerse.
func ToTerseResult(r ResultSummary) ResultSummaryTerse {
	return ResultSummaryTerse{
		URL:               r.URL,
		FilePath:          r.FilePath,
		Status:            ToTerseStatus(r.Status),
		Error:             r.Error,
		FileSizeBytes:     r.FileSizeBytes,
		EstimatedTokens:   r.EstimatedTokens,
		ContentType:       ToTerseContentType(r.ContentType),
		ExtractionQuality: ToTerseQuality(r.ExtractionQuality),
		ConfidenceDist:    [3]int{r.ConfidenceDist["high"], r.ConfidenceDist["medium"], r.ConfidenceDist["low"]},
		BlockTypeDist:     r.BlockTypeDist,
	}
}

// toTerseStats converts Stats to StatsTerse.
func ToTerseStats(s Stats) StatsTerse {
	return StatsTerse{
		Total:    s.TotalURLs,
		Success:  s.Successful,
		Failed:   s.Failed,
		Time:     s.TotalTimeSeconds,
		Keywords: s.TopKeywords,
	}
}

// Works with both ResultSummary (v1) and ResultSummaryTerse (v2).
