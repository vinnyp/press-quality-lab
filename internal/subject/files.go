package subject

import "strings"

// FileEntry describes one seed entry observed by the grade input reader.
type FileEntry struct {
	Path      string
	Directory bool
	Digest    string
}

// FileEvidence holds observations only. Content sizes are unbounded; the reader
// supplies digests and the count of token starts without retaining file bytes.
type FileEvidence struct {
	BinaryDigest      *string
	NamedDigests      []string
	Seed              []FileEntry
	SecretOccurrences int
}

// CheckFiles applies content rules to the input reader's completed observations.
// Unsafe or unreadable paths are refused by that reader before this point.
func CheckFiles(s File, in FileEvidence) *Error {
	seedPlaceholder := placeholder(s["seed"])
	seedFiles := map[string]string{}
	for _, entry := range in.Seed {
		if !entry.Directory {
			seedFiles[entry.Path] = entry.Digest
		}
	}
	if store, ok := s["store"].(string); ok && !seedPlaceholder {
		if _, exists := seedFiles[store]; !exists {
			return &Error{Rule: "QS9"}
		}
	}
	if in.BinaryDigest != nil {
		for _, digest := range in.NamedDigests {
			if digest == *in.BinaryDigest {
				return &Error{Rule: "QS12"}
			}
		}
		for _, entry := range in.Seed {
			if !entry.Directory && entry.Digest == *in.BinaryDigest {
				return &Error{Rule: "QS12"}
			}
		}
	}
	for _, v := range arr(s["bookkeeping"]) {
		name := text(v)
		if store, ok := s["store"].(string); ok && insideHome(name, store) {
			return &Error{Rule: "QP7"}
		}
		if !seedPlaceholder {
			for _, entry := range in.Seed {
				if insideHome(name, entry.Path) {
					return &Error{Rule: "QP7"}
				}
			}
		}
	}
	secret := obj(s["secret"])
	if secret == nil || placeholder(secret) {
		return nil
	}
	secretPath := text(secret["path"])
	if !placeholder(s["store"]) && s["store"] == secretPath {
		return &Error{Rule: "QP10"}
	}
	if !seedPlaceholder {
		if _, exists := seedFiles[secretPath]; !exists {
			return &Error{Rule: "QP10"}
		}
		if in.SecretOccurrences != 1 {
			return &Error{Rule: "QP10"}
		}
	}
	return nil
}
func insideHome(parent, child string) bool {
	return child == parent || strings.HasPrefix(child, parent+"/")
}
