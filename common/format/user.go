package format

// UserTag creates a unique identifier by combining tag and uuid.
// Uses string concatenation for better performance than fmt.Sprintf.
func UserTag(tag, uuid string) string {
	// Pre-allocate exact size needed: len(tag) + 1 (separator) + len(uuid)
	b := make([]byte, 0, len(tag)+1+len(uuid))
	b = append(b, tag...)
	b = append(b, '|')
	b = append(b, uuid...)
	return string(b)
}
