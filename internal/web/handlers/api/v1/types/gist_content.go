package types

// GistFileInput is the per-file body shape used by both POST and PATCH
// /gists. Pointer fields let JSON `null` / missing keys be distinguished from
// real values, which matters most on PATCH:
//
//   - content  - file body. On CREATE, required for the file to be added
//     (entries without content are silently dropped). On PATCH, leave it
//     unset to keep the current content; set it to replace.
//   - filename - only used on PATCH, where a set value renames the targeted
//     file. On CREATE it is ignored: the map key is the filename.
type GistFileInput struct {
	Content  *string `json:"content,omitempty"`
	Filename *string `json:"filename,omitempty"`
}

// GistInput is the unified request body for POST and PATCH /api/gists.
// Every field is optional / nilable so handlers can tell "client didn't send
// this" from "client explicitly set this", which is what the PATCH semantics
// require: files from the previous version of the gist that aren't explicitly
// changed during an edit are unchanged.
//
// Handler-specific interpretation:
//
//   - Description - CREATE: nil treated as empty. PATCH: nil = no change.
//   - Title       - Opengist extension. CREATE: nil = derive from first
//     filename. PATCH: nil = no change.
//   - Visibility  - Opengist extension. CREATE: nil = defaults to public.
//     PATCH: nil = no change.
//   - Files       - CREATE: keys define filenames; entries with nil
//     content are skipped. PATCH: keys must match existing filenames;
//     null entry (or empty content+filename) deletes; unknown key with
//     content adds a new file.
type GistInput struct {
	Description *string                   `json:"description,omitempty"`
	Files       map[string]*GistFileInput `json:"files,omitempty"`
	Title       *string                   `json:"title,omitempty"`
	Visibility  *string                   `json:"visibility,omitempty"`
}
