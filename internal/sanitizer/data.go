package sanitizer

import "net/url"

type SanitizedHTMLDoc struct {
	discoveredUrls []url.URL
}

func (s *SanitizedHTMLDoc) GetDiscoveredURLs() []url.URL {
	return s.discoveredUrls
}
