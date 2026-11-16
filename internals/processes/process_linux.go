//go:build linux

package processes

type linuxDiscoverer struct{}

func NewDiscoverer() ProcessDiscoverer {
	return &linuxDiscoverer{}
}

func (ld *linuxDiscoverer) List() ([]Process, error) {
	return nil, nil
}

func (ld *linuxDiscoverer) GetByPID(pid uint32) (*Process, error) {
	return nil, nil
}

func (ld *linuxDiscoverer) Usage(pid uint32) (*ResourceUsage, error) {
	return nil, nil
}
