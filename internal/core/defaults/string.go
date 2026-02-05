package defaults

func StringOrDefault(s string, defaultValue string) string {
	if s == "" {
		return defaultValue
	}
	return s
}

func StringPtrOrDefault(s *string, defaultValue string) string {
	if s == nil {
		return defaultValue
	}
	return *s
}

func StringPtrOrDefaultPtr(s *string, defaultValue string) *string {
	if s == nil || *s == "" {
		return &defaultValue
	}
	return s
}

func Uint32OrDefault(u uint32, defaultValue uint32) uint32 {
	if u == 0 {
		return defaultValue
	}
	return u
}

func Uint32PtrOrDefaultPtr(u *uint32, defaultValue uint32) *uint32 {
	if u == nil {
		return &defaultValue
	}
	return u
}

func Uint32PtrOrDefault(u *uint32, defaultValue uint32) uint32 {
	if u == nil {
		return defaultValue
	}
	return *u
}
