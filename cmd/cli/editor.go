package main

import (
	"os"
	"os/exec"
	"strings"
)

// DefaultEditor is nano because I prefer it.
const DefaultEditor = "nano"

// PreferredEditorResolver is a function that returns an editor that the user prefers to use, such as the configured
// `$EDITOR` environment variable.
type PreferredEditorResolver func() string

// GetPreferredEditorFromEnvironment returns the user's editor as defined by the `$EDITOR` environment variable, or
// the `DefaultEditor` if it is not set.
func GetPreferredEditorFromEnvironment() string {
	editor := os.Getenv("EDITOR")

	if editor == "" {
		return DefaultEditor
	}

	return editor
}

func resolveEditorArguments(executable string, filename string) []string {
	args := []string{filename}

	if strings.Contains(executable, "Visual Studio Code.app") {
		args = append([]string{"--wait"}, args...)
	}

	// Other common editors
	//
	// ...
	//

	return args
}

// OpenFileInEditor opens filename in a text editor.
func OpenFileInEditor(filename string, resolveEditor PreferredEditorResolver) error {
	// Get the full executable path for the editor.
	executable, err := exec.LookPath(resolveEditor())
	if err != nil {
		return err
	}

	cmd := exec.Command(executable, resolveEditorArguments(executable, filename)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// CaptureInputFromEditor opens a temporary file in a text editor and returns the written bytes on success or an error
// on failure. It handles deletion of the temporary file behind the scenes.
func CaptureInputFromEditor(resolveEditor PreferredEditorResolver) ([]byte, error) {
	file, err := os.CreateTemp(os.TempDir(), "*")
	if err != nil {
		return []byte{}, err
	}

	filename := file.Name()

	// Defer removal of the temporary file in case any of the next steps fail.
	defer os.Remove(filename)

	if err = file.Close(); err != nil {
		return []byte{}, err
	}

	if err = OpenFileInEditor(filename, resolveEditor); err != nil {
		return []byte{}, err
	}

	bytes, err := os.ReadFile(filename)
	if err != nil {
		return []byte{}, err
	}

	return bytes, nil
}
