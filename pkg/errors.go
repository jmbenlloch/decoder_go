package decoder

import "fmt"

// ErrOpenFile represents an error when opening a file.
type ErrOpenFile struct {
	Filename string
	Err      error
}

func (e *ErrOpenFile) Error() string {
	return fmt.Sprintf("error opening file %q: %v", e.Filename, e.Err)
}

// ErrCreateGroup represents an error when creating a group.
type ErrCreateGroup struct {
	GroupName string
	Err       error
}

func (e *ErrCreateGroup) Error() string {
	return fmt.Sprintf("error creating group %q: %v", e.GroupName, e.Err)
}

// ErrCreateTable represents an error when creating a table.
type ErrCreateTable struct {
	TableName string
	Err       error
}

func (e *ErrCreateTable) Error() string {
	return fmt.Sprintf("error creating table %q: %v", e.TableName, e.Err)
}
