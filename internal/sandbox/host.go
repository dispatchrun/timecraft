package sandbox

import "net"

func Host() Namespace { return hostNamespace{} }

type hostNamespace struct{}

func (hostNamespace) InterfaceByIndex(index int) (Interface, error) {
	i, err := net.InterfaceByIndex(index)
	if err != nil {
		return nil, err
	}
	return hostInterface{i}, nil
}

func (hostNamespace) InterfaceByName(name string) (Interface, error) {
	i, err := net.InterfaceByName(name)
	if err != nil {
		return nil, err
	}
	return hostInterface{i}, nil
}

func (hostNamespace) Interfaces() ([]Interface, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	hostInterfaces := make([]Interface, len(interfaces))
	for i := range interfaces {
		hostInterfaces[i] = hostInterface{&interfaces[i]}
	}
	return hostInterfaces, nil
}

type hostInterface struct{ *net.Interface }

func (i hostInterface) Index() int { return i.Interface.Index }

func (i hostInterface) MTU() int { return i.Interface.MTU }

func (i hostInterface) Name() string { return i.Interface.Name }

func (i hostInterface) HardwareAddr() net.HardwareAddr { return i.Interface.HardwareAddr }

func (i hostInterface) Flags() net.Flags { return i.Interface.Flags }
