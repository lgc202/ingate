package options

import "fmt"

func errRequiredOption(name string) error {
	return fmt.Errorf("%s must not be empty", name)
}
