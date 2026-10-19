package domain

import (
	"github.com/google/uuid"
	"path/filepath"
)

const SubjectIdentityKey = "organism/subject/v1"
const SubjectPersonalDomain = "subject-personal-v1"
const SubjectResidentPolicy = "life-recap-v1"

// SubjectIdentity is the one core SnapshotStore registration of an organism
// and its existing ACTMEM authority. Profile, role, Session and presentation
// fields cannot participate in this identity.
type SubjectIdentity struct {
	Version     int    `json:"version"`
	SubjectID   string `json:"subject_id"`
	DomainID    string `json:"domain_id"`
	AuthorityID string `json:"authority_id"`
	OwnerPath   string `json:"owner_path"`
}

func (s SubjectIdentity) Valid() bool {
	subject, e1 := uuid.Parse(s.SubjectID)
	authority, e2 := uuid.Parse(s.AuthorityID)
	return s.Version == 1 && e1 == nil && e2 == nil && subject != uuid.Nil && authority != uuid.Nil && subject != authority && subject.String() == s.SubjectID && authority.String() == s.AuthorityID && s.DomainID == SubjectPersonalDomain && filepath.IsAbs(s.OwnerPath) && filepath.Clean(s.OwnerPath) == s.OwnerPath
}
