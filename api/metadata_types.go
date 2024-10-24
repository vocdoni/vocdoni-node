package api

// ElectionMetadata contains the process metadata fields as stored on ipfs
type ElectionMetadata struct {
	Title       LanguageString     `json:"title"`
	Version     string             `json:"version"`
	Description LanguageString     `json:"description"`
	Media       ProcessMedia       `json:"media,omitempty"`
	Meta        any                `json:"meta,omitempty"`
	Questions   []Question         `json:"questions,omitempty"`
	Type        ElectionProperties `json:"type,omitempty"`
}

// LanguageString is a wrapper for multi-language strings, specified in metadata.
//
//	example {"default": "hello", "en": "hello", "es": "hola"}
type LanguageString map[string]string

// ProcessMedia holds the process metadata's header and streamURI
type ProcessMedia struct {
	Header    string `json:"header,omitempty"`
	StreamURI string `json:"streamUri,omitempty"`
}

// ElectionProperties describes how a process results should be displayed and aggregated
type ElectionProperties struct {
	Name       string `json:"name"`
	Properties any    `json:"properties"`
}

// Question contains metadata for one single question of a process
type Question struct {
	Choices     []ChoiceMetadata `json:"choices"`
	Description LanguageString   `json:"description"`
	Title       LanguageString   `json:"title"`
	Meta        any              `json:"meta,omitempty"`
}

// ChoiceMetadata contains metadata for one choice of a question
type ChoiceMetadata struct {
	Title LanguageString `json:"title"`
	Value uint32         `json:"value"`
	Meta  any            `json:"meta,omitempty"`
}

// AccountMetadata is the metadata for an organization
type AccountMetadata struct {
	Version     string         `json:"version,omitempty"`
	Languages   []string       `json:"languages,omitempty"`
	Name        LanguageString `json:"name,omitempty"`
	Description LanguageString `json:"description,omitempty"`
	NewsFeed    LanguageString `json:"newsFeed,omitempty"`
	Media       *AccountMedia  `json:"media,omitempty"`
	Meta        any            `json:"meta,omitempty"`
	Actions     any            `json:"actions,omitempty"`
}

// AccountMedia stores the avatar, header, and logo for an entity metadata
type AccountMedia struct {
	Avatar string `json:"avatar,omitempty"`
	Header string `json:"header,omitempty"`
	Logo   string `json:"logo,omitempty"`
}
