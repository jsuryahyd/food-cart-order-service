package testutil

import (
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	// "strings"
)

// ProjectRoot returns the absolute path to the project root (where go.mod is located)
func ProjectRoot() string {
	_, b, _, _ := runtime.Caller(0)
	// b = .../internal/common/testutil/testutil.go
	root := filepath.Join(filepath.Dir(b), "../../..")
	absRoot, err := filepath.Abs(root)
	if err != nil {
		panic(err)
	}
	return absRoot
}

// ConfigPath returns the absolute path to the config.yaml
func ConfigPath() string {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "configs/config.yaml"
	}
	return filepath.Join(ProjectRoot(), configPath)
}

func MigrationsPath() string {
	projectRoot := ProjectRoot()
	// Adjust this if your migrations are in a different subfolder, e.g., "db/migrations"
	migrationsAbsPath := filepath.Join(projectRoot, "db/migrations")

	// Construct the URL using net/url, which handles OS specifics correctly.
	// The Path field will automatically add the leading slash for Unix-like paths
	// and handle drive letters correctly for Windows (e.g., /C:/path).
	u := url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(migrationsAbsPath), // Ensure forward slashes for the URL path
	}

	// For Windows drive letters, url.URL's Path field will format it correctly as /C:/...
	// so u.String() should produce "file:///C:/..."
	// For Unix, it will produce "file:///home/user/..."
	return u.String()
}