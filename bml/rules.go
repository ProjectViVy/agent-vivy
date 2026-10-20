package bml

// CompareAndWriteMemRules replaces the handbook only while its exact content
// and file existence still match the authorized read. The Home mutex serializes
// these writes; proposed content normalization happens after the comparison.
func (h *Home) CompareAndWriteMemRules(content, expectedContent string, expectedExists bool) (MemRulesDocument, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	current, err := h.ReadMemRules()
	if err != nil {
		return MemRulesDocument{}, err
	}
	if current.Content != expectedContent || (current.Source == MemRulesSourceFile) != expectedExists {
		return MemRulesDocument{}, &HomeError{code: HomeCodeRevisionConflict, message: "memory rules changed; read the handbook again"}
	}
	return h.writeMemRules(content)
}
