package modules

// matchOperation reports whether eventOperation matches any allowed operation pattern.
func matchOperation(allowed []string, eventOperation string) bool {
	if eventOperation == "" {
		eventOperation = "apply"
	}
	for _, op := range allowed {
		if op == "*" || op == eventOperation {
			return true
		}
	}
	return false
}
