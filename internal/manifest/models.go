package manifest

import "time"

// Document represents a row in the documents table.
type Document struct {
	ID           string
	SourcePath   string
	FileType     FileType
	FileSize     int64
	PageCount    *int
	HasTextLayer bool
	NeedsOCR     bool
	QualityFlags *string
	Language     *Language
	ModifiedAt   *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Classification represents a row in the classifications table.
type Classification struct {
	ID               int64
	DocumentID       string
	Source           ClassSource
	Model            *string
	Tipo             *string
	TipoOriginal     *string
	Subject          *string
	SubjectHierarchy *string
	Autor            *string
	Titulo           *string
	Anio             *string
	Editorial        *string
	Edicion          *string
	Description      *string
	Confidence       float64
	Accepted         bool
	AcceptedAt       *time.Time
	Reason           *string
	CreatedAt        time.Time
}

// SubjectGroup represents a row in the subject_groups table.
type SubjectGroup struct {
	ID        int64
	Name      string
	NameEN    *string
	NameES    *string
	CreatedAt time.Time
}

// SubjectAlias represents a row in the subject_aliases table.
type SubjectAlias struct {
	Alias     string
	GroupID   int64
	Source    AliasSource
	CreatedAt time.Time
}

// Organization represents a row in the organization table.
type Organization struct {
	DocumentID string
	SourcePath string
	TargetPath string
	BackupPath *string
	AppliedAt  time.Time
}

// Duplicate represents a row in the duplicates table.
type Duplicate struct {
	ID          int64
	DocumentID  string
	DuplicateOf string
	Similarity  float64
	Reason      string
	CreatedAt   time.Time
}

// Tag represents a row in the tags table.
type Tag struct {
	ID       int64
	Key      string
	LabelEN  *string
	LabelES  *string
	Category TagCategory
}

// Stats contains summary statistics about the manifest.
type Stats struct {
	Documents       int            `json:"documents"`
	Classifications int            `json:"classifications"`
	Accepted        int            `json:"accepted"`
	ByType          map[string]int `json:"by_type"`
}

// classificationNullable holds nullable fields from a classification row
// before assigning them to the Classification struct.
type classificationNullable struct {
	Model, Tipo, TipoOriginal, Subject, SubjectHierarchy         *string
	Autor, Titulo, Anio, Editorial, Edicion, Description, Reason *string
	AcceptedAt                                                   *time.Time
}
