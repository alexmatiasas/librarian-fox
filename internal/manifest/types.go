package manifest

// FileType represents the file format of a document.
type FileType string

const (
	FileTypePDF  FileType = "pdf"
	FileTypeEPUB FileType = "epub"
	FileTypeDjVu FileType = "djvu"
	FileTypeDOCX FileType = "docx"
	FileTypeDOC  FileType = "doc"
)

// Language represents the ISO 639-3 language code.
type Language string

const (
	LanguageEN Language = "eng"
	LanguageES Language = "spa"
	LanguageDE Language = "deu"
	LanguageJA Language = "jpn"
)

// ClassSource indicates how a classification was produced.
type ClassSource string

const (
	SourceHeuristic ClassSource = "heuristic"
	SourceLLM       ClassSource = "llm"
	SourceManual    ClassSource = "manual"
)

// AliasSource indicates how a subject alias was created.
type AliasSource string

const (
	AliasAuto   AliasSource = "auto"
	AliasManual AliasSource = "manual"
)

// TagCategory groups tags by purpose.
type TagCategory string

const (
	TagStatus   TagCategory = "status"
	TagPriority TagCategory = "priority"
	TagCustom   TagCategory = "custom"
)
