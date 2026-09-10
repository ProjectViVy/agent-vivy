package module

import "context"

// Host is the common identity surface passed during construction. Focused
// Port packages extend it with only the capabilities their Provider needs.
type Host interface {
	ModuleID() string
}

// Module is the build-linked unit instantiated by generated Assembly code.
type Module interface {
	Descriptor() Descriptor
	Construct(context.Context, Host) (Instance, error)
}

// Instance owns the code-Module lifecycle. Generated Assembly code starts in
// dependency order and stops/closes in reverse order.
type Instance interface {
	Start(context.Context) error
	Ready(context.Context) error
	Stop(context.Context) error
	Close(context.Context) error
}
