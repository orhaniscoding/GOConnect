package ipam

// TODO v2: Implement ACL/DNS and advanced policies. For now, provide a trivial IP allocator stub.

type Allocator struct{}

func New() *Allocator { return &Allocator{} }
func (a *Allocator) Allocate() string { return "100.64.0.2/32" }

