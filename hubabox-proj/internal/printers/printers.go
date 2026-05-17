package printers

// Entry is one print queue visible on the hub host (read-only admin view).
type Entry struct {
	Name    string
	Status  string
	Default bool
}

// List returns printers on this machine. errHint is non-empty when listing failed but is safe to show admins.
func List() (entries []Entry, errHint string) {
	return listPlatform()
}
