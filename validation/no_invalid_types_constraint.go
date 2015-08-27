package validation

import (
	"fmt"

	"github.com/fgrosse/goldi"
)

// The NoInvalidTypesConstraint checks all types that none of the registered types is invalid
type NoInvalidTypesConstraint struct{}

func (c *NoInvalidTypesConstraint) Validate(container *goldi.Container) (err error) {
	for typeID, typeFactory := range container.TypeRegistry {
		if goldi.IsValid(typeFactory) == false {
			return fmt.Errorf("type %q is invalid: %s", typeID, typeFactory.(error))
		}
	}

	return nil
}
