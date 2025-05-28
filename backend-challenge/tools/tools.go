// This package imports things required by build scripts, to ensure that they are vendored
// - tools.go is a common convention to track build dependencies.
// - The build tag `tools` prevents the file from being built into your application.
// - The `_` import statement indicates that these are imports for side effects (like adding to go.mod).

package tools

import (
	_ "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen"
)
