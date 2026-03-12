package discovery

func Discover(scanners []Scanner) *ScanResult {
	combined := &ScanResult{}
	seen := make(map[string]bool)

	for _, scanner := range scanners {
		result := scanner.Scan()

		combined.Errors = append(combined.Errors, result.Errors...)

		for _, src := range result.Sources {
			if seen[src.Name] {
				continue
			}
			seen[src.Name] = true
			combined.Sources = append(combined.Sources, src)
		}
	}

	return combined
}
