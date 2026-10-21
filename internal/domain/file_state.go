package domain

// FileReadState is private bounded filesystem state, never a public projection.
// A nil *FileReadState denotes a historical read without available exact bytes.
type FileReadState struct {
	Exists  bool
	Content []byte
}
