package service

// Service represents a common interface for all MicroCloud services.
type Service interface {
	Bootstrap() error
	IssueToken(peer string) (string, error)
	Join(token string) error
	ClusterMembers() (map[string]string, error)

	Type() ServiceType
	Name() string
	Address() string
	Port() int
}
